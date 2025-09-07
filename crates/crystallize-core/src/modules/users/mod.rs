use std::process::Command;

use crate::utils::{
  exec::exec_chroot,
  exec_eval, files, files_eval,
  install::{self, InstallError},
};

/// Generate New user
pub fn new_user(
  username: &str,
  hasroot: bool,
  password: &str,
  do_hash_pass: bool,
  shell: &str,
) -> Result<(), Box<dyn std::error::Error>> {
  let final_password = if do_hash_pass {
    hash_pass(password)?
  } else {
    password.to_string()
  };

  let shell_to_install = match shell {
    "fish" => "fish",
    "zsh" => "zsh",
    _ => "bash",
  };

  install::install(vec![shell_to_install])?;

  let shell_path = match shell {
    "fish" => "/usr/bin/fish",
    "zsh" => "/usr/bin/zsh",
    "bash" | &_ => "/bin/bash",
  };

  exec_eval(
    exec_chroot(
      "useradd",
      vec![
        String::from("-m"),
        String::from("-s"),
        String::from(shell_path),
        String::from("-p"),
        final_password.trim().to_string(),
        String::from(username),
      ],
    ),
    &format!("Create user {username}"),
  );

  if hasroot {
    exec_eval(
      exec_chroot(
        "usermod",
        vec![
          String::from("-aG"),
          String::from("wheel"),
          String::from(username),
        ],
      ),
      &format!("Add user {username} to wheel group"),
    );

    files_eval(
      files::sed_file(
        "/mnt/etc/sudoers",
        "# %wheel ALL=(ALL:ALL) ALL",
        "%wheel ALL=(ALL:ALL) ALL",
      ),
      "Add wheel group to sudoers",
    );

    files_eval(
      files::append_file("/mnt/etc/sudoers", "\nDefaults pwfeedback\n"),
      "Add pwfeedback to sudoers",
    );

    files_eval(
      files::create_directory("/mnt/var/lib/AccountsService/users/"),
      "Create /mnt/var/lib/AccountsService",
    );

    files::create_file(&format!("/mnt/var/lib/AccountsService/users/{username}"))?;

    files_eval(
      files::append_file(
        &format!("/mnt/var/lib/AccountsService/users/{username}"),
        "[User]\nSession=plasma\n",
      ),
      &format!("Populate AccountsService user file for {username}"),
    );
  }

  Ok(())
}

/// Hash password
pub fn hash_pass(password: &str) -> Result<String, InstallError> {
  let output = Command::new("openssl")
    .args(["passwd", "-1", password])
    .output()
    .map_err(|e| InstallError::PasswordHashError(format!("Failed to execute openssl: {e}")))?;

  if !output.status.success() {
    return Err(InstallError::PasswordHashError(format!(
      "OpenSSL failed: {}",
      String::from_utf8_lossy(&output.stderr)
    )));
  }

  let hash = String::from_utf8(output.stdout)
    .map_err(|e| InstallError::PasswordHashError(format!("Invalid UTF-8 in password hash: {e}")))?
    .trim()
    .to_string();

  Ok(hash)
}

/// Set root password
pub fn root_pass(root_pass: &str) {
  exec_eval(
    exec_chroot(
      "bash",
      vec![
        String::from("-c"),
        format!("'usermod --password {root_pass} root'"),
      ],
    ),
    "set root password",
  );
}
