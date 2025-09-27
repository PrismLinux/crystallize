package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"crystallize-cli/internal/modules/base"
	"crystallize-cli/internal/modules/desktops"
	"crystallize-cli/internal/modules/locale"
	"crystallize-cli/internal/modules/network"
	"crystallize-cli/internal/modules/partition"
	"crystallize-cli/internal/modules/users"
	"crystallize-cli/internal/utils"
)

// Config represents the installation configuration
type Config struct {
	Partition  PartitionConfig  `json:"partition"`
	Bootloader BootloaderConfig `json:"bootloader"`
	Locale     LocaleConfig     `json:"locale"`
	Networking NetworkConfig    `json:"networking"`
	Users      []UserConfig     `json:"users"`
	RootPass   string           `json:"rootpass"`
	Desktop    string           `json:"desktop"`
	Zram       uint64           `json:"zram"`
	// Nvidia        bool             `json:"nvidia"`
	ExtraPackages []string `json:"extra_packages"`
	Kernel        string   `json:"kernel"`
	Flatpak       bool     `json:"flatpak"`
}

// PartitionConfig contains partition configuration
type PartitionConfig struct {
	Device     string   `json:"device"`
	Mode       string   `json:"mode"`
	EFI        bool     `json:"efi"`
	Partitions []string `json:"partitions"`
}

// BootloaderConfig contains bootloader configuration
type BootloaderConfig struct {
	Type     string `json:"type"`
	Location string `json:"location"`
}

// LocaleConfig contains locale configuration
type LocaleConfig struct {
	Locale   []string `json:"locale"`
	Keymap   string   `json:"keymap"`
	Timezone string   `json:"timezone"`
}

// NetworkConfig contains network configuration
type NetworkConfig struct {
	Hostname string `json:"hostname"`
	IPv6     bool   `json:"ipv6"`
}

// UserConfig contains user configuration
type UserConfig struct {
	Name     string `json:"name"`
	Password string `json:"password"`
	HasRoot  bool   `json:"hasroot"`
	Shell    string `json:"shell"`
}

// normalizeDevicePath ensures the device path is properly formatted
func normalizeDevicePath(device string) string {
	// Remove any existing /dev/ prefix to avoid double /dev/dev/
	device = strings.TrimPrefix(device, "/dev/")

	// Add /dev/ prefix
	return "/dev/" + device
}

// ReadConfig reads and executes the installation configuration
func ReadConfig(configPath string) error {
	config, err := LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Defer the cleanup function. It will now run regardless of whether
	// the installation succeeds or fails, ensuring mount points are cleaned up.
	defer config.cleanupInstallation()

	utils.LogInfo("Starting installation process...")

	// Setup partitions
	if err := config.setupPartitions(); err != nil {
		return fmt.Errorf("partition setup failed: %w", err)
	}

	// Install base system
	if err := config.installBaseSystem(); err != nil {
		return fmt.Errorf("base system installation failed: %w", err)
	}

	// Setup bootloader
	if err := config.setupBootloader(); err != nil {
		return fmt.Errorf("bootloader setup failed: %w", err)
	}

	// Configure locale
	config.configureLocale()

	// Setup networking
	config.setupNetworking()

	// Install desktop environment
	if err := config.installDesktop(); err != nil {
		return fmt.Errorf("desktop installation failed: %w", err)
	}

	// Create users
	if err := config.createUsers(); err != nil {
		return fmt.Errorf("user creation failed: %w", err)
	}

	// Finalize installation
	if err := config.finalizeInstallation(); err != nil {
		return fmt.Errorf("installation finalization failed: %w", err)
	}

	utils.LogInfo("Installation completed successfully! You may reboot now.")
	return nil
}

// LoadConfig loads configuration from file
func LoadConfig(configPath string) (*Config, error) {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var config Config
	if err := json.Unmarshal(content, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}

	utils.LogDebug("Successfully loaded config from %s", configPath)
	return &config, nil
}

// parsePartitions parses partition configuration strings
func (c *Config) parsePartitions() ([]*partition.PartitionType, error) {
	var partitions []*partition.PartitionType

	for _, partitionStr := range c.Partition.Partitions {
		parts := strings.Split(partitionStr, ":")
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid partition format: %s", partitionStr)
		}

		part := partition.NewPartition(parts[0], parts[1], parts[2])
		partitions = append(partitions, part)
	}

	return partitions, nil
}

