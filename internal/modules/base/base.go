package base

import (
	"crystallize-cli/internal/utils"
	"fmt"
	"os"
	"slices"
	"strings"
)

var (
	SupportedKernels = []string{
		"linux-cachyos",
		"linux616-tkg-bore-llvm",
		"linux-zen",
		"linux",
	}

	BasePackages = []string{
		// Base Arch
		"base",
		"cachyos-ananicy-rules-git",
		"linux-firmware",
		"nano",
		"sudo",
		"curl",
		"wget",
		"openssh",
		// Base Prism
		"prism",
		"prismlinux",
		"prismlinux-themes-fish",
		// Extras
		"btrfs-progs",
		"xfsprogs",
		"terminus-font",
		"ttf-liberation",
		"bash",
		"bash-completion",
		"glibc-locales",
		"fwupd",
		"unzip",
		// Repositories
		"archlinux-keyring",
		"archlinuxcn-keyring",
		"archlinuxcn-mirrorlist-git",
		"chaotic-keyring",
		"chaotic-mirrorlist",
	}
)

const (
	defaultKernel    = "linux-cachyos"
	defaultZramSize  = 0 // 0 means auto-calculate
	zramAutoConfig   = "[zram0]\nzram-size = min(ram / 2, 4096)\ncompression-algorithm = zstd"
	zramManualConfig = "[zram0]\nzram-size = %d\ncompression-algorithm = zstd"
)

// InstallBasePackages installs base system packages with the specified kernel
func InstallBasePackages(kernel string) error {
	utils.LogInfo("Installing base packages to /mnt")

	// Ensure /mnt/etc directory exists
	if err := utils.CreateDirectory("/mnt/etc"); err != nil {
		utils.LogWarn("Failed to create /mnt/etc: %v", err)
	}

	// Validate and normalize kernel selection
	kernelPkg := normalizeKernel(kernel)
	utils.LogDebug("Selected kernel: %s", kernelPkg)

	// Build package list with kernel and headers
	packages := buildPackageList(kernelPkg)

	// Install all packages
	if err := utils.InstallBase(packages); err != nil {
		utils.LogError("Failed to install base packages: %v", err)
		return err
	}

	utils.LogInfo("✓ Base packages installed successfully")
	return nil
}

// normalizeKernel validates the kernel choice and returns a supported kernel
func normalizeKernel(kernel string) string {
	// Use default if empty
	if kernel == "" {
		return defaultKernel
	}

	// Check if kernel is supported
	if slices.Contains(SupportedKernels, kernel) {
		return kernel
	}

	// Fall back to default for unsupported kernels
	utils.LogWarn("Unknown kernel: %s, using %s instead", kernel, defaultKernel)
	return defaultKernel
}

// buildPackageList creates the complete package list including kernel and headers
func buildPackageList(kernel string) []string {
	headers := kernel + "-headers"
	packages := make([]string, 0, len(BasePackages)+2)
	packages = append(packages, BasePackages...)
	packages = append(packages, kernel, headers)
	return packages
}

// SetupArchlinuxKeyring initializes the Arch Linux keyring in the chroot environment
func SetupArchlinuxKeyring() error {
	utils.LogInfo("Setting up Arch Linux keyring in chroot")

	// Verify pacman-key exists
	if err := utils.ExecChroot("which", "pacman-key"); err != nil {
		utils.LogError("pacman-key not found in chroot environment")
		return err
	}

	// Initialize and populate keyring
	if err := initializeKeyring(); err != nil {
		return err
	}

	// Copy pacman configuration
	if err := copyPacmanConfig(); err != nil {
		// Log but don't fail - non-critical
		utils.LogError("Failed to copy pacman configuration: %v", err)
	}

	// Copy mirrorlist configuration
	if err := copyMirrorlist(); err != nil {
		// Log but don't fail - non-critical
		utils.LogWarn("Failed to copy mirrorlist, network performance may be degraded: %v", err)
	}

	utils.LogInfo("✓ Keyring setup complete")
	return nil
}

// initializeKeyring runs pacman-key initialization steps
func initializeKeyring() error {
	keyringSteps := []struct {
		arg         string
		description string
	}{
		{"--init", "Initialize pacman keyring"},
		{"--populate", "Populate pacman keyring"},
	}

	for _, step := range keyringSteps {
		utils.LogInfo("Running: pacman-key %s", step.arg)

		if err := utils.ExecChroot("pacman-key", step.arg); err != nil {
			utils.LogError("Failed to %s: %v", strings.ToLower(step.description), err)
			return err
		}

		utils.LogInfo("✓ %s", step.description)
	}

	return nil
}

