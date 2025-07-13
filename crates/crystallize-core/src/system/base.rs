use log::warn;
use std::path::PathBuf;

use crate::{
  system::{
    exec::{exec, exec_chroot},
    files::{self, append_file},
    install,
  },
  utils::{crash, exec_eval, files_eval},
};

pub fn install_base_packages(kernel: String) {
  std::fs::create_dir_all("/mnt/etc").unwrap();
  let kernel_to_install = if kernel.is_empty() {
    "linux"
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
    // Base Prism
    "about",
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
    // Common packages for all desktops
    "wireplumber",
    "cups",
    "cups-pdf",
    "bluez",
    "bluez-cups",
    "bash-completion",
    "ttf-liberation",
    "dnsmasq",
    // Repository
    "archlinux-keyring",
    "chaotic-keyring",
    "chaotic-mirrorlist",
    // ArchLinux CN
    "archlinuxcn-keyring",
  ]);
  files::copy_file("/etc/pacman.conf", "/mnt/etc/pacman.conf");

  exec_eval(
    exec_chroot(
      "systemctl",
      vec![String::from("enable"), String::from("bluetooth")],
    ),
    "Enable bluetooth",
  );

  exec_eval(
    exec_chroot(
      "systemctl",
      vec![String::from("enable"), String::from("cups")],
    ),
    "Enable CUPS",
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
    "grub",
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
  exec_eval(
    exec_chroot(
      "grub-install",
      vec![
        String::from("--target=x86_64-efi"),
        format!("--efi-directory={}", efi_str),
        String::from("--bootloader-id=prismlinux"),
      ],
    ),
    "install grub as efi without --removable",
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
  install::install(vec!["grub", "prismlinux-themes-grub", "os-prober"]);
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

pub fn setup_timeshift() {
  install::install(vec!["timeshift", "timeshift-autosnap", "grub-btrfs"]);
  exec_eval(
    exec_chroot("timeshift", vec![String::from("--btrfs")]),
    "setup timeshift",
  )
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

pub fn install_zram() {
  install::install(vec!["zram-generator"]);
  files::create_file("/mnt/etc/systemd/zram-generator.conf");
  files_eval(
    files::append_file("/mnt/etc/systemd/zram-generator.conf", "[zram0]"),
    "Write zram-generator config",
  );
}
