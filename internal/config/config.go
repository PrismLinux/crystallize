package config

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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

const (
	installTimeout = 2 * time.Hour
	mountStabilize = 500 * time.Millisecond
	unmountDelay   = 200 * time.Millisecond
)

// BootloaderType represents supported bootloader types
type BootloaderType string

const (
	BootloaderGrubEFI    BootloaderType = "grub-efi"
	BootloaderGrubLegacy BootloaderType = "grub-legacy"
)

// Config represents the installation configuration
type Config struct {
	Partition     PartitionConfig  `json:"partition"`
	Bootloader    BootloaderConfig `json:"bootloader"`
	Locale        LocaleConfig     `json:"locale"`
	Networking    NetworkConfig    `json:"networking"`
	Users         []UserConfig     `json:"users"`
	RootPass      string           `json:"rootpass"`
	Desktop       string           `json:"desktop"`
	Zram          uint64           `json:"zram"`
	ExtraPackages []string         `json:"extra_packages"`
	Kernel        string           `json:"kernel"`
	Flatpak       bool             `json:"flatpak"`
}

type PartitionConfig struct {
	Device     string   `json:"device"`
	Mode       string   `json:"mode"`
	EFI        bool     `json:"efi"`
	Partitions []string `json:"partitions"`
}

type BootloaderConfig struct {
	Type     string `json:"type"`
	Location string `json:"location"`
}

type LocaleConfig struct {
	Locale   []string `json:"locale"`
	Keymap   string   `json:"keymap"`
	Timezone string   `json:"timezone"`
}

type NetworkConfig struct {
	Hostname string `json:"hostname"`
	IPv6     bool   `json:"ipv6"`
}

type UserConfig struct {
	Name     string `json:"name"`
	Password string `json:"password"`
	HasRoot  bool   `json:"hasroot"`
	Shell    string `json:"shell"`
}

// Installer orchestrates the installation process
type Installer struct {
	config      *Config
	mountPoints []string
}

// Stage represents an installation stage
type Stage struct {
	Name     string
	Required bool
	Execute  func(context.Context) error
}

// mountSpec represents a filesystem mount
type mountSpec struct {
	source string
	target string
	fstype string
	bind   bool
}

func (m mountSpec) mount() error {
	if m.bind {
		return utils.Exec("mount", "--bind", m.source, m.target)
	}
	return utils.Exec("mount", "-t", m.fstype, m.source, m.target)
}

// ReadConfig is the main entry point for installation
func ReadConfig(configPath string) error {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return err
	}

	installer := NewInstaller(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), installTimeout)
	defer cancel()

	return installer.Install(ctx)
}

// LoadConfig loads and validates configuration from file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, utils.NewErrorf("read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, utils.NewErrorf("parse config JSON: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, utils.NewErrorf("config validation failed: %w", err)
	}

	utils.LogDebug("Configuration loaded from %s", path)
	return &cfg, nil
}

// Validate performs comprehensive configuration validation
func (c *Config) Validate() error {
	// Required field validators
	validators := []struct {
		check   bool
		message string
	}{
		{c.Partition.Device == "", "partition device is required"},
		{c.Bootloader.Type == "", "bootloader type is required"},
		{!isValidBootloader(c.Bootloader.Type), utils.Sprintf("invalid bootloader type '%s' (valid: grub-efi, grub-legacy)", c.Bootloader.Type)},
		{c.Bootloader.Location == "", "bootloader location is required"},
		{len(c.Locale.Locale) == 0, "at least one locale is required"},
		{c.Locale.Keymap == "", "keymap is required"},
		{c.Locale.Timezone == "", "timezone is required"},
		{c.Networking.Hostname == "", "hostname is required"},
		{len(c.Users) == 0, "at least one user is required"},
		{c.RootPass == "", "root password is required"},
		{c.Kernel == "", "kernel package is required"},
	}

	for _, v := range validators {
		if v.check {
			return utils.NewError(v.message)
		}
	}

	// Validate users
	for i, user := range c.Users {
		if user.Name == "" {
			return utils.NewErrorf("user %d: name is required", i)
		}
		if user.Password == "" {
			return utils.NewErrorf("user %d: password is required", i)
		}
	}

	// Validate partitions format
	for i, part := range c.Partition.Partitions {
		if strings.Count(part, ":") != 2 {
			return utils.NewErrorf("partition %d: invalid format '%s' (expected: type:size:mountpoint)", i, part)
		}
	}

	return nil
}

