use crate::utils::{exec::exec_chroot, exec_eval, files, files_eval};

pub fn set_timezone(timezone: &str) {
  exec_eval(
    exec_chroot(
      "ln",
      vec![
        String::from("-sf"),
        format!("/usr/share/zoneinfo/{}", timezone),
        String::from("/etc/localtime"),
      ],
    ),
    "Set timezone",
  );
  exec_eval(
    exec_chroot("hwclock", vec![String::from("--systohc")]),
    "Set system clock",
  );
}

pub fn set_locale(locale: String) {
  files_eval(
    files::append_file("/mnt/etc/locale.gen", "en_US.UTF-8 UTF-8"),
    "add en_US.UTF-8 UTF-8 to locale.gen",
  );
  let _ = files::create_file("/mnt/etc/locale.conf");
  files_eval(
    files::append_file("/mnt/etc/locale.conf", "LANG=en_US.UTF-8"),
    "edit locale.conf",
  );
  for i in (0..locale.split(' ').count()).step_by(2) {
    files_eval(
      files::append_file(
        "/mnt/etc/locale.gen",
        &format!(
          "{} {}\n",
          locale.split(' ').collect::<Vec<&str>>()[i],
          locale.split(' ').collect::<Vec<&str>>()[i + 1]
        ),
      ),
      "add locales to locale.gen",
    );
    if locale.split(' ').collect::<Vec<&str>>()[i] != "en_US.UTF-8" {
      files_eval(
        files::sed_file(
          "/mnt/etc/locale.conf",
          "en_US.UTF-8",
          locale.split(' ').collect::<Vec<&str>>()[i],
        ),
        format!(
          "Set locale {} in /etc/locale.conf",
          locale.split(' ').collect::<Vec<&str>>()[i]
        )
        .as_str(),
      );
    }
  }
  exec_eval(exec_chroot("locale-gen", vec![]), "generate locales");
}

pub fn set_keyboard(keyboard: &str) {
  exec_eval(
    exec_chroot(
      "localectl",
      vec![String::from("set-x11-keymap"), String::from(keyboard)],
    ),
    "Set x11 keymap",
  );
  exec_eval(
    exec_chroot(
      "localectl",
      vec![String::from("set-keymap"), String::from(keyboard)],
    ),
    "Set global keymap",
  );
}
