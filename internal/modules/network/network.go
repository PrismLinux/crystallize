package network

import "crystallize-cli/internal/utils"

// SetHostname sets the system hostname
func SetHostname(hostname string) {
	utils.LogInfo("Setting hostname to %s", hostname)
	_ = utils.CreateFile("/mnt/etc/hostname")
	utils.FilesEval(utils.AppendFile("/mnt/etc/hostname", hostname), "set hostname")
}

// CreateHosts creates the /etc/hosts file
func CreateHosts() {
	_ = utils.CreateFile("/mnt/etc/hosts")
	utils.FilesEval(
		utils.AppendFile("/mnt/etc/hosts", "127.0.0.1     localhost"),
		"create /etc/hosts",
	)
}

// EnableIPv6 enables IPv6 localhost entry
func EnableIPv6() {
	utils.FilesEval(
		utils.AppendFile("/mnt/etc/hosts", "::1 localhost"),
		"add ipv6 localhost",
	)
}
