use log::warn;

pub mod grub;
pub mod nvidia;

use crate::utils::{
  exec::{exec, exec_chroot},
  exec_eval,
  files::{self, create_directory},
  files_eval, install,
};

const SUPPORTED_KERNELS: &[&str] = &["linux-zen", "linux-lts", "linux-hardened"];

const BASE_PACKAGES: &[&str] = &[
  // Base Arch
  "base",
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
  "lsb-release",
  // Base Prism
  "about",
  "prism",
  "prismlinux-mirrorlist",
  "prismlinux-hooks",
  "prismlinux-themes-fish",
  // Extras
  "fastfetch",
  "base-devel",
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
  // Fonts
  "noto-fonts",
  "noto-fonts-emoji",
  "noto-fonts-cjk",
  "noto-fonts-extra",
  "ttf-nerd-fonts-symbols-common",
  // Graphics
  "xf86-video-amdgpu",
  "xf86-video-intel",
  "xf86-video-nouveau",
  "xf86-video-vesa",
  "mesa",
  "vulkan-intel",
  "vulkan-radeon",
  "vulkan-icd-loader",
  // Repositories
  "archlinux-keyring",
  "archlinuxcn-keyring",
  "chaotic-keyring",
  "chaotic-mirrorlist",
];

pub fn install_base_packages(kernel: String) {
  create_directory("/mnt/etc").unwrap();

  let kernel_pkg = match kernel.as_str() {
    "" => "linux-zen",
    k if SUPPORTED_KERNELS.contains(&k) => k,
    k => {
      warn!("Unknown kernel: {k}, using linux-zen instead");
      "linux-zen"
    }
  };

  let headers = format!("{kernel_pkg}-headers");
  let mut packages = Vec::with_capacity(BASE_PACKAGES.len() + 2);
  packages.extend_from_slice(BASE_PACKAGES);
  packages.push(kernel_pkg);
  packages.push(&headers);

  install::install(packages);
  files::copy_file("/etc/pacman.conf", "/mnt/etc/pacman.conf");
}

/// Update mirror keyring
pub fn setup_archlinux_keyring() -> Result<(), Box<dyn std::error::Error>> {
  let keyring_steps = [
    ("--init", "Initialize pacman keyring"),
    ("--refresh", "Refresh pacman keyring"),
    ("--populate", "Populate pacman keyring"),
  ];

  for (arg, description) in keyring_steps {
    match exec_chroot("pacman-key", vec![arg.to_string()]) {
      Ok(status) if status.success() => {
        println!("✓ {description}");
      }
      Ok(status) => {
        eprintln!(
          "✗ {} failed with exit code: {:?}",
          description,
          status.code()
        );
        return Err(format!("Failed to {}", description.to_lowercase()).into());
      }
      Err(e) => {
        eprintln!("✗ {description} failed: {e}");
        return Err(e.into());
      }
    }
  }
  Ok(())
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
pub fn copy_live_config() {}

/// Using Zram over standard Swap
pub fn install_zram(size: u64) {
  install::install(vec!["zram-generator"]);
  files::create_file("/mnt/etc/systemd/zram-generator.conf");

  exec_eval(
    exec_chroot(
      "echo 1 >",
      vec![String::from("/sys/module/zswap/parameters/enabled")],
    ),
    "enable zram",
  );

  let zram_config = if size == 0 {
    "[zram0]\nzram-size = min(ram / 2, 4096)\ncompression-algorithm = zstd"
  } else {
    &format!("[zram0]\nzram-size = {size}\ncompression-algorithm = zstd")
  };

  files_eval(
    files::append_file("/mnt/etc/systemd/zram-generator.conf", zram_config),
    "Write zram-generator config",
  );
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
