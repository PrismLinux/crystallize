use crate::utils::{exec::exec_chroot, exec_eval, install::install};

pub(super) fn networkmanager() {
  install(vec!["networkmanager"]);
  enable_service("NetworkManager", "Enable NetworkManager");
}

pub(super) fn bluetooth() {
  enable_service("bluetooth", "Enable Bluetooth");
}

pub(super) fn ufw() {
  install(vec!["ufw"]);
  enable_service("ufw", "Enable Firewall");
}

pub(super) fn cups() {
  enable_service("cups", "Enable Cups");
}

pub(super) fn tuned_ppd() {
  install(vec!["tuned-ppd", "tuned"]);
  enable_service("tuned-ppd", "Enable Power Manager");
}

pub(super) fn enable_service(service: &str, logmsg: &str) {
  log::debug!("Enabling {service}");
  exec_eval(
    exec_chroot(
      "systemctl",
      vec![String::from("enable"), String::from(service)],
    ),
    (*logmsg).to_string().as_str(),
  );
}
