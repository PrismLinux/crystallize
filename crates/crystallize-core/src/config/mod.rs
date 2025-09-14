use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};
use std::{fs, path::PathBuf, thread, time::Duration};

use crate::{
  cli::{self, DesktopSetup, PartitionMode},
  modules::{
    base::{self, grub, nvidia},
    desktops, locale, network, partition, users,
  },
  utils::{exec::exec, exec_eval, install},
};

#[derive(Serialize, Deserialize, Debug)]
pub struct Config {
  partition: PartitionConfig,
  bootloader: BootloaderConfig,
  locale: LocaleConfig,
  networking: NetworkConfig,
  users: Vec<UserConfig>,
  rootpass: String,
  desktop: String,
  zram: u64,
  nvidia: bool,
  extra_packages: Vec<String>,
  kernel: String,
  flatpak: bool,
}

#[derive(Serialize, Deserialize, Debug)]
struct PartitionConfig {
  device: String,
  mode: PartitionMode,
  efi: bool,
  partitions: Vec<String>,
}

#[derive(Serialize, Deserialize, Debug)]
struct BootloaderConfig {
  r#type: String,
  location: String,
}

#[derive(Serialize, Deserialize, Debug)]
struct LocaleConfig {
  locale: Vec<String>,
  keymap: String,
  timezone: String,
}

#[derive(Serialize, Deserialize, Debug)]
struct NetworkConfig {
  hostname: String,
  ipv6: bool,
}

#[derive(Serialize, Deserialize, Debug)]
struct UserConfig {
  name: String,
  password: String,
  hasroot: bool,
  shell: String,
}

impl Config {
  pub fn from_file(path: &PathBuf) -> Result<Self> {
    let content = fs::read_to_string(path)
      .with_context(|| format!("Failed to read config file: {}", path.display()))?;

    let config: Self = serde_json::from_str(&content)
      .with_context(|| format!("Failed to parse config file: {}", path.display()))?;

    log::debug!("Successfully loaded config from {}", path.display());
    Ok(config)
  }

  fn parse_partitions(&self) -> Result<Vec<cli::Partition>> {
    self
      .partition
      .partitions
      .iter()
      .map(|partition| {
        let parts: Vec<&str> = partition.split(':').collect();
        if parts.len() != 3 {
          anyhow::bail!("Invalid partition format: {partition}");
        }
        Ok(cli::Partition::new(
          parts[0].to_string(),
          parts[1].to_string(),
          parts[2].to_string(),
        ))
      })
      .collect()
  }

  fn setup_partitions(&self) -> Result<()> {
    log::info!("Block device: /dev/{}", self.partition.device);
    log::info!("Partitioning mode: {:?}", self.partition.mode);
    log::info!("EFI mode: {}", self.partition.efi);

    let mut partitions = self.parse_partitions()?;
    let device = PathBuf::from("/dev/").join(&self.partition.device);

    partition::partition(
      &device,
      &self.partition.mode,
      self.partition.efi,
      &mut partitions,
    );
    Ok(())
  }

  fn install_base_system(&self) -> Result<(), Box<dyn std::error::Error>> {
    log::info!("Setting up base system...");

    // Ensure essential host system tools are available
    Self::ensure_host_tools();

    // Install base packages first
    base::install_base_packages(&self.kernel.clone())?;

    // Setup the chroot environment after base packages are installed
    Self::prepare_chroot_environment();

    // Now setup keyring inside the chroot where pacman-key exists
    base::setup_archlinux_keyring().context("Failed to setup keyring")?;

    // Generate fstab early so it's available for other operations
    base::genfstab();

    // Install additional components if requested
    if self.flatpak {
      base::install_flatpak()?;
    }

    Ok(())
  }

  fn ensure_host_tools() {
    log::debug!("Ensuring essential host tools are available");

    // Check for essential commands on the host system
    let essential_tools = vec!["cat", "mount", "umount", "chroot", "pacstrap"];

    for tool in essential_tools {
      let result = exec("which", &[tool]);
      match result {
        Ok(status) if status.success() => {
          log::debug!("Found essential tool: {tool}");
        }
        _ => {
          log::warn!("Essential tool {tool} not found on host system");
        }
      }
    }
  }

  fn prepare_chroot_environment() {
    log::debug!("Preparing chroot environment");

    // Create essential directories first
    let essential_dirs = vec![
      "/mnt/proc",
      "/mnt/sys",
      "/mnt/dev",
      "/mnt/dev/pts",
      "/mnt/tmp",
      "/mnt/run",
    ];

    for dir in essential_dirs {
      let result = exec("mkdir", &["-p", dir]);
      if result.is_err() {
        log::warn!("Failed to create directory: {dir}");
      }
    }

    // Mount essential filesystems
    exec_eval(
      exec("mount", &["-t", "proc", "proc", "/mnt/proc"]),
      "mount proc filesystem",
    );

    exec_eval(
      exec("mount", &["-t", "sysfs", "sysfs", "/mnt/sys"]),
      "mount sys filesystem",
    );

    exec_eval(
      exec("mount", &["--bind", "/dev", "/mnt/dev"]),
      "bind mount dev filesystem",
    );

    // Mount devpts for proper terminal support
    exec_eval(
      exec("mount", &["-t", "devpts", "devpts", "/mnt/dev/pts"]),
      "mount devpts filesystem",
    );

    // Wait for mounts to stabilize
    thread::sleep(Duration::from_millis(1000));
  }

