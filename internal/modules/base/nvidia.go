package base

import (
	"crystallize-cli/internal/utils"
	"fmt"
	"os/exec"
	"strings"
)

// NvidiaDriver represents different NVIDIA driver versions
type NvidiaDriver int

const (
	NvidiaOpen NvidiaDriver = iota
	NvidiaLegacy470
	NvidiaLegacy390
	NvidiaLegacy340
)

// Packages returns the required packages for the driver
func (d NvidiaDriver) Packages() []string {
	switch d {
	case NvidiaOpen:
		return []string{
			"dkms",
			"nvidia-open",
			"nvidia-open-dkms",
			"nvidia-utils",
			"egl-wayland",
		}
	case NvidiaLegacy470:
		return []string{
			"dkms",
			"nvidia-470xx-dkms",
			"nvidia-470xx-utils",
			"egl-wayland",
		}
	case NvidiaLegacy390:
		return []string{
			"dkms",
			"nvidia-390xx-dkms",
			"nvidia-390xx-utils",
			"egl-wayland",
		}
	case NvidiaLegacy340:
		return []string{
			"dkms",
			"nvidia-340xx-dkms",
			"nvidia-340xx-utils",
		}
	default:
		return []string{}
	}
}

// detectNvidiaGPU detects NVIDIA GPU information
func detectNvidiaGPU() (string, error) {
	cmd := exec.Command("lspci", "-nn")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to run lspci: %w", err)
	}

	outputStr := string(output)
	for line := range strings.SplitSeq(outputStr, "\n") {
		if strings.Contains(strings.ToLower(line), "nvidia") &&
			(strings.Contains(strings.ToLower(line), "vga") ||
				strings.Contains(strings.ToLower(line), "3d")) {
			return line, nil
		}
	}

	return "", fmt.Errorf("no NVIDIA GPU detected")
}

// determineDriverVersion determines the appropriate driver version
func determineDriverVersion(gpuInfo string) NvidiaDriver {
	gpuLower := strings.ToLower(gpuInfo)

	// RTX 20xx series and newer - use open driver
	if strings.Contains(gpuLower, "rtx 2") ||
		strings.Contains(gpuLower, "rtx 3") ||
		strings.Contains(gpuLower, "rtx 4") ||
		strings.Contains(gpuLower, "rtx 5") ||
		strings.Contains(gpuLower, "gtx 16") {
		return NvidiaOpen
	}

	// GTX 10xx series and some RTX cards
	if strings.Contains(gpuLower, "gtx 10") ||
		strings.Contains(gpuLower, "gtx 1050") ||
		strings.Contains(gpuLower, "gtx 1060") ||
		strings.Contains(gpuLower, "gtx 1070") ||
		strings.Contains(gpuLower, "gtx 1080") ||
		strings.Contains(gpuLower, "titan x") {
		return NvidiaLegacy470
	}

	// GTX 9xx and older supported cards + GT series
	if strings.Contains(gpuLower, "gtx 9") ||
		strings.Contains(gpuLower, "gtx 8") ||
		strings.Contains(gpuLower, "gtx 7") ||
		strings.Contains(gpuLower, "gtx 6") ||
		strings.Contains(gpuLower, "gt 6") ||
		strings.Contains(gpuLower, "gt 7") ||
		strings.Contains(gpuLower, "gt 730") ||
		strings.Contains(gpuLower, "gt 740") ||
		strings.Contains(gpuLower, "gt 710") ||
		strings.Contains(gpuLower, "gt 720") ||
		strings.Contains(gpuLower, "quadro") {
		return NvidiaLegacy390
	}

	// Very old cards
	if strings.Contains(gpuLower, "gtx 4") ||
		strings.Contains(gpuLower, "gtx 5") ||
		strings.Contains(gpuLower, "gt 4") ||
		strings.Contains(gpuLower, "gt 5") ||
		strings.Contains(gpuLower, "geforce 8") ||
		strings.Contains(gpuLower, "geforce 9") {
		return NvidiaLegacy340
	}

	// Default to open driver for newer unknown cards
	return NvidiaOpen
}

// InstallNvidia installs appropriate NVIDIA drivers
func InstallNvidia() error {
	utils.LogInfo("Detecting NVIDIA GPU...")

	gpuInfo, err := detectNvidiaGPU()
	if err != nil {
		utils.LogInfo("No NVIDIA GPU detected, skipping driver installation.")
		return nil // Continue installation without error
	}

	utils.LogInfo("Detected NVIDIA GPU: %s", gpuInfo)

	driver := determineDriverVersion(gpuInfo)
	packages := driver.Packages()

	utils.LogInfo("Installing NVIDIA driver: %v", driver)
	utils.LogInfo("Packages: %v", packages)

	if err := utils.Install(packages); err != nil {
		return fmt.Errorf("failed to install nvidia packages: %w", err)
	}

	// Apply nvidia module in grub
	if err := configureGrubForNvidia(); err != nil {
		return fmt.Errorf("failed to configure GRUB for NVIDIA: %w", err)
	}

	// Apply initcpio modules (skip for very old 340xx driver)
	if driver != NvidiaLegacy340 {
		if err := configureInitcpioForNvidia(); err != nil {
			return fmt.Errorf("failed to configure initcpio for NVIDIA: %w", err)
		}
	}

	utils.LogInfo("NVIDIA driver installation completed successfully!")
	return nil
}

// configureGrubForNvidia configures GRUB for NVIDIA
func configureGrubForNvidia() error {
	grubContent, err := utils.ReadFile("/mnt/etc/default/grub")
	if err != nil {
		grubContent = ""
	}

	lines := strings.Split(grubContent, "\n")
	grubConfFound := false

	for i, line := range lines {
		if strings.HasPrefix(line, "GRUB_CMDLINE_LINUX_DEFAULT=") {
			grubConfFound = true
			if !strings.Contains(line, "nvidia-drm.modeset=1") {
				lines[i] = strings.Replace(line,
					`GRUB_CMDLINE_LINUX_DEFAULT="`,
					`GRUB_CMDLINE_LINUX_DEFAULT="nvidia-drm.modeset=1 `,
					1)
			}
			break
		}
	}

	if !grubConfFound {
		lines = append(lines, `GRUB_CMDLINE_LINUX_DEFAULT="nvidia-drm.modeset=1"`)
	}

	newGrubContent := strings.Join(lines, "\n")
	return utils.WriteFile("/mnt/etc/default/grub", newGrubContent)
}

// configureInitcpioForNvidia configures initcpio for NVIDIA
func configureInitcpioForNvidia() error {
	initcpioContent, err := utils.ReadFile("/mnt/etc/mkinitcpio.conf")
	if err != nil {
		initcpioContent = ""
	}

	lines := strings.Split(initcpioContent, "\n")
	initcpioConfFound := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "MODULES=") && !strings.HasPrefix(trimmed, "#") {
			initcpioConfFound = true
			lines[i] = "MODULES=(nvidia nvidia_modeset nvidia_uvm nvidia_drm)"
			break
		}
	}

	if !initcpioConfFound {
		lines = append(lines, "MODULES=(nvidia nvidia_modeset nvidia_uvm nvidia_drm)")
	}

	newInitcpioContent := strings.Join(lines, "\n")
	return utils.WriteFile("/mnt/etc/mkinitcpio.conf", newInitcpioContent)
}
