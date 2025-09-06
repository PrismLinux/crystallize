use log;
use std::path::{Path, PathBuf};

use crate::{
  cli::{self, PartitionMode},
  utils::{
    crash,
    exec::exec,
    exec_eval,
    files::{self, create_directory},
    files_eval,
  },
};

const BOOT_SIZE: &str = "513MiB";
const BOOT_START: &str = "1MiB";

/// Supported filesystem types
#[derive(Debug, Clone, PartialEq)]
enum FilesystemType {
  Ext4,
  Fat32,
  Btrfs,
  Xfs,
  NoFormat,
}

impl FilesystemType {
  /// Parse filesystem type from string
  fn from_str(filesystem: &str) -> Option<Self> {
    match filesystem.to_lowercase().as_str() {
      "ext4" => Some(Self::Ext4),
      "fat32" => Some(Self::Fat32),
      "btrfs" => Some(Self::Btrfs),
      "xfs" => Some(Self::Xfs),
      "noformat" | "don't format" => Some(Self::NoFormat),
      _ => None,
    }
  }

  /// Get mkfs command for the filesystem
  fn command(&self) -> &'static str {
    match self {
      Self::Ext4 => "mkfs.ext4",
      Self::Fat32 => "mkfs.fat",
      Self::Btrfs => "mkfs.btrfs",
      Self::Xfs => "mkfs.xfs",
      Self::NoFormat => unreachable!("NoFormat should not call command()"),
    }
  }

  /// Get mkfs arguments for the filesystem
  fn args(&self) -> Vec<&'static str> {
    match self {
      Self::Ext4 => vec!["-F"],
      Self::Fat32 => vec!["-F32"],
      Self::Btrfs => vec!["-f"],
      Self::Xfs => vec!["-f"],
      Self::NoFormat => unreachable!("NoFormat should not call args()"),
    }
  }

  /// Check if filesystem requires formatting
  fn needs_formatting(&self) -> bool {
    !matches!(self, Self::NoFormat)
  }

  /// Get display name for logging
  fn display_name(&self) -> &'static str {
    match self {
      Self::Ext4 => "ext4",
      Self::Fat32 => "fat32",
      Self::Btrfs => "btrfs",
      Self::Xfs => "xfs",
      Self::NoFormat => "noformat",
    }
  }
}

/// Device and partition number parser
struct DeviceParser;

impl DeviceParser {
  /// Parse block device into device path and partition number
  fn parse(blockdevice: &str) -> (String, String) {
    if blockdevice.contains("nvme") || blockdevice.contains("mmcblk") {
      Self::parse_nvme_mmc(blockdevice)
    } else {
      Self::parse_regular(blockdevice)
    }
  }

  /// Check if device uses NVMe or MMC naming convention
  fn is_nvme_or_mmc(device_str: &str) -> bool {
    device_str.contains("nvme") || device_str.contains("mmcblk")
  }

  /// Generate partition names for a device
  fn get_partition_names(device: &Path, partition_nums: &[u8]) -> Vec<String> {
    let device_str = device.to_string_lossy();
    let is_nvme_mmc = Self::is_nvme_or_mmc(&device_str);

    partition_nums
      .iter()
      .map(|&num| {
        if is_nvme_mmc {
          format!("{device_str}p{num}")
        } else {
          format!("{device_str}{num}")
        }
      })
      .collect()
  }

  fn parse_nvme_mmc(blockdevice: &str) -> (String, String) {
    if let Some(p_pos) = blockdevice.rfind('p') {
      let device_part = &blockdevice[..p_pos];
      let partition_part = &blockdevice[p_pos + 1..];
      (device_part.to_string(), partition_part.to_string())
    } else {
      log::warn!("Could not parse NVMe/MMC device partition: {blockdevice}");
      (blockdevice.to_string(), "1".to_string())
    }
  }

  fn parse_regular(blockdevice: &str) -> (String, String) {
    let mut device_part = blockdevice.to_string();
    let partition_part;

    // Find the last digit sequence
    let chars: Vec<char> = blockdevice.chars().collect();
    let mut digit_start = chars.len();

    // Find where the digits start (from the end)
    for (i, &c) in chars.iter().enumerate().rev() {
      if c.is_ascii_digit() {
        digit_start = i;
      } else {
        break;
      }
    }

    if digit_start < chars.len() {
      device_part = chars[..digit_start].iter().collect();
      partition_part = chars[digit_start..].iter().collect();
    } else {
      // No digits found, assume it's just the device
      log::warn!("No partition number found in {blockdevice}, assuming partition 1");
      partition_part = "1".to_string();
    }

    if partition_part.is_empty() {
      log::warn!("Empty partition number for {blockdevice}, assuming partition 1");
      (device_part, "1".to_string())
    } else {
      (device_part, partition_part)
    }
  }
}

