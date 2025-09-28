package partition

import (
	"crystallize-cli/internal/utils"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type PartitionMode string

const (
	PartitionModeAuto   PartitionMode = "auto"
	PartitionModeManual PartitionMode = "manual"

	BootSize       = "513MiB"
	BootStart      = "1MiB"
	KernelWaitTime = 2 * time.Second
)

type PartitionType struct {
	Mountpoint  string `json:"mountpoint"`
	BlockDevice string `json:"blockdevice"`
	Filesystem  string `json:"filesystem"`
}

type FilesystemType int

const (
	Ext4 FilesystemType = iota
	Fat32
	Btrfs
	Xfs
	NoFormat
)

var (
	dontFormatRegex   = regexp.MustCompile(`(?i)^(don'?t|do\s*not|no|skip|none)[\s\-_]*(format|fmt)?$`)
	partitionRegex    = regexp.MustCompile(`^(.+?)(\d+)$`)
	cleanupMountsList = []string{"/mnt/boot", "/mnt/dev", "/mnt/proc", "/mnt/sys", "/mnt"}
)

// Public functions

func ParsePartitionMode(mode string) (PartitionMode, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "auto":
		return PartitionModeAuto, nil
	case "manual":
		return PartitionModeManual, nil
	default:
		return PartitionModeAuto, fmt.Errorf("unknown partition mode: %s, defaulting to auto", mode)
	}
}

func NewPartition(mountpoint, blockdevice, filesystem string) *PartitionType {
	return &PartitionType{
		Mountpoint:  mountpoint,
		BlockDevice: blockdevice,
		Filesystem:  filesystem,
	}
}

func Partition(device string, mode PartitionMode, efi bool, partitions []*PartitionType) error {
	utils.LogInfo("Starting partitioning - Mode: %v, EFI: %t, Device: %s", mode, efi, device)

	cleanup := &MountManager{}
	cleanup.CleanupAll()

	switch mode {
	case PartitionModeAuto:
		return handleAutoPartition(device, efi)
	case PartitionModeManual:
		return handleManualPartition(partitions, efi)
	default:
		return fmt.Errorf("unsupported partition mode: %v", mode)
	}
}

