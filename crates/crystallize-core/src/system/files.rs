use std::fs::{self, File, OpenOptions};
use std::io::{self, prelude::*};
use std::path::Path;

use crate::utils::crash;

pub fn create_file(path: &str) {
  let returncode = File::create(path);
  match returncode {
    Ok(_) => {
      log::info!("Create {path}");
    }
    Err(e) => {
      crash(format!("Create {path}: Failed with error {e}"), 1);
    }
  }
}

pub fn copy_file(path: &str, destpath: &str) {
  let return_code = std::fs::copy(path, destpath);
  match return_code {
    Ok(_) => {
      log::info!("Copy {path} to {destpath}");
    }
    Err(e) => {
      crash(
        format!("Copy {path} to {destpath}: Failed with error {e}"),
        1,
      );
    }
  }
}

pub fn append_file(path: &str, content: &str) -> std::io::Result<()> {
  log::info!("Append '{}' to file {}", content.trim_end(), path);
  let mut file = OpenOptions::new().append(true).open(path)?;
  file.write_all(format!("\n{content}\n").as_bytes())?;
  Ok(())
}

pub fn sed_file(path: &str, find: &str, replace: &str) -> std::io::Result<()> {
  log::info!("Sed '{find}' to '{replace}' in file {path}");
  let contents = fs::read_to_string(path)?;
  let new_contents = contents.replace(find, replace);
  let mut file = OpenOptions::new().write(true).truncate(true).open(path)?;
  file.write_all(new_contents.as_bytes())?;
  Ok(())
}

pub fn create_directory(path: &str) -> std::io::Result<()> {
  std::fs::create_dir_all(path)
}

pub fn copy_dir(source: impl AsRef<Path>, destination: impl AsRef<Path>) -> io::Result<()> {
  fs::create_dir_all(&destination)?;

  for entry in fs::read_dir(source)? {
    match entry {
      Ok(entry) => {
        match entry.file_type() {
          Ok(filetype) => {
            if filetype.is_dir() {
              // Recursively copy the subdirectory
              copy_dir(entry.path(), destination.as_ref().join(entry.file_name()))?;
            } else {
              // Copy the file
              fs::copy(entry.path(), destination.as_ref().join(entry.file_name()))?;
            }
          }
          Err(e) => {
            eprintln!("Failed to read file type: {e}");
            return Err(e);
          }
        }
      }
      Err(e) => {
        eprintln!("Failed to read directory entry: {e}");
        return Err(e);
      }
    }
  }

  Ok(())
}
