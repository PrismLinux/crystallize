use crate::utils::{exec::exec_chroot, exec_eval, files, files_eval, install::install};

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
    "plasma-desktop",
    "plasma-workspace",
    "plasma-pa",
    "plasma-nm",
    "kinfocenter",
    "ark",
    "spectacle",
    "mpv",
    "powerdevil",
    "plasma-firewall",
    "skanpage",
    "kio-admin",
    "sddm-kcm",
    "kwalletmanager",
    "plasma-systemmonitor",
    "konsole",
    "plasma-browser-integration",
    "kde-gtk-config",
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
    "gnome-shell",
    "gnome-shell-extensions",
    "gnome-shell-extension-appindicator",
    "gnome-browser-connector",
    "gnome-tweaks",
    "nautilus",
    "gnome-control-center",
    "gnome-console",
    "gdm",
  ]);

  // Enable extentions
  let extensions = vec!["appindicatorsupport@rgcjonas.gmail.com"];

  for extension in extensions {
    exec_eval(
      exec_chroot(
        "gnome-extensions enable",
        vec![String::from(extension), String::from("--quiet")],
      ),
      "",
    );
  }

  enable_service("gdm", "Enabling gdm");
}
