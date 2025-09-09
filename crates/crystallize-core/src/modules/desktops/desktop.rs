use crate::utils::install::install;

use super::services::enable_service;

pub(super) fn graphics() -> Result<(), Box<dyn std::error::Error>> {
  install(vec!["prismlinux-graphics"])?;
  Ok(())
}

pub(super) fn packages() -> Result<(), Box<dyn std::error::Error>> {
  install(vec![
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
  install(vec![
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
  install(vec![
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

// -------------[WM]-------------

pub(super) fn hyprland() -> Result<(), Box<dyn std::error::Error>> {
  install(vec!["prismlinux-hyprland-settings", "sddm"])?;
  enable_service("sddm", "Enable sddm");
  Ok(())
}
