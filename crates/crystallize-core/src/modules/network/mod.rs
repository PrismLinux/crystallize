use crate::utils::{
  files::{append_file, create_file},
  files_eval,
};

pub fn set_hostname(hostname: &str) {
  println!("Setting hostname to {hostname}");
  let _ = create_file("/mnt/etc/hostname");
  files_eval(append_file("/mnt/etc/hostname", hostname), "set hostname");
}

pub fn create_hosts() {
  let _ = create_file("/mnt/etc/hosts");
  files_eval(
    append_file("/mnt/etc/hosts", "127.0.0.1     localhost"),
    "create /etc/hosts",
  );
}

pub fn enable_ipv6() {
  files_eval(
    append_file("/mnt/etc/hosts", "::1 localhost"),
    "add ipv6 localhost",
  );
}
