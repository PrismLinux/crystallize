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
	if err := installNetworkManager(); err != nil {
		return fmt.Errorf("failed to install NetworkManager: %w", err)
	}
	if err := installFirewalld(); err != nil {
		return fmt.Errorf("failed to install Firewalld: %w", err)
	}

	// Install the specific desktop environment
	utils.LogInfo("Installing %s environment...", desktopSetup)
	var installErr error
	switch desktopSetup {
	case DesktopPlasma:
		installErr = installPlasma()
	case DesktopGnome:
		installErr = installGnome()
	case DesktopCosmic:
		installErr = installCosmic()
	case DesktopCinnamon:
		installErr = installCinnamon()
	case DesktopHyprland:
		installErr = InstallHyprland()
	default:
		// This case handles any unsupported desktop strings
		return fmt.Errorf("unsupported desktop environment: %s", desktopSetup)
	}

	if installErr != nil {
		return fmt.Errorf("failed to install %s: %w", desktopSetup, installErr)
	}

	// Install common services
	if err := installBluetooth(); err != nil {
		return fmt.Errorf("failed to install Bluetooth: %w", err)
	}
	if err := installCups(); err != nil {
		return fmt.Errorf("failed to install CUPS: %w", err)
	}
	if err := installTunedPPD(); err != nil {
		return fmt.Errorf("failed to install Power Manager: %w", err)
	}

	utils.LogInfo("Desktop setup completed successfully.")
	return nil
}