// copyPacmanConfig copies pacman.conf to the chroot
func copyPacmanConfig() error {
	if err := utils.CopyFileIfExists("/etc/pacman.conf", "/mnt/etc/pacman.conf"); err != nil {
		return err
	}
	utils.LogInfo("✓ Copied pacman configuration")
	return nil
}

// copyMirrorlist copies mirrorlist configuration to the chroot
func copyMirrorlist() error {
	if err := utils.CopyDirectoryFiltered("/etc/pacman.d/", "/mnt/etc/pacman.d/"); err != nil {
		return err
	}
	utils.LogInfo("✓ Copied mirrorlist configuration")
	return nil
}

// CopyLiveConfig copies configuration from LiveISO to the installed system
func CopyLiveConfig() {
	utils.LogInfo("Copying live configuration")

	// Copy vconsole configuration
	if err := utils.CopyFileIfExists("/etc/vconsole.conf", "/mnt/etc/vconsole.conf"); err != nil {
		utils.LogWarn("Failed to copy vconsole.conf: %v", err)
	} else {
		utils.LogInfo("✓ Copied console configuration")
	}
}

// Genfstab generates the filesystem table
func Genfstab() error {
	utils.LogInfo("Generating fstab")

	// Ensure /mnt/etc exists
	if err := utils.CreateDirectory("/mnt/etc"); err != nil {
		utils.LogError("Failed to create /mnt/etc directory: %v", err)
		return err
	}

	// Generate fstab
	if err := utils.Exec("bash", "-c", "genfstab -U /mnt >> /mnt/etc/fstab"); err != nil {
		utils.LogError("Failed to generate fstab: %v", err)
		return err
	}

	// Verify fstab was created successfully
	if err := verifyFstab(); err != nil {
		utils.LogError("fstab verification failed: %v", err)
		return err
	}

	utils.LogInfo("✓ Generated fstab successfully")
	return nil
}

// verifyFstab checks that the generated fstab exists and has content
func verifyFstab() error {
	fstabPath := "/mnt/etc/fstab"

	// Check if fstab exists
	if !utils.Exists(fstabPath) {
		utils.LogError("fstab was not created")
		return fmt.Errorf("fstab was not created")
	}

	// Read and verify content
	content, err := os.ReadFile(fstabPath)
	if err != nil {
		utils.LogError("Failed to read generated fstab: %v", err)
		return err
	}

	// Check for meaningful content
	if len(strings.TrimSpace(string(content))) == 0 {
		utils.LogError("Generated fstab is empty")
		return fmt.Errorf("generated fstab is empty")
	}

	return nil
}

// InstallZram installs and configures ZRAM swap
func InstallZram(size uint64) error {
	utils.LogInfo("Installing and configuring ZRAM")

	// Install zram-generator package
	if err := utils.Install([]string{"zram-generator"}); err != nil {
		utils.LogError("Failed to install zram-generator: %v", err)
		return err
	}

	// Create systemd directory
	if err := utils.CreateDirectory("/mnt/etc/systemd"); err != nil {
		utils.LogError("Failed to create systemd directory: %v", err)
		return err
	}

	// Generate and write ZRAM configuration
	zramConfig := generateZramConfig(size)
	if err := utils.WriteFile("/mnt/etc/systemd/zram-generator.conf", zramConfig); err != nil {
		utils.LogError("Failed to write zram config: %v", err)
		return err
	}

	utils.LogInfo("✓ ZRAM configuration complete")
	return nil
}

// generateZramConfig creates the ZRAM configuration string
func generateZramConfig(size uint64) string {
	if size == defaultZramSize {
		utils.LogDebug("Using automatic ZRAM size calculation")
		return zramAutoConfig
	}

	utils.LogDebug("Using manual ZRAM size: %d MB", size)
	return fmt.Sprintf(zramManualConfig, size)
}

// InstallFlatpak installs Flatpak package manager and adds Flathub repository
func InstallFlatpak() error {
	utils.LogInfo("Installing Flatpak")

	// Install flatpak package
	if err := utils.Install([]string{"flatpak"}); err != nil {
		utils.LogError("Failed to install flatpak: %v", err)
		return err
	}

	// Add Flathub repository
	if err := addFlathubRepo(); err != nil {
		utils.LogError("Failed to add Flathub repository: %v", err)
		return err
	}

	utils.LogInfo("✓ Flatpak installation complete")
	return nil
}

// addFlathubRepo adds the Flathub repository to Flatpak
func addFlathubRepo() error {
	return utils.ExecChroot(
		"flatpak",
		"remote-add",
		"--if-not-exists",
		"flathub",
		"https://flathub.org/repo/flathub.flatpakrepo",
	)
}