// NewInstaller creates a new installer instance
func NewInstaller(cfg *Config) *Installer {
	return &Installer{
		config:      cfg,
		mountPoints: make([]string, 0, 8),
	}
}

// Install executes the installation process
func (i *Installer) Install(ctx context.Context) error {
	defer i.cleanup()

	i.logBanner()
	stages := i.buildStages()

	for idx, stage := range stages {
		select {
		case <-ctx.Done():
			return utils.NewErrorf("installation cancelled: %w", ctx.Err())
		default:
		}

		utils.LogInfo("[%d/%d] %s", idx+1, len(stages), stage.Name)

		if err := stage.Execute(ctx); err != nil {
			if stage.Required {
				utils.LogError("Stage failed: %v", err)
				return utils.NewErrorf("%s: %w", stage.Name, err)
			}
			utils.LogWarn("%s failed (non-critical): %v", stage.Name, err)
		}
	}

	i.logCompletion()
	return nil
}

// logBanner prints installation header
func (i *Installer) logBanner() {
	utils.LogInfo("═══════════════════════════════════════════")
	utils.LogInfo("  PrismLinux Installation")
	utils.LogInfo("═══════════════════════════════════════════")
	utils.LogInfo("Device: %s", i.config.Partition.Device)
	utils.LogInfo("Desktop: %s", i.config.Desktop)
	utils.LogInfo("Kernel: %s", i.config.Kernel)
	utils.LogInfo("═══════════════════════════════════════════")
}

// logCompletion prints installation footer
func (i *Installer) logCompletion() {
	utils.LogInfo("═══════════════════════════════════════════")
	utils.LogInfo("✓ Installation completed successfully!")
	utils.LogInfo("  You can now reboot into your new system")
	utils.LogInfo("═══════════════════════════════════════════")
}

// buildStages constructs the installation pipeline
func (i *Installer) buildStages() []Stage {
	return []Stage{
		{Name: "Partition Setup", Required: true, Execute: i.setupPartitions},
		{Name: "Base System", Required: true, Execute: i.installBaseSystem},
		{Name: "Chroot Environment", Required: true, Execute: i.setupChroot},
		{Name: "System Keyring", Required: true, Execute: i.setupKeyring},
		{Name: "File System Table", Required: true, Execute: i.generateFstab},
		{Name: "Bootloader", Required: true, Execute: i.setupBootloader},
		{Name: "Locale Configuration", Required: true, Execute: i.configureLocale},
		{Name: "Network Configuration", Required: true, Execute: i.setupNetworking},
		{Name: "User Accounts", Required: true, Execute: i.createUsers},
		{Name: "Desktop Environment", Required: false, Execute: i.installDesktop},
		{Name: "Additional Packages", Required: false, Execute: i.installExtras},
		{Name: "System Finalization", Required: true, Execute: i.finalize},
	}
}

// setupPartitions configures disk partitioning
func (i *Installer) setupPartitions(ctx context.Context) error {
	devicePath := normalizeDevicePath(i.config.Partition.Device)

	mode, err := partition.ParsePartitionMode(i.config.Partition.Mode)
	if err != nil {
		utils.LogWarn("Using default partition mode: %v", err)
	}

	partitions, err := parsePartitions(i.config.Partition.Partitions)
	if err != nil {
		return err
	}

	utils.LogDebug("Device: %s, Mode: %s, EFI: %t", devicePath, i.config.Partition.Mode, i.config.Partition.EFI)

	if err := partition.Partition(devicePath, mode, i.config.Partition.EFI, partitions); err != nil {
		return utils.NewErrorf("partition disk: %w", err)
	}

	i.mountPoints = append(i.mountPoints, "/mnt")
	return nil
}

