use log::warn;

pub mod grub;
pub mod nvidia;

use crate::utils::{
  exec::{exec, exec_chroot},
  exec_eval,
  files::{self, copy_dir, create_directory},
  files_eval, install,
};

pub fn install_base_packages(kernel: String) {
  create_directory("/mnt/etc").unwrap();
  let kernel_to_install = if kernel.is_empty() {
    "linux-zen"
  } else {
    match kernel.as_str() {
      "linux-zen" => "linux-zen",
      "linux-lts" => "linux-lts",
      "linux-hardened" => "linux-hardened",
      _ => {
        warn!("Unknown kernel: {kernel}, using default instead");
        "linux-zen"
      }
    }
  };
  install::install(vec![
    // Base Arch
    "base",
    kernel_to_install,
    format!("{kernel_to_install}-headers").as_str(),
    "linux-firmware",
    "sof-firmware",
    "man-db",
    "man-pages",
    "nano",
    "sudo",
    "curl",
    "wget",
    "openssh",
    "iptables",
    // Base Prism
    "about",
    "prism",
    "prismlinux-mirrorlist",
    "prismlinux-hooks",
    "prismlinux-themes-fish",
    // Extra goodies
    "fastfetch",
    "base-devel",
    // Fonts
    "noto-fonts",
    "noto-fonts-emoji",
    "noto-fonts-cjk",
    "noto-fonts-extra",
    "ttf-nerd-fonts-symbols-common",
    "wireplumber",
    "ttf-liberation",
    "dnsmasq",
    "bash",
    "glibc-locales",
    "bash-completion",
    "inxi",
    "acpi",
    "fwupd",
    "ntp",
    "unzip",
    "packagekit",
    // Graphic drivers
    "xf86-video-amdgpu",
    "xf86-video-intel",
    "xf86-video-nouveau",
    "xf86-video-vesa",
    "mesa",
    "vulkan-intel",
    "vulkan-radeon",
    "vulkan-icd-loader",
    // Repository
    "archlinux-keyring",
    "archlinuxcn-keyring",
    "chaotic-keyring",
    "chaotic-mirrorlist",
  ]);
  files::copy_file("/etc/pacman.conf", "/mnt/etc/pacman.conf");
}

/// Update mirror keyring
pub fn setup_archlinux_keyring() {
  exec_eval(
    exec_chroot("pacman-key", vec![String::from("--init")]),
    "Initialize pacman keyring",
  );
  exec_eval(
    exec_chroot("pacman-key", vec![String::from("--populate")]),
    "Populate pacman keyring",
  );
}

pub fn genfstab() {
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

/// Copy configuration from LiveISO to System
pub fn copy_live_config() {
  let _ = copy_dir(
    "/etc/NetworkManager/system-connections/",
    "/mnt/etc/NetworkManager/system-connections/",
  );
}

/// Using Zram over standard Swap
pub fn install_zram(size: u64) {
  let size_mb = &*size.to_string();
  install::install(vec!["zram-generator"]);
  files::create_file("/mnt/etc/systemd/zram-generator.conf");

  exec_eval(
    exec_chroot(
      "echo 1 >",
      vec![String::from("/sys/module/zswap/parameters/enabled")],
    ),
    "enable zram",
  );

  match size_mb {
    "0" => {
      files_eval(
        files::append_file(
          "/mnt/etc/systemd/zram-generator.conf",
          "[zram0]\nzram-size = min(ram / 2, 4096)\ncompression-algorithm = zstd",
        ),
        "Write zram-generator config",
      );
    }
    _ => {
      files_eval(
        files::append_file(
          "/mnt/etc/systemd/zram-generator.conf",
          &format!("[zram0]\nzram-size = {size_mb}\ncompression-algorithm = zstd").to_string(),
        ),
        "Write zram-generator config",
      );
    }
  };
}

pub fn install_homemgr() {
  install::install(vec!["nix"]);
}

pub fn install_flatpak() {
  install::install(vec!["flatpak"]);
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
  )
}
