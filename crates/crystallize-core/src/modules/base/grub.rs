use std::path::{Path, PathBuf};

use crate::utils::{
  crash,
  exec::exec_chroot,
  exec_eval,
  files::sed_file,
  files_eval,
  install::{self, InstallError},
};

const GRUB_PACKAGES: &[&str] = &[
  "prismlinux/grub",
  "efibootmgr",
  "prismlinux-themes-grub",
  "os-prober",
];

const GRUB_LEGACY_PACKAGES: &[&str] = &["prismlinux/grub", "prismlinux-themes-grub", "os-prober"];

const GRUB_THEME_CONFIG: &str = "GRUB_THEME=\"/usr/share/grub/themes/prismlinux/theme.txt\"";
const GRUB_CONFIG_PATH: &str = "/boot/grub/grub.cfg";
const GRUB_DEFAULT_CONFIG: &str = "/mnt/etc/default/grub";

/// Apply GRUB theme and generate config
fn configure_grub_theme_and_config() {
  files_eval(
    sed_file(
      GRUB_DEFAULT_CONFIG,
      "#GRUB_THEME=\"/path/to/theme.txt\"",
      GRUB_THEME_CONFIG,
    ),
    "Enable Grub Theme",
  );

  exec_eval(
    exec_chroot("grub-mkconfig", &["-o", GRUB_CONFIG_PATH]),
    "Create grub.cfg",
  );
}

/// Validate that the EFI directory exists
fn validate_efi_directory(efi_path: &str) {
  if !std::path::Path::new(&format!("/mnt{efi_path}")).exists() {
    crash(format!("The efidir {efi_path} doesn't exist"), 1);
  }
}

/// Install main GRUB EFI bootloader with proper boot entry
fn install_main_efi_bootloader(efi_str: &str) {
  exec_eval(
    exec_chroot(
      "grub-install",
      &[
        "--target=x86_64-efi",
        &format!("--efi-directory={efi_str}"),
        "--bootloader-id=PrismLinux",
        "--recheck",
      ],
    ),
    "install grub as efi with proper boot entry",
  );
}

/// Install fallback EFI bootloader for compatibility
fn install_fallback_efi_bootloader(efi_str: &str) {
  exec_eval(
    exec_chroot(
      "grub-install",
      &[
        "--target=x86_64-efi",
        &format!("--efi-directory={efi_str}"),
        "--bootloader-id=PrismLinux-fallback",
        "--removable",
        "--recheck",
      ],
    ),
    "install grub as fallback efi bootloader",
  );
}

/// Set default bootentry
fn set_default_boot_entry() {
  exec_eval(
    exec_chroot(
      "sh",
      &[
        "-c",
        "efibootmgr | grep 'PrismLinux' | head -1 | cut -c5-8 | xargs -I {} efibootmgr --bootorder {}",
      ],
    ),
    "Set default boot entry",
  );
}

/// Install Legacy GRUB bootloader
fn install_legacy_grub(device: &str) {
  exec_eval(
    exec_chroot("grub-install", &["--target=i386-pc", "--recheck", device]),
    "install grub as legacy",
  );
}

pub fn install_bootloader_efi(efidir: &PathBuf) -> Result<(), Box<dyn std::error::Error>> {
  // Install required packages
  install::install(GRUB_PACKAGES)?;

  // Prepare EFI directory path
  let efidir = std::path::Path::new("/mnt").join(efidir);
  let efi_str = efidir.to_str().ok_or_else(|| {
    InstallError::IoError("EFI directory path contains invalid UTF-8 characters".to_string())
  })?;

  // Validate EFI directory exists
  validate_efi_directory(efi_str);

  // Install main GRUB EFI bootloader
  install_main_efi_bootloader(efi_str);

  // Install fallback bootloader for compatibility
  install_fallback_efi_bootloader(efi_str);

  // Configure theme and generate GRUB config
  configure_grub_theme_and_config();

  // Set default boot entry
  set_default_boot_entry();
  Ok(())
}

pub fn install_bootloader_legacy(device: &Path) -> Result<(), Box<dyn std::error::Error>> {
  // Install required packages
  install::install(GRUB_LEGACY_PACKAGES)?;

  // Validate device exists
  if !device.exists() {
    crash(format!("The device {} does not exist", device.display()), 1);
  }

  let device_str = device.to_string_lossy();

  // Install Legacy GRUB
  install_legacy_grub(&device_str);

  // Configure theme and generate GRUB config
  configure_grub_theme_and_config();
  Ok(())
}
