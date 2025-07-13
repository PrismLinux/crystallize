use std::process::Command;

use crate::{system::partition::umount, utils::exec_eval};

pub fn install(pkgs: Vec<&str>) {
  exec_eval(
    Command::new("pacstrap").arg("/mnt").args(&pkgs).status(),
    format!("Install packages {}", pkgs.join(", ")).as_str(),
  );
  umount("/mnt/dev");
}
