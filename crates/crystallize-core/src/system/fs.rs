use crate::utils::error::crash;
use std::{
  fs::{self, File, OpenOptions},
  io::Write,
};

pub fn create_file(path: &str) {
  let return_code = File::create(path);
  match return_code {
    Ok(_) => {
      log::info!("Create {path}")
    }
    Err(e) => {
      crash(format!("Create {path}: Failed with error {e}"), 1);
    }
  }
}

pub fn copy_file(path: &str, destpath: &str) {
  let return_code = fs::copy(path, destpath);
  match return_code {
    Ok(_) => {
      log::info!("Copy {path} to {destpath}")
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
  fs::create_dir_all(path)
}
