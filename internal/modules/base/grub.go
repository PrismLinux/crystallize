package base

import (
	"crystallize-cli/internal/utils"
	"fmt"
	"path/filepath"
)

const (
	GrubThemeConfig   = `GRUB_THEME="/usr/share/grub/themes/prismlinux/theme.txt"`
	GrubConfigPath    = "/boot/grub/grub.cfg"
	GrubDefaultConfig = "/mnt/etc/default/grub"
)

var (
	GrubPackages = []string{
		"prismlinux/grub",
		"efibootmgr",
		"prismlinux-themes-grub",
		"os-prober",
	}

	GrubLegacyPackages = []string{
		"prismlinux/grub",
		"prismlinux-themes-grub",
		"os-prober",
	}
)

// InstallBootloaderEFI installs GRUB for EFI systems
func InstallBootloaderEFI(efidir string) error {
	// Install required packages
	if err := utils.Install(GrubPackages); err != nil {
		return fmt.Errorf("failed to install grub packages: %w", err)
	}

	// Prepare EFI directory path
	efiPath := filepath.Join("/mnt", efidir)

	// Validate EFI directory exists
	if !utils.Exists(efiPath) {
		return fmt.Errorf("the efidir %s doesn't exist", efidir)
	}

	// Install main GRUB EFI bootloader
	utils.ExecEval(
		utils.ExecChroot("grub-install",
			"--target=x86_64-efi",
			fmt.Sprintf("--efi-directory=%s", efidir),
			"--bootloader-id=PrismLinux",
			"--recheck"),
		"install grub as efi with proper boot entry",
	)

	// Install fallback EFI bootloader for compatibility
	utils.ExecEval(
		utils.ExecChroot("grub-install",
			"--target=x86_64-efi",
			fmt.Sprintf("--efi-directory=%s", efidir),
			"--bootloader-id=PrismLinux-fallback",
			"--removable",
			"--recheck"),
		"install grub as fallback efi bootloader",
	)

	// Configure theme and generate GRUB config
	configureGrubThemeAndConfig()

	// Set default boot entry
	utils.ExecEval(
		utils.ExecChroot("sh", "-c",
			"efibootmgr | grep 'PrismLinux' | head -1 | cut -c5-8 | xargs -I {} efibootmgr --bootorder {}"),
		"Set default boot entry",
	)

	return nil
}

// InstallBootloaderLegacy installs GRUB for Legacy BIOS systems
func InstallBootloaderLegacy(device string) error {
	// Install required packages
	if err := utils.Install(GrubLegacyPackages); err != nil {
		return fmt.Errorf("failed to install grub packages: %w", err)
	}

	// Validate device exists
	if !utils.Exists(device) {
		return fmt.Errorf("the device %s does not exist", device)
	}

	// Install Legacy GRUB
	utils.ExecEval(
		utils.ExecChroot("grub-install", "--target=i386-pc", "--recheck", device),
		"install grub as legacy",
	)

	// Configure theme and generate GRUB config
	configureGrubThemeAndConfig()
	return nil
}

// configureGrubThemeAndConfig applies GRUB theme and generates config
func configureGrubThemeAndConfig() {
	utils.FilesEval(
		utils.SedFile(
			GrubDefaultConfig,
			`#GRUB_THEME="/path/to/theme.txt"`,
			GrubThemeConfig,
		),
		"Enable Grub Theme",
	)

	utils.ExecEval(
		utils.ExecChroot("grub-mkconfig", "-o", GrubConfigPath),
		"Create grub.cfg",
	)
}