// installBaseSystem installs base packages
func (i *Installer) installBaseSystem(ctx context.Context) error {
	if err := base.InstallBasePackages(i.config.Kernel); err != nil {
		return utils.NewErrorf("install base packages: %w", err)
	}
	return nil
}

// setupChroot prepares the chroot environment
func (i *Installer) setupChroot(ctx context.Context) error {
	utils.LogDebug("Preparing chroot environment")

	dirs := []string{"/mnt/proc", "/mnt/sys", "/mnt/dev", "/mnt/dev/pts", "/mnt/run"}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return utils.NewErrorf("create directory %s: %w", dir, err)
		}
	}

	mounts := []mountSpec{
		{source: "proc", target: "/mnt/proc", fstype: "proc"},
		{source: "sysfs", target: "/mnt/sys", fstype: "sysfs"},
		{source: "/dev", target: "/mnt/dev", bind: true},
		{source: "devpts", target: "/mnt/dev/pts", fstype: "devpts"},
	}

	for _, m := range mounts {
		if err := m.mount(); err != nil {
			return utils.NewErrorf("mount %s: %w", m.target, err)
		}
		i.mountPoints = append(i.mountPoints, m.target)
	}

	time.Sleep(mountStabilize)
	return nil
}

// setupKeyring initializes the package keyring
func (i *Installer) setupKeyring(ctx context.Context) error {
	return base.SetupArchlinuxKeyring()
}

// generateFstab generates the filesystem table
func (i *Installer) generateFstab(ctx context.Context) error {
	base.Genfstab()
	return nil
}

// setupBootloader installs the bootloader
func (i *Installer) setupBootloader(ctx context.Context) error {
	utils.LogDebug("Bootloader: %s at %s", i.config.Bootloader.Type, i.config.Bootloader.Location)

	switch BootloaderType(i.config.Bootloader.Type) {
	case BootloaderGrubEFI:
		return base.InstallBootloaderEFI(i.config.Bootloader.Location)
	case BootloaderGrubLegacy:
		return base.InstallBootloaderLegacy(i.config.Bootloader.Location)
	default:
		return utils.NewErrorf("unsupported bootloader: %s", i.config.Bootloader.Type)
	}
}

// configureLocale sets up system locale
func (i *Installer) configureLocale(ctx context.Context) error {
	utils.LogDebug("Locale: %v, Keymap: %s, Timezone: %s",
		i.config.Locale.Locale, i.config.Locale.Keymap, i.config.Locale.Timezone)

	locale.SetLocale(strings.Join(i.config.Locale.Locale, " "))
	locale.SetKeyboard()
	locale.SetTimezone(i.config.Locale.Timezone)
	return nil
}

// setupNetworking configures network settings
func (i *Installer) setupNetworking(ctx context.Context) error {
	utils.LogDebug("Hostname: %s, IPv6: %t", i.config.Networking.Hostname, i.config.Networking.IPv6)

	network.SetHostname(i.config.Networking.Hostname)
	network.CreateHosts()

	if i.config.Networking.IPv6 {
		network.EnableIPv6()
	}
	return nil
}

// createUsers creates system users
func (i *Installer) createUsers(ctx context.Context) error {
	for _, user := range i.config.Users {
		utils.LogDebug("Creating user: %s (root: %t, shell: %s)", user.Name, user.HasRoot, user.Shell)

		if err := users.NewUser(user.Name, user.HasRoot, user.Password, false, user.Shell); err != nil {
			return utils.NewErrorf("create user %s: %w", user.Name, err)
		}
	}

	users.RootPass(i.config.RootPass)
	return nil
}

// installDesktop installs the desktop environment
func (i *Installer) installDesktop(ctx context.Context) error {
	if i.config.Desktop == "" || strings.ToLower(i.config.Desktop) == "none" {
		utils.LogInfo("Skipping desktop environment installation")
		return nil
	}

	desktop, err := parseDesktop(i.config.Desktop)
	if err != nil {
		return err
	}

	return desktops.InstallDesktopSetup(desktop)
}

