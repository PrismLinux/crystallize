use log::warn;
use std::path::PathBuf;

use crate::{
  system::{
    exec::{exec, exec_chroot},
    files::{self, append_file, copy_dir, create_directory},
    install,
  },
  utils::{crash, exec_eval, files_eval},
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
    "btop",
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

  exec_eval(
    exec_chroot(
      "systemctl",
      vec![String::from("enable"), String::from("bluetooth")],
    ),
    "Enable bluetooth",
  );
}

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

pub fn install_bootloader_efi(efidir: PathBuf) {
  install::install(vec![
    "prismlinux/grub",
    "efibootmgr",
    "prismlinux-themes-grub",
    "os-prober",
  ]);
  let efidir = std::path::Path::new("/mnt").join(efidir);
  let efi_str = efidir.to_str().unwrap();
  if !std::path::Path::new(&format!("/mnt{efi_str}")).exists() {
    crash(format!("The efidir {efidir:?} doesn't exist"), 1);
  }
  exec_eval(
    exec_chroot(
      "grub-install",
      vec![
        String::from("--target=x86_64-efi"),
        format!("--efi-directory={}", efi_str),
        String::from("--bootloader-id=prismlinux"),
        String::from("--removable"),
      ],
    ),
    "install grub as efi with --removable",
  );

  files_eval(
    append_file(
      "/mnt/etc/default/grub",
      "GRUB_THEME=\"/usr/share/grub/themes/prismlinux/theme.txt\"",
    ),
    "enable prismlinux grub theme",
  );

  exec_eval(
    exec_chroot(
      "grub-mkconfig",
      vec![String::from("-o"), String::from("/boot/grub/grub.cfg")],
    ),
    "create grub.cfg",
  );
}

pub fn install_bootloader_legacy(device: PathBuf) {
  install::install(vec![
    "prismlinux/grub",
    "prismlinux-themes-grub",
    "os-prober",
  ]);
  if !device.exists() {
    crash(format!("The device {device:?} does not exist"), 1);
  }
  let device = device.to_string_lossy().to_string();
  exec_eval(
    exec_chroot(
      "grub-install",
      vec![String::from("--target=i386-pc"), device],
    ),
    "install grub as legacy",
  );

  files_eval(
    append_file(
      "/mnt/etc/default/grub",
      "GRUB_THEME=\"/usr/share/grub/themes/prismlinux/theme.txt\"",
    ),
    "enable prismlinux grub theme",
  );

  exec_eval(
    exec_chroot(
      "grub-mkconfig",
      vec![String::from("-o"), String::from("/boot/grub/grub.cfg")],
    ),
    "create grub.cfg",
  );
}

pub fn copy_live_config() {
  let _ = copy_dir(
    "/etc/NetworkManager/system-connections/",
    "/mnt/etc/NetworkManager/system-connections/",
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

pub fn install_nvidia() {
  install::install(vec![
    "dkms",
    "nvidia",
    "nvidia-dkms",
    "nvidia-utils",
    "egl-wayland",
  ]);

  // Apply nvidia module in grub
  let grub_cmdline_content = std::fs::read_to_string("/mnt/etc/default/grub").unwrap_or_default();
  let mut grub_conf_found = false;
  let mut lines: Vec<String> = grub_cmdline_content
    .lines()
    .map(|line| {
      if line.starts_with("GRUB_CMDLINE_LINUX_DEFAULT=") {
        grub_conf_found = true;
        if line.contains("nvidia-drm.modeset=1") {
          line.to_string() // Already there, do nothing
        } else {
          line.replace(
            "GRUB_CMDLINE_LINUX_DEFAULT=\"",
            "GRUB_CMDLINE_LINUX_DEFAULT=\"nvidia-drm.modeset=1 ",
          )
        }
      } else {
        line.to_string()
      }
    })
    .collect();
  if !grub_conf_found {
    lines.push("GRUB_CMDLINE_LINUX_DEFAULT=\"nvidia-drm.modeset=1\"".to_string());
  }
  let new_grub_content = lines.join("\n");
  std::fs::write("/mnt/etc/default/grub", new_grub_content).unwrap();

  // Apply initcpio modules
  let mkinitcpio_content = std::fs::read_to_string("/mnt/etc/mkinitcpio.conf").unwrap_or_default();
  let mut mkinitcpio_conf_found = false;
  let mapped_lines: Vec<String> = mkinitcpio_content
    .lines()
    .map(|line| {
      if line.trim_start().starts_with("MODULES=") && !line.trim_start().starts_with("#") {
        mkinitcpio_conf_found = true;
        "MODULES=(nvidia nvidia_modeset nvidia_uvm nvidia_drm)".to_string()
      } else {
        line.to_string()
      }
    })
    .collect();

  let mut final_lines = mapped_lines;
  if !mkinitcpio_conf_found {
    final_lines.push("MODULES=(nvidia nvidia_modeset nvidia_uvm nvidia_drm)".to_string());
  }
  let new_initcpio_content = final_lines.join("\n");
  std::fs::write("/mnt/etc/mkinitcpio.conf", new_initcpio_content).unwrap();
}

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
