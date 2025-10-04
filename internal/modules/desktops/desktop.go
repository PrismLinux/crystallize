package desktops

import (
	"crystallize-cli/internal/utils"
)

// Common packages shared across all desktop environments
var (
	graphicsPackages    = []string{"prismlinux-graphics"}
	baseDesktopPackages = []string{
		"about",
		// Sound
		"pipewire",
		"pipewire-alsa",
		"pipewire-jack",
		"pipewire-pulse",
		"gst-plugin-pipewire",
		"libpulse",
		"sof-firmware",
		"wireplumber",
		// Desktop essentials
		"floorp",
		"xdg-user-dirs",
		"wpa_supplicant",
		"xdg-utils",
		"noto-fonts",
		"noto-fonts-cjk",
		"noto-fonts-emoji",
		"noto-fonts-extra",
	}
)

// desktopConfig holds configuration for a desktop environment
type desktopConfig struct {
	packages    []string
	service     string
	serviceDesc string
	postInstall func() error
}

// install executes the installation for this desktop configuration
func (dc *desktopConfig) install() error {
	// Build complete package list
	allPackages := buildPackageList(dc.packages)

	// Install packages
	if err := utils.Install(allPackages); err != nil {
		utils.LogError("Package installation failed: %v", err)
		return err
	}

	// Run post-install hook if provided
	if dc.postInstall != nil {
		if err := dc.postInstall(); err != nil {
			utils.LogError("Post-install hook failed: %v", err)
			return err
		}
	}

	// Enable service if specified
	if dc.service != "" {
		EnableService(dc.service, dc.serviceDesc)
	}

	return nil
}

// desktopConfigs maps desktop environments to their configurations
var desktopConfigs = map[DesktopSetup]*desktopConfig{
	DesktopPlasma: {
		packages: []string{
			"prismlinux-plasma-settings",
			"sddm",
			"ghostty",
			"gwenview",
			"dolphin",
			"plasma-systemmonitor",
		},
		service:     "sddm",
		serviceDesc: "SDDM display manager",
	},
	DesktopGnome: {
		packages: []string{
			"prismlinux-gnome-settings",
			"nautilus",
			"amberol",
			"mpv",
			"loupe",
			"gnome-system-monitor",
			"gdm",
		},
		service:     "gdm",
		serviceDesc: "GDM display manager",
	},
	DesktopCosmic: {
		packages: []string{
			"prismlinux-cosmic-settings",
			"cosmic-files",
			"cosmic-greeter",
			"loupe",
			"ghostty",
		},
		service:     "cosmic-greeter",
		serviceDesc: "Cosmic Greeter display manager",
	},
	DesktopCinnamon: {
		packages: []string{
			"prismlinux-cinnamon-settings",
			"nemo",
			"loupe",
			"ghostty",
			"lightdm",
			"lightdm-gtk-greeter",
			"lightdm-gtk-greeter-settings",
		},
		service:     "lightdm",
		serviceDesc: "LightDM display manager",
		postInstall: configureLightDM,
	},
	DesktopHyprland: {
		packages: []string{
			"prismlinux-hyprland-quickshellbase",
			"sddm",
		},
		service:     "sddm",
		serviceDesc: "SDDM display manager",
	},
}

// buildPackageList combines base packages with desktop-specific packages
func buildPackageList(desktopPackages []string) []string {
	totalSize := len(desktopPackages) + len(graphicsPackages) + len(baseDesktopPackages)
	packages := make([]string, 0, totalSize)

	packages = append(packages, graphicsPackages...)
	packages = append(packages, baseDesktopPackages...)
	packages = append(packages, desktopPackages...)

	return packages
}

// configureLightDM sets up LightDM configuration
func configureLightDM() error {
	config := "[SeatDefaults]\ngreeter-session=lightdm-gtk-greeter\n"
	if err := utils.AppendFile("/mnt/etc/lightdm/lightdm.conf", config); err != nil {
		utils.LogError("Failed to configure LightDM: %v", err)
		return err
	}
	utils.LogInfo("LightDM configuration applied")
	return nil
}
