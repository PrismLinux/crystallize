package desktops

import "crystallize-cli/internal/utils"

var (
	installGraphics        = []string{"prismlinux-graphics"}
	installDesktopPackages = []string{
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
		// Desktop
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

func buildPackages(base []string) []string {
	pkgs := make([]string, 0, len(base)+len(installGraphics)+len(installDesktopPackages))
	pkgs = append(pkgs, base...)
	pkgs = append(pkgs, installGraphics...)
	pkgs = append(pkgs, installDesktopPackages...)
	return pkgs
}

// -------------[DE]-------------
func installAndEnable(packages []string, service string, serviceMsg string, post func() error) error {
	if err := utils.Install(packages); err != nil {
		return err
	}
	if post != nil {
		if err := post(); err != nil {
			return err
		}
	}
	if service != "" {
		enableService(service, serviceMsg)
	}
	return nil
}

func installPlasma() error {
	base := []string{
		"prismlinux-plasma-settings",
		"sddm",
		"ghostty",
		"gwenview",
		"dolphin",
		"plasma-systemmonitor",
	}
	return installAndEnable(buildPackages(base), "sddm", "Enable SDDM", nil)
}

func installGnome() error {
	base := []string{
		"prismlinux-gnome-settings",
		"nautilus",
		"amberol",
		"mpv",
		"loupe",
		"gnome-system-monitor",
		"gdm",
	}
	return installAndEnable(buildPackages(base), "gdm", "Enabling GDM", nil)
}

func installCosmic() error {
	base := []string{
		"prismlinux-cosmic-settings",
		"cosmic-files",
		"cosmic-greeter",
		"loupe",
		"ghossty",
	}
	return installAndEnable(buildPackages(base), "cosmic-greeter", "Enabling Cosmic Greeter", nil)
}

func installCinnamon() error {
	base := []string{
		"prismlinux-cinnamon-settings",
		"nemo",
		"loupe",
		"ghossty",
		"lightdm",
		"lightdm-gtk-greeter",
		"lightdm-gtk-greeter-settings",
	}
	post := func() error {
		utils.FilesEval(
			utils.AppendFile(
				"/mnt/etc/lightdm/lightdm.conf",
				"[SeatDefaults]\ngreeter-session=lightdm-gtk-greeter\n",
			),
			"Add lightdm greeter",
		)
		return nil
	}
	return installAndEnable(buildPackages(base), "lightdm", "Enabling LightDM", post)
}

// -------------[WM]-------------

func InstallHyprland() error {
	base := []string{
		"prismlinux-hyprland-quickshellbase",
		"sddm",
	}
	return installAndEnable(buildPackages(base), "sddm", "Enable SDDM", nil)
}
