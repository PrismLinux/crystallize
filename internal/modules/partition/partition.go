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

type PartitionMode string

const (
	PartitionModeAuto   PartitionMode = "auto"
	PartitionModeManual PartitionMode = "manual"
)

// ParsePartitionMode parses partition mode from string (case insensitive)
func ParsePartitionMode(mode string) (PartitionMode, error) {
	switch strings.ToLower(mode) {
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

// NewPartition creates a new partition configuration
func NewPartition(mountpoint, blockdevice, filesystem string) *PartitionType {
	return &PartitionType{
		Mountpoint:  mountpoint,
		BlockDevice: blockdevice,
		Filesystem:  filesystem,
	}
}

const (
	BootSize  = "513MiB"
	BootStart = "1MiB"
)

// FilesystemType represents different filesystem types
type FilesystemType int

const (
	Ext4 FilesystemType = iota
	Fat32
	Btrfs
	Xfs
	NoFormat
)

// FilesystemFromString parses filesystem type from string
func FilesystemFromString(fs string) FilesystemType {
	switch strings.ToLower(fs) {
	case "ext4":
		return Ext4
	case "fat32":
		return Fat32
	case "btrfs":
		return Btrfs
	case "xfs":
		return Xfs
	case "noformat", "don't format":
		return NoFormat
	default:
		utils.LogWarn("Unknown filesystem %s, defaulting to ext4", fs)
		return Ext4
	}
}

// Command returns the mkfs command for the filesystem
func (fs FilesystemType) Command() string {
	switch fs {
	case Ext4:
		return "mkfs.ext4"
	case Fat32:
		return "mkfs.fat"
	case Btrfs:
		return "mkfs.btrfs"
	case Xfs:
		return "mkfs.xfs"
	default:
		return ""
	}
}

// Args returns the mkfs arguments for the filesystem
func (fs FilesystemType) Args() []string {
	switch fs {
	case Ext4:
		return []string{"-F"}
	case Fat32:
		return []string{"-F32"}
	case Btrfs, Xfs:
		return []string{"-f"}
	default:
		return []string{}
	}
}

// NeedsFormatting returns true if filesystem requires formatting
func (fs FilesystemType) NeedsFormatting() bool {
	return fs != NoFormat
}

// DisplayName returns display name for logging
func (fs FilesystemType) DisplayName() string {
	switch fs {
	case Ext4:
		return "ext4"
	case Fat32:
		return "fat32"
	case Btrfs:
		return "btrfs"
	case Xfs:
		return "xfs"
	default:
		return "noformat"
	}
}

// DeviceParser handles device and partition parsing
type DeviceParser struct{}

// Parse parses block device into device path and partition number
func (DeviceParser) Parse(blockdevice string) (string, string) {
	if strings.Contains(blockdevice, "nvme") || strings.Contains(blockdevice, "mmcblk") {
		return parseNvmeMmc(blockdevice)
	}
	return parseRegular(blockdevice)
}

// IsNvmeOrMmc checks if device uses NVMe or MMC naming convention
func (DeviceParser) IsNvmeOrMmc(deviceStr string) bool {
	return strings.Contains(deviceStr, "nvme") || strings.Contains(deviceStr, "mmcblk")
}

// GetPartitionNames generates partition names for a device
func (dp DeviceParser) GetPartitionNames(device string, partitionNums []int) []string {
	isNvmeMmc := dp.IsNvmeOrMmc(device)

	var partitions []string
	for _, num := range partitionNums {
		if isNvmeMmc {
			partitions = append(partitions, fmt.Sprintf("%sp%d", device, num))
		} else {
			partitions = append(partitions, fmt.Sprintf("%s%d", device, num))
		}
	}

	return partitions
}

func parseNvmeMmc(blockdevice string) (string, string) {
	if idx := strings.LastIndex(blockdevice, "p"); idx != -1 {
		device := blockdevice[:idx]
		partition := blockdevice[idx+1:]
		return device, partition
	}

	utils.LogWarn("Could not parse NVMe/MMC device partition: %s", blockdevice)
	return blockdevice, "1"
}

func parseRegular(blockdevice string) (string, string) {
	re := regexp.MustCompile(`^(.+?)(\d+)$`)
	matches := re.FindStringSubmatch(blockdevice)

	if len(matches) == 3 {
		return matches[1], matches[2]
	}

	utils.LogWarn("No partition number found in %s, assuming partition 1", blockdevice)
	return blockdevice, "1"
}

// BootFlags handles boot flag management
type BootFlags struct {
	parser DeviceParser
}

// Set sets appropriate boot flags for the boot partition
func (bf BootFlags) Set(blockdevice string, efi bool) {
	device, partitionNum := bf.parser.Parse(blockdevice)

	utils.LogDebug("Setting boot flags for device: %s, partition: %s", device, partitionNum)

	// Validate partition number
	if partitionNum == "" || !isValidPartitionNum(partitionNum) {
		utils.LogError("Invalid partition number '%s' for device %s", partitionNum, device)
		utils.Crash(fmt.Sprintf("Cannot set boot flags: invalid partition number '%s' for %s", partitionNum, blockdevice), 1)
		return
	}

	flag := "boot"
	description := "set boot flag on boot partition"
	bootType := "BIOS"

	if efi {
		flag = "esp"
		description = "set ESP flag on boot partition"
		bootType = "UEFI"
	}

	utils.LogInfo("Setting '%s' flag for %s boot on partition %s of device %s", flag, bootType, partitionNum, device)

	// Use the base device (without partition number) for parted
	utils.ExecEval(
		utils.Exec("parted", "-s", device, "set", partitionNum, flag, "on"),
		description,
	)

	utils.LogInfo("Boot flag '%s' successfully set on %s", flag, blockdevice)
}

// SetAutoFlags sets boot flags for auto-partitioned devices
func (bf BootFlags) SetAutoFlags(device string, efi bool) {
	partitions := bf.parser.GetPartitionNames(device, []int{1})
	bootPartition := partitions[0]

	utils.LogInfo("Setting boot flags on auto-created boot partition: %s", bootPartition)
	bf.Set(bootPartition, efi)
}

// TrySet safely attempts to set boot flags, with error handling
func (bf BootFlags) TrySet(blockdevice string, efi bool) bool {
	device, partitionNum := bf.parser.Parse(blockdevice)

	// Validate partition number
	if partitionNum == "" || !isValidPartitionNum(partitionNum) {
		utils.LogWarn("Cannot set boot flags on %s: invalid partition format", blockdevice)
		return false
	}

	// Check if device exists
	if !utils.Exists(device) {
		utils.LogWarn("Cannot set boot flags on %s: device %s does not exist", blockdevice, device)
		return false
	}

	flag := "boot"
	if efi {
		flag = "esp"
	}

	utils.LogInfo("Setting '%s' flag on %s (device: %s, partition: %s)", flag, blockdevice, device, partitionNum)

	if err := utils.Exec("parted", "-s", device, "set", partitionNum, flag, "on"); err == nil {
		utils.LogInfo("Successfully set '%s' flag on %s", flag, blockdevice)
		return true
	}

	utils.LogWarn("Failed to set '%s' flag on %s", flag, blockdevice)
	utils.LogWarn("Please manually set the %s flag on this partition using parted", flag)
	return false
}

func isValidPartitionNum(partitionNum string) bool {
	_, err := strconv.Atoi(partitionNum)
	return err == nil
}

// PartitionTable handles partition table creation and management
type PartitionTable struct{}

// Create creates appropriate partition table and partitions for the device
func (PartitionTable) Create(device string, efi bool) error {
	// Ensure device is not mounted
	_ = utils.Exec("umount", device)

	if efi {
		return createGPT(device)
	}
	return createMBR(device)
}

func createGPT(deviceStr string) error {
	utils.LogInfo("Creating GPT partition table for UEFI boot")

	// Create GPT partition table
	utils.ExecEval(
		utils.Exec("parted", "-s", deviceStr, "mklabel", "gpt"),
		"create GPT label",
	)

	// Create EFI System Partition (ESP)
	utils.ExecEval(
		utils.Exec("parted", "-s", deviceStr, "mkpart", "ESP", "fat32", BootStart, BootSize),
		"create EFI system partition",
	)

	// Create root partition
	utils.ExecEval(
		utils.Exec("parted", "-s", deviceStr, "mkpart", "root", "ext4", BootSize, "100%"),
		"create root partition",
	)

	return nil
}

func createMBR(deviceStr string) error {
	utils.LogInfo("Creating MBR partition table for BIOS boot")

	// Create MBR partition table
	utils.ExecEval(
		utils.Exec("parted", "-s", deviceStr, "mklabel", "msdos"),
		"create MBR label",
	)

	// Create boot partition
	utils.ExecEval(
		utils.Exec("parted", "-s", deviceStr, "mkpart", "primary", "ext4", BootStart, BootSize),
		"create boot partition",
	)

	// Create root partition
	utils.ExecEval(
		utils.Exec("parted", "-s", deviceStr, "mkpart", "primary", "ext4", BootSize, "100%"),
		"create root partition",
	)

	return nil
}

// MountManager handles mount point management
type MountManager struct{}

// CleanupMounts cleans up any existing mounts before partitioning
func (MountManager) CleanupMounts() {
	utils.LogDebug("Cleaning up existing mount points")
	cleanupMountsList := []string{"/mnt/boot", "/mnt/dev", "/mnt/proc", "/mnt/sys", "/mnt"}

	for _, mountPoint := range cleanupMountsList {
		_ = utils.Exec("umount", "-R", mountPoint)
	}
}

// IsMounted checks if a path is currently mounted
func (MountManager) IsMounted(mountpoint string) bool {
	err := utils.Exec("mountpoint", "-q", mountpoint)
	return err == nil
}

// EnsureMountpointExists ensures mount point directory exists
func (MountManager) EnsureMountpointExists(mountpoint string) {
	if !utils.Exists(mountpoint) {
		utils.LogDebug("Mount point %s does not exist. Creating...", mountpoint)
		if err := utils.CreateDirectory(mountpoint); err != nil {
			utils.Crash(fmt.Sprintf("Failed to create mount point %s: %v", mountpoint, err), 1)
		}
	}
}

// UnmountIfMounted unmounts if already mounted
func (mm MountManager) UnmountIfMounted(mountpoint string) {
	if mm.IsMounted(mountpoint) {
		utils.LogWarn("Mountpoint %s is already mounted, unmounting first", mountpoint)
		_ = utils.Exec("umount", mountpoint)
	}
}

// FilesystemFormatter handles filesystem formatting operations
type FilesystemFormatter struct{}

// Format formats a partition with the specified filesystem
func (FilesystemFormatter) Format(blockdevice string, fsType FilesystemType) {
	if !fsType.NeedsFormatting() {
		utils.LogDebug("Skipping formatting for %s (noformat specified)", blockdevice)
		return
	}

	utils.LogInfo("Formatting %s as %s", blockdevice, fsType.DisplayName())

	args := append(fsType.Args(), blockdevice)

	utils.ExecEval(
		utils.Exec(fsType.Command(), args...),
		fmt.Sprintf("format %s as %s", blockdevice, fsType.DisplayName()),
	)

	utils.LogInfo("Successfully formatted %s as %s", blockdevice, fsType.DisplayName())
}

// FormatAutoPartition formats partition based on EFI requirements
func (ff FilesystemFormatter) FormatAutoPartition(partition string, isBoot bool, efi bool) {
	// Ensure partition is unmounted before formatting
	_ = utils.Exec("umount", partition)

	var fsType FilesystemType
	if isBoot {
		if efi {
			utils.LogInfo("Formatting UEFI boot partition %s as FAT32", partition)
			fsType = Fat32
		} else {
			utils.LogInfo("Formatting BIOS boot partition %s as ext4", partition)
			fsType = Ext4
		}
	} else {
		utils.LogInfo("Formatting root partition %s as ext4", partition)
		fsType = Ext4
	}

	ff.Format(partition, fsType)
}

// Partition is the main entry point for partitioning operations
func Partition(device string, mode PartitionMode, efi bool, partitions []*PartitionType) error {
	utils.LogInfo("Starting partitioning process - Mode: %v, EFI: %t, Device: %s", mode, efi, device)

	mountManager := MountManager{}
	mountManager.CleanupMounts()

	switch mode {
	case PartitionModeAuto:
		return partitionAuto(device, efi)
	case PartitionModeManual:
		return partitionManual(partitions, efi)
	default:
		return fmt.Errorf("unsupported partition mode: %v", mode)
	}
}

// partitionAuto handles automatic partitioning
func partitionAuto(device string, efi bool) error {
	if !utils.Exists(device) {
		return fmt.Errorf("the device %s doesn't exist", device)
	}

	utils.LogInfo("Automatically partitioning %s", device)

	// Create partition table and partitions
	partitionTable := PartitionTable{}
	if err := partitionTable.Create(device, efi); err != nil {
		return fmt.Errorf("failed to create partition table: %w", err)
	}

	// Wait for partitions to be recognized by the kernel
	time.Sleep(2 * time.Second)

	// Set boot flags immediately after partition creation
	bootFlags := BootFlags{parser: DeviceParser{}}
	bootFlags.SetAutoFlags(device, efi)

	// Format and mount partitions
	return formatAndMountAuto(device, efi)
}

// partitionManual handles manual partitioning
func partitionManual(partitions []*PartitionType, efi bool) error {
	utils.LogInfo("Manual partitioning with %d partitions", len(partitions))

	// Sort partitions by mountpoint length to ensure proper mounting order
	sort.Slice(partitions, func(i, j int) bool {
		return len(partitions[i].Mountpoint) < len(partitions[j].Mountpoint)
	})

	for _, partition := range partitions {
		utils.LogDebug("Processing partition: %s -> %s (%s)",
			partition.BlockDevice, partition.Mountpoint, partition.Filesystem)

		if err := FormatMount(partition.Mountpoint, partition.Filesystem, partition.BlockDevice, efi); err != nil {
			return fmt.Errorf("failed to format/mount partition %s: %w", partition.BlockDevice, err)
		}
	}

	utils.LogInfo("Manual partitioning completed successfully")
	return nil
}

// formatAndMountAuto formats and mounts auto-created partitions
func formatAndMountAuto(device string, efi bool) error {
	parser := DeviceParser{}
	partitions := parser.GetPartitionNames(device, []int{1, 2})
	bootPartition, rootPartition := partitions[0], partitions[1]

	utils.LogInfo("Auto partition layout - Boot: %s, Root: %s", bootPartition, rootPartition)

	// Format partitions
	formatter := FilesystemFormatter{}
	formatter.FormatAutoPartition(bootPartition, true, efi)
	formatter.FormatAutoPartition(rootPartition, false, efi)

	// Mount root partition first
	utils.LogInfo("Mounting root partition")
	if err := Mount(rootPartition, "/mnt", ""); err != nil {
		return fmt.Errorf("failed to mount root partition: %w", err)
	}

	// Create boot directory and mount boot partition
	utils.LogInfo("Creating boot directory and mounting boot partition")
	utils.FilesEval(utils.CreateDirectory("/mnt/boot"), "create /mnt/boot")
	if err := Mount(bootPartition, "/mnt/boot", ""); err != nil {
		return fmt.Errorf("failed to mount boot partition: %w", err)
	}

	if efi {
		utils.LogInfo("UEFI setup complete - ESP (FAT32) mounted at /mnt/boot with ESP flag set")
	} else {
		utils.LogInfo("BIOS setup complete - Boot partition (ext4) mounted at /mnt/boot with boot flag set")
	}

	return nil
}

// FormatMount handles formatting and mounting of partitions
func FormatMount(mountpoint, filesystem, blockdevice string, efi bool) error {
	utils.LogInfo("Formatting and mounting %s at %s with filesystem %s", blockdevice, mountpoint, filesystem)

	mountManager := MountManager{}
	formatter := FilesystemFormatter{}

	// Unmount if already mounted
	_ = utils.Exec("umount", blockdevice)

	// Parse filesystem type
	fsType := FilesystemFromString(filesystem)
	if fsType == NoFormat {
		utils.LogInfo("Skipping format for %s (noformat specified)", blockdevice)
	} else {
		formatter.Format(blockdevice, fsType)
	}

	// Ensure mount point exists
	mountManager.EnsureMountpointExists(mountpoint)

	// Mount the partition
	if err := Mount(blockdevice, mountpoint, ""); err != nil {
		return fmt.Errorf("failed to mount %s at %s: %w", blockdevice, mountpoint, err)
	}

	// Set boot flags for boot-related mountpoints
	if mountpoint == "/boot" || mountpoint == "/mnt/boot" {
		utils.LogInfo("Attempting to set boot flags for boot partition %s", blockdevice)
		bootFlags := BootFlags{parser: DeviceParser{}}
		bootFlags.TrySet(blockdevice, efi)
	}

	return nil
}

// Mount mounts a partition at the specified mountpoint
func Mount(partition, mountpoint, options string) error {
	logMessage := fmt.Sprintf("Mounting %s at %s", partition, mountpoint)
	if options != "" {
		logMessage = fmt.Sprintf("Mounting %s at %s with options: %s", partition, mountpoint, options)
	}
	utils.LogDebug("%s", logMessage)

	mountManager := MountManager{}

	// Ensure the mountpoint exists
	mountManager.EnsureMountpointExists(mountpoint)

	// Unmount if already mounted
	mountManager.UnmountIfMounted(mountpoint)

	// Prepare mount command
	var args []string
	if options == "" {
		args = []string{partition, mountpoint}
	} else {
		args = []string{partition, mountpoint, "-o", options}
	}

	description := fmt.Sprintf("mount %s at %s", partition, mountpoint)
	if options != "" {
		description = fmt.Sprintf("mount %s with options %s at %s", partition, options, mountpoint)
	}

	utils.ExecEval(utils.Exec("mount", args...), description)
	utils.LogInfo("Successfully mounted %s at %s", partition, mountpoint)
	return nil
}

// Umount unmounts a mountpoint
func Umount(mountpoint string) {
	utils.LogInfo("Unmounting %s", mountpoint)
	utils.ExecEval(
		utils.Exec("umount", mountpoint),
		fmt.Sprintf("unmount %s", mountpoint),
	)
}