/// Boot flag management
struct BootFlags;

impl BootFlags {
  /// Set appropriate boot flags for the boot partition
  fn set(blockdevice: &str, efi: bool) {
    let (device, partition_num) = DeviceParser::parse(blockdevice);

    log::debug!("Setting boot flags for device: {device}, partition: {partition_num}");

    // Validate that we have a valid partition number
    if partition_num.is_empty() || !partition_num.chars().all(|c| c.is_ascii_digit()) {
      log::error!("Invalid partition number '{partition_num}' for device {device}");
      crash(
        format!(
          "Cannot set boot flags: invalid partition number '{partition_num}' for {blockdevice}"
        ),
        1,
      );
    }

    let flag = if efi { "esp" } else { "boot" };
    let description = if efi {
      "set ESP flag on boot partition"
    } else {
      "set boot flag on boot partition"
    };

    log::info!(
      "Setting '{flag}' flag for {} boot on partition {partition_num} of device {device}",
      if efi { "UEFI" } else { "BIOS" }
    );

    // Use the base device (without partition number) for parted
    exec_eval(
      exec(
        "parted",
        vec![
          String::from("-s"),
          device,
          String::from("set"),
          partition_num,
          String::from(flag),
          String::from("on"),
        ],
      ),
      description,
    );

    log::info!("Boot flag '{flag}' successfully set on {blockdevice}");
  }

  /// Set boot flags for auto-partitioned devices
  fn set_auto_flags(device: &Path, efi: bool) {
    let partitions = DeviceParser::get_partition_names(device, &[1]);
    let boot_partition = &partitions[0];

    log::info!("Setting boot flags on auto-created boot partition: {boot_partition}");
    Self::set(boot_partition, efi);
  }

  /// Safely attempt to set boot flags, with error handling for invalid partitions
  fn try_set(blockdevice: &str, efi: bool) -> bool {
    let (device, partition_num) = DeviceParser::parse(blockdevice);

    // Validate partition number
    if partition_num.is_empty() || !partition_num.chars().all(|c| c.is_ascii_digit()) {
      log::warn!("Cannot set boot flags on {blockdevice}: invalid partition format");
      return false;
    }

    // Check if device exists
    if !std::path::Path::new(&device).exists() {
      log::warn!("Cannot set boot flags on {blockdevice}: device {device} does not exist");
      return false;
    }

    let flag = if efi { "esp" } else { "boot" };

    log::info!(
      "Setting '{flag}' flag on {blockdevice} (device: {device}, partition: {partition_num})"
    );

    match exec(
      "parted",
      vec![
        String::from("-s"),
        device,
        String::from("set"),
        partition_num,
        String::from(flag),
        String::from("on"),
      ],
    ) {
      Ok(status) if status.success() => {
        log::info!("Successfully set '{flag}' flag on {blockdevice}");
        true
      }
      Ok(_) => {
        log::warn!("Failed to set '{flag}' flag on {blockdevice}: parted command failed");
        false
      }
      Err(e) => {
        log::warn!("Failed to set '{flag}' flag on {blockdevice}: {e}");
        false
      }
    }
  }
}

/// Partition table creation and management
struct PartitionTable;

impl PartitionTable {
  /// Create appropriate partition table and partitions for the device
  fn create(device: &Path, efi: bool) {
    let device_str = device.to_string_lossy().to_string();

    // Ensure device is not mounted
    let _ = exec("umount", vec![String::from(&device_str)]);

    if efi {
      Self::create_gpt(&device_str);
    } else {
      Self::create_mbr(&device_str);
    }
  }

  fn create_gpt(device_str: &str) {
    log::info!("Creating GPT partition table for UEFI boot");

    // Create GPT partition table
    exec_eval(
      exec(
        "parted",
        vec![
          String::from("-s"),
          String::from(device_str),
          String::from("mklabel"),
          String::from("gpt"),
        ],
      ),
      "create GPT label",
    );

    // Create EFI System Partition (ESP)
    exec_eval(
      exec(
        "parted",
        vec![
          String::from("-s"),
          String::from(device_str),
          String::from("mkpart"),
          String::from("ESP"),
          String::from("fat32"),
          String::from(BOOT_START),
          String::from(BOOT_SIZE),
        ],
      ),
      "create EFI system partition",
    );

    // Create root partition
    exec_eval(
      exec(
        "parted",
        vec![
          String::from("-s"),
          String::from(device_str),
          String::from("mkpart"),
          String::from("root"),
          String::from("ext4"),
          String::from(BOOT_SIZE),
          String::from("100%"),
        ],
      ),
      "create root partition",
    );
  }

