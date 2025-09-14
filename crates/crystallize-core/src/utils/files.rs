use regex::Regex;

use crate::utils::crash;
use std::fs::{self, File, OpenOptions};
use std::io::{self, prelude::*};
use std::path::Path;

/// Create a new file, creating parent directories if needed
pub fn create_file(path: &str) -> std::io::Result<()> {
  // Create parent directories if they don't exist
  if let Some(parent) = Path::new(path).parent() {
    fs::create_dir_all(parent)?;
  }

  match File::create(path) {
    Ok(_) => {
      log::info!("Created file: {path}");
      Ok(())
    }
    Err(e) => {
      log::error!("Failed to create file {path}: {e}");
      Err(e)
    }
  }
}

/// Create a file with initial content
pub fn create_file_with_content(path: &str, content: &str) -> std::io::Result<()> {
  // Create parent directories if they don't exist
  if let Some(parent) = Path::new(path).parent() {
    fs::create_dir_all(parent)?;
  }

  match fs::write(path, content) {
    Ok(()) => {
      log::info!("Created file with content: {path}");
      Ok(())
    }
    Err(e) => {
      log::error!("Failed to create file {path} with content: {e}");
      Err(e)
    }
  }
}

/// Copy a file from source to destination
pub fn copy_file(path: &str, destpath: &str) -> std::io::Result<()> {
  // Create destination parent directories if they don't exist
  if let Some(parent) = Path::new(destpath).parent() {
    fs::create_dir_all(parent)?;
  }

  match fs::copy(path, destpath) {
    Ok(_) => {
      log::info!("Copied {path} to {destpath}");
      Ok(())
    }
    Err(e) => {
      log::error!("Failed to copy {path} to {destpath}: {e}");
      Err(e)
    }
  }
}

/// Legacy `copy_file` function that crashes on error
pub fn copy_file_or_crash(path: &str, destpath: &str) {
  match copy_file(path, destpath) {
    Ok(()) => {}
    Err(e) => {
      crash(
        format!("Copy {path} to {destpath}: Failed with error {e}"),
        1,
      );
    }
  }
}

/// Append content to a file
pub fn append_file(path: &str, content: &str) -> std::io::Result<()> {
  log::info!("Appending content to file: {path}");

  // Create the file if it doesn't exist
  if !Path::new(path).exists() {
    create_file(path)?;
  }

  let mut file = OpenOptions::new().create(true).append(true).open(path)?;

  // Add newline before content if file is not empty
  let file_metadata = file.metadata()?;
  let prefix = if file_metadata.len() > 0 { "\n" } else { "" };

  file.write_all(format!("{}{}\n", prefix, content.trim_end()).as_bytes())?;
  Ok(())
}

/// Write content to a file, overwriting existing content
pub fn write_file(path: &str, content: &str) -> std::io::Result<()> {
  // Create parent directories if they don't exist
  if let Some(parent) = Path::new(path).parent() {
    fs::create_dir_all(parent)?;
  }

  match fs::write(path, content) {
    Ok(()) => {
      log::info!("Wrote content to file: {path}");
      Ok(())
    }
    Err(e) => {
      log::error!("Failed to write to file {path}: {e}");
      Err(e)
    }
  }
}

/// Replace text in a file
pub fn sed_file(path: &str, find: &str, replace: &str) -> std::io::Result<()> {
  log::info!("Replacing '{find}' with '{replace}' in file {path}");

  let contents = fs::read_to_string(path)?;
  let new_contents = contents.replace(find, replace);

  fs::write(path, new_contents)?;
  Ok(())
}

/// Replace text in a file using regex
pub fn sed_file_regex(
  path: &str,
  pattern: &str,
  replace: &str,
) -> Result<(), Box<dyn std::error::Error>> {
  log::info!("Replacing pattern '{pattern}' with '{replace}' in file {path}");

  let contents = fs::read_to_string(path)?;

  // Compile the regex pattern
  let re = Regex::new(pattern)?;

  // Perform the replacement
  let new_contents = re.replace_all(&contents, replace);

  // Only write if there were changes
  if new_contents == contents {
    log::info!("No matches found, file unchanged");
  } else {
    fs::write(path, new_contents.as_ref())?;
    log::info!("File updated successfully");
  }

  Ok(())
}

