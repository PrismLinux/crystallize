use crate::utils::exec::{exec, exec_chroot, exec_chroot_with_output};
use indicatif::{MultiProgress, ProgressBar, ProgressStyle};
use regex::Regex;
use std::fs::File;
use std::io::{BufRead, BufReader, Write};
use std::process::{Command, Stdio};
use std::sync::{Arc, Mutex};

/// Configuration for progress tracking
#[derive(Clone)]
pub struct ProgressConfig {
  pub show_eta: bool,
  pub detailed_logging: bool,
}

impl Default for ProgressConfig {
  fn default() -> Self {
    Self {
      show_eta: true,
      detailed_logging: true,
    }
  }
}

/// Progress bar collection for different operations
#[derive(Clone)]
pub struct ProgressBars {
  pub download: ProgressBar,
  pub build: Option<ProgressBar>,
  pub install: ProgressBar,
}

impl ProgressBars {
  fn new_standard(multi: &MultiProgress, config: &ProgressConfig) -> Self {
    let download_template = if config.show_eta {
      "{spinner:.cyan} [{elapsed_precise}] Downloading packages ({pos} downloaded) ETA: {eta}"
    } else {
      "{spinner:.cyan} [{elapsed_precise}] Downloading packages ({pos} downloaded)"
    };

    let install_template = if config.show_eta {
      "{spinner:.green} [{elapsed_precise}] Installing packages ({pos} installed) ETA: {eta}"
    } else {
      "{spinner:.green} [{elapsed_precise}] Installing packages ({pos} installed)"
    };

    let download_pb = multi.add(ProgressBar::new_spinner());
    download_pb.set_style(
      ProgressStyle::default_spinner()
        .template(download_template)
        .unwrap(),
    );

    let install_pb = multi.add(ProgressBar::new_spinner());
    install_pb.set_style(
      ProgressStyle::default_spinner()
        .template(install_template)
        .unwrap(),
    );

    Self {
      download: download_pb,
      build: None,
      install: install_pb,
    }
  }

  fn new_aur(multi: &MultiProgress, config: &ProgressConfig) -> Self {
    let download_template = if config.show_eta {
      "{spinner:.magenta} [{elapsed_precise}] Downloading AUR packages ({pos} downloaded) ETA: {eta}"
    } else {
      "{spinner:.magenta} [{elapsed_precise}] Downloading AUR packages ({pos} downloaded)"
    };

    let build_template = if config.show_eta {
      "{spinner:.yellow} [{elapsed_precise}] Building AUR packages ({pos} built) ETA: {eta}"
    } else {
      "{spinner:.yellow} [{elapsed_precise}] Building AUR packages ({pos} built)"
    };

    let install_template = if config.show_eta {
      "{spinner:.green} [{elapsed_precise}] Installing AUR packages ({pos} installed) ETA: {eta}"
    } else {
      "{spinner:.green} [{elapsed_precise}] Installing AUR packages ({pos} installed)"
    };

    let download_pb = multi.add(ProgressBar::new_spinner());
    download_pb.set_style(
      ProgressStyle::default_spinner()
        .template(download_template)
        .unwrap(),
    );

    let build_pb = multi.add(ProgressBar::new_spinner());
    build_pb.set_style(
      ProgressStyle::default_spinner()
        .template(build_template)
        .unwrap(),
    );

    let install_pb = multi.add(ProgressBar::new_spinner());
    install_pb.set_style(
      ProgressStyle::default_spinner()
        .template(install_template)
        .unwrap(),
    );

    Self {
      download: download_pb,
      build: Some(build_pb),
      install: install_pb,
    }
  }

  fn finish_all(&self, success: bool) {
    if success {
      self.download.finish_with_message("Downloads complete");
      if let Some(build) = &self.build {
        build.finish_with_message("Builds complete");
      }
      self.install.finish_with_message("Installation complete");
    } else {
      self.download.finish_with_message("Downloads failed");
      if let Some(build) = &self.build {
        build.finish_with_message("Builds failed");
      }
      self.install.finish_with_message("Installation failed");
    }
  }