  fn create_mbr(device_str: &str) {
    log::info!("Creating MBR partition table for BIOS boot");

    // Create MBR partition table
    exec_eval(
      exec(
        "parted",
        vec![
          String::from("-s"),
          String::from(device_str),
          String::from("mklabel"),
          String::from("msdos"),
        ],
      ),
      "create MBR label",
    );

    // Create boot partition
    exec_eval(
      exec(
        "parted",
        vec![
          String::from("-s"),
          String::from(device_str),
          String::from("mkpart"),
          String::from("primary"),
          String::from("ext4"),
          String::from(BOOT_START),
          String::from(BOOT_SIZE),
        ],
      ),
      "create boot partition",
    );

    // Create root partition
    exec_eval(
      exec(
        "parted",
        vec![
          String::from("-s"),
          String::from(device_str),
          String::from("mkpart"),
          String::from("primary"),
          String::from("ext4"),
          String::from(BOOT_SIZE),
          String::from("100%"),
        ],
      ),
      "create root partition",
    );
  }
}

/// Mount point management
struct MountManager;

impl MountManager {
  /// Standard mount points to clean up during partitioning
  const CLEANUP_MOUNTS: &'static [&'static str] =
    &["/mnt/boot", "/mnt/dev", "/mnt/proc", "/mnt/sys", "/mnt"];

  /// Clean up any existing mounts before partitioning
  fn cleanup() {
    log::debug!("Cleaning up existing mount points");
    for mount_point in Self::CLEANUP_MOUNTS {
      let _ = exec(
        "umount",
        vec![String::from("-R"), String::from(*mount_point)],
      );
    }
  }

  /// Check if a path is currently mounted
  fn is_mounted(mountpoint: &str) -> bool {
    exec(
      "mountpoint",
      vec![String::from("-q"), String::from(mountpoint)],
    )
    .map(|status| status.success())
    .unwrap_or(false)
  }

  /// Ensure mount point directory exists
  fn ensure_mountpoint_exists(mountpoint: &str) {
    if !Path::new(mountpoint).exists() {
      log::debug!("Mount point {mountpoint} does not exist. Creating...");
      if let Err(e) = create_directory(mountpoint) {
        crash(format!("Failed to create mount point {mountpoint}: {e}"), 1);
      }
    }
  }

  /// Unmount if already mounted
  fn unmount_if_mounted(mountpoint: &str) {
    if Self::is_mounted(mountpoint) {
      log::warn!("Mountpoint {mountpoint} is already mounted, unmounting first");
      let _ = exec("umount", vec![String::from(mountpoint)]);
    }
  }
}

/// Filesystem formatting operations
struct FilesystemFormatter;

impl FilesystemFormatter {
  /// Format a partition with the specified filesystem
  fn format(blockdevice: &str, fs_type: &FilesystemType) {
    if !fs_type.needs_formatting() {
      log::debug!("Skipping formatting for {blockdevice} (noformat specified)");
      return;
    }

    log::info!("Formatting {blockdevice} as {}", fs_type.display_name());

    let mut args = fs_type
      .args()
      .iter()
      .map(|&s| String::from(s))
      .collect::<Vec<_>>();
    args.push(String::from(blockdevice));

    exec_eval(
      exec(fs_type.command(), args),
      &format!("format {blockdevice} as {}", fs_type.display_name()),
    );

    log::info!(
      "Successfully formatted {blockdevice} as {}",
      fs_type.display_name()
    );
  }

  /// Format partition based on EFI requirements
  fn format_auto_partition(partition: &str, is_boot: bool, efi: bool) {
    // Ensure partition is unmounted before formatting
    let _ = exec("umount", vec![String::from(partition)]);

    let fs_type = if is_boot {
      if efi {
        log::info!("Formatting UEFI boot partition {partition} as FAT32");
        FilesystemType::Fat32
      } else {
        log::info!("Formatting BIOS boot partition {partition} as ext4");
        FilesystemType::Ext4
      }
    } else {
      log::info!("Formatting root partition {partition} as ext4");
      FilesystemType::Ext4
    };

    Self::format(partition, &fs_type);
  }
}

