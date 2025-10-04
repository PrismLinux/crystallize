package base

import (
	"crystallize-cli/internal/utils"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// DriverVariant represents NVIDIA driver variants
type DriverVariant int

const (
	DriverOpen DriverVariant = iota
	DriverLegacy470
	DriverLegacy390
	DriverLegacy340
	DriverNone
)

// String returns the driver name
func (d DriverVariant) String() string {
	names := map[DriverVariant]string{
		DriverOpen:      "nvidia-open (latest)",
		DriverLegacy470: "nvidia-470xx (legacy)",
		DriverLegacy390: "nvidia-390xx (legacy)",
		DriverLegacy340: "nvidia-340xx (legacy)",
		DriverNone:      "none",
	}
	return names[d]
}

// Packages returns required packages for the driver
func (d DriverVariant) Packages() []string {
	basePackages := []string{"dkms"}

	packageSets := map[DriverVariant][]string{
		DriverOpen: {
			"nvidia-open",
			"nvidia-open-dkms",
			"nvidia-utils",
			"egl-wayland",
		},
		DriverLegacy470: {
			"nvidia-470xx-dkms",
			"nvidia-470xx-utils",
			"egl-wayland",
		},
		DriverLegacy390: {
			"nvidia-390xx-dkms",
			"nvidia-390xx-utils",
			"egl-wayland",
		},
		DriverLegacy340: {
			"nvidia-340xx-dkms",
			"nvidia-340xx-utils",
		},
	}

	packages := packageSets[d]
	if len(packages) == 0 {
		return []string{}
	}

	return append(basePackages, packages...)
}

// RequiresKernelModules indicates if driver needs kernel modules
func (d DriverVariant) RequiresKernelModules() bool {
	return d != DriverLegacy340 && d != DriverNone
}

// GPUInfo contains information about detected GPU
type GPUInfo struct {
	Name        string
	DeviceID    string
	VendorID    string
	SubsystemID string
	RawLine     string
}

// pciIDPattern matches PCI device IDs in lspci output
var pciIDPattern = regexp.MustCompile(`\[([0-9a-f]{4}):([0-9a-f]{4})\]`)

// DetectNvidiaGPU detects NVIDIA GPU using lspci
func DetectNvidiaGPU() (*GPUInfo, error) {
	cmd := exec.Command("lspci", "-nn")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("lspci execution failed: %w", err)
	}

	outputStr := string(output)
	lines := strings.SplitSeq(outputStr, "\n")

	for line := range lines {
		if !isNvidiaGPULine(line) {
			continue
		}

		gpu := parseGPUInfo(line)
		if gpu != nil {
			return gpu, nil
		}
	}

	return nil, fmt.Errorf("no NVIDIA GPU detected")
}

// isNvidiaGPULine checks if line contains NVIDIA GPU info
func isNvidiaGPULine(line string) bool {
	lower := strings.ToLower(line)
	hasNvidia := strings.Contains(lower, "nvidia")
	hasVGA := strings.Contains(lower, "vga") ||
		strings.Contains(lower, "3d controller") ||
		strings.Contains(lower, "display controller")
	return hasNvidia && hasVGA
}

// parseGPUInfo extracts GPU information from lspci line
func parseGPUInfo(line string) *GPUInfo {
	matches := pciIDPattern.FindAllStringSubmatch(line, -1)
	if len(matches) < 1 {
		return nil
	}

	// First match is typically [vendorID:deviceID]
	vendorID := matches[0][1]
	deviceID := matches[0][2]

	// Extract GPU name (everything after the IDs)
	parts := strings.SplitN(line, ":", 3)
	name := "Unknown NVIDIA GPU"
	if len(parts) == 3 {
		name = strings.TrimSpace(parts[2])
		// Remove PCI IDs from name
		name = pciIDPattern.ReplaceAllString(name, "")
		name = strings.TrimSpace(name)
	}

	return &GPUInfo{
		Name:     name,
		VendorID: vendorID,
		DeviceID: deviceID,
		RawLine:  line,
	}
}