// setupPartitions configures disk partitioning
func (c *Config) setupPartitions() error {
	// Normalize device path FIRST to avoid /dev/dev/ issues
	devicePath := normalizeDevicePath(c.Partition.Device)

	utils.LogInfo("Block device: %s", devicePath)
	utils.LogInfo("Partitioning mode: %s", c.Partition.Mode)
	utils.LogInfo("EFI mode: %t", c.Partition.EFI)

	// Parse partition mode (case insensitive)
	mode, err := partition.ParsePartitionMode(c.Partition.Mode)
	if err != nil {
		utils.LogWarn("%v", err) // Log warning but continue with default
	}

	partitions, err := c.parsePartitions()
	if err != nil {
		return err
	}

	return partition.Partition(devicePath, mode, c.Partition.EFI, partitions)
}

// installBaseSystem installs the base system packages
func (c *Config) installBaseSystem() error {
	utils.LogInfo("Setting up base system...")

	// Ensure essential host system tools are available
	c.ensureHostTools()

	// Install base packages first
	if err := base.InstallBasePackages(c.Kernel); err != nil {
		return fmt.Errorf("failed to install base packages: %w", err)
	}

	// Setup the chroot environment after base packages are installed
	c.prepareChrootEnvironment()

	// Now setup keyring inside the chroot where pacman-key exists
	if err := base.SetupArchlinuxKeyring(); err != nil {
		return fmt.Errorf("failed to setup keyring: %w", err)
	}

	// Copy live system config files to the new installation
	base.CopyLiveConfig()

	// Generate fstab early so it's available for other operations
	base.Genfstab()

	// Install additional components if requested
	if c.Flatpak {
		if err := base.InstallFlatpak(); err != nil {
			return fmt.Errorf("failed to install flatpak: %w", err)
		}
	}

	return nil
}

// ensureHostTools checks for essential host system tools
func (c *Config) ensureHostTools() {
	utils.LogDebug("Ensuring essential host tools are available")

	essentialTools := []string{"cat", "mount", "umount", "chroot", "pacstrap"}

	for _, tool := range essentialTools {
		if err := utils.Exec("which", tool); err == nil {
			utils.LogDebug("Found essential tool: %s", tool)
		} else {
			utils.LogWarn("Essential tool %s not found on host system", tool)
		}
	}
}

// prepareChrootEnvironment prepares the chroot environment
func (c *Config) prepareChrootEnvironment() {
	utils.LogDebug("Preparing chroot environment")

	// Create essential directories first
	essentialDirs := []string{
		"/mnt/proc", "/mnt/sys", "/mnt/dev", "/mnt/dev/pts", "/mnt/tmp", "/mnt/run",
	}

	for _, dir := range essentialDirs {
		if err := utils.Exec("mkdir", "-p", dir); err != nil {
			utils.LogWarn("Failed to create directory: %s", dir)
		}
	}

	// Mount essential filesystems
	utils.ExecEval(utils.Exec("mount", "-t", "proc", "proc", "/mnt/proc"), "mount proc filesystem")
	utils.ExecEval(utils.Exec("mount", "-t", "sysfs", "sysfs", "/mnt/sys"), "mount sys filesystem")
	utils.ExecEval(utils.Exec("mount", "--bind", "/dev", "/mnt/dev"), "bind mount dev filesystem")
	utils.ExecEval(utils.Exec("mount", "-t", "devpts", "devpts", "/mnt/dev/pts"), "mount devpts filesystem")

	// Wait for mounts to stabilize
	time.Sleep(time.Second)
}

// setupBootloader installs and configures the bootloader
func (c *Config) setupBootloader() error {
	utils.LogInfo("Installing bootloader: %s", c.Bootloader.Type)
	utils.LogInfo("Bootloader location: %s", c.Bootloader.Location)

	switch c.Bootloader.Type {
	case "grub-efi":
		return base.InstallBootloaderEFI(c.Bootloader.Location)
	case "grub-legacy":
		return base.InstallBootloaderLegacy(c.Bootloader.Location)
	default:
		return fmt.Errorf("unsupported bootloader type: %s", c.Bootloader.Type)
	}
}

