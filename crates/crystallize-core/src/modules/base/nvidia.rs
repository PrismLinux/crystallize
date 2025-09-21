use crate::utils::install::{self, InstallError};
use std::process::Command;

#[derive(Debug)]
pub enum NvidiaDriver {
  Open,      // nvidia-open (>=RTX 20xx series)
  Legacy470, // nvidia-470xx
  Legacy390, // nvidia-390xx
  Legacy340, // nvidia-340xx
}

impl NvidiaDriver {
  #[must_use]
  pub fn packages(&self) -> Vec<&'static str> {
    match self {
      Self::Open => vec![
        "dkms",
        "nvidia-open",
        "nvidia-open-dkms",
        "nvidia-utils",
        "egl-wayland",
      ],
      Self::Legacy470 => vec![
        "dkms",
        "nvidia-470xx-dkms",
        "nvidia-470xx-utils",
        "egl-wayland",
      ],
      Self::Legacy390 => vec![
        "dkms",
        "nvidia-390xx-dkms",
        "nvidia-390xx-utils",
        "egl-wayland",
      ],
      Self::Legacy340 => vec!["dkms", "nvidia-340xx-dkms", "nvidia-340xx-utils"],
    }
  }
}

fn detect_nvidia_gpu() -> Result<Option<String>, Box<dyn std::error::Error>> {
  let output = Command::new("lspci")
    .args(["-nn"])
    .output()
    .map_err(|e| InstallError::IoError(format!("Failed to run lspci: {e}")))?;

  let output_str = String::from_utf8_lossy(&output.stdout);

  for line in output_str.lines() {
    if line.to_lowercase().contains("nvidia")
      && (line.to_lowercase().contains("vga") || line.to_lowercase().contains("3d"))
    {
      return Ok(Some(String::from(line)));
    }
  }

  Ok(None)
}

fn determine_driver_version(gpu_info: &str) -> NvidiaDriver {
  let gpu_lower = gpu_info.to_lowercase();

  // RTX 20xx series and newer - use open driver
  if gpu_lower.contains("rtx 20")
    || gpu_lower.contains("rtx 30")
    || gpu_lower.contains("rtx 40")
    || gpu_lower.contains("rtx 50")
    || gpu_lower.contains("gtx 16")
  {
    return NvidiaDriver::Open;
  }

  // GTX 10xx series and some RTX cards
  if gpu_lower.contains("gtx 10")
    || gpu_lower.contains("gtx 1050")
    || gpu_lower.contains("gtx 1060")
    || gpu_lower.contains("gtx 1070")
    || gpu_lower.contains("gtx 1080")
    || gpu_lower.contains("titan x")
  {
    return NvidiaDriver::Legacy470;
  }

  // GTX 9xx and older supported cards + GT series
  if gpu_lower.contains("gtx 9")
    || gpu_lower.contains("gtx 8")
    || gpu_lower.contains("gtx 7")
    || gpu_lower.contains("gtx 6")
    || gpu_lower.contains("gt 6")
    || gpu_lower.contains("gt 7")
    || gpu_lower.contains("gt 730")
    || gpu_lower.contains("gt 740")
    || gpu_lower.contains("gt 710")
    || gpu_lower.contains("gt 720")
    || gpu_lower.contains("quadro")
  {
    return NvidiaDriver::Legacy390;
  }

  // Very old cards
  if gpu_lower.contains("gtx 4")
    || gpu_lower.contains("gtx 5")
    || gpu_lower.contains("gt 4")
    || gpu_lower.contains("gt 5")
    || gpu_lower.contains("geforce 8")
    || gpu_lower.contains("geforce 9")
  {
    return NvidiaDriver::Legacy340;
  }

  // Default to open driver for newer unknown cards
  NvidiaDriver::Open
}

