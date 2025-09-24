package desktops

import "crystallize-cli/internal/utils"

func installGraphics() error {
	return utils.Install([]string{"prismlinux-graphics"})
}

func installDesktopPackages() error {
	packages := []string{
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
		"xdg-user-dirs",
		"wpa_supplicant",
		"xdg-utils",
	}
	return utils.Install(packages)
}

// -------------[DE]-------------

func installPlasma() error {
	packages := []string{
		"prismlinux-plasma-settings",
		"sddm",
		"ghostty",
		"dolphin",
		"plasma-systemmonitor",
	}
	if err := utils.Install(packages); err != nil {
		return err
	}
	enableService("sddm", "Enable SDDM")
	return nil
}

func installGnome() error {
	packages := []string{
		"prismlinux-gnome-settings",
		"nautilus",
		"amberol",
		"mpv",
		"loupe",
		"gnome-system-monitor",
		"gdm",
	}
	if err := utils.Install(packages); err != nil {
		return err
	}
	enableService("gdm", "Enabling GDM")
	return nil
}

func installCosmic() error {
	packages := []string{
		"prismlinux-cosmic-settings",
		"cosmic-files",
		"cosmic-greeter",
		"ghossty",
	}
	if err := utils.Install(packages); err != nil {
		return err
	}
	enableService("cosmic-greeter", "Enabling Cosmic Greeter")
	return nil
}

func installCinnamon() error {
	packages := []string{
		"prismlinux-cinnamon-settings",
		"nemo",
		"ghossty",
		"lightdm",
		"lightdm-gtk-greeter",
		"lightdm-gtk-greeter-settings",
	}
	if err := utils.Install(packages); err != nil {
		return err
	}

	utils.FilesEval(
		utils.AppendFile(
			"/mnt/etc/lightdm/lightdm.conf",
			"[SeatDefaults]\ngreeter-session=lightdm-gtk-greeter\n",
		),
		"Add lightdm greeter",
	)

	enableService("lightdm", "Enabling LightDM")
	return nil
}
