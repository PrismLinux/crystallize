use std::process::Command;

pub fn exec(command: &str, args: Vec<String>) -> Result<std::process::ExitStatus, std::io::Error> {
  Command::new(command).args(args).status()
}

pub fn exec_chroot(
  command: &str,
  args: Vec<String>,
) -> Result<std::process::ExitStatus, std::io::Error> {
  Command::new("arch-chroot")
    .arg("/mnt")
    .arg(command)
    .args(args)
    .status()
}