  fn update_to_determinate(&self, total: u64) {
    self.download.set_length(total);
    self.download.set_style(
      ProgressStyle::default_bar()
        .template("{spinner:.cyan} [{elapsed_precise}] [{bar:40.cyan}] {pos}/{len} Downloading")
        .unwrap()
        .progress_chars("#>-"),
    );

    if let Some(build) = &self.build {
      build.set_length(total);
      build.set_style(
        ProgressStyle::default_bar()
          .template("{spinner:.yellow} [{elapsed_precise}] [{bar:40.yellow}] {pos}/{len} Building")
          .unwrap()
          .progress_chars("#>-"),
      );
    }

    self.install.set_length(total);
    self.install.set_style(
      ProgressStyle::default_bar()
        .template("{spinner:.green} [{elapsed_precise}] [{bar:40.green}] {pos}/{len} Installing")
        .unwrap()
        .progress_chars("#>-"),
    );
  }
}

/// Validate package names
fn validate_packages(pkgs: &[&str]) -> Result<(), String> {
  for pkg in pkgs {
    if pkg.is_empty() || pkg.contains(' ') || pkg.starts_with('-') {
      return Err(format!("Invalid package name: {pkg}"));
    }
  }
  Ok(())
}

/// Install packages using pacstrap with progress tracking
pub fn install_base(pkgs: Vec<&str>) -> Result<(), String> {
  install_base_with_config(pkgs, &ProgressConfig::default())
}

/// Install packages using pacstrap with custom configuration
pub fn install_base_with_config(pkgs: Vec<&str>, config: &ProgressConfig) -> Result<(), String> {
  if pkgs.is_empty() {
    return Ok(());
  }

  validate_packages(&pkgs)?;

  let pkg_args: Vec<String> = pkgs.iter().map(|&s| s.to_string()).collect();
  let log = File::create("/tmp/prismlinux-pacstrap.log")
    .map_err(|e| format!("Failed to create log file: {e}"))?;

  let multi_progress = MultiProgress::new();
  let progress_bars = ProgressBars::new_standard(&multi_progress, config);
  let total_packages = Arc::new(Mutex::new(None::<u64>));

  let mut args = vec![String::from("/mnt")];
  args.extend(pkg_args);

  let mut cmd = Command::new("pacstrap");
  cmd.args(&args);

  exec_with_progress(
    cmd,
    &format!("Install base packages {}", pkgs.join(", ")),
    log,
    progress_bars,
    total_packages,
    config,
  )
}

/// Install packages in chroot environment
pub fn install(pkgs: Vec<&str>) -> Result<(), String> {
  install_with_config(pkgs, &ProgressConfig::default())
}

/// Install packages in chroot with custom configuration
pub fn install_with_config(pkgs: Vec<&str>, config: &ProgressConfig) -> Result<(), String> {
  if pkgs.is_empty() {
    return Ok(());
  }

  validate_packages(&pkgs)?;

  if config.detailed_logging {
    log::info!("Installing packages in chroot: {}", pkgs.join(", "));
  }

  let log = File::create("/tmp/prismlinux-install.log")
    .map_err(|e| format!("Failed to create log file: {e}"))?;

  let multi_progress = MultiProgress::new();
  let progress_bars = ProgressBars::new_standard(&multi_progress, config);
  let total_packages = Arc::new(Mutex::new(None::<u64>));

  let mut pacman_args = vec![
    String::from("-S"),
    String::from("--noconfirm"),
    String::from("--needed"),
  ];
  pacman_args.extend(pkgs.iter().map(|&s| s.to_string()));

  let full_command = format!("pacman {}", pacman_args.join(" "));
  let mut cmd = Command::new("arch-chroot");
  cmd.args(["/mnt", "bash", "-c", &full_command]);

  exec_with_progress(
    cmd,
    &format!("Install packages {}", pkgs.join(", ")),
    log,
    progress_bars,
    total_packages,
    config,
  )
}

