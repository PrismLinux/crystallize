use crate::utils::{exec::exec_chroot, exec_eval, files::copy_dir, install::install};

pub(super) fn networkmanager() -> Result<(), Box<dyn std::error::Error>> {
  install(vec!["networkmanager"])?;
  enable_service("NetworkManager", "Enable NetworkManager");

  let _ = copy_dir(
    "/etc/NetworkManager/system-connections/",
    "/mnt/etc/NetworkManager/system-connections/",
  );
  Ok(())
}

pub(super) fn bluetooth() -> Result<(), Box<dyn std::error::Error>> {
  install(vec!["bluez"])?;
  enable_service("bluetooth", "Enable Bluetooth");
  Ok(())
}

pub(super) fn cups() -> Result<(), Box<dyn std::error::Error>> {
  install(vec!["cups", "cups-pdf", "bluez-cups"])?;
  enable_service("cups", "Enable Cups");
  Ok(())
}

pub(super) fn tuned_ppd() -> Result<(), Box<dyn std::error::Error>> {
  install(vec!["tuned-ppd", "tuned"])?;
  enable_service("tuned-ppd", "Enable Power Manager");
  Ok(())
}

pub(super) fn ufw() -> Result<(), Box<dyn std::error::Error>> {
  install(vec!["ufw"])?;

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
  Ok(())
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
