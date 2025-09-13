use crate::utils::install::{self, InstallError};

// --------- [ TODO ] ---------
// Need to try later: `https://github.com/Frogging-Family/nvidia-all`.
//
// Currently it is not in the Chaotic AUR repo. Might be better to add to our own repo.
// Also it supports more cards/drivers.
//
// But for now, this is sufficient for the newest NVIDIA cards.
// ----------------------------

pub fn install_nvidia() -> Result<(), Box<dyn std::error::Error>> {
  install::install(&[
    "dkms",
    "nvidia-open",
    "nvidia-open-dkms",
    "nvidia-utils",
    "egl-wayland",
  ])?;

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
  std::fs::write("/mnt/etc/default/grub", new_grub_content)
    .map_err(|e| InstallError::IoError(format!("Failed to write GRUB config: {e}")))?;

  // Apply initcpio modules
  let mkinitcpio_content = std::fs::read_to_string("/mnt/etc/mkinitcpio.conf").unwrap_or_default();
  let mut mkinitcpio_conf_found = false;
  let mapped_lines: Vec<String> = mkinitcpio_content
    .lines()
    .map(|line| {
      if line.trim_start().starts_with("MODULES=") && !line.trim_start().starts_with('#') {
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
  std::fs::write("/mnt/etc/mkinitcpio.conf", new_initcpio_content)
    .map_err(|e| InstallError::IoError(format!("Failed to write mkinitcpio config: {e}")))?;
  Ok(())
}
