use crate::utils::{files, files_eval, install::install};

use super::services::enable_service;

pub(super) fn graphics() -> Result<(), Box<dyn std::error::Error>> {
  install(&["prismlinux-graphics"])?;
  Ok(())
}

pub(super) fn packages() -> Result<(), Box<dyn std::error::Error>> {
  install(&[
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
  ])?;
  Ok(())
}

// -------------[DE]-------------

pub(super) fn plasma() -> Result<(), Box<dyn std::error::Error>> {
  install(&[
    "prismlinux-plasma-settings",
    "sddm",
    "rio",
    "dolphin",
    "plasma-systemmonitor",
  ])?;
  enable_service("sddm", "Enable sddm");
  Ok(())
}

pub(super) fn gnome() -> Result<(), Box<dyn std::error::Error>> {
  install(&[
    "prismlinux-gnome-settings",
    "nautilus",
    "amberol",
    "mpv",
    "loupe",
    "gnome-system-monitor",
    "gdm",
  ])?;

  enable_service("gdm", "Enabling gdm");
  Ok(())
}

pub(super) fn cosmic() -> Result<(), Box<dyn std::error::Error>> {
  install(&[
    "cosmic-app-library",
    "cosmic-applets",
    "cosmic-bg",
    "cosmic-files",
    "cosmic-greeter",
    "cosmic-idle",
    "cosmic-launcher",
    "cosmic-notifications",
    "cosmic-osd",
    "cosmic-panel",
    "cosmic-randr",
    "cosmic-screenshot",
    "cosmic-session",
    "cosmic-settings",
    "cosmic-settings-daemon",
    "cosmic-wallpapers",
    "cosmic-workspaces",
    "xdg-desktop-portal-cosmic",
  ])?;

  enable_service("gdm", "Enabling gdm");
  Ok(())
}

pub(super) fn cinnamon() -> Result<(), Box<dyn std::error::Error>> {
  install(&[
    "cinnamon",
    "mint-themes",
    "mint-y-icons",
    "mint-x-icons",
    "nemo",
    "gnome-shell",
    "loupe",
    "lightdm",
    "lightdm-gtk-greeter",
    "lightdm-gtk-greeter-settings",
    "metacity"
  ])?;

  files_eval(
    files::append_file(
      "/mnt/etc/lightdm/lightdm.conf",
      "[SeatDefaults]\ngreeter-session=lightdm-gtk-greeter\n",
    ),
    "Add lightdm greeter",
  );

  enable_service("lightdm", "Enabling gdm");
  Ok(())
}

// -------------[WM]-------------
