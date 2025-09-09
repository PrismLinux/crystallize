use anyhow::{Result, anyhow};
use log::warn;
pub mod grub;
pub mod nvidia;

use crate::utils::{
  exec::{exec, exec_chroot},
  exec_eval,
  files::{self, create_directory},
  install,
};

const SUPPORTED_KERNELS: &[&str] = &["linux-cachyos", "linux616-tkg-bore", "linux-zen", "linux"];

const BASE_PACKAGES: &[&str] = &[
  // Base Arch
  "base",
  "cachyos-ananicy-rules-git",
  "linux-firmware",
  "nano",
  "sudo",
  "curl",
  "wget",
  "openssh",
  "iptables",
  // Base Prism
  "prism",
  "prismlinux",
  "prismlinux-themes-fish",
  // Extras
  "btrfs-progs",
  "xfsprogs",
  "ttf-liberation",
  "bash",
  "bash-completion",
  "glibc-locales",
  "fwupd",
  "unzip",
  // Repositories
  "archlinux-keyring",
  "archlinuxcn-keyring",
  "chaotic-keyring",
  "chaotic-mirrorlist",
];

pub fn install_base_packages(kernel: &str) -> Result<(), Box<dyn std::error::Error>> {
  log::info!("Installing base packages to /mnt");

  // Ensure /mnt/etc exists
  if let Err(e) = create_directory("/mnt/etc") {
    log::warn!("Failed to create /mnt/etc: {e}");
  }

  let kernel_pkg = match kernel {
    "" => "linux-cachyos",
    k if SUPPORTED_KERNELS.contains(&k) => k,
    k => {
      warn!("Unknown kernel: {k}, using linux-cachyos instead");
      "linux-cachyos"
    }
  };

  let headers = format!("{kernel_pkg}-headers");
  let mut packages = Vec::with_capacity(BASE_PACKAGES.len() + 2);
  packages.extend_from_slice(BASE_PACKAGES);
  packages.push(kernel_pkg);
  packages.push(&headers);

  install::install_base(packages)?;

  // Copy pacman configuration
  if let Err(e) = files::copy_file("/etc/pacman.conf", "/mnt/etc/pacman.conf") {
    log::error!("Failed to copy pacman.conf: {e}");
  }

  Ok(())
}

/// Update mirror keyring
pub fn setup_archlinux_keyring() -> Result<()> {
  log::info!("Setting up Arch Linux keyring in chroot");

  // Verify that pacman-key exists in the chroot
  match exec_chroot("which", vec!["pacman-key".to_string()]) {
    Ok(status) if status.success() => {
      log::debug!("pacman-key found in chroot");
    }
    _ => {
      return Err(anyhow!(
        "pacman-key not found in chroot environment. Base packages may not be installed properly."
      ));
    }
  }

  let keyring_steps = [
    ("--init", "Initialize pacman keyring"),
    ("--populate", "Populate pacman keyring"),
  ];

  for (arg, description) in keyring_steps {
    log::info!("Running: pacman-key {arg}");
    match exec_chroot("pacman-key", vec![arg.to_string()]) {
      Ok(status) if status.success() => {
        log::info!("✓ {description}");
      }
      Ok(status) => {
        let error_msg = format!(
          "✗ {} failed with exit code: {:?}",
          description,
          status.code()
        );
        log::error!("{error_msg}");
        return Err(anyhow!("Failed to {}", description.to_lowercase()));
      }
      Err(e) => {
        let error_msg = format!("✗ {description} failed: {e}");
        log::error!("{error_msg}");
        return Err(e.into());
      }
    }
  }

  // Refresh the keyring
  log::info!("Refreshing keyring");
  match exec_chroot("pacman", vec!["-Sy".to_string(), "--noconfirm".to_string()]) {
    Ok(status) if status.success() => {
      log::info!("✓ Keyring refreshed successfully");
    }
    Ok(status) => {
      log::warn!("Keyring refresh failed with exit code: {:?}", status.code());
    }
    Err(e) => {
      log::warn!("Keyring refresh failed: {e}");
    }
  }

  Ok(())
}

/// Generate Fstab
pub fn genfstab() {
  log::info!("Generating fstab");
  exec_eval(
    exec(
      "bash",
      vec![
        String::from("-c"),
        String::from("genfstab -U /mnt >> /mnt/etc/fstab"),
      ],
    ),
    "Generate fstab",
  );
}

/// Copy configuration from `LiveISO` to System
pub fn copy_live_config() {
  log::info!("Copying live configuration");
}

/// Using Zram
pub fn install_zram(size: u64) -> Result<(), Box<dyn std::error::Error>> {
  log::info!("Installing and configuring ZRAM");

  install::install(vec!["zram-generator"])?;

  // Ensure the systemd directory exists
  if let Err(e) = files::create_directory("/mnt/etc/systemd") {
    log::error!("Failed to create systemd directory: {e}");
    return Ok(());
  }

  let zram_config = if size == 0 {
    "[zram0]\nzram-size = min(ram / 2, 4096)\ncompression-algorithm = zstd"
  } else {
    &format!("[zram0]\nzram-size = {size}\ncompression-algorithm = zstd")
  };

  if let Err(e) = files::write_file("/mnt/etc/systemd/zram-generator.conf", zram_config) {
    log::error!("Failed to write zram config: {e}");
    return Ok(());
  }

  log::info!("ZRAM configuration complete");
  Ok(())
}

pub fn install_homemgr() -> Result<(), Box<dyn std::error::Error>> {
  log::info!("Installing Nix package manager");
  install::install(vec!["nix"])?;
  Ok(())
}

pub fn install_flatpak() -> Result<(), Box<dyn std::error::Error>> {
  log::info!("Installing Flatpak");
  install::install(vec!["flatpak"])?;

  exec_eval(
    exec_chroot(
      "flatpak",
      vec![
        String::from("remote-add"),
        String::from("--if-not-exists"),
        String::from("flathub"),
        String::from("https://flathub.org/repo/flathub.flatpakrepo"),
      ],
    ),
    "add flathub remote",
  );
  Ok(())
}
