use crate::{
  cli::DesktopSetup,
  system::{exec::exec_chroot, files, install::install},
  utils::{exec_eval, files_eval},
};

pub fn install_desktop_setup(desktop_setup: DesktopSetup) {
  log::debug!("Installing {desktop_setup:?}");
  match desktop_setup {
    DesktopSetup::Gnome => install_gnome(),
    DesktopSetup::Kde => install_kde(),
    DesktopSetup::Cinnamon => install_cinnamon(),
    DesktopSetup::None => log::debug!("No desktop setup selected"),
  }

  install_ufw();
  install_networkmanager();
  if desktop_setup != DesktopSetup::None {
    desktop_packages();
    install_tuned_ppd();
    enable_cups();
  }
}

fn desktop_packages() {
  install(vec![
    "pipewire",
    "pipewire-pulse",
    "pipewire-alsa",
    "bluez",
    "bluez-cups",
    "cups",
    "cups-pdf",
    "packagekit-qt6",
    "gnome-packagekit",
    "xdg-user-dirs",
    "zen-browser",
  ]);
}

fn install_networkmanager() {
  install(vec!["networkmanager"]);
  exec_eval(
    exec_chroot(
      "systemctl",
      vec![String::from("enable"), String::from("NetworkManager")],
    ),
    "Enable network manager",
  );
}

fn enable_cups() {
  exec_eval(
    exec_chroot(
      "systemctl",
      vec![String::from("enable"), String::from("cups")],
    ),
    "Enable CUPS",
  );
}

fn install_tuned_ppd() {
  install(vec!["tuned-ppd", "tuned"]);
  exec_eval(
    exec_chroot(
      "systemctl",
      vec![String::from("enable"), String::from("tuned-ppd")],
    ),
    "Enable power manager (tuned-ppd)",
  );
}

fn install_ufw() {
  install(vec!["ufw"]);
  exec_eval(
    exec_chroot(
      "systemctl",
      vec![String::from("enable"), String::from("ufw")],
    ),
    "Enable ufw firewall",
  );
}

fn install_cinnamon() {
  install(vec![
    "xorg",
    "cinnamon",
    "pipewire",
    "pipewire-pulse",
    "pipewire-alsa",
    "pipewire-jack",
    "wireplumber",
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
  enable_dm("lightdm");
}

fn install_kde() {
  install(vec![
    "xorg",
    "plasma-desktop",
    "plasma-workspace",
    "plasma-pa",
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
    "pipewire",
    "pipewire-pulse",
    "pipewire-alsa",
    "pipewire-jack",
    "wireplumber",
    "sddm",
  ]);
  enable_dm("sddm");
}

fn install_gnome() {
  install(vec![
    "gnome",
    "pipewire",
    "pipewire-pulse",
    "pipewire-alsa",
    "pipewire-jack",
    "wireplumber",
    "gdm",
  ]);
  enable_dm("gdm");
}

fn enable_dm(dm: &str) {
  log::debug!("Enabling {dm}");
  exec_eval(
    exec_chroot("systemctl", vec![String::from("enable"), String::from(dm)]),
    format!("Enable {dm}").as_str(),
  );
}