// Public API functions
pub fn fmt_mount(mountpoint: &str, filesystem: &str, blockdevice: &str, efi: bool) {
  log::info!("Formatting and mounting {blockdevice} at {mountpoint} with filesystem {filesystem}");

  // Unmount if already mounted
  let _ = exec("umount", vec![String::from(blockdevice)]);

  // Parse filesystem type
  let fs_type = match FilesystemType::from_str(filesystem) {
    Some(fs) => fs,
    None => {
      crash(
        format!("Unknown filesystem {filesystem}, used in partition {blockdevice}"),
        1,
      );
    }
  };

  // Handle no-format case
  if !fs_type.needs_formatting() {
    log::info!("Skipping format for {blockdevice} (noformat specified)");
  } else {
    // Format the partition
    FilesystemFormatter::format(blockdevice, &fs_type);
  }

  // Ensure mount point exists
  MountManager::ensure_mountpoint_exists(mountpoint);

  // Mount the partition
  mount(blockdevice, mountpoint, "");

  // Set boot flags for boot-related mountpoints
  if mountpoint == "/boot" || mountpoint == "/mnt/boot" {
    log::info!("Attempting to set boot flags for boot partition {blockdevice}");
    if !BootFlags::try_set(blockdevice, efi) {
      log::warn!("Could not set boot flags on {blockdevice} - this may cause boot issues");
      log::warn!(
        "Please manually set the {} flag on this partition using parted",
        if efi { "esp" } else { "boot" }
      );
    }
  }
}

pub fn partition(
  device: PathBuf,
  mode: PartitionMode,
  efi: bool,
  partitions: &mut Vec<cli::Partition>,
) {
  log::info!("Starting partitioning process - Mode: {mode:?}, EFI: {efi}, Device: {device:?}");

  MountManager::cleanup();

  match mode {
    PartitionMode::Auto => {
      if !device.exists() {
        crash(format!("The device {device:?} doesn't exist"), 1);
      }
      log::info!("Automatically partitioning {device:?}");

      // Create partition table and partitions
      PartitionTable::create(&device, efi);

      // Set boot flags immediately after partition creation
      BootFlags::set_auto_flags(&device, efi);

      // Format and mount partitions
      format_and_mount_auto(&device, efi);

      log::info!("Auto partitioning completed successfully");
    }
    PartitionMode::Manual => {
      log::info!("Manual partitioning with {} partitions", partitions.len());

      // Sort partitions by mountpoint length to ensure proper mounting order
      partitions.sort_by(|a, b| a.mountpoint.len().cmp(&b.mountpoint.len()));

      for partition in partitions {
        log::debug!(
          "Processing partition: {} -> {} ({})",
          partition.blockdevice,
          partition.mountpoint,
          partition.filesystem
        );
        fmt_mount(
          &partition.mountpoint,
          &partition.filesystem,
          &partition.blockdevice,
          efi,
        );
      }

      log::info!("Manual partitioning completed successfully");
    }
  }
}

pub fn mount(partition: &str, mountpoint: &str, options: &str) {
  let log_message = if options.is_empty() {
    format!("Mounting {partition} at {mountpoint}")
  } else {
    format!("Mounting {partition} at {mountpoint} with options: {options}")
  };
  log::debug!("{log_message}");

  // Ensure the mountpoint exists
  MountManager::ensure_mountpoint_exists(mountpoint);

  // Unmount if already mounted
  MountManager::unmount_if_mounted(mountpoint);

  // Prepare mount command
  let mut mount_args = vec![String::from(partition), String::from(mountpoint)];

  if !options.is_empty() {
    mount_args.extend_from_slice(&[String::from("-o"), String::from(options)]);
  }

  let description = if options.is_empty() {
    format!("mount {partition} at {mountpoint}")
  } else {
    format!("mount {partition} with options {options} at {mountpoint}")
  };

  exec_eval(exec("mount", mount_args), &description);

  log::info!("Successfully mounted {partition} at {mountpoint}");
}

pub fn umount(mountpoint: &str) {
  log::info!("Unmounting {mountpoint}");
  exec_eval(
    exec("umount", vec![String::from(mountpoint)]),
    &format!("unmount {mountpoint}"),
  );
}

// Helper function for auto partitioning
fn format_and_mount_auto(device: &Path, efi: bool) {
  let partitions = DeviceParser::get_partition_names(device, &[1, 2]);
  let (boot_partition, root_partition) = (&partitions[0], &partitions[1]);

  log::info!("Auto partition layout - Boot: {boot_partition}, Root: {root_partition}");

  // Format partitions
  FilesystemFormatter::format_auto_partition(boot_partition, true, efi);
  FilesystemFormatter::format_auto_partition(root_partition, false, efi);

  // Mount root partition first
  log::info!("Mounting root partition");
  mount(root_partition, "/mnt", "");

  // Create boot directory and mount boot partition
  log::info!("Creating boot directory and mounting boot partition");
  files_eval(files::create_directory("/mnt/boot"), "create /mnt/boot");
  mount(boot_partition, "/mnt/boot", "");

  if efi {
    log::info!("UEFI setup complete - ESP (FAT32) mounted at /mnt/boot with ESP flag set");
  } else {
    log::info!(
      "BIOS setup complete - Boot partition (ext4) mounted at /mnt/boot with boot flag set"
    );
  }
}
