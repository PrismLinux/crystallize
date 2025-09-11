use std::process::Command;

pub fn exec(command: &str, args: &[&str]) -> Result<std::process::ExitStatus, std::io::Error> {
  log::debug!("Executing: {} {}", command, args.join(" "));
  Command::new(command).args(args).status()
}

pub fn exec_chroot(
  command: &str,
  args: &[&str],
) -> Result<std::process::ExitStatus, std::io::Error> {
  let full_command = if args.is_empty() {
    command.to_string()
  } else {
    format!("{} {}", command, args.join(" "))
  };

  log::debug!("Executing in chroot: {}", full_command);

  Command::new("arch-chroot")
    .args(["/mnt", "bash", "-c", &full_command])
    .status()
}

pub fn exec_chroot_with_output(
  command: &str,
  args: &[&str],
) -> Result<std::process::Output, std::io::Error> {
  let full_command = if args.is_empty() {
    command.to_string()
  } else {
    format!("{} {}", command, args.join(" "))
  };

  log::debug!("Executing in chroot with output: {}", full_command);

  Command::new("arch-chroot")
    .args(["/mnt", "bash", "-c", &full_command])
    .output()
}
