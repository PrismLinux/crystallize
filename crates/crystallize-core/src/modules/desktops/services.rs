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
  install(vec!["bluez"]);
  enable_service("bluetooth", "Enable Bluetooth");
}

pub(super) fn cups() {
  install(vec!["cups", "cups-pdf", "bluez-cups"]);
  enable_service("cups", "Enable Cups");
}

pub(super) fn tuned_ppd() {
  install(vec!["tuned-ppd", "tuned"]);
  enable_service("tuned-ppd", "Enable Power Manager");
}

pub(super) fn ufw() {
  install(vec!["ufw"]);

  // Configure default rules
  exec_eval(
    exec_chroot(
      "ufw",
      vec![
        String::from("--force"),
        String::from("default"),
        String::from("deny"),
        String::from("incoming"),
      ],
    ),
    "Set UFW default deny incoming",
  );

  exec_eval(
    exec_chroot(
      "ufw",
      vec![
        String::from("--force"),
        String::from("default"),
        String::from("allow"),
        String::from("outgoing"),
      ],
    ),
    "Set UFW default allow outgoing",
  );

  // Enable service for autostart
  enable_service("ufw", "Enable UFW service");
}

pub(super) fn enable_service(service: &str, logmsg: &str) {
  log::debug!("Enabling {service}");
  exec_eval(
    exec_chroot(
      "systemctl",
      vec![
        String::from("--no-reload"),
        String::from("enable"),
        String::from(service),
      ],
    ),
    (*logmsg).to_string().as_str(),
  );
}
