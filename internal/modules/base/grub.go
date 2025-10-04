package base

import (
	"crystallize-cli/internal/utils"
	"path/filepath"
)

const (
	// GRUB theme configuration
	grubThemeConfig      = `GRUB_THEME="/usr/share/grub/themes/prismlinux/theme.txt"`
	grubThemePlaceholder = `#GRUB_THEME="/path/to/theme.txt"`

	// GRUB paths
	grubConfigPath    = "/boot/grub/grub.cfg"
	grubDefaultConfig = "/mnt/etc/default/grub"

	// Bootloader IDs
	bootloaderIDMain     = "PrismLinux"
	bootloaderIDFallback = "PrismLinux-fallback"
)

var (
	// GrubPackages are the required packages for EFI GRUB installation
	grubEFIPackages = []string{
		"prismlinux/grub",
		"efibootmgr",
		"prismlinux-themes-grub",
		"os-prober",
	}

	// GrubLegacyPackages are the required packages for Legacy BIOS GRUB installation
	grubLegacyPackages = []string{
		"prismlinux/grub",
		"prismlinux-themes-grub",
		"os-prober",
	}
)

// InstallBootloaderEFI installs GRUB for EFI systems
func InstallBootloaderEFI(efidir string) error {
	utils.LogInfo("Installing GRUB bootloader for EFI system")

	// Install required packages
	if err := installGrubPackages(grubEFIPackages); err != nil {
		return err
	}

	// Validate EFI directory
	efiPath := filepath.Join("/mnt", efidir)
	if err := validatePath(efiPath, "EFI directory"); err != nil {
		utils.LogError("EFI directory validation failed: %v", err)
		return err
	}

	// Install main EFI bootloader
	if err := installMainEFIBootloader(efidir); err != nil {
		utils.LogError("Failed to install main EFI bootloader: %v", err)
		return err
	}

	// Install fallback EFI bootloader
	if err := installFallbackEFIBootloader(efidir); err != nil {
		utils.LogError("Failed to install fallback EFI bootloader: %v", err)
		return err
	}

	// Configure theme and generate GRUB config
	if err := configureGrubThemeAndConfig(); err != nil {
		utils.LogError("Failed to configure GRUB: %v", err)
		return err
	}

	// Set default boot entry
	if err := setDefaultBootEntry(); err != nil {
		// Non-fatal: log warning but continue
		utils.LogWarn("Failed to set default boot entry: %v", err)
	}

	utils.LogInfo("✓ GRUB EFI bootloader installed successfully")
	return nil
}

// InstallBootloaderLegacy installs GRUB for Legacy BIOS systems
func InstallBootloaderLegacy(device string) error {
	utils.LogInfo("Installing GRUB bootloader for Legacy BIOS system")

	// Install required packages
	if err := installGrubPackages(grubLegacyPackages); err != nil {
		return err
	}

	// Validate device exists
	if err := validatePath(device, "boot device"); err != nil {
		utils.LogError("Boot device validation failed: %v", err)
		return err
	}

	// Install Legacy GRUB to the device
	utils.LogInfo("Installing GRUB to %s", device)
	if err := utils.ExecChroot("grub-install", "--target=i386-pc", "--recheck", device); err != nil {
		utils.LogError("Failed to install Legacy GRUB: %v", err)
		return err
	}
	utils.LogInfo("✓ GRUB installed to %s", device)

	// Configure theme and generate GRUB config
	if err := configureGrubThemeAndConfig(); err != nil {
		utils.LogError("Failed to configure GRUB: %v", err)
		return err
	}

	utils.LogInfo("✓ GRUB Legacy bootloader installed successfully")
	return nil
}

// installGrubPackages installs the required GRUB packages
func installGrubPackages(packages []string) error {
	utils.LogInfo("Installing GRUB packages...")
	if err := utils.Install(packages); err != nil {
		utils.LogError("Failed to install GRUB packages: %v", err)
		return err
	}
	utils.LogInfo("✓ GRUB packages installed")
	return nil
}

// validatePath checks if a path exists
func validatePath(path, description string) error {
	if !utils.Exists(path) {
		return utils.NewErrorf("%s does not exist: %s", description, path)
	}
	utils.LogDebug("Validated %s: %s", description, path)
	return nil
}

// installMainEFIBootloader installs the main EFI bootloader
func installMainEFIBootloader(efidir string) error {
	utils.LogInfo("Installing main EFI bootloader...")

	err := utils.ExecChroot(
		"grub-install",
		"--target=x86_64-efi",
		utils.Sprintf("--efi-directory=%s", efidir),
		utils.Sprintf("--bootloader-id=%s", bootloaderIDMain),
		"--recheck",
	)

	if err != nil {
		return err
	}

	utils.LogInfo("✓ Main EFI bootloader installed")
	return nil
}

// installFallbackEFIBootloader installs the fallback EFI bootloader
func installFallbackEFIBootloader(efidir string) error {
	utils.LogInfo("Installing fallback EFI bootloader...")

	err := utils.ExecChroot(
		"grub-install",
		"--target=x86_64-efi",
		utils.Sprintf("--efi-directory=%s", efidir),
		utils.Sprintf("--bootloader-id=%s", bootloaderIDFallback),
		"--removable",
		"--recheck",
	)

	if err != nil {
		return err
	}

	utils.LogInfo("✓ Fallback EFI bootloader installed")
	return nil
}

// configureGrubThemeAndConfig applies GRUB theme and generates configuration
func configureGrubThemeAndConfig() error {
	utils.LogInfo("Configuring GRUB theme...")

	// Apply GRUB theme
	if err := applyGrubTheme(); err != nil {
		return err
	}

	// Generate GRUB configuration
	utils.LogInfo("Generating GRUB configuration...")
	if err := utils.ExecChroot("grub-mkconfig", "-o", grubConfigPath); err != nil {
		return err
	}
	utils.LogInfo("✓ GRUB configuration generated")

	return nil
}

// applyGrubTheme enables the PrismLinux GRUB theme
func applyGrubTheme() error {
	if err := utils.SedFile(grubDefaultConfig, grubThemePlaceholder, grubThemeConfig); err != nil {
		return err
	}
	utils.LogInfo("✓ GRUB theme enabled")
	return nil
}

// setDefaultBootEntry sets PrismLinux as the default boot entry
func setDefaultBootEntry() error {
	utils.LogInfo("Setting default boot entry...")

	// Get the first PrismLinux entry and set it as default
	err := utils.ExecChroot(
		"sh", "-c",
		utils.Sprintf(
			"efibootmgr | grep '%s' | head -1 | cut -c5-8 | xargs -I {} efibootmgr --bootorder {}",
			bootloaderIDMain,
		),
	)

	if err != nil {
		return err
	}

	utils.LogInfo("✓ Default boot entry set")
	return nil
}
