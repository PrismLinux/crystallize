use crate::utils::{files, files_eval, install::install};

use super::services::enable_service;

pub(super) fn packages() {
  install(vec![
    "pipewire",
    "pipewire-pulse",
    "pipewire-alsa",
    "wireplumber",
    "bluez",
    "bluez-cups",
    "cups",
    "cups-pdf",
    "xdg-user-dirs",
    "zen-browser",
  ]);
}

pub(super) fn kde() {
  install(vec![
    "prismlinux-kde-settings",
    "konsole",
    "plasma-systemmonitor",
    "dolphin",
    "sddm",
  ]);
  enable_service("sddm", "Enable sddm");
}

pub(super) fn cinnamon() {
  install(vec![
    "xorg",
    "cinnamon",
    "lightdm",
    "lightdm-gtk-greeter",
    "lightdm-gtk-greeter-settings",
    "metacity",
    "gnome-shell",
    "gnome-terminal",
  ]);
  files_eval(
    files::append_file(
      "/mnt/etc/lightdm/lightdm.conf",
      "[SeatDefaults]\ngreeter-session=lightdm-gtk-greeter\n",
    ),
    "Add lightdm greeter",
  );
  enable_service("lightdm", "Enabling LightDM");
}

pub(super) fn gnome() {
  install(vec![
    "amberol",
    "mpv",
    "prismlinux-gnome-settings",
    "nautilus",
    "loupe",
    "gnome-system-monitor",
    "gdm",
  ]);

  enable_service("gdm", "Enabling gdm");
}
