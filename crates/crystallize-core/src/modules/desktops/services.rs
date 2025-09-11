use crate::utils::{exec::exec_chroot, exec_eval, files::copy_dir, install::install};

pub(super) fn networkmanager() -> Result<(), Box<dyn std::error::Error>> {
  install(&["networkmanager"])?;
  enable_service("NetworkManager", "Enable NetworkManager");

  let _ = copy_dir(
    "/etc/NetworkManager/system-connections/",
    "/mnt/etc/NetworkManager/system-connections/",
  );
  Ok(())
}

pub(super) fn firewalld() -> Result<(), Box<dyn std::error::Error>> {
  install(&["firewalld"])?;
  enable_service("firewalld", "Enable Firewalld service");
  Ok(())
}

pub(super) fn tuned_ppd() -> Result<(), Box<dyn std::error::Error>> {
  install(&["tuned-ppd", "tuned"])?;
  enable_service("tuned-ppd", "Enable Power Manager");
  Ok(())
}

pub(super) fn bluetooth() -> Result<(), Box<dyn std::error::Error>> {
  install(&["bluez"])?;
  enable_service("bluetooth", "Enable Bluetooth");
  Ok(())
}

pub(super) fn cups() -> Result<(), Box<dyn std::error::Error>> {
  install(&["cups", "cups-pdf", "bluez-cups"])?;
  enable_service("cups", "Enable Cups");
  Ok(())
}

pub(super) fn enable_service(service: &str, logmsg: &str) {
  log::debug!("Enabling {service}");
  exec_eval(
    exec_chroot("systemctl", &["--no-reload", "enable", service]),
    logmsg,
  );
}
