use std::path::PathBuf;

use crate::utils::{crash, exec::exec_chroot, exec_eval, files::append_file, files_eval, install};

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
