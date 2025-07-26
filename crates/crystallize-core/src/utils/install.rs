use crate::utils::exec::exec_chroot;
use crate::utils::exec_eval;
use std::process::Command;

/// Install packages using pacstrap
pub fn install_base(pkgs: Vec<&str>) {
  exec_eval(
    Command::new("pacstrap").arg("/mnt").args(&pkgs).status(),
    format!("Install base packages {}", pkgs.join(", ")).as_str(),
  );
}

/// Install packages in the chroot environment
pub fn install(pkgs: Vec<&str>) {
  if pkgs.is_empty() {
    return;
  }

  log::info!("Installing packages in chroot: {}", pkgs.join(", "));

  let mut pacman_args = vec![
    String::from("-S"),
    String::from("--noconfirm"),
    String::from("--needed"),
  ];

  // Add all package names
  pacman_args.extend(pkgs.iter().map(|&s| s.to_string()));

  exec_eval(
    exec_chroot("pacman", pacman_args),
    format!("Install packages {}", pkgs.join(", ")).as_str(),
  );
}

/// Install AUR packages using an AUR helper
pub fn install_aur(pkgs: Vec<&str>) {
  if pkgs.is_empty() {
    return;
  }

  log::info!("Installing AUR packages: {}", pkgs.join(", "));

  // Try different AUR helpers in order of preference
  let aur_helpers = ["prism", "paru"];

  for helper in &aur_helpers {
    match exec_chroot("which", vec![helper.to_string()]) {
      Ok(status) if status.success() => {
        log::info!("Using AUR helper: {helper}");
        let mut helper_args = vec![String::from("-S"), String::from("--noconfirm")];
        helper_args.extend(pkgs.iter().map(|&s| s.to_string()));

        exec_eval(
          exec_chroot(helper, helper_args),
          format!("Install AUR packages {} with {}", pkgs.join(", "), helper).as_str(),
        );
        return;
      }
      _ => continue,
    }
  }

  log::warn!(
    "No AUR helper found, skipping AUR packages: {}",
    pkgs.join(", ")
  );
}

/// Update package databases
pub fn update_databases() {
  log::info!("Updating package databases");
  exec_eval(
    exec_chroot(
      "pacman",
      vec![String::from("-Sy"), String::from("--noconfirm")],
    ),
    "Update package databases",
  );
}

/// Upgrade all packages
pub fn upgrade_system() {
  log::info!("Upgrading system packages");
  exec_eval(
    exec_chroot(
      "pacman",
      vec![String::from("-Syu"), String::from("--noconfirm")],
    ),
    "Upgrade system packages",
  );
}
