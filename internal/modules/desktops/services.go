package desktops

import (
	"crystallize-cli/internal/utils"
	"os"
	"path/filepath"
)

const (
	networkConnectionsDir = "/etc/NetworkManager/system-connections/"
	mntNetworkDir         = "/mnt/etc/NetworkManager/system-connections/"
	dirPerms              = 0700
	filePerms             = 0600
)

// installNetworkManager installs NetworkManager and migrates existing connections
func installNetworkManager() error {
	utils.LogInfo("Installing NetworkManager...")

	// Install NetworkManager package
	if err := utils.Install([]string{"networkmanager"}); err != nil {
		utils.LogError("Failed to install NetworkManager: %v", err)
		return err
	}

	// Migrate network connections if they exist
	if err := migrateNetworkConnections(); err != nil {
		// Log warning but don't fail - this is not critical
		utils.LogWarn("Network connection migration failed: %v", err)
	}

	// Enable the NetworkManager service
	EnableService("NetworkManager", "Enable NetworkManager")
	return nil
}

// migrateNetworkConnections copies existing network connections to the new system
func migrateNetworkConnections() error {
	if !utils.Exists(networkConnectionsDir) {
		utils.LogDebug("No existing network connections to migrate")
		return nil
	}

	utils.LogInfo("Migrating network connections...")

	// Copy the entire directory
	if err := utils.CopyDirectory(networkConnectionsDir, mntNetworkDir); err != nil {
		utils.LogError("Copy directory failed: %v", err)
		return err
	}

	// Set directory permissions
	if err := utils.SetPermissions(mntNetworkDir, dirPerms); err != nil {
		utils.LogError("Set directory permissions failed: %v", err)
		return err
	}

	// Set file permissions for each connection file
	if err := setConnectionFilePermissions(mntNetworkDir); err != nil {
		utils.LogError("Set file permissions failed: %v", err)
		return err
	}

	utils.LogInfo("Network connections migrated successfully")
	return nil
}

// setConnectionFilePermissions sets secure permissions on connection files
func setConnectionFilePermissions(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		utils.LogError("Read directory failed: %v", err)
		return err
	}

	for _, entry := range entries {
		// Skip directories, only process files
		if entry.IsDir() {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		if err := utils.SetPermissions(filePath, filePerms); err != nil {
			// Log but continue - partial success is better than complete failure
			utils.LogWarn("Failed to set permissions for %s: %v", entry.Name(), err)
		}
	}

	return nil
}

// installFirewalld installs and enables the firewall service
func installFirewalld() error {
	utils.LogInfo("Installing Firewalld...")

	if err := utils.Install([]string{"firewalld"}); err != nil {
		utils.LogError("Failed to install Firewalld: %v", err)
		return err
	}

	EnableService("firewalld", "Enable Firewalld service")
	return nil
}

// installTunedPPD installs and enables power management
func installTunedPPD() error {
	utils.LogInfo("Installing power management (Tuned PPD)...")

	if err := utils.Install([]string{"tuned-ppd", "tuned"}); err != nil {
		utils.LogError("Failed to install Tuned PPD: %v", err)
		return err
	}

	EnableService("tuned-ppd", "Enable Power Manager")
	return nil
}

// installBluetooth installs and enables Bluetooth support
func installBluetooth() error {
	utils.LogInfo("Installing Bluetooth support...")

	if err := utils.Install([]string{"bluez"}); err != nil {
		utils.LogError("Failed to install Bluez: %v", err)
		return err
	}

	EnableService("bluetooth", "Enable Bluetooth")
	return nil
}

// installCups installs and enables the printing system
func installCups() error {
	utils.LogInfo("Installing CUPS printing system...")

	packages := []string{"cups", "cups-pdf", "bluez-cups"}
	if err := utils.Install(packages); err != nil {
		utils.LogError("Failed to install CUPS: %v", err)
		return err
	}

	EnableService("cups", "Enable CUPS")
	return nil
}

// EnableService enables a systemd service in the chroot environment
func EnableService(service, logMsg string) {
	utils.LogDebug("Enabling service: %s", service)
	utils.ExecEval(
		utils.ExecChroot("systemctl", "--no-reload", "enable", service),
		logMsg,
	)
}