// DetermineDriver selects appropriate driver based on GPU info
func DetermineDriver(gpu *GPUInfo) DriverVariant {
	if gpu == nil {
		return DriverNone
	}

	// Use device ID for precise matching when available
	if gpu.DeviceID != "" {
		if driver := driverByDeviceID(gpu.DeviceID); driver != DriverNone {
			return driver
		}
	}

	// Fallback to name-based detection
	return driverByName(gpu.Name)
}

// driverByDeviceID determines driver by PCI device ID
func driverByDeviceID(deviceID string) DriverVariant {
	// Convert to uppercase for comparison
	deviceID = strings.ToUpper(deviceID)

	// Turing and later (RTX 20xx+, GTX 16xx) - Open driver
	// Device IDs: 1E00-1FFF (Turing), 2000-2FFF (Ampere), 2500-26FF (Ada)
	if (deviceID >= "1E00" && deviceID <= "1FFF") ||
		(deviceID >= "2000" && deviceID <= "2FFF") ||
		(deviceID >= "2500" && deviceID <= "26FF") {
		return DriverOpen
	}

	// Pascal (GTX 10xx series) - 470 legacy
	// Device IDs: 1B00-1DFF
	if deviceID >= "1B00" && deviceID <= "1DFF" {
		return DriverLegacy470
	}

	// Maxwell (GTX 9xx, 7xx) and older Kepler - 390 legacy
	// Device IDs: 1000-1AFF
	if deviceID >= "1000" && deviceID <= "1AFF" {
		return DriverLegacy390
	}

	// Very old cards (Fermi and older) - 340 legacy
	// Device IDs: 0000-0FFF
	if deviceID >= "0000" && deviceID <= "0FFF" {
		return DriverLegacy340
	}

	return DriverNone
}

// driverByName determines driver by GPU name (fallback)
func driverByName(name string) DriverVariant {
	lower := strings.ToLower(name)

	// Modern cards - Open driver
	modernPatterns := []string{
		"rtx 20", "rtx 30", "rtx 40", "rtx 50",
		"gtx 16", "rtx 2", "rtx 3", "rtx 4", "rtx 5",
	}
	for _, pattern := range modernPatterns {
		if strings.Contains(lower, pattern) {
			return DriverOpen
		}
	}

	// Pascal generation - 470 legacy
	pascalPatterns := []string{
		"gtx 10", "titan x", "titan xp",
	}
	for _, pattern := range pascalPatterns {
		if strings.Contains(lower, pattern) {
			return DriverLegacy470
		}
	}

	// Maxwell/Kepler - 390 legacy
	maxwellPatterns := []string{
		"gtx 9", "gtx 8", "gtx 7", "gtx 6",
		"gt 6", "gt 7", "gt 710", "gt 720", "gt 730", "gt 740",
		"quadro k", "quadro m",
	}
	for _, pattern := range maxwellPatterns {
		if strings.Contains(lower, pattern) {
			return DriverLegacy390
		}
	}

	// Very old cards - 340 legacy
	oldPatterns := []string{
		"gtx 4", "gtx 5", "gt 4", "gt 5",
		"geforce 8", "geforce 9", "geforce 2", "geforce 3",
	}
	for _, pattern := range oldPatterns {
		if strings.Contains(lower, pattern) {
			return DriverLegacy340
		}
	}

	// Default for unknown newer cards
	return DriverOpen
}

// InstallNvidia detects and installs appropriate NVIDIA drivers
func InstallNvidia() error {
	utils.LogInfo("Detecting NVIDIA GPU...")

	gpu, err := DetectNvidiaGPU()
	if err != nil {
		utils.LogInfo("No NVIDIA GPU detected, skipping driver installation")
		return nil
	}

	utils.LogInfo("Detected GPU: %s", gpu.Name)
	utils.LogDebug("GPU Device ID: %s, Vendor ID: %s", gpu.DeviceID, gpu.VendorID)

	driver := DetermineDriver(gpu)
	if driver == DriverNone {
		utils.LogWarn("Could not determine appropriate driver for GPU")
		return nil
	}

	utils.LogInfo("Selected driver: %s", driver.String())

	packages := driver.Packages()
	if len(packages) == 0 {
		utils.LogInfo("No driver packages to install")
		return nil
	}

	utils.LogDebug("Installing packages: %v", packages)
	if err := utils.Install(packages); err != nil {
		return fmt.Errorf("install nvidia packages: %w", err)
	}

	// Configure GRUB
	if err := ConfigureGrubForNvidia(); err != nil {
		return fmt.Errorf("configure GRUB: %w", err)
	}

	// Configure initramfs (except for very old drivers)
	if driver.RequiresKernelModules() {
		if err := ConfigureInitcpioForNvidia(); err != nil {
			return fmt.Errorf("configure initcpio: %w", err)
		}
	}

	utils.LogInfo("NVIDIA driver installation completed successfully")
	return nil
}

