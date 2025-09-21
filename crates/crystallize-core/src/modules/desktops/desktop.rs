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
    "ghostty",
    "dolphin",
    "plasma-systemmonitor",
  ])?;
  enable_service("sddm", "Enable SDDM");
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

  enable_service("gdm", "Enabling GDM");
  Ok(())
}

pub(super) fn cosmic() -> Result<(), Box<dyn std::error::Error>> {
  install(&[
    "prismlinux-cosmic-settings",
    "cosmic-files",
    "cosmic-greeter",
    "ghossty",
  ])?;

  enable_service("cosmic-greeter", "Enabling Cosmic Greeter");
  Ok(())
}

pub(super) fn cinnamon() -> Result<(), Box<dyn std::error::Error>> {
  install(&[
    "prismlinux-cinnamon-settings",
    "nemo",
    "ghossty",
    "lightdm",
    "lightdm-gtk-greeter",
    "lightdm-gtk-greeter-settings",
  ])?;

  files_eval(
    files::append_file(
      "/mnt/etc/lightdm/lightdm.conf",
      "[SeatDefaults]\ngreeter-session=lightdm-gtk-greeter\n",
    ),
    "Add lightdm greeter",
  );

  enable_service("lightdm", "Enabling LightDM");
  Ok(())
}

// -------------[WM]-------------
