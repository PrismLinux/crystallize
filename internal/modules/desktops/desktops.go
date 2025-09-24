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
	DesktopNone     DesktopSetup = "none"
)

// InstallDesktopSetup installs the selected desktop environment
func InstallDesktopSetup(desktopSetup DesktopSetup) error {
	utils.LogDebug("Installing %v", desktopSetup)

	if err := installNetworkManager(); err != nil {
		return fmt.Errorf("failed to install NetworkManager: %w", err)
	}

	if err := installFirewalld(); err != nil {
		return fmt.Errorf("failed to install Firewalld: %w", err)
	}

	// Install only the selected desktop environment and its specific packages
	switch desktopSetup {
	case DesktopGnome:
		if err := installGraphics(); err != nil {
			return err
		}
		if err := installDesktopPackages(); err != nil {
			return err
		}
		if err := installGnome(); err != nil {
			return err
		}
	case DesktopPlasma:
		if err := installGraphics(); err != nil {
			return err
		}
		if err := installDesktopPackages(); err != nil {
			return err
		}
		if err := installPlasma(); err != nil {
			return err
		}
	case DesktopCosmic:
		if err := installGraphics(); err != nil {
			return err
		}
		if err := installDesktopPackages(); err != nil {
			return err
		}
		if err := installCosmic(); err != nil {
			return err
		}
	case DesktopCinnamon:
		if err := installGraphics(); err != nil {
			return err
		}
		if err := installDesktopPackages(); err != nil {
			return err
		}
		if err := installCinnamon(); err != nil {
			return err
		}
	case DesktopNone:
		utils.LogDebug("No desktop setup selected")
		return nil
	}

	// Common services for all desktop environments
	if err := installBluetooth(); err != nil {
		return fmt.Errorf("failed to install Bluetooth: %w", err)
	}
	if err := installCups(); err != nil {
		return fmt.Errorf("failed to install CUPS: %w", err)
	}
	if err := installTunedPPD(); err != nil {
		return fmt.Errorf("failed to install Power Manager: %w", err)
	}

	return nil
}
