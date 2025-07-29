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

pub fn fmt_mount(mountpoint: &str, filesystem: &str, blockdevice: &str) {
  let _ = exec("umount", vec![String::from(blockdevice)]);

  let fs_command = match filesystem {
    "ext4" => (
      "mkfs.ext4",
      vec![String::from("-F"), String::from(blockdevice)],
    ),
    "fat32" => (
      "mkfs.fat",
      vec![String::from("-F32"), String::from(blockdevice)],
    ),
    "btrfs" => {
      log::info!("Installing btrfs-progs on host system");
      let install_result = exec(
        "pacman",
        vec![
          String::from("-S"),
          String::from("--noconfirm"),
          String::from("--needed"),
          String::from("btrfs-progs"),
        ],
      );

      exec_eval(install_result, "Install btrfs-progs on host system");
      (
        "mkfs.btrfs",
        vec![String::from("-f"), String::from(blockdevice)],
      )
    }
    "xfs" => {
      log::info!("Installing xfsprogs on host system");
      let install_result = exec(
        "pacman",
        vec![
          String::from("-S"),
          String::from("--noconfirm"),
          String::from("--needed"),
          String::from("xfsprogs"),
        ],
      );
      exec_eval(install_result, "Install xfsprogs on host system");
      (
        "mkfs.xfs",
        vec![String::from("-f"), String::from(blockdevice)],
      )
    }
    "noformat" | "don't format" => {
      log::debug!("Not formatting {blockdevice}");
      return;
    }
    _ => {
      crash(
        format!("Unknown filesystem {filesystem}, used in partition {blockdevice}"),
        1,
      );
    }
  };

  exec_eval(
    exec(fs_command.0, fs_command.1),
    format!("Formatting {blockdevice} as {filesystem}").as_str(),
  );

  exec_eval(
    exec("mkdir", vec![String::from("-p"), String::from(mountpoint)]),
    format!("Creating mountpoint {mountpoint} for {blockdevice}").as_str(),
  );
  mount(blockdevice, mountpoint, "");
}

pub fn partition(
  device: PathBuf,
  mode: PartitionMode,
  efi: bool,
  partitions: &mut Vec<cli::Partition>,
) {
  println!("{mode:?}");

  cleanup_mounts();

  match mode {
    PartitionMode::Auto => {
      if !device.exists() {
        crash(format!("The device {device:?} doesn't exist"), 1);
      }
      log::debug!("automatically partitioning {device:?}");
      partition_with_efi(&device);

      if device.to_string_lossy().contains("nvme") || device.to_string_lossy().contains("mmcblk") {
        part_nvme(&device, efi);
      } else {
        part_disk(&device, efi);
      }
    }
    PartitionMode::Manual => {
      log::debug!("Manual partitioning");
      partitions.sort_by(|a, b| a.mountpoint.len().cmp(&b.mountpoint.len()));
      for partition in partitions {
        fmt_mount(
          &partition.mountpoint,
          &partition.filesystem,
          &partition.blockdevice,
        );
        if &partition.mountpoint == "/boot/efi" {
          exec_eval(
            exec(
              "parted",
              vec![
                String::from("-s"),
                String::from(&partition.blockdevice),
                String::from("set"),
                String::from("1"),
                String::from("esp"),
                String::from("on"),
              ],
            ),
            "set EFI partition as ESP",
          );
        }
      }
    }
  }
}

fn cleanup_mounts() {
  let mount_points = vec![
    "/mnt/boot/efi",
    "/mnt/boot",
    "/mnt/dev",
    "/mnt/proc",
    "/mnt/sys",
    "/mnt",
  ];

  for mount_point in mount_points {
    let _ = exec("umount", vec![String::from(mount_point)]);
  }
}

fn partition_with_efi(device: &Path) {
  let device = device.to_string_lossy().to_string();

  // Ensure device is not mounted
  let _ = exec("umount", vec![String::from(&device)]);

  exec_eval(
    exec(
      "parted",
      vec![
        String::from("-s"),
        String::from(&device),
        String::from("mklabel"),
        String::from("gpt"),
      ],
    ),
    format!("create gpt label on {}", &device).as_str(),
  );
  exec_eval(
    exec(
      "parted",
      vec![
        String::from("-s"),
        String::from(&device),
        String::from("mkpart"),
        String::from("fat32"),
        String::from("0"),
        String::from("300"),
      ],
    ),
    "create EFI partition",
  );
  exec_eval(
    exec(
      "parted",
      vec![
        String::from("-s"),
        String::from(&device),
        String::from("set"),
        String::from("1"),
        String::from("esp"),
        String::from("on"),
      ],
    ),
    "set EFI partition as ESP",
  );
  exec_eval(
    exec(
      "parted",
      vec![
        String::from("-s"),
        device,
        String::from("mkpart"),
        String::from("primary"),
        String::from("ext4"),
        String::from("512MIB"),
        String::from("100%"),
      ],
    ),
    "create ext4 root partition",
  );
}