pub fn install_nvidia() -> Result<(), Box<dyn std::error::Error>> {
  println!("Detecting NVIDIA GPU...");

  let gpu_info = detect_nvidia_gpu()?
    .ok_or_else(|| InstallError::IoError(String::from("No NVIDIA GPU detected")))?;

  println!("Detected NVIDIA GPU: {gpu_info}");

  let driver = determine_driver_version(&gpu_info);
  let packages = driver.packages();

  println!("Installing NVIDIA driver: {driver:?}");
  println!("Packages: {packages:?}");

  install::install(&packages)?;

  // Apply nvidia module in grub
  let grub_cmdline_content = std::fs::read_to_string("/mnt/etc/default/grub").unwrap_or_default();
  let mut grub_conf_found = false;
  let mut lines: Vec<String> = grub_cmdline_content
    .lines()
    .map(|line| {
      if line.starts_with("GRUB_CMDLINE_LINUX_DEFAULT=") {
        grub_conf_found = true;
        if line.contains("nvidia-drm.modeset=1") {
          String::from(line) // Already there, do nothing
        } else {
          line.replace(
            "GRUB_CMDLINE_LINUX_DEFAULT=\"",
            "GRUB_CMDLINE_LINUX_DEFAULT=\"nvidia-drm.modeset=1 ",
          )
        }
      } else {
        String::from(line)
      }
    })
    .collect();

  if !grub_conf_found {
    lines.push(String::from(
      "GRUB_CMDLINE_LINUX_DEFAULT=\"nvidia-drm.modeset=1\"",
    ));
  }

  let new_grub_content = lines.join("\n");
  std::fs::write("/mnt/etc/default/grub", new_grub_content)
    .map_err(|e| InstallError::IoError(format!("Failed to write GRUB config: {e}")))?;

  // Apply initcpio modules (skip for very old 340xx driver)
  if !matches!(driver, NvidiaDriver::Legacy340) {
    let mkinitcpio_content =
      std::fs::read_to_string("/mnt/etc/mkinitcpio.conf").unwrap_or_default();
    let mut mkinitcpio_conf_found = false;
    let mapped_lines: Vec<String> = mkinitcpio_content
      .lines()
      .map(|line| {
        if line.trim_start().starts_with("MODULES=") && !line.trim_start().starts_with('#') {
          mkinitcpio_conf_found = true;
          String::from("MODULES=(nvidia nvidia_modeset nvidia_uvm nvidia_drm)")
        } else {
          String::from(line)
        }
      })
      .collect();

    let mut final_lines = mapped_lines;
    if !mkinitcpio_conf_found {
      final_lines.push(String::from(
        "MODULES=(nvidia nvidia_modeset nvidia_uvm nvidia_drm)",
      ));
    }

    let new_initcpio_content = final_lines.join("\n");
    std::fs::write("/mnt/etc/mkinitcpio.conf", new_initcpio_content)
      .map_err(|e| InstallError::IoError(format!("Failed to write mkinitcpio config: {e}")))?;
  }

  println!("NVIDIA driver installation completed successfully!");
  Ok(())
}

mod tests {
  #[cfg(test)]
  use super::*;

  #[test]
  fn test_determine_driver_version_open() {
    // RTX 20xx series
    assert!(matches!(
      determine_driver_version(
        "01:00.0 VGA compatible controller [0300]: NVIDIA Corporation TU104 [GeForce RTX 2080] [10de:1e87]"
      ),
      NvidiaDriver::Open
    ));

    // RTX 30xx series
    assert!(matches!(
      determine_driver_version(
        "01:00.0 VGA compatible controller [0300]: NVIDIA Corporation GA102 [GeForce RTX 3080] [10de:2206]"
      ),
      NvidiaDriver::Open
    ));

    // RTX 40xx series
    assert!(matches!(
      determine_driver_version(
        "01:00.0 VGA compatible controller [0300]: NVIDIA Corporation AD103 [GeForce RTX 4080] [10de:2782]"
      ),
      NvidiaDriver::Open
    ));

    // GTX 16xx series
    assert!(matches!(
      determine_driver_version(
        "01:00.0 VGA compatible controller [0300]: NVIDIA Corporation TU117 [GeForce GTX 1650] [10de:1f82]"
      ),
      NvidiaDriver::Open
    ));
  }

  #[test]
  fn test_determine_driver_version_legacy470() {
    // GTX 10xx series
    assert!(matches!(
      determine_driver_version(
        "01:00.0 VGA compatible controller [0300]: NVIDIA Corporation GP104 [GeForce GTX 1080] [10de:1b80]"
      ),
      NvidiaDriver::Legacy470
    ));

    assert!(matches!(
      determine_driver_version(
        "01:00.0 VGA compatible controller [0300]: NVIDIA Corporation GP106 [GeForce GTX 1060 6GB] [10de:1c03]"
      ),
      NvidiaDriver::Legacy470
    ));

    assert!(matches!(
      determine_driver_version(
        "01:00.0 VGA compatible controller [0300]: NVIDIA Corporation GP102 [GeForce GTX 1070] [10de:1b81]"
      ),
      NvidiaDriver::Legacy470
    ));

    // Titan X
    assert!(matches!(
      determine_driver_version(
        "01:00.0 VGA compatible controller [0300]: NVIDIA Corporation GM200 [GeForce GTX TITAN X] [10de:17c2]"
      ),
      NvidiaDriver::Legacy470
    ));
  }

