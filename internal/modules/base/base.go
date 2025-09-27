package base

import (
	"crystallize-cli/internal/utils"
	"fmt"
	"os"
	"strings"
)

var (
	SupportedKernels = []string{"linux-cachyos", "linux616-tkg-bore-llvm", "linux-zen", "linux"}

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

// InstallBasePackages installs base system packages
func InstallBasePackages(kernel string) error {
	utils.LogInfo("Installing base packages to /mnt")

	if err := utils.CreateDirectory("/mnt/etc"); err != nil {
		utils.LogWarn("Failed to create /mnt/etc: %v", err)
	}

	kernelPkg := kernel
	if kernelPkg == "" {
		kernelPkg = "linux-cachyos"
	}

	supported := false
	for _, k := range SupportedKernels {
		if k == kernelPkg {
			supported = true
			break
		}
	}

	if !supported {
		utils.LogWarn("Unknown kernel: %s, using linux-cachyos instead", kernelPkg)
		kernelPkg = "linux-cachyos"
	}

	headers := kernelPkg + "-headers"
	packages := make([]string, 0, len(BasePackages)+2)
	packages = append(packages, BasePackages...)
	packages = append(packages, kernelPkg, headers)

	if err := utils.InstallBase(packages); err != nil {
		return fmt.Errorf("failed to install base packages: %w", err)
	}

	return nil
}

// SetupArchlinuxKeyring initializes the Arch Linux keyring
func SetupArchlinuxKeyring() error {
	utils.LogInfo("Setting up Arch Linux keyring in chroot")

	if err := utils.ExecChroot("which", "pacman-key"); err != nil {
		return fmt.Errorf("pacman-key not found in chroot environment. Base packages may not be installed properly")
	}

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
			return fmt.Errorf("failed to %s", strings.ToLower(step.description))
		}
		utils.LogInfo("✓ %s", step.description)
	}

	return nil
}

// CopyLiveConfig copies configuration from LiveISO to System with proper error handling
func CopyLiveConfig() {
	utils.LogInfo("Copying live configuration")

	// Copy pacman configuration with error handling
	if err := utils.CopyFileIfExists("/etc/pacman.conf", "/mnt/etc/pacman.conf"); err != nil {
		utils.LogError("Failed to copy pacman.conf: %v", err)
	} else {
		utils.LogInfo("✓ Copied pacman configuration")
	}

	// Copy mirrorlist directory with filtering for problematic files
	if err := utils.CopyDirectoryFiltered("/etc/pacman.d/", "/mnt/etc/pacman.d/"); err != nil {
		utils.LogWarn("Failed to copy mirrorlist, network performance may be degraded: %v", err)
	} else {
		utils.LogInfo("✓ Copied mirrorlist configuration")
	}

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
		return fmt.Errorf("failed to create /mnt/etc directory: %w", err)
	}

	// Generate fstab
	if err := utils.Exec("bash", "-c", "genfstab -U /mnt >> /mnt/etc/fstab"); err != nil {
		return fmt.Errorf("failed to generate fstab: %w", err)
	}

	// Verify fstab was created and has content
	if !utils.Exists("/mnt/etc/fstab") {
		return fmt.Errorf("fstab was not created")
	}

	// Check if fstab has meaningful content
	content, err := os.ReadFile("/mnt/etc/fstab")
	if err != nil {
		return fmt.Errorf("failed to read generated fstab: %w", err)
	}

	if len(strings.TrimSpace(string(content))) == 0 {
		return fmt.Errorf("generated fstab is empty")
	}

	utils.LogInfo("✓ Generated fstab successfully")
	return nil
}

// InstallZram installs and configures ZRAM
func InstallZram(size uint64) error {
	utils.LogInfo("Installing and configuring ZRAM")

	if err := utils.Install([]string{"zram-generator"}); err != nil {
		return fmt.Errorf("failed to install zram-generator: %w", err)
	}

	if err := utils.CreateDirectory("/mnt/etc/systemd"); err != nil {
		utils.LogError("Failed to create systemd directory: %v", err)
		return fmt.Errorf("failed to create systemd directory: %w", err)
	}

	var zramConfig string
	if size == 0 {
		zramConfig = "[zram0]\nzram-size = min(ram / 2, 4096)\ncompression-algorithm = zstd"
	} else {
		zramConfig = fmt.Sprintf("[zram0]\nzram-size = %d\ncompression-algorithm = zstd", size)
	}

	if err := utils.WriteFile("/mnt/etc/systemd/zram-generator.conf", zramConfig); err != nil {
		return fmt.Errorf("failed to write zram config: %w", err)
	}

	utils.LogInfo("✓ ZRAM configuration complete")
	return nil
}

// InstallFlatpak installs Flatpak package manager
func InstallFlatpak() error {
	utils.LogInfo("Installing Flatpak")

	if err := utils.Install([]string{"flatpak"}); err != nil {
		return fmt.Errorf("failed to install flatpak: %w", err)
	}

	if err := utils.ExecChroot("flatpak", "remote-add", "--if-not-exists", "flathub", "https://flathub.org/repo/flathub.flatpakrepo"); err != nil {
		return fmt.Errorf("failed to add flathub remote: %w", err)
	}

	utils.LogInfo("✓ Flatpak installation complete")
	return nil
}
