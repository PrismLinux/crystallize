package desktops

import (
	"crystallize-cli/internal/utils"
	"fmt"
)

// DesktopSetup represents the desktop environment to install
type DesktopSetup string

const (
	DesktopGnome    DesktopSetup = "gnome"
	DesktopPlasma   DesktopSetup = "plasma"
	DesktopCosmic   DesktopSetup = "cosmic"
	DesktopCinnamon DesktopSetup = "cinnamon"
	DesktopHyprland DesktopSetup = "hyprland"
	DesktopNone     DesktopSetup = "none"
)

// InstallDesktopSetup installs the selected desktop environment
func InstallDesktopSetup(desktopSetup DesktopSetup) error {
	utils.LogDebug("Starting desktop setup for: %v", desktopSetup)

	// Exit early if no desktop environment is selected
	if desktopSetup == DesktopNone {
		utils.LogInfo("No desktop setup selected, skipping.")
		return nil
	}

	// Install common components for all graphical environments
	if err := installCommonComponents(); err != nil {
		return fmt.Errorf("failed to install common components: %w", err)
	}

	// Install the specific desktop environment
	utils.LogInfo("Installing %s environment...", desktopSetup)
	if err := installDesktopEnvironment(desktopSetup); err != nil {
		return fmt.Errorf("failed to install %s: %w", desktopSetup, err)
	}

	// Install common services
	if err := installCommonServices(); err != nil {
		return fmt.Errorf("failed to install common services: %w", err)
	}

	utils.LogInfo("Desktop setup completed successfully.")
	return nil
}

// installCommonComponents installs networking and firewall
func installCommonComponents() error {
	if err := installNetworkManager(); err != nil {
		utils.LogError("NetworkManager installation failed: %v", err)
		return err
	}
	if err := installFirewalld(); err != nil {
		utils.LogError("Firewalld installation failed: %v", err)
		return err
	}
	return nil
}

// installDesktopEnvironment routes to the appropriate installer
func installDesktopEnvironment(desktop DesktopSetup) error {
	config, exists := desktopConfigs[desktop]
	if !exists {
		err := fmt.Errorf("unsupported desktop environment: %s", desktop)
		utils.LogError("%v", err)
		return err
	}
	return config.install()
}

// installCommonServices installs bluetooth, printing, and power management
func installCommonServices() error {
	if err := installBluetooth(); err != nil {
		utils.LogError("Bluetooth installation failed: %v", err)
		return err
	}
	if err := installCups(); err != nil {
		utils.LogError("CUPS installation failed: %v", err)
		return err
	}
	if err := installTunedPPD(); err != nil {
		utils.LogError("Power Manager installation failed: %v", err)
		return err
	}
	return nil
}