/// Find available AUR helper
fn find_aur_helper() -> Result<String, String> {
  let aur_helpers = ["prism", "paru", "yay", "trizen"];

  for helper in &aur_helpers {
    match exec_chroot_with_output("which", vec![helper.to_string()]) {
      Ok(output) if output.status.success() => {
        log::info!("Using AUR helper: {helper}");
        return Ok(helper.to_string());
      }
      _ => continue,
    }
  }

  Err("No AUR helper found".to_string())
}

/// Install AUR packages
pub fn install_aur(pkgs: Vec<&str>) -> Result<(), String> {
  install_aur_with_config(pkgs, &ProgressConfig::default())
}

/// Install AUR packages with custom configuration
pub fn install_aur_with_config(pkgs: Vec<&str>, config: &ProgressConfig) -> Result<(), String> {
  if pkgs.is_empty() {
    return Ok(());
  }

  validate_packages(&pkgs)?;

  if config.detailed_logging {
    log::info!("Installing AUR packages: {}", pkgs.join(", "));
  }

  let helper = find_aur_helper()?;
  let log = File::create("/tmp/prismlinux-aur.log")
    .map_err(|e| format!("Failed to create log file: {e}"))?;

  let multi_progress = MultiProgress::new();
  let progress_bars = ProgressBars::new_aur(&multi_progress, config);
  let total_packages = Arc::new(Mutex::new(None::<u64>));

  let mut helper_args = vec![String::from("-S"), String::from("--noconfirm")];
  helper_args.extend(pkgs.iter().map(|&s| s.to_string()));

  let full_command = format!("{} {}", helper, helper_args.join(" "));
  let mut cmd = Command::new("arch-chroot");
  cmd.args(["/mnt", "bash", "-c", &full_command]);

  exec_with_progress(
    cmd,
    &format!("Install AUR packages {} with {}", pkgs.join(", "), helper),
    log,
    progress_bars,
    total_packages,
    config,
  )
}

/// Update package databases
pub fn update_databases() -> Result<(), String> {
  update_databases_with_config(&ProgressConfig::default())
}

/// Update databases with custom configuration
pub fn update_databases_with_config(config: &ProgressConfig) -> Result<(), String> {
  if config.detailed_logging {
    log::info!("Updating package databases");
  }

  let log = File::create("/tmp/prismlinux-update.log")
    .map_err(|e| format!("Failed to create log file: {e}"))?;

  let multi_progress = MultiProgress::new();
  let update_pb = multi_progress.add(ProgressBar::new_spinner());
  update_pb.set_style(
    ProgressStyle::default_spinner()
      .template("{spinner:.blue} [{elapsed_precise}] Updating package databases...")
      .unwrap(),
  );

  let full_command = "pacman -Sy --noconfirm";
  let mut cmd = Command::new("arch-chroot");
  cmd.args(["/mnt", "bash", "-c", full_command]);

  exec_simple_with_progress(cmd, "Update package databases", log, update_pb)
}

/// Upgrade system packages
pub fn upgrade_system() -> Result<(), String> {
  upgrade_system_with_config(&ProgressConfig::default())
}

/// Upgrade system with custom configuration
pub fn upgrade_system_with_config(config: &ProgressConfig) -> Result<(), String> {
  if config.detailed_logging {
    log::info!("Upgrading system packages");
  }

  let log = File::create("/tmp/prismlinux-upgrade.log")
    .map_err(|e| format!("Failed to create log file: {e}"))?;

  let multi_progress = MultiProgress::new();
  let progress_bars = ProgressBars::new_standard(&multi_progress, config);
  let total_packages = Arc::new(Mutex::new(None::<u64>));

  let full_command = "pacman -Syu --noconfirm";
  let mut cmd = Command::new("arch-chroot");
  cmd.args(["/mnt", "bash", "-c", full_command]);

  exec_with_progress(
    cmd,
    "Upgrade system packages",
    log,
    progress_bars,
    total_packages,
    config,
  )
}