  fn setup_bootloader(&self) -> Result<(), Box<dyn std::error::Error>> {
    log::info!("Installing bootloader: {}", self.bootloader.r#type);
    log::info!("Bootloader location: {}", self.bootloader.location);

    if self.bootloader.r#type == "grub-efi" {
      grub::install_bootloader_efi(&PathBuf::from(&self.bootloader.location))?;
    } else if self.bootloader.r#type == "grub-legacy" {
      grub::install_bootloader_legacy(&PathBuf::from(&self.bootloader.location))?;
    }

    Ok(())
  }

  fn configure_locale(&self) {
    log::info!("Configuring locale: {:?}", self.locale.locale);
    log::info!("Keyboard layout: {}", self.locale.keymap);
    log::info!("Timezone: {}", self.locale.timezone);

    locale::set_locale(&self.locale.locale.join(" "));
    locale::set_keyboard(&self.locale.keymap);
    locale::set_timezone(&self.locale.timezone);
  }

  fn setup_networking(&self) {
    log::info!("Hostname: {}", self.networking.hostname);
    log::info!("IPv6 enabled: {}", self.networking.ipv6);

    network::set_hostname(&self.networking.hostname);
    network::create_hosts();

    if self.networking.ipv6 {
      network::enable_ipv6();
    }
  }

  fn install_desktop(&self) -> Result<(), Box<dyn std::error::Error>> {
    log::info!("Installing desktop: {}", self.desktop);

    let desktop_setup = match self.desktop.to_lowercase().as_str() {
      "plasma" | "kde" => Some(DesktopSetup::Plasma),
      "gnome" => Some(DesktopSetup::Gnome),
      "cosmic" => Some(DesktopSetup::Cosmic),
      "cinnamon" => Some(DesktopSetup::Cinnamon),
      "none" => Some(DesktopSetup::None),
      _ => {
        log::warn!("Unknown desktop: {}, skipping", self.desktop);
        None
      }
    };

    if let Some(setup) = desktop_setup {
      desktops::install_desktop_setup(setup)?;
    }
    Ok(())
  }

  fn create_users(&self) -> Result<(), Box<dyn std::error::Error>> {
    for user in &self.users {
      log::info!("Creating user: {}", user.name);
      log::debug!("User has root: {}", user.hasroot);
      log::debug!("User shell: {}", user.shell);

      users::new_user(&user.name, user.hasroot, &user.password, false, &user.shell)?;
    }

    log::info!("Setting root password");
    users::root_pass(&self.rootpass);
    Ok(())
  }

  fn finalize_installation(&self) -> Result<(), Box<dyn std::error::Error>> {
    log::info!("Finalizing installation...");

    base::copy_live_config();

    if self.nvidia {
      log::info!("Installing NVIDIA drivers");
      nvidia::install_nvidia()?;
    }

    // Setup ZRAM
    let zram_info = if self.zram == 0 {
      "auto (min(ram/2, 4096))".to_string()
    } else {
      format!("{}MB", self.zram)
    };
    log::info!("Configuring ZRAM: {zram_info}");
    base::install_zram(self.zram)?;

    // Install extra packages
    if !self.extra_packages.is_empty() {
      log::info!("Installing extra packages: {:?}", self.extra_packages);
      let packages: Vec<&str> = self.extra_packages.iter().map(String::as_str).collect();
      install::install(&packages)?;
    }

    // Clean up mount points
    Self::cleanup_installation();

    Ok(())
  }

  fn cleanup_installation() {
    log::debug!("Cleaning up installation mounts");

    // Wait a moment for any pending operations to complete
    thread::sleep(Duration::from_millis(1000));

    // Unmount in reverse order with multiple attempts if needed
    let mount_points = vec![
      "/mnt/dev/pts",
      "/mnt/dev",
      "/mnt/proc",
      "/mnt/sys",
      "/mnt/boot",
    ];

    for mount_point in mount_points {
      // Try normal unmount first
      match exec("umount", &[mount_point]) {
        Ok(status) if status.success() => {
          log::debug!("Successfully unmounted: {mount_point}");
        }
        _ => {
          // Try lazy unmount if normal unmount fails
          log::debug!("Trying lazy unmount for: {mount_point}");
          match exec("umount", &["-l", mount_point]) {
            Ok(status) if status.success() => {
              log::debug!("Successfully lazy unmounted: {mount_point}");
            }
            _ => {
              log::debug!("Failed to unmount: {mount_point} (may not be mounted)");
            }
          }
        }
      }

      // Small delay between unmount attempts
      thread::sleep(Duration::from_millis(200));
    }
  }
}

pub fn read_config(config_path: &PathBuf) -> Result<(), Box<dyn std::error::Error>> {
  let config = Config::from_file(config_path)?;

  config.setup_partitions()?;

  config.install_base_system()?;

  config.setup_bootloader()?;

  config.configure_locale();

  config.setup_networking();

  config.install_desktop()?;

  config.create_users()?;

  config.finalize_installation()?;

  log::info!("Installation completed successfully! You may reboot now.");
  Ok(())
}
