use std::process::Command;

// QUESTION: It really works?
pub fn exec(command: &str, args: Vec<String>) -> Result<std::process::ExitStatus, std::io::Error> {
  Command::new(command).args(args).status()
}

pub fn exec_chroot(
  command: &str,
  args: Vec<String>,
) -> Result<std::process::ExitStatus, std::io::Error> {
  Command::new("bash")
    .args([
      "-c",
      format!("arch-chroot /mnt {} {}", command, args.join(" ")).as_str(),
    ])
    .status()
}