/// Create directory and all parent directories
pub fn create_directory(path: &str) -> std::io::Result<()> {
  match fs::create_dir_all(path) {
    Ok(()) => {
      log::debug!("Created directory: {path}");
      Ok(())
    }
    Err(e) if e.kind() == io::ErrorKind::AlreadyExists => {
      log::debug!("Directory already exists: {path}");
      Ok(())
    }
    Err(e) => {
      log::error!("Failed to create directory {path}: {e}");
      Err(e)
    }
  }
}

/// Check if a file or directory exists
#[must_use]
pub fn exists(path: &str) -> bool {
  Path::new(path).exists()
}

/// Check if path is a directory
#[must_use]
pub fn is_directory(path: &str) -> bool {
  Path::new(path).is_dir()
}

/// Check if path is a file
#[must_use]
pub fn is_file(path: &str) -> bool {
  Path::new(path).is_file()
}

/// Read file content as string
pub fn read_file(path: &str) -> std::io::Result<String> {
  match fs::read_to_string(path) {
    Ok(content) => {
      log::debug!("Read file: {path}");
      Ok(content)
    }
    Err(e) => {
      log::error!("Failed to read file {path}: {e}");
      Err(e)
    }
  }
}

/// Copy directory recursively
pub fn copy_dir(source: impl AsRef<Path>, destination: impl AsRef<Path>) -> io::Result<()> {
  let source = source.as_ref();
  let destination = destination.as_ref();

  log::info!(
    "Copying directory {} to {}",
    source.display(),
    destination.display()
  );

  fs::create_dir_all(destination)?;

  for entry in fs::read_dir(source)? {
    let entry = entry?;
    let file_type = entry.file_type()?;
    let source_path = entry.path();
    let dest_path = destination.join(entry.file_name());

    if file_type.is_dir() {
      // Recursively copy subdirectory
      copy_dir(&source_path, &dest_path)?;
    } else if file_type.is_file() {
      // Copy file
      fs::copy(&source_path, &dest_path)?;
    } else if file_type.is_symlink() {
      // Handle symlinks
      let link_target = fs::read_link(&source_path)?;
      std::os::unix::fs::symlink(&link_target, &dest_path)?;
    }
  }

  Ok(())
}

/// Remove file or directory
pub fn remove(path: &str) -> std::io::Result<()> {
  let path_obj = Path::new(path);

  if path_obj.is_dir() {
    match fs::remove_dir_all(path) {
      Ok(()) => {
        log::info!("Removed directory: {path}");
        Ok(())
      }
      Err(e) => {
        log::error!("Failed to remove directory {path}: {e}");
        Err(e)
      }
    }
  } else if path_obj.is_file() {
    match fs::remove_file(path) {
      Ok(()) => {
        log::info!("Removed file: {path}");
        Ok(())
      }
      Err(e) => {
        log::error!("Failed to remove file {path}: {e}");
        Err(e)
      }
    }
  } else {
    log::warn!("Path does not exist: {path}");
    Ok(())
  }
}

/// Set file permissions
pub fn set_permissions(path: &str, mode: u32) -> std::io::Result<()> {
  use std::os::unix::fs::PermissionsExt;

  let permissions = std::fs::Permissions::from_mode(mode);
  match fs::set_permissions(path, permissions) {
    Ok(()) => {
      log::info!("Set permissions {mode:o} on {path}");
      Ok(())
    }
    Err(e) => {
      log::error!("Failed to set permissions on {path}: {e}");
      Err(e)
    }
  }
}

/// Make file executable
pub fn make_executable(path: &str) -> std::io::Result<()> {
  set_permissions(path, 0o755)
}

/// Ensure directory exists with proper permissions
pub fn ensure_directory(path: &str, create_parents: bool) -> std::io::Result<()> {
  if exists(path) {
    if is_directory(path) {
      log::debug!("Directory already exists: {path}");
      return Ok(());
    }

    return Err(io::Error::new(
      io::ErrorKind::AlreadyExists,
      format!("{path} exists but is not a directory"),
    ));
  }

  if create_parents {
    create_directory(path)
  } else {
    match fs::create_dir(path) {
      Ok(()) => {
        log::info!("Created directory: {path}");
        Ok(())
      }
      Err(e) => {
        log::error!("Failed to create directory {path}: {e}");
        Err(e)
      }
    }
  }
}