func Mount(partition, mountpoint, options string) error {
	if err := validateMountInputs(partition, mountpoint); err != nil {
		return err
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

func Umount(mountpoint string) error {
	if mountpoint == "" {
		return fmt.Errorf("mountpoint cannot be empty")
	}

	utils.ExecEval(utils.Exec("umount", mountpoint), fmt.Sprintf("unmount %s", mountpoint))
	return nil
}

// Filesystem handling

func FilesystemFromString(fs string) FilesystemType {
	if fs == "" {
		utils.LogWarn("Empty filesystem string, defaulting to ext4")
		return Ext4
	}

	normalized := strings.TrimSpace(strings.ToLower(fs))

	if fsType, found := getExactFilesystemMatch(normalized); found {
		return fsType
	}

	if isNoFormatVariation(normalized) {
		utils.LogDebug("Detected no-format variation: %s", fs)
		return NoFormat
	}

	utils.LogWarn("Unknown filesystem %s, defaulting to ext4", fs)
	return Ext4
}

func (fs FilesystemType) Command() string {
	commands := map[FilesystemType]string{
		Ext4:  "mkfs.ext4",
		Fat32: "mkfs.fat",
		Btrfs: "mkfs.btrfs",
		Xfs:   "mkfs.xfs",
	}
	return commands[fs]
}

func (fs FilesystemType) Args() []string {
	args := map[FilesystemType][]string{
		Ext4:     {"-F"},
		Fat32:    {"-F32"},
		Btrfs:    {"-f"},
		Xfs:      {"-f"},
		NoFormat: {},
	}
	return args[fs]
}

func (fs FilesystemType) NeedsFormatting() bool {
	return fs != NoFormat
}

func (fs FilesystemType) DisplayName() string {
	names := map[FilesystemType]string{
		Ext4:     "ext4",
		Fat32:    "fat32",
		Btrfs:    "btrfs",
		Xfs:      "xfs",
		NoFormat: "noformat",
	}
	return names[fs]
}

// Device parsing

type DeviceParser struct{}

func (dp *DeviceParser) Parse(blockdevice string) (device, partition string) {
	if dp.isNvmeOrMmc(blockdevice) {
		return dp.parseNvmeMmc(blockdevice)
	}
	return dp.parseRegular(blockdevice)
}

func (dp *DeviceParser) GetPartitionNames(device string, nums []int) []string {
	isSpecial := dp.isNvmeOrMmc(device)
	partitions := make([]string, len(nums))

	for i, num := range nums {
		if isSpecial {
			partitions[i] = fmt.Sprintf("%sp%d", device, num)
		} else {
			partitions[i] = fmt.Sprintf("%s%d", device, num)
		}
	}
	return partitions
}

func (dp *DeviceParser) isNvmeOrMmc(device string) bool {
	return strings.Contains(device, "nvme") || strings.Contains(device, "mmcblk")
}

func (dp *DeviceParser) parseNvmeMmc(blockdevice string) (string, string) {
	if idx := strings.LastIndex(blockdevice, "p"); idx != -1 {
		return blockdevice[:idx], blockdevice[idx+1:]
	}
	utils.LogWarn("Could not parse NVMe/MMC device: %s", blockdevice)
	return blockdevice, "1"
}

func (dp *DeviceParser) parseRegular(blockdevice string) (string, string) {
	matches := partitionRegex.FindStringSubmatch(blockdevice)
	if len(matches) == 3 {
		return matches[1], matches[2]
	}
	utils.LogWarn("No partition number found in %s, assuming partition 1", blockdevice)
	return blockdevice, "1"
}

// Boot flag management

type BootFlags struct {
	parser *DeviceParser
}

func NewBootFlags() *BootFlags {
	return &BootFlags{parser: &DeviceParser{}}
}

func (bf *BootFlags) Set(blockdevice string, efi bool) error {
	device, partitionNum := bf.parser.Parse(blockdevice)

	if err := bf.validatePartition(device, partitionNum, blockdevice); err != nil {
		return err
	}

	flag, bootType := bf.getBootFlagInfo(efi)

	utils.LogInfo("Setting '%s' flag for %s boot on partition %s", flag, bootType, partitionNum)
	utils.ExecEval(
		utils.Exec("parted", "-s", device, "set", partitionNum, flag, "on"),
		fmt.Sprintf("set %s flag", flag),
	)

	return nil
}

func (bf *BootFlags) TrySet(blockdevice string, efi bool) bool {
	if err := bf.Set(blockdevice, efi); err != nil {
		utils.LogWarn("Failed to set boot flags on %s: %v", blockdevice, err)
		return false
	}
	return true
}

func (bf *BootFlags) validatePartition(device, partitionNum, blockdevice string) error {
	if partitionNum == "" || !isValidPartitionNum(partitionNum) {
		return fmt.Errorf("invalid partition number '%s' for device %s", partitionNum, device)
	}

	if !utils.Exists(device) {
		return fmt.Errorf("device %s does not exist", device)
	}

	return nil
}

func (bf *BootFlags) getBootFlagInfo(efi bool) (flag, bootType string) {
	if efi {
		return "esp", "UEFI"
	}
	return "boot", "BIOS"
}

// Partition table management

type PartitionTable struct{}

func (pt *PartitionTable) Create(device string, efi bool) error {
	utils.Exec("umount", device) // Ignore errors

	if efi {
		return pt.createGPT(device)
	}
	return pt.createMBR(device)
}

func (pt *PartitionTable) createGPT(device string) error {
	utils.LogInfo("Creating GPT partition table for UEFI")

	commands := []struct {
		args []string
		desc string
	}{
		{[]string{"-s", device, "mklabel", "gpt"}, "create GPT label"},
		{[]string{"-s", device, "mkpart", "ESP", "fat32", BootStart, BootSize}, "create ESP"},
		{[]string{"-s", device, "mkpart", "root", "ext4", BootSize, "100%"}, "create root partition"},
	}

	return pt.executePartedCommands(commands)
}

func (pt *PartitionTable) createMBR(device string) error {
	utils.LogInfo("Creating MBR partition table for BIOS")

	commands := []struct {
		args []string
		desc string
	}{
		{[]string{"-s", device, "mklabel", "msdos"}, "create MBR label"},
		{[]string{"-s", device, "mkpart", "primary", "ext4", BootStart, BootSize}, "create boot partition"},
		{[]string{"-s", device, "mkpart", "primary", "ext4", BootSize, "100%"}, "create root partition"},
	}

	return pt.executePartedCommands(commands)
}

func (pt *PartitionTable) executePartedCommands(commands []struct {
	args []string
	desc string
}) error {
	for _, cmd := range commands {
		utils.ExecEval(utils.Exec("parted", cmd.args...), cmd.desc)
	}
	return nil
}

// Mount management

type MountManager struct{}

func (mm *MountManager) CleanupAll() {
	utils.LogDebug("Cleaning up mount points")
	for _, mountPoint := range cleanupMountsList {
		utils.Exec("umount", "-R", mountPoint) // Ignore errors
	}
}

func (mm *MountManager) IsMounted(mountpoint string) bool {
	return utils.Exec("mountpoint", "-q", mountpoint) == nil
}

func (mm *MountManager) EnsureExists(mountpoint string) {
	if !utils.Exists(mountpoint) {
		if err := utils.CreateDirectory(mountpoint); err != nil {
			utils.Crash(fmt.Sprintf("Failed to create mount point %s: %v", mountpoint, err), 1)
		}
	}
}

func (mm *MountManager) UnmountIfMounted(mountpoint string) {
	if mm.IsMounted(mountpoint) {
		utils.LogWarn("Unmounting already mounted %s", mountpoint)
		utils.Exec("umount", mountpoint) // Ignore errors
	}
}

// Filesystem formatting

type FilesystemFormatter struct{}

func (ff *FilesystemFormatter) Format(blockdevice string, fsType FilesystemType) error {
	if !fsType.NeedsFormatting() {
		utils.LogDebug("Skipping format for %s (noformat)", blockdevice)
		return nil
	}

	utils.LogInfo("Formatting %s as %s", blockdevice, fsType.DisplayName())
	args := append(fsType.Args(), blockdevice)

	utils.ExecEval(
		utils.Exec(fsType.Command(), args...),
		fmt.Sprintf("format %s as %s", blockdevice, fsType.DisplayName()),
	)

	return nil
}

func (ff *FilesystemFormatter) FormatAutoPartition(partition string, isBoot, efi bool) error {
	utils.Exec("umount", partition) // Ignore errors

	var fsType FilesystemType
	switch {
	case isBoot && efi:
		fsType = Fat32
	case isBoot && !efi:
		fsType = Ext4
	default:
		fsType = Ext4
	}

	return ff.Format(partition, fsType)
}

// Main partition handling

func handleAutoPartition(device string, efi bool) error {
	if !utils.Exists(device) {
		return fmt.Errorf("device %s does not exist", device)
	}

	table := &PartitionTable{}
	if err := table.Create(device, efi); err != nil {
		return fmt.Errorf("failed to create partition table: %w", err)
	}

	time.Sleep(KernelWaitTime)

	bootFlags := NewBootFlags()
	parser := &DeviceParser{}
	partitions := parser.GetPartitionNames(device, []int{1})

	if err := bootFlags.Set(partitions[0], efi); err != nil {
		utils.LogWarn("Failed to set boot flags: %v", err)
	}

	return formatAndMountAutoPartitions(device, efi)
}

func handleManualPartition(partitions []*PartitionType, efi bool) error {
	utils.LogInfo("Manual partitioning with %d partitions", len(partitions))

	validPartitions := filterValidPartitions(partitions)
	sortPartitionsByMountpoint(validPartitions)

	for _, partition := range validPartitions {
		utils.LogDebug("Processing: %s -> %s (%s)",
			partition.BlockDevice, partition.Mountpoint, partition.Filesystem)

		if err := formatAndMount(partition, efi); err != nil {
			return fmt.Errorf("failed to process partition %s: %w", partition.BlockDevice, err)
		}
	}

	return nil
}

func formatAndMountAutoPartitions(device string, efi bool) error {
	parser := &DeviceParser{}
	partitions := parser.GetPartitionNames(device, []int{1, 2})
	bootPartition, rootPartition := partitions[0], partitions[1]

	formatter := &FilesystemFormatter{}

	if err := formatter.FormatAutoPartition(bootPartition, true, efi); err != nil {
		return fmt.Errorf("failed to format boot partition: %w", err)
	}

	if err := formatter.FormatAutoPartition(rootPartition, false, efi); err != nil {
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

func formatAndMount(partition *PartitionType, efi bool) error {
	if err := validatePartition(partition); err != nil {
		return err
	}

	utils.Exec("umount", partition.BlockDevice) // Ignore errors

	fsType := FilesystemFromString(partition.Filesystem)
	if fsType == NoFormat {
		utils.LogDebug("Skipping format and mount for %s (noformat)", partition.BlockDevice)
		return nil
	}

	formatter := &FilesystemFormatter{}
	if err := formatter.Format(partition.BlockDevice, fsType); err != nil {
		return err
	}

	manager := &MountManager{}
	manager.EnsureExists(partition.Mountpoint)

	if err := Mount(partition.BlockDevice, partition.Mountpoint, ""); err != nil {
		return err
	}

	if isBootMountpoint(partition.Mountpoint) {
		bootFlags := NewBootFlags()
		bootFlags.TrySet(partition.BlockDevice, efi)
	}

	return nil
}

// Helper functions

func getExactFilesystemMatch(fs string) (FilesystemType, bool) {
	matches := map[string]FilesystemType{
		"ext4":      Ext4,
		"fat32":     Fat32,
		"btrfs":     Btrfs,
		"xfs":       Xfs,
		"noformat":  NoFormat,
		"no-format": NoFormat,
		"no format": NoFormat,
	}

	fsType, found := matches[fs]
	return fsType, found
}

func isNoFormatVariation(input string) bool {
	return isDontFormatVariation(input) ||
		dontFormatRegex.MatchString(input) ||
		containsNoFormatKeywords(input)
}

func isDontFormatVariation(input string) bool {
	cleaned := cleanString(input)
	variations := []string{
		"dontformat", "donotformat", "do not format", "dont fmt",
		"do not fmt", "no formatting", "noformatting", "skip formatting", "skipformatting",
	}

	for _, variation := range variations {
		if cleaned == cleanString(variation) {
			return true
		}
	}
	return false
}

func containsNoFormatKeywords(input string) bool {
	keywordSets := [][]string{
		{"don", "format"}, {"do", "not", "format"},
		{"skip", "format"}, {"no", "format"},
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

func cleanString(s string) string {
	var result strings.Builder

	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			result.WriteRune(unicode.ToLower(r))
		} else if unicode.IsSpace(r) && result.Len() > 0 {
			lastChar := result.String()[result.Len()-1]
			if lastChar != ' ' {
				result.WriteRune(' ')
			}
		}
	}

	return strings.TrimSpace(result.String())
}

func validatePartition(partition *PartitionType) error {
	if partition.BlockDevice == "" {
		return fmt.Errorf("empty blockdevice for mountpoint %s", partition.Mountpoint)
	}

	if !utils.Exists(partition.BlockDevice) {
		return fmt.Errorf("blockdevice %s does not exist", partition.BlockDevice)
	}

	return nil
}

func validateMountInputs(partition, mountpoint string) error {
	if partition == "" {
		return fmt.Errorf("partition cannot be empty")
	}
	if mountpoint == "" {
		return fmt.Errorf("mountpoint cannot be empty")
	}
	return nil
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

func sortPartitionsByMountpoint(partitions []*PartitionType) {
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

func isBootMountpoint(mountpoint string) bool {
	return mountpoint == "/boot" || mountpoint == "/mnt/boot"
}

func isValidPartitionNum(partitionNum string) bool {
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