  #[test]
  fn test_determine_driver_version_legacy390() {
    // GTX 9xx series
    assert!(matches!(
      determine_driver_version(
        "01:00.0 VGA compatible controller [0300]: NVIDIA Corporation GM204 [GeForce GTX 980] [10de:13c0]"
      ),
      NvidiaDriver::Legacy390
    ));

    assert!(matches!(
      determine_driver_version(
        "01:00.0 VGA compatible controller [0300]: NVIDIA Corporation GM206 [GeForce GTX 960] [10de:1401]"
      ),
      NvidiaDriver::Legacy390
    ));

    // GTX 7xx series
    assert!(matches!(
      determine_driver_version(
        "01:00.0 VGA compatible controller [0300]: NVIDIA Corporation GK104 [GeForce GTX 770] [10de:1184]"
      ),
      NvidiaDriver::Legacy390
    ));

    // GTX 6xx series
    assert!(matches!(
      determine_driver_version(
        "01:00.0 VGA compatible controller [0300]: NVIDIA Corporation GK106 [GeForce GTX 660] [10de:11c0]"
      ),
      NvidiaDriver::Legacy390
    ));

    // Quadro
    assert!(matches!(
      determine_driver_version(
        "01:00.0 VGA compatible controller [0300]: NVIDIA Corporation GK107GL [Quadro K2000] [10de:0ffb]"
      ),
      NvidiaDriver::Legacy390
    ));
  }

  #[test]
  fn test_determine_driver_version_legacy340() {
    // GTX 4xx series
    assert!(matches!(
      determine_driver_version(
        "01:00.0 VGA compatible controller [0300]: NVIDIA Corporation GF100 [GeForce GTX 480] [10de:06c0]"
      ),
      NvidiaDriver::Legacy340
    ));

    // GTX 5xx series
    assert!(matches!(
      determine_driver_version(
        "01:00.0 VGA compatible controller [0300]: NVIDIA Corporation GF110 [GeForce GTX 580] [10de:1080]"
      ),
      NvidiaDriver::Legacy340
    ));

    // GeForce 8xxx series
    assert!(matches!(
      determine_driver_version(
        "01:00.0 VGA compatible controller [0300]: NVIDIA Corporation G80 [GeForce 8800 GTX] [10de:0191]"
      ),
      NvidiaDriver::Legacy340
    ));

    // GeForce 9xxx series
    assert!(matches!(
      determine_driver_version(
        "01:00.0 VGA compatible controller [0300]: NVIDIA Corporation G92 [GeForce 9800 GTX] [10de:0614]"
      ),
      NvidiaDriver::Legacy340
    ));
  }

  #[test]
  fn test_case_insensitive_detection() {
    // Test uppercase
    assert!(matches!(
      determine_driver_version(
        "01:00.0 VGA compatible controller [0300]: NVIDIA Corporation TU104 [GEFORCE RTX 2080] [10de:1e87]"
      ),
      NvidiaDriver::Open
    ));

    // Test mixed case
    assert!(matches!(
      determine_driver_version(
        "01:00.0 VGA compatible controller [0300]: NVIDIA Corporation GP104 [GeForce Gtx 1080] [10de:1b80]"
      ),
      NvidiaDriver::Legacy470
    ));
  }

  #[test]
  fn test_unknown_card_defaults_to_open() {
    // Unknown/future card should default to open driver
    assert!(matches!(
      determine_driver_version(
        "01:00.0 VGA compatible controller [0300]: NVIDIA Corporation Unknown Future Card [10de:9999]"
      ),
      NvidiaDriver::Open
    ));

    // Empty string should default to open
    assert!(matches!(determine_driver_version(""), NvidiaDriver::Open));
  }

  #[test]
  fn test_edge_cases() {
    // GTX 1050 (should be Legacy470, not Open despite being 10xx)
    assert!(matches!(
      determine_driver_version(
        "01:00.0 VGA compatible controller [0300]: NVIDIA Corporation GP107 [GeForce GTX 1050] [10de:1c81]"
      ),
      NvidiaDriver::Legacy470
    ));

    // Test partial matches don't trigger false positives
    assert!(matches!(
      determine_driver_version(
        "01:00.0 VGA compatible controller [0300]: NVIDIA Corporation SomeCard [GeForce GT 210] [10de:0a65]"
      ),
      NvidiaDriver::Open // Should default to Open as GT 210 doesn't match specific patterns
    ));
  }
}

// Integration test that could be run with actual hardware
#[cfg(test)]
mod integration_tests {
  use super::*;

  #[test]
  #[ignore = "Real GPU Test"] // Use `cargo test -- --ignored` to run this test
  fn test_actual_gpu_detection() {
    // This test will only work on systems with NVIDIA GPUs
    // and requires lspci to be installed
    match detect_nvidia_gpu() {
      Ok(Some(gpu_info)) => {
        println!("Detected GPU: {gpu_info}");
        let driver = determine_driver_version(&gpu_info);
        println!("Determined driver: {driver:?}");
        println!("Required packages: {:?}", driver.packages());
        // This test just prints info, doesn't assert anything
        // since we don't know what GPU the test system has
      }
      Ok(None) => {
        println!("No NVIDIA GPU detected on this system");
      }
      Err(e) => {
        println!("Error detecting GPU: {e}");
      }
    }
  }
}