fn part_nvme(device: &Path, efi: bool) {
  let device = device.to_string_lossy().to_string();
  if efi {
    // Ensure partitions are unmounted before formatting
    let _ = exec("umount", vec![format!("{}p1", device)]);
    let _ = exec("umount", vec![format!("{}p2", device)]);

    exec_eval(
      exec(
        "mkfs.fat",
        vec![String::from("-F32"), format!("{}p1", device)],
      ),
      format!("format {device}p1 as fat32").as_str(),
    );
    exec_eval(
      exec(
        "mkfs.ext4",
        vec![String::from("-F"), format!("{}p2", device)],
      ),
      format!("format {device}p2 as ext4").as_str(),
    );
    mount(format!("{device}p2").as_str(), "/mnt", "");
    files_eval(files::create_directory("/mnt/boot"), "create /mnt/boot");
    files_eval(
      files::create_directory("/mnt/boot/efi"),
      "create /mnt/boot/efi",
    );
    mount(format!("{device}p1").as_str(), "/mnt/boot/efi", "");
    exec_eval(
      exec(
        "parted",
        vec![
          String::from("-s"),
          String::from(&device),
          String::from("set"),
          String::from("1"),
          String::from("esp"),
          String::from("on"),
        ],
      ),
      "set EFI partition as ESP",
    );
  } else if !efi {
    let _ = exec("umount", vec![format!("{}p1", device)]);
    let _ = exec("umount", vec![format!("{}p2", device)]);

    exec_eval(
      exec(
        "mkfs.ext4",
        vec![String::from("-F"), format!("{}p1", device)],
      ),
      format!("format {device}p1 as ext4").as_str(),
    );
    exec_eval(
      exec(
        "mkfs.ext4",
        vec![String::from("-F"), format!("{}p2", device)],
      ),
      format!("format {device}p2 as ext4").as_str(),
    );
    mount(format!("{device}p2").as_str(), "/mnt/", "");
    files_eval(files::create_directory("/mnt/boot"), "create /mnt/boot");
    mount(format!("{device}p1").as_str(), "/mnt/boot", "");
  } else {
    crash("NVMe devices must be partitioned with EFI", 1);
  }
}

fn part_disk(device: &Path, efi: bool) {
  let device = device.to_string_lossy().to_string();
  if efi {
    // Ensure partitions are unmounted before formatting
    let _ = exec("umount", vec![format!("{}1", device)]);
    let _ = exec("umount", vec![format!("{}2", device)]);

    exec_eval(
      exec(
        "mkfs.fat",
        vec![String::from("-F32"), format!("{}1", device)],
      ),
      format!("format {device}1 as fat32").as_str(),
    );
    exec_eval(
      exec(
        "mkfs.ext4",
        vec![String::from("-F"), format!("{}2", device)],
      ),
      format!("format {device}2 as ext4").as_str(),
    );
    mount(format!("{device}2").as_str(), "/mnt", "");
    files_eval(files::create_directory("/mnt/boot"), "create /mnt/boot");
    files_eval(
      files::create_directory("/mnt/boot/efi"),
      "create /mnt/boot/efi",
    );
    mount(format!("{device}1").as_str(), "/mnt/boot/efi", "");
    exec_eval(
      exec(
        "parted",
        vec![
          String::from("-s"),
          String::from(&device),
          String::from("set"),
          String::from("1"),
          String::from("esp"),
          String::from("on"),
        ],
      ),
      "set EFI partition as ESP",
    );
  } else if !efi {
    let _ = exec("umount", vec![format!("{}1", device)]);
    let _ = exec("umount", vec![format!("{}2", device)]);

    exec_eval(
      exec(
        "mkfs.ext4",
        vec![String::from("-F"), format!("{}1", device)],
      ),
      format!("format {device}1 as ext4").as_str(),
    );
    exec_eval(
      exec(
        "mkfs.ext4",
        vec![String::from("-F"), format!("{}2", device)],
      ),
      format!("format {device}2 as ext4").as_str(),
    );
    mount(format!("{device}2").as_str(), "/mnt/", "");
    files_eval(
      files::create_directory("/mnt/boot"),
      "create directory /mnt/boot",
    );
    mount(format!("{device}1").as_str(), "/mnt/boot", "");
  } else {
    crash("Disk devices must be partitioned with EFI", 1);
  }
}

pub fn mount(partition: &str, mountpoint: &str, options: &str) {
  // Ensure the mountpoint exists, create it if necessary
  if !Path::new(mountpoint).exists() {
    log::debug!("Mount point {mountpoint} does not exist. Creating...");
    if let Err(e) = create_directory(mountpoint) {
      crash(format!("Failed to create mount point {mountpoint}: {e}"), 1);
    }
  }

  // Proceed with mounting
  if !options.is_empty() {
    exec_eval(
      exec(
        "mount",
        vec![
          String::from(partition),
          String::from(mountpoint),
          String::from("-o"),
          String::from(options),
        ],
      ),
      format!("mount {partition} with options {options} at {mountpoint}").as_str(),
    );
  } else {
    exec_eval(
      exec(
        "mount",
        vec![String::from(partition), String::from(mountpoint)],
      ),
      format!("mount {partition} with no options at {mountpoint}").as_str(),
    );
  }
}

pub fn umount(mountpoint: &str) {
  exec_eval(
    exec("umount", vec![String::from(mountpoint)]),
    format!("unmount {mountpoint}").as_str(),
  );
}
