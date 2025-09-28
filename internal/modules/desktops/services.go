package desktops

import (
	"crystallize-cli/internal/utils"
	"os"
	"path/filepath"
)

func installNetworkManager() error {
	// Install NetworkManager package
	if err := utils.Install([]string{"networkmanager"}); err != nil {
		return err
	}

	// Copy network connections if they exist
	if utils.Exists("/etc/NetworkManager/system-connections/") {
		if err := utils.CopyDirectory(
			"/etc/NetworkManager/system-connections/",
			"/mnt/etc/NetworkManager/system-connections/",
		); err != nil {
			utils.LogWarn("Failed to copy network connections: %v", err)
		} else {
			// Set permissions for the directory to 0700
			if err := utils.SetPermissions("/mnt/etc/NetworkManager/system-connections/", 0700); err != nil {
				utils.LogWarn("Failed to set permissions for directory: %v", err)
			}

			// Set permissions for each file inside the directory to 0600
			entries, err := os.ReadDir("/mnt/etc/NetworkManager/system-connections/")
			if err != nil {
				utils.LogWarn("Failed to read directory: %v", err)
			}

			for _, entry := range entries {
				filePath := filepath.Join("/mnt/etc/NetworkManager/system-connections/", entry.Name())

				// Skip directories, only set permissions for files
				if entry.IsDir() {
					continue
				}

				// Set file permissions to 0600
				if err := utils.SetPermissions(filePath, 0600); err != nil {
					utils.LogWarn("Failed to set permissions for file %s: %v", filePath, err)
				}
			}
		}
	}

	// Enable the NetworkManager service
	enableService("NetworkManager", "Enable NetworkManager")
	return nil
}

func installFirewalld() error {
	if err := utils.Install([]string{"firewalld"}); err != nil {
		return err
	}
	enableService("firewalld", "Enable Firewalld service")
	return nil
}

func installTunedPPD() error {
	if err := utils.Install([]string{"tuned-ppd", "tuned"}); err != nil {
		return err
	}
	enableService("tuned-ppd", "Enable Power Manager")
	return nil
}

func installBluetooth() error {
	if err := utils.Install([]string{"bluez"}); err != nil {
		return err
	}
	enableService("bluetooth", "Enable Bluetooth")
	return nil
}

func installCups() error {
	if err := utils.Install([]string{"cups", "cups-pdf", "bluez-cups"}); err != nil {
		return err
	}
	enableService("cups", "Enable Cups")
	return nil
}

func enableService(service, logMsg string) {
	utils.LogDebug("Enabling %s", service)
	utils.ExecEval(
		utils.ExecChroot("systemctl", "--no-reload", "enable", service),
		logMsg,
	)
}
