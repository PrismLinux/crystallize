package partition

import (
	"crystallize-cli/internal/utils"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Constants
const (
	BootSize       = "513MiB"
	BootStart      = "1MiB"
	KernelWaitTime = 2 * time.Second
)

// Mode represents the “partitioning mode“
type PartitionMode string

const (
	PartitionModeAuto   PartitionMode = "auto"
	PartitionModeManual PartitionMode = "manual"
)

// ParsePartitionMode converts a string to a PartitionMode
func ParsePartitionMode(mode string) (PartitionMode, error) {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	switch normalized {
	case "auto":
		return PartitionModeAuto, nil
	case "manual":
		return PartitionModeManual, nil
	default:
		return PartitionModeAuto, fmt.Errorf("unknown partition mode: %s, defaulting to auto", mode)
	}
}

// PartitionType represents a disk partition configuration
type PartitionType struct {
	Mountpoint  string `json:"mountpoint"`
	BlockDevice string `json:"blockdevice"`
	Filesystem  string `json:"filesystem"`
}

// NewPartition creates a new PartitionType instance
func NewPartition(mountpoint, blockdevice, filesystem string) *PartitionType {
	return &PartitionType{
		Mountpoint:  mountpoint,
		BlockDevice: blockdevice,
		Filesystem:  filesystem,
	}
}

// Validate checks if the partition configuration is valid
func (p *PartitionType) Validate() error {
	if p.BlockDevice == "" {
		return fmt.Errorf("empty blockdevice for mountpoint %s", p.Mountpoint)
	}
	if !utils.Exists(p.BlockDevice) {
		return fmt.Errorf("blockdevice %s does not exist", p.BlockDevice)
	}
	return nil
}

// IsBootMount checks if this partition is a boot mountpoint
func (p *PartitionType) IsBootMount() bool {
	return p.Mountpoint == "/boot" || p.Mountpoint == "/mnt/boot"
}

// FilesystemType represents different filesystem types
type FilesystemType int

const (
	Ext4 FilesystemType = iota
	Fat32
	Btrfs
	Xfs
	NoFormat
)

var (
	filesystemMap = map[string]FilesystemType{
		"ext4":         Ext4,
		"fat32":        Fat32,
		"btrfs":        Btrfs,
		"xfs":          Xfs,
		"noformat":     NoFormat,
		"no-format":    NoFormat,
		"no format":    NoFormat,
		"don't format": NoFormat,
	}

	filesystemCommands = map[FilesystemType]string{
		Ext4:  "mkfs.ext4",
		Fat32: "mkfs.fat",
		Btrfs: "mkfs.btrfs",
		Xfs:   "mkfs.xfs",
	}

	filesystemArgs = map[FilesystemType][]string{
		Ext4:     {"-F"},
		Fat32:    {"-F32"},
		Btrfs:    {"-f"},
		Xfs:      {"-f"},
		NoFormat: {},
	}

	filesystemNames = map[FilesystemType]string{
		Ext4:     "ext4",
		Fat32:    "fat32",
		Btrfs:    "btrfs",
		Xfs:      "xfs",
		NoFormat: "noformat",
	}

	noFormatRegex  = regexp.MustCompile(`(?i)^(don'?t|do\s*not|no|skip|none)[\s\-_]*(format|fmt)?$`)
	partitionRegex = regexp.MustCompile(`^(.+?)(\d+)$`)

	cleanupMounts = []string{"/mnt/boot", "/mnt/dev", "/mnt/proc", "/mnt/sys", "/mnt"}
)

// ParseFilesystem converts a string to FilesystemType
func ParseFilesystem(fs string) FilesystemType {
	if fs == "" {
		utils.LogWarn("Empty filesystem string, defaulting to ext4")
		return Ext4
	}

	normalized := strings.TrimSpace(strings.ToLower(fs))

	if fsType, found := filesystemMap[normalized]; found {
		return fsType
	}

	if isNoFormatVariation(normalized) {
		utils.LogDebug("Detected no-format variation: %s", fs)
		return NoFormat
	}

	utils.LogWarn("Unknown filesystem %s, defaulting to ext4", fs)
	return Ext4
}

// Command returns the mkfs command for this filesystem
func (fs FilesystemType) Command() string {
	return filesystemCommands[fs]
}

// Args returns the command-line arguments for formatting
func (fs FilesystemType) Args() []string {
	return filesystemArgs[fs]
}

// NeedsFormatting returns true if this filesystem should be formatted
func (fs FilesystemType) NeedsFormatting() bool {
	return fs != NoFormat
}

// String returns the display name of the filesystem
func (fs FilesystemType) String() string {
	return filesystemNames[fs]
}

// DeviceParser handles parsing of device names and partition numbers
type DeviceParser struct{}

// Parse extracts the device name and partition number
func (dp *DeviceParser) Parse(blockdevice string) (device, partition string) {
	if dp.isSpecialDevice(blockdevice) {
		return dp.parseSpecialDevice(blockdevice)
	}
	return dp.parseStandardDevice(blockdevice)
}

// GetPartitionNames generates partition names for given partition numbers
func (dp *DeviceParser) GetPartitionNames(device string, nums []int) []string {
	partitions := make([]string, len(nums))
	separator := dp.getPartitionSeparator(device)

	for i, num := range nums {
		partitions[i] = fmt.Sprintf("%s%s%d", device, separator, num)
	}
	return partitions
}

func (dp *DeviceParser) isSpecialDevice(device string) bool {
	return strings.Contains(device, "nvme") || strings.Contains(device, "mmcblk")
}

func (dp *DeviceParser) getPartitionSeparator(device string) string {
	if dp.isSpecialDevice(device) {
		return "p"
	}
	return ""
}

func (dp *DeviceParser) parseSpecialDevice(blockdevice string) (string, string) {
	if idx := strings.LastIndex(blockdevice, "p"); idx != -1 {
		return blockdevice[:idx], blockdevice[idx+1:]
	}
	utils.LogWarn("Could not parse special device: %s", blockdevice)
	return blockdevice, "1"
}

func (dp *DeviceParser) parseStandardDevice(blockdevice string) (string, string) {
	matches := partitionRegex.FindStringSubmatch(blockdevice)
	if len(matches) == 3 {
		return matches[1], matches[2]
	}
	utils.LogWarn("No partition number found in %s, assuming partition 1", blockdevice)
	return blockdevice, "1"
}

// BootFlagManager handles setting boot flags on partitions
type BootFlagManager struct {
	parser *DeviceParser
}

// NewBootFlagManager creates a new BootFlagManager
func NewBootFlagManager() *BootFlagManager {
	return &BootFlagManager{parser: &DeviceParser{}}
}

// Set sets the appropriate boot flag on a partition
func (bfm *BootFlagManager) Set(blockdevice string, efi bool) error {
	device, partitionNum := bfm.parser.Parse(blockdevice)

	if err := bfm.validate(device, partitionNum); err != nil {
		return err
	}

	flag := bfm.getFlag(efi)
	bootType := bfm.getBootType(efi)

	utils.LogInfo("Setting '%s' flag for %s boot on partition %s", flag, bootType, partitionNum)
	utils.ExecEval(
		utils.Exec("parted", "-s", device, "set", partitionNum, flag, "on"),
		fmt.Sprintf("set %s flag", flag),
	)

	return nil
}

// TrySet attempts to set boot flags, logging errors without failing
func (bfm *BootFlagManager) TrySet(blockdevice string, efi bool) bool {
	if err := bfm.Set(blockdevice, efi); err != nil {
		utils.LogWarn("Failed to set boot flags on %s: %v", blockdevice, err)
		return false
	}
	return true
}

func (bfm *BootFlagManager) validate(device, partitionNum string) error {
	if partitionNum == "" || !isValidPartitionNumber(partitionNum) {
		return fmt.Errorf("invalid partition number '%s' for device %s", partitionNum, device)
	}
	if !utils.Exists(device) {
		return fmt.Errorf("device %s does not exist", device)
	}
	return nil
}

func (bfm *BootFlagManager) getFlag(efi bool) string {
	if efi {
		return "esp"
	}
	return "boot"
}

func (bfm *BootFlagManager) getBootType(efi bool) string {
	if efi {
		return "UEFI"
	}
	return "BIOS"
}

// PartitionTable manages partition table creation
type PartitionTable struct{}

// Create creates a new partition table on the device
func (pt *PartitionTable) Create(device string, efi bool) error {
	utils.Exec("umount", device) // Ignore errors

	if efi {
		return pt.createGPT(device)
	}
	return pt.createMBR(device)
}

func (pt *PartitionTable) createGPT(device string) error {
	utils.LogInfo("Creating GPT partition table for UEFI")

	commands := []partedCommand{
		{[]string{"-s", device, "mklabel", "gpt"}, "create GPT label"},
		{[]string{"-s", device, "mkpart", "ESP", "fat32", BootStart, BootSize}, "create ESP"},
		{[]string{"-s", device, "mkpart", "root", "ext4", BootSize, "100%"}, "create root partition"},
	}

	return pt.executeCommands(commands)
}

func (pt *PartitionTable) createMBR(device string) error {
	utils.LogInfo("Creating MBR partition table for BIOS")

	commands := []partedCommand{
		{[]string{"-s", device, "mklabel", "msdos"}, "create MBR label"},
		{[]string{"-s", device, "mkpart", "primary", "ext4", BootStart, BootSize}, "create boot partition"},
		{[]string{"-s", device, "mkpart", "primary", "ext4", BootSize, "100%"}, "create root partition"},
	}

	return pt.executeCommands(commands)
}

type partedCommand struct {
	args []string
	desc string
}

func (pt *PartitionTable) executeCommands(commands []partedCommand) error {
	for _, cmd := range commands {
		utils.ExecEval(utils.Exec("parted", cmd.args...), cmd.desc)
	}
	return nil
}

// MountManager handles mount operations
type MountManager struct{}

// CleanupAll unmounts all known mount points
func (mm *MountManager) CleanupAll() {
	utils.LogDebug("Cleaning up mount points")
	for _, mountPoint := range cleanupMounts {
		utils.Exec("umount", "-R", mountPoint) // Ignore errors
	}
}

// IsMounted checks if a path is currently mounted
func (mm *MountManager) IsMounted(mountpoint string) bool {
	return utils.Exec("mountpoint", "-q", mountpoint) == nil
}

// EnsureExists creates the mount point directory if it doesn't exist
func (mm *MountManager) EnsureExists(mountpoint string) {
	if !utils.Exists(mountpoint) {
		if err := utils.CreateDirectory(mountpoint); err != nil {
			utils.Crash(fmt.Sprintf("Failed to create mount point %s: %v", mountpoint, err), 1)
		}
	}
}

// UnmountIfMounted unmounts a path if it's currently mounted
func (mm *MountManager) UnmountIfMounted(mountpoint string) {
	if mm.IsMounted(mountpoint) {
		utils.LogWarn("Unmounting already mounted %s", mountpoint)
		utils.Exec("umount", mountpoint) // Ignore errors
	}
}

// FilesystemFormatter handles filesystem formatting operations
type FilesystemFormatter struct{}

// Format formats a block device with the specified filesystem
func (ff *FilesystemFormatter) Format(blockdevice string, fsType FilesystemType) error {
	if !fsType.NeedsFormatting() {
		utils.LogDebug("Skipping format for %s (noformat)", blockdevice)
		return nil
	}

	utils.LogInfo("Formatting %s as %s", blockdevice, fsType.String())
	args := append(fsType.Args(), blockdevice)

	utils.ExecEval(
		utils.Exec(fsType.Command(), args...),
		fmt.Sprintf("format %s as %s", blockdevice, fsType.String()),
	)

	return nil
}

// FormatAutoPartition formats a partition based on its role in auto mode
func (ff *FilesystemFormatter) FormatAutoPartition(partition string, isBoot, efi bool) error {
	utils.Exec("umount", partition) // Ignore errors

	fsType := ff.determineFilesystem(isBoot, efi)
	return ff.Format(partition, fsType)
}

func (ff *FilesystemFormatter) determineFilesystem(isBoot, efi bool) FilesystemType {
	if isBoot && efi {
		return Fat32
	}
	return Ext4
}

// Partitioner orchestrates the partitioning process
type Partitioner struct {
	table     *PartitionTable
	formatter *FilesystemFormatter
	bootFlags *BootFlagManager
	parser    *DeviceParser
	manager   *MountManager
}

// NewPartitioner creates a new Partitioner instance
func NewPartitioner() *Partitioner {
	return &Partitioner{
		table:     &PartitionTable{},
		formatter: &FilesystemFormatter{},
		bootFlags: NewBootFlagManager(),
		parser:    &DeviceParser{},
		manager:   &MountManager{},
	}
}

// Partition executes the partitioning process
func Partition(device string, mode PartitionMode, efi bool, partitions []*PartitionType) error {
	utils.LogInfo("Starting partitioning - Mode: %v, EFI: %t, Device: %s", mode, efi, device)

	p := NewPartitioner()
	p.manager.CleanupAll()

	switch mode {
	case PartitionModeAuto:
		return p.autoPartition(device, efi)
	case PartitionModeManual:
		return p.manualPartition(partitions, efi)
	default:
		return fmt.Errorf("unsupported partition mode: %v", mode)
	}
}

func (p *Partitioner) autoPartition(device string, efi bool) error {
	if !utils.Exists(device) {
		return fmt.Errorf("device %s does not exist", device)
	}

	if err := p.table.Create(device, efi); err != nil {
		return fmt.Errorf("failed to create partition table: %w", err)
	}

	time.Sleep(KernelWaitTime)

	partitions := p.parser.GetPartitionNames(device, []int{1})
	if err := p.bootFlags.Set(partitions[0], efi); err != nil {
		utils.LogWarn("Failed to set boot flags: %v", err)
	}

	return p.formatAndMountAuto(device, efi)
}

func (p *Partitioner) manualPartition(partitions []*PartitionType, efi bool) error {
	utils.LogInfo("Manual partitioning with %d partitions", len(partitions))

	validPartitions := filterValidPartitions(partitions)
	sortByMountpoint(validPartitions)

	for _, partition := range validPartitions {
		utils.LogDebug("Processing: %s -> %s (%s)",
			partition.BlockDevice, partition.Mountpoint, partition.Filesystem)

		if err := p.formatAndMount(partition, efi); err != nil {
			return fmt.Errorf("failed to process partition %s: %w", partition.BlockDevice, err)
		}
	}

	return nil
}

func (p *Partitioner) formatAndMountAuto(device string, efi bool) error {
	partitions := p.parser.GetPartitionNames(device, []int{1, 2})
	bootPartition, rootPartition := partitions[0], partitions[1]

	if err := p.formatter.FormatAutoPartition(bootPartition, true, efi); err != nil {
		return fmt.Errorf("failed to format boot partition: %w", err)
	}

	if err := p.formatter.FormatAutoPartition(rootPartition, false, efi); err != nil {
		return fmt.Errorf("failed to format root partition: %w", err)
	}

	if err := Mount(rootPartition, "/mnt", ""); err != nil {
		return fmt.Errorf("failed to mount root: %w", err)
	}

	utils.FilesEval(utils.CreateDirectory("/mnt/boot"), "create /mnt/boot")

	if err := Mount(bootPartition, "/mnt/boot", ""); err != nil {
		return fmt.Errorf("failed to mount boot: %w", err)
	}

	logSetupComplete(efi)
	return nil
}

func (p *Partitioner) formatAndMount(partition *PartitionType, efi bool) error {
	if err := partition.Validate(); err != nil {
		return err
	}

	utils.Exec("umount", partition.BlockDevice) // Ignore errors

	fsType := ParseFilesystem(partition.Filesystem)
	if fsType == NoFormat {
		utils.LogDebug("Skipping format and mount for %s (noformat)", partition.BlockDevice)
		return nil
	}

	if err := p.formatter.Format(partition.BlockDevice, fsType); err != nil {
		return err
	}

	p.manager.EnsureExists(partition.Mountpoint)

	if err := Mount(partition.BlockDevice, partition.Mountpoint, ""); err != nil {
		return err
	}

	if partition.IsBootMount() {
		p.bootFlags.TrySet(partition.BlockDevice, efi)
	}

	return nil
}

// Mount mounts a partition at the specified mountpoint
func Mount(partition, mountpoint, options string) error {
	if partition == "" {
		return fmt.Errorf("partition cannot be empty")
	}
	if mountpoint == "" {
		return fmt.Errorf("mountpoint cannot be empty")
	}

	manager := &MountManager{}
	manager.EnsureExists(mountpoint)
	manager.UnmountIfMounted(mountpoint)

	args := buildMountArgs(partition, mountpoint, options)
	description := buildMountDescription(partition, mountpoint, options)

	utils.ExecEval(utils.Exec("mount", args...), description)
	utils.LogInfo("Successfully mounted %s at %s", partition, mountpoint)
	return nil
}

// Umount unmounts a mountpoint
func Umount(mountpoint string) error {
	if mountpoint == "" {
		return fmt.Errorf("mountpoint cannot be empty")
	}

	utils.ExecEval(utils.Exec("umount", mountpoint), fmt.Sprintf("unmount %s", mountpoint))
	return nil
}

// Helper functions

func isNoFormatVariation(input string) bool {
	return noFormatRegex.MatchString(input) || containsNoFormatKeywords(input)
}

func containsNoFormatKeywords(input string) bool {
	keywordSets := [][]string{
		{"don", "format"},
		{"do", "not", "format"},
		{"skip", "format"},
		{"no", "format"},
	}

	for _, keywords := range keywordSets {
		if containsAllKeywords(input, keywords) {
			return true
		}
	}
	return false
}

func containsAllKeywords(input string, keywords []string) bool {
	for _, keyword := range keywords {
		if !strings.Contains(input, keyword) {
			return false
		}
	}
	return true
}

func filterValidPartitions(partitions []*PartitionType) []*PartitionType {
	var valid []*PartitionType
	for _, partition := range partitions {
		if partition.BlockDevice == "" {
			utils.LogInfo("Skipping partition with empty blockdevice: %s", partition.Mountpoint)
			continue
		}
		valid = append(valid, partition)
	}
	return valid
}

func sortByMountpoint(partitions []*PartitionType) {
	sort.Slice(partitions, func(i, j int) bool {
		return len(partitions[i].Mountpoint) < len(partitions[j].Mountpoint)
	})
}

func buildMountArgs(partition, mountpoint, options string) []string {
	args := []string{partition, mountpoint}
	if options != "" {
		args = append(args, "-o", options)
	}
	return args
}

func buildMountDescription(partition, mountpoint, options string) string {
	if options == "" {
		return fmt.Sprintf("mount %s at %s", partition, mountpoint)
	}
	return fmt.Sprintf("mount %s at %s with options %s", partition, mountpoint, options)
}

func isValidPartitionNumber(partitionNum string) bool {
	_, err := strconv.Atoi(partitionNum)
	return err == nil
}

func logSetupComplete(efi bool) {
	if efi {
		utils.LogInfo("UEFI setup complete - ESP (FAT32) mounted at /mnt/boot")
	} else {
		utils.LogInfo("BIOS setup complete - Boot partition (ext4) mounted at /mnt/boot")
	}
}
