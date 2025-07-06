use crate::system::command::exec_chroot;
use crate::system::fs;
use crate::utils::eval::{exec_eval, files_eval};

pub fn set_timezone(timezone: &str) {
    exec_eval(
        exec_chroot(
            "ln",
            vec![
                "-sf".to_string(),
                format!("/usr/share/zoneinfo/{}", timezone),
                "/etc/localtime".to_string(),
            ],
        ),
        "Set timezone",
    );
    exec_eval(
        exec_chroot("hwclock", vec!["--systohc".to_string()]),
        "Set system clock",
    );
}

pub fn set_locale(locale: String) {
    files_eval(
        fs::append_file("/mnt/etc/locale.gen", "en_US.UTF-8 UTF-8"),
        "add en_US.UTF-8 UTF-8 to locale.gen",
    );
    fs::create_file("/mnt/etc/locale.conf");
    files_eval(
        fs::append_file("/mnt/etc/locale.conf", "LANG=en_US.UTF-8"),
        "edit locale.conf",
    );
    for i in (0..locale.split(' ').count()).step_by(2) {
        files_eval(
            fs::append_file(
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
                fs::sed_file(
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
    fs::create_file("/mnt/etc/vconsole.conf");
    files_eval(
        fs::append_file(
            "/mnt/etc/vconsole.conf",
            format!("KEYMAP={keyboard}").as_str(),
        ),
        "set keyboard layout in vconsole",
    );
    exec_eval(
        exec_chroot(
            "localectl",
            vec!["set-x11-keymap".to_string(), keyboard.to_string()],
        ),
        "Set x11 keymap",
    );
    exec_eval(
        exec_chroot(
            "localectl",
            vec!["set-keymap".to_string(), keyboard.to_string()],
        ),
        "Set global keymap",
    );
}