// ConfigureGrubForNvidia adds NVIDIA parameters to GRUB config
func ConfigureGrubForNvidia() error {
	const grubPath = "/mnt/etc/default/grub"
	const nvidiaParam = "nvidia-drm.modeset=1"

	content, err := utils.ReadFile(grubPath)
	if err != nil {
		return fmt.Errorf("read grub config: %w", err)
	}

	// Check if already configured
	if strings.Contains(content, nvidiaParam) {
		utils.LogDebug("GRUB already configured for NVIDIA")
		return nil
	}

	lines := strings.Split(content, "\n")
	modified := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip comments
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(trimmed, "GRUB_CMDLINE_LINUX_DEFAULT=") {
			lines[i] = addParameterToGrubLine(line, nvidiaParam)
			modified = true
			break
		}
	}

	// Add line if not found
	if !modified {
		lines = append(lines, fmt.Sprintf(`GRUB_CMDLINE_LINUX_DEFAULT="%s"`, nvidiaParam))
	}

	newContent := strings.Join(lines, "\n")
	if err := utils.WriteFile(grubPath, newContent); err != nil {
		return fmt.Errorf("write grub config: %w", err)
	}

	utils.LogDebug("GRUB configured for NVIDIA")
	return nil
}

// addParameterToGrubLine adds parameter to GRUB_CMDLINE_LINUX_DEFAULT line
func addParameterToGrubLine(line, param string) string {
	// Find the quoted section
	startQuote := strings.Index(line, `"`)
	endQuote := strings.LastIndex(line, `"`)

	if startQuote == -1 || endQuote == -1 || startQuote == endQuote {
		// Malformed line, append parameter safely
		return fmt.Sprintf(`GRUB_CMDLINE_LINUX_DEFAULT="%s"`, param)
	}

	// Extract existing parameters
	existing := line[startQuote+1 : endQuote]

	// Add new parameter
	if existing == "" {
		existing = param
	} else {
		existing = param + " " + existing
	}

	// Reconstruct line
	return fmt.Sprintf(`GRUB_CMDLINE_LINUX_DEFAULT="%s"`, existing)
}

// ConfigureInitcpioForNvidia adds NVIDIA modules to initramfs
func ConfigureInitcpioForNvidia() error {
	const initcpioPath = "/mnt/etc/mkinitcpio.conf"
	const nvidiaModules = "nvidia nvidia_modeset nvidia_uvm nvidia_drm"

	content, err := utils.ReadFile(initcpioPath)
	if err != nil {
		return fmt.Errorf("read mkinitcpio.conf: %w", err)
	}

	// Check if already configured
	if strings.Contains(content, "nvidia_drm") {
		utils.LogDebug("mkinitcpio already configured for NVIDIA")
		return nil
	}

	lines := strings.Split(content, "\n")
	modified := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip comments
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(trimmed, "MODULES=") {
			lines[i] = fmt.Sprintf("MODULES=(%s)", nvidiaModules)
			modified = true
			break
		}
	}

	// Add MODULES line if not found
	if !modified {
		// Find a good place to insert (after comments at top)
		insertIdx := 0
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				insertIdx = i
				break
			}
		}

		newLine := fmt.Sprintf("MODULES=(%s)", nvidiaModules)
		lines = append(lines[:insertIdx], append([]string{newLine}, lines[insertIdx:]...)...)
	}

	newContent := strings.Join(lines, "\n")
	if err := utils.WriteFile(initcpioPath, newContent); err != nil {
		return fmt.Errorf("write mkinitcpio.conf: %w", err)
	}

	utils.LogDebug("mkinitcpio configured for NVIDIA")
	return nil
}
