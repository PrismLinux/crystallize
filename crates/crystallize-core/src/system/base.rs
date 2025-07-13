use crate::system::command::{exec, exec_chroot};
use crate::system::install::install;
use crate::system::{fs, install};
use crate::utils::error::crash;
use crate::utils::eval::exec_eval;
use log::warn;
use std::path::PathBuf;

pub fn install_base_packages(kernel: String) {
  std::fs::create_dir_all("/mnt/etc").unwrap();
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
    "grep",
    // Base PrismLinux
    "about",
    "prismlinux-themes-fish",
    // Fonts
    "noto-fonts",
    "noto-fonts-cjk",
    "noto-fonts-extra",
    "ttf-nerd-fonts-symbols-common",
    // Common packages for all desktops
    "pipewire",
    "pipewire-pulse",
    "pipewire-alsa",
    // "pipewire-jack",
    "wireplumber",
    "tuned-ppd",
    "cups",
    "cups-pdf",
    "bluez",
    "bluez-cups",
    "ttf-liberation",
    "dnsmasq",
    "xdg-user-dirs",
    "firewalld",
    "librewolf",
    "bash",
    "bash-completion",
    "inxi",
    "acpi",
    "btop",
    "fwupd",
    "ntp",
    "packagekit",
    "unzip",
    // Graphic drivers
    "xf86-video-amdgpu",
    "xf86-video-intel",
    "xf86-video-nouveau",
    // "xf86-video-vmware",
    "xf86-video-vesa",
    "mesa",
    "vulkan-intel",
    "vulkan-radeon",
    "vulkan-icd-loader",
    // "virtualbox-guest-utils",
    // Chaotic-AUR
    "archlinux-keyring",
    "chaotic-keyring",
    "chaotic-mirrorlist",
    // ArchLinux CN
    "archlinuxcn-keyring",
  ]);
  fs::copy_file("/etc/pacman.conf", "/mnt/etc/pacman.conf");

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
      vec![String::from("enable"), String::from("firewalld")],
    ),
    "Enable firewalld",
  );

  exec_eval(
    exec_chroot(
      "systemctl",
      vec![String::from("enable"), String::from("tuned-ppd")],
    ),
    "Enable tuned-ppd power manager",
  );
  exec_eval(
    exec_chroot(
      "systemctl",
      vec![String::from("enable"), String::from("cups")],
    ),
    "Enable CUPS",
  );
}

pub fn setup_archlinux_keyring() {
  exec_eval(
    exec_chroot("pacman-key", vec![String::from("--init")]),
    "Initialize pacman keyring",
  );
  exec_eval(
    exec_chroot(
      "pacman-key",
      vec![String::from("--populate"), String::from("archlinux")],
    ),
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
  install::install(vec!["grub", "efibootmgr", "os-prober"]);
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
  exec_eval(
    exec_chroot(
      "grub-mkconfig",
      vec![String::from("-o"), String::from("/boot/grub/grub.cfg")],
    ),
    "create grub.cfg",
  );
}

pub fn install_bootloader_legacy(device: PathBuf) {
  install::install(vec!["grub", "os-prober"]);
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
  exec_eval(
    exec_chroot(
      "grub-mkconfig",
      vec![String::from("-o"), String::from("/boot/grub/grub.cfg")],
    ),
    "create grub.cfg",
  );
}

pub fn copy_live_config() {
  fs::copy_file("/etc/pacman.conf", "/mnt/etc/pacman.conf");
  fs::copy_file("/etc/prismlinux-version", "/mnt/etc/prismlinux-version");
  // std::fs::create_dir_all("/mnt/etc/sddm.conf.d").unwrap();
  // fs::copy_file(
  //   "/etc/sddm.conf.d/settings.conf",
  //   "/mnt/etc/sddm.conf.d/settings.conf",
  // );
  // fs::copy_file("/etc/sddm.conf", "/mnt/etc/sddm.conf");
  // fs::copy_file("/etc/mkinitcpio.conf", "/mnt/etc/mkinitcpio.conf");
}

pub fn install_nvidia() {
  install(vec![
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

pub fn enable_swap(size: u64) {
  let size_mb = size.to_string();
  exec_eval(
    exec(
      "fallocate",
      vec![
        String::from("-l"),
        format!("{}M", size_mb),
        String::from("/mnt/swapfile"),
      ],
    ),
    "Create swapfile",
  );
  exec_eval(
    exec(
      "chmod",
      vec![String::from("600"), String::from("/mnt/swapfile")],
    ),
    "Set swapfile permissions",
  );
  exec_eval(
    exec("mkswap", vec![String::from("/mnt/swapfile")]),
    "Format swapfile",
  );
  std::fs::write("/mnt/etc/fstab", "\n/swapfile none swap defaults 0 0\n").unwrap();
}