// configureLocale sets up system locale
func (c *Config) configureLocale() {
	utils.LogInfo("Configuring locale: %v", c.Locale.Locale)
	utils.LogInfo("Keyboard layout: %s", c.Locale.Keymap)
	utils.LogInfo("Timezone: %s", c.Locale.Timezone)

	locale.SetLocale(strings.Join(c.Locale.Locale, " "))
	locale.SetKeyboard()
	locale.SetTimezone(c.Locale.Timezone)
}

// setupNetworking configures network settings
func (c *Config) setupNetworking() {
	utils.LogInfo("Hostname: %s", c.Networking.Hostname)
	utils.LogInfo("IPv6 enabled: %t", c.Networking.IPv6)

	network.SetHostname(c.Networking.Hostname)
	network.CreateHosts()

	if c.Networking.IPv6 {
		network.EnableIPv6()
	}
}

// installDesktop installs the selected desktop environment
func (c *Config) installDesktop() error {
	utils.LogInfo("Installing desktop: %s", c.Desktop)

	switch strings.ToLower(c.Desktop) {
	case "plasma", "kde":
		return desktops.InstallDesktopSetup(desktops.DesktopPlasma)
	case "gnome":
		return desktops.InstallDesktopSetup(desktops.DesktopGnome)
	case "cosmic":
		return desktops.InstallDesktopSetup(desktops.DesktopCosmic)
	case "cinnamon":
		return desktops.InstallDesktopSetup(desktops.DesktopCinnamon)
	case "none":
		return desktops.InstallDesktopSetup(desktops.DesktopNone)
	default:
		utils.LogWarn("Unknown desktop: %s, skipping", c.Desktop)
		return nil
	}
}

// createUsers creates system users
func (c *Config) createUsers() error {
	for _, user := range c.Users {
		utils.LogInfo("Creating user: %s", user.Name)
		utils.LogDebug("User has root: %t", user.HasRoot)
		utils.LogDebug("User shell: %s", user.Shell)

		if err := users.NewUser(user.Name, user.HasRoot, user.Password, false, user.Shell); err != nil {
			return fmt.Errorf("failed to create user %s: %w", user.Name, err)
		}
	}

	utils.LogInfo("Setting root password")
	users.RootPass(c.RootPass)
	return nil
}

// finalizeInstallation completes the installation process
func (c *Config) finalizeInstallation() error {
	utils.LogInfo("Finalizing installation...")

	if err := base.InstallNvidia(); err != nil {
		return fmt.Errorf("failed to install nvidia drivers: %w", err)
	}

	// Setup ZRAM
	zramInfo := "auto (min(ram/2, 4096))"
	if c.Zram != 0 {
		zramInfo = fmt.Sprintf("%dMB", c.Zram)
	}

	utils.LogInfo("Configuring ZRAM: %s", zramInfo)
	if err := base.InstallZram(c.Zram); err != nil {
		return fmt.Errorf("failed to configure zram: %w", err)
	}

	// Install extra packages
	if len(c.ExtraPackages) > 0 {
		utils.LogInfo("Installing extra packages: %v", c.ExtraPackages)
		if err := utils.Install(c.ExtraPackages); err != nil {
			return fmt.Errorf("failed to install extra packages: %w", err)
		}
	}

	return nil
}

// cleanupInstallation cleans up installation mounts
func (c *Config) cleanupInstallation() {
	utils.LogInfo("Cleaning up installation mounts...")

	// Wait a moment for any pending operations to complete
	time.Sleep(time.Second)

	// Unmount in reverse order, including /mnt itself at the very end.
	mountPoints := []string{"/mnt/dev/pts", "/mnt/dev", "/mnt/proc", "/mnt/sys", "/mnt/boot", "/mnt"}

	for _, mountPoint := range mountPoints {
		// Try normal unmount first
		if err := utils.Exec("umount", mountPoint); err == nil {
			utils.LogDebug("Successfully unmounted: %s", mountPoint)
		} else {
			// Try lazy unmount if normal unmount fails
			utils.LogDebug("Trying lazy unmount for: %s", mountPoint)
			if err := utils.Exec("umount", "-l", mountPoint); err == nil {
				utils.LogDebug("Successfully lazy unmounted: %s", mountPoint)
			} else {
				utils.LogDebug("Failed to unmount: %s (may not be mounted or is busy)", mountPoint)
			}
		}

		// Small delay between unmount attempts
		time.Sleep(200 * time.Millisecond)
	}
}
