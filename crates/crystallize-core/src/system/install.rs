use crate::system::partition::unmount;
use crate::utils::eval::exec_eval;
use std::process::Command;

pub fn install(pkgs: Vec<&str>) {
    exec_eval(
        Command::new("pacstrap").arg("/mnt").args(&pkgs).status(),
        format!("Install packages {}", pkgs.join(", ")).as_str(),
    );
    unmount("/mnt/dev");
}
