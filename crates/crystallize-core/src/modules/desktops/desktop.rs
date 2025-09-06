use crate::utils::{files, files_eval, install::install};

use super::services::enable_service;

pub(super) fn graphics() -> Result<(), Box<dyn std::error::Error>> {
  install(vec!["prismlinux-graphics"])?;
  Ok(())
}

pub(super) fn packages() -> Result<(), Box<dyn std::error::Error>> {
  install(vec![
    "about",
    "pipewire",
    "pipewire-alsa",
    "pipewire-pulse",
    "pipewire-jack",
    "gst-plugin-pipewire",
    "wireplumber",
    "xdg-user-dirs",
    "noto-fonts",
    "noto-fonts-emoji",
    "noto-fonts-cjk",
    "noto-fonts-extra",
  ])?;
  Ok(())
}

// -------------[DE]-------------

pub(super) fn kde() -> Result<(), Box<dyn std::error::Error>> {
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

pub(super) fn cinnamon() -> Result<(), Box<dyn std::error::Error>> {
  install(vec![
    "cinnamon",
    "rio",
    "metacity",
    "gnome-shell",
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