/// Generic function for executing commands with progress tracking
fn exec_with_progress(
  mut cmd: Command,
  description: &str,
  log: File,
  progress_bars: ProgressBars,
  total_packages: Arc<Mutex<Option<u64>>>,
  config: &ProgressConfig,
) -> Result<(), String> {
  let mut child = cmd
    .stdout(Stdio::piped())
    .stderr(Stdio::piped())
    .spawn()
    .map_err(|e| format!("Failed to spawn process: {e}"))?;

  let stdout = child.stdout.take().unwrap();
  let stderr = child.stderr.take().unwrap();
  let mut handles = vec![];

  // Handle stdout
  let progress_bars_out = progress_bars.clone();
  let total_packages_out = total_packages.clone();
  let config_out = config.clone();
  let out_handle = std::thread::spawn({
    let mut log = log.try_clone().unwrap();
    move || {
      let reader = BufReader::new(stdout);
      for line in reader.lines().map_while(Result::ok) {
        if config_out.detailed_logging {
          writeln!(log, "{line}").ok();
        }
        process_package_line(&line, &progress_bars_out, &total_packages_out);
      }
    }
  });
  handles.push(out_handle);

  // Handle stderr
  let progress_bars_err = progress_bars.clone();
  let total_packages_err = total_packages.clone();
  let config_err = config.clone();
  let err_handle = std::thread::spawn({
    let mut log = log.try_clone().unwrap();
    move || {
      let reader = BufReader::new(stderr);
      for line in reader.lines().map_while(Result::ok) {
        if config_err.detailed_logging {
          writeln!(log, "{line}").ok();
        }
        process_package_line(&line, &progress_bars_err, &total_packages_err);
      }
    }
  });
  handles.push(err_handle);

  let status = child
    .wait()
    .map_err(|e| format!("Failed to wait for process: {e}"))?;

  // Wait for all threads to complete
  for handle in handles {
    handle
      .join()
      .map_err(|_| "Thread join failed".to_string())?;
  }

  let success = status.success();
  progress_bars.finish_all(success);

  if success {
    if config.detailed_logging {
      log::info!("{description} completed successfully");
    }
    Ok(())
  } else {
    Err(format!(
      "{} failed with exit code: {:?}",
      description,
      status.code()
    ))
  }
}

/// Simple progress tracking for basic operations
fn exec_simple_with_progress(
  mut cmd: Command,
  description: &str,
  log: File,
  progress_pb: ProgressBar,
) -> Result<(), String> {
  let mut child = cmd
    .stdout(Stdio::piped())
    .stderr(Stdio::piped())
    .spawn()
    .map_err(|e| format!("Failed to spawn process: {e}"))?;

  let stdout = child.stdout.take().unwrap();
  let stderr = child.stderr.take().unwrap();
  let mut handles = vec![];

  // Handle stdout
  let progress_pb_out = progress_pb.clone();
  let out_handle = std::thread::spawn({
    let mut log = log.try_clone().unwrap();
    move || {
      let reader = BufReader::new(stdout);
      for line in reader.lines().map_while(Result::ok) {
        writeln!(log, "{line}").ok();
        progress_pb_out.tick();
      }
    }
  });
  handles.push(out_handle);

  // Handle stderr
  let progress_pb_err = progress_pb.clone();
  let err_handle = std::thread::spawn({
    let mut log = log.try_clone().unwrap();
    move || {
      let reader = BufReader::new(stderr);
      for line in reader.lines().map_while(Result::ok) {
        writeln!(log, "{line}").ok();
        progress_pb_err.tick();
      }
    }
  });
  handles.push(err_handle);

  let status = child
    .wait()
    .map_err(|e| format!("Failed to wait for process: {e}"))?;

  for handle in handles {
    handle
      .join()
      .map_err(|_| "Thread join failed".to_string())?;
  }

  let success = status.success();
  if success {
    progress_pb.finish_with_message(format!("{description} complete"));
    log::info!("{description} completed successfully");
    Ok(())
  } else {
    progress_pb.finish_with_message(format!("{description} failed"));
    Err(format!(
      "{} failed with exit code: {:?}",
      description,
      status.code()
    ))
  }
}

