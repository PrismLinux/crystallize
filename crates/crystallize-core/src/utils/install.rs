use std::process::Command;

use crate::modules::partition::umount;

use super::exec_eval;

pub fn install(pkgs: Vec<&str>) {
  exec_eval(
    Command::new("pacstrap").arg("/mnt").args(&pkgs).status(),
    format!("Install packages {}", pkgs.join(", ")).as_str(),
  );
  umount("/mnt/dev");
}
