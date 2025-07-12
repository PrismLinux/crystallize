use crate::cli::DesktopSetup;
use crate::system::command::exec_chroot;
use crate::system::install::install;
use crate::utils::eval::exec_eval;

pub fn install_desktop_setup(desktop_setup: DesktopSetup) {
  log::debug!("Installing {desktop_setup:?}");
  match desktop_setup {
    DesktopSetup::Kde => install_kde(),
    DesktopSetup::Gnome => install_gnome(),
    DesktopSetup::None => log::debug!("No desktop setup selected"),
  }
  install_networkmanager();
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

fn install_kde() {
  install(vec![
    "sddm",
    "plasma-desktop",
    "konsole",
    "kate",
    "dolphin",
    "ark",
    "plasma-workspace",
    "papirus-icon-theme",
    "plasma-firewall"
  ]);
  enable_dm("sddm");
}

fn install_gnome() {
  install(vec!["gdm", "gnome", "gedit", "nautilus"]);
  enable_dm("gdm");
}

fn enable_dm(dm: &str) {
  log::debug!("Enabling {dm}");
  exec_eval(
    exec_chroot("systemctl", vec![String::from("enable"), String::from(dm)]),
    format!("Enable {dm}").as_str(),
  );
}