/// Process package-related output lines
fn process_package_line(
  line: &str,
  progress_bars: &ProgressBars,
  total_packages: &Arc<Mutex<Option<u64>>>,
) {
  let line_lower = line.to_lowercase();

  // Look for total package count - create regex each time (less optimal but no new deps)
  if let Ok(regex) = Regex::new(r"packages?\s*\((\d+)\)") {
    if let Some(captures) = regex.captures(&line_lower) {
      if let Ok(total) = captures.get(1).unwrap().as_str().parse::<u64>() {
        let mut total_guard = total_packages.lock().unwrap();
        if total_guard.is_none() {
          *total_guard = Some(total);
          progress_bars.update_to_determinate(total);
        }
      }
    }
  }

  // Check for different operation types
  if line_lower.contains("downloading")
    || line_lower.contains("retrieving")
    || line_lower.contains("fetching")
    || line_lower.contains("cloning")
  {
    progress_bars.download.inc(1);
  }

  if let Some(build_pb) = &progress_bars.build {
    if line_lower.contains("building")
      || line_lower.contains("making")
      || line_lower.contains("compiling")
    {
      build_pb.inc(1);
    }
  }

  if line_lower.contains("installing") || line_lower.contains("upgrading") {
    progress_bars.install.inc(1);
  }
}

/// Backward compatibility functions
pub fn install_simple(pkgs: Vec<&str>) -> Result<(), String> {
  if pkgs.is_empty() {
    return Ok(());
  }

  validate_packages(&pkgs)?;
  log::info!("Installing packages in chroot: {}", pkgs.join(", "));

  let mut pacman_args = vec![
    String::from("-S"),
    String::from("--noconfirm"),
    String::from("--needed"),
  ];
  pacman_args.extend(pkgs.iter().map(|&s| s.to_string()));

  match exec_chroot("pacman", pacman_args) {
    Ok(status) if status.success() => {
      log::info!("Packages installed successfully: {}", pkgs.join(", "));
      Ok(())
    }
    Ok(status) => Err(format!(
      "Package installation failed with exit code: {:?}",
      status.code()
    )),
    Err(e) => Err(format!("Failed to execute package installation: {e}")),
  }
}

pub fn install_base_simple(pkgs: Vec<&str>) -> Result<(), String> {
  if pkgs.is_empty() {
    return Ok(());
  }

  validate_packages(&pkgs)?;

  let mut args = vec!["/mnt".to_string()];
  args.extend(pkgs.iter().map(|&s| s.to_string()));

  match exec("pacstrap", args) {
    Ok(status) if status.success() => {
      log::info!("Base packages installed successfully: {}", pkgs.join(", "));
      Ok(())
    }
    Ok(status) => Err(format!(
      "Base package installation failed with exit code: {:?}",
      status.code()
    )),
    Err(e) => Err(format!("Failed to execute base package installation: {e}")),
  }
}

/// Batch operations for efficiency
pub fn install_multiple_categories(
  base_pkgs: Vec<&str>,
  regular_pkgs: Vec<&str>,
  aur_pkgs: Vec<&str>,
  config: &ProgressConfig,
) -> Result<(), String> {
  if !base_pkgs.is_empty() {
    install_base_with_config(base_pkgs, config)?;
  }

  if !regular_pkgs.is_empty() {
    install_with_config(regular_pkgs, config)?;
  }

  if !aur_pkgs.is_empty() {
    install_aur_with_config(aur_pkgs, config)?;
  }

  Ok(())
}

/// Check if packages are already installed
pub fn check_installed(pkgs: &[&str]) -> Result<Vec<String>, String> {
  let mut not_installed = Vec::new();

  for pkg in pkgs {
    match exec_chroot_with_output("pacman", vec!["-Q".to_string(), pkg.to_string()]) {
      Ok(output) if !output.status.success() => {
        not_installed.push(pkg.to_string());
      }
      Err(_) => {
        not_installed.push(pkg.to_string());
      }
      _ => {} // Package is installed
    }
  }

  Ok(not_installed)
}
