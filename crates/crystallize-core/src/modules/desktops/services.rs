use crate::utils::{exec::exec_chroot, exec_eval, files::copy_dir, install::install};

pub(super) fn networkmanager() {
  install(vec!["networkmanager"]);
  enable_service("NetworkManager", "Enable NetworkManager");
  let _ = copy_dir(
    "/etc/NetworkManager/system-connections/",
    "/mnt/etc/NetworkManager/system-connections/",
  );
}

pub(super) fn bluetooth() {
  enable_service("bluetooth", "Enable Bluetooth");
}

pub(super) fn ufw() {
  install(vec!["ufw"]);
  exec_eval(
    exec_chroot("ufw", vec![String::from("enable")]),
    "Enable Firewall",
  );
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