// installExtras installs additional packages
func (i *Installer) installExtras(ctx context.Context) error {
	if i.config.Flatpak {
		if err := base.InstallFlatpak(); err != nil {
			return utils.NewErrorf("install flatpak: %w", err)
		}
	}

	if len(i.config.ExtraPackages) > 0 {
		utils.LogDebug("Extra packages: %v", i.config.ExtraPackages)
		if err := utils.Install(i.config.ExtraPackages); err != nil {
			return utils.NewErrorf("install extra packages: %w", err)
		}
	}

	return nil
}

// finalize performs final installation steps
func (i *Installer) finalize(ctx context.Context) error {
	if err := base.InstallNvidia(); err != nil {
		utils.LogWarn("NVIDIA driver installation: %v", err)
	}

	base.CopyLiveConfig()

	zramSize := i.config.Zram
	if zramSize == 0 {
		utils.LogDebug("ZRAM: auto-configured")
	} else {
		utils.LogDebug("ZRAM: %d MB", zramSize)
	}

	if err := base.InstallZram(zramSize); err != nil {
		return utils.NewErrorf("configure zram: %w", err)
	}

	return nil
}

// cleanup unmounts filesystems
func (i *Installer) cleanup() {
	if len(i.mountPoints) == 0 {
		return
	}

	utils.LogInfo("Cleaning up mount points...")
	time.Sleep(time.Second)

	for idx := len(i.mountPoints) - 1; idx >= 0; idx-- {
		i.unmountPoint(i.mountPoints[idx])
		time.Sleep(unmountDelay)
	}
}

// unmountPoint attempts to unmount with fallback to lazy unmount
func (i *Installer) unmountPoint(mp string) {
	if err := utils.Exec("umount", mp); err == nil {
		utils.LogDebug("Unmounted: %s", mp)
		return
	}

	if err := utils.Exec("umount", "-l", mp); err == nil {
		utils.LogDebug("Lazy unmounted: %s", mp)
	} else {
		utils.LogDebug("Failed to unmount: %s", mp)
	}
}

// parsePartitions parses partition configuration strings
func parsePartitions(specs []string) ([]*partition.PartitionType, error) {
	partitions := make([]*partition.PartitionType, 0, len(specs))

	for i, spec := range specs {
		parts := strings.SplitN(spec, ":", 3)
		if len(parts) != 3 {
			return nil, utils.NewErrorf("partition %d: invalid format '%s'", i, spec)
		}

		partitions = append(partitions, partition.NewPartition(parts[0], parts[1], parts[2]))
	}

	return partitions, nil
}

// parseDesktop converts string to DesktopSetup
func parseDesktop(name string) (desktops.DesktopSetup, error) {
	desktopMap := map[string]desktops.DesktopSetup{
		"plasma":   desktops.DesktopPlasma,
		"kde":      desktops.DesktopPlasma,
		"gnome":    desktops.DesktopGnome,
		"cosmic":   desktops.DesktopCosmic,
		"cinnamon": desktops.DesktopCinnamon,
		"hyprland": desktops.DesktopHyprland,
		"none":     desktops.DesktopNone,
	}

	desktop, ok := desktopMap[strings.ToLower(name)]
	if !ok {
		return "", utils.NewErrorf("unsupported desktop '%s' (valid: plasma, kde, gnome, cosmic, cinnamon, hyprland, none)", name)
	}

	return desktop, nil
}

// isValidBootloader checks if bootloader type is valid
func isValidBootloader(bl string) bool {
	return bl == string(BootloaderGrubEFI) || bl == string(BootloaderGrubLegacy)
}

// normalizeDevicePath ensures proper device path formatting
func normalizeDevicePath(device string) string {
	device = strings.TrimPrefix(device, "/dev/")
	device = filepath.Clean(device)
	return "/dev/" + device
}
