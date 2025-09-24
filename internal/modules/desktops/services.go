package desktops

import (
	"crystallize-cli/internal/utils"
)

func installNetworkManager() error {
	if err := utils.Install([]string{"networkmanager"}); err != nil {
		return err
	}
	enableService("NetworkManager", "Enable NetworkManager")

	// Copy network connections if they exist
	if utils.Exists("/etc/NetworkManager/system-connections/") {
		if err := utils.CopyDirectory(
			"/etc/NetworkManager/system-connections/",
			"/mnt/etc/NetworkManager/system-connections/",
		); err != nil {
			utils.LogWarn("Failed to copy network connections: %v", err)
		}
	}
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
