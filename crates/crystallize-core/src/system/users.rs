use crate::system::command::exec_chroot;
use crate::system::{fs, install};
use crate::utils::eval::{exec_eval, files_eval};

pub fn new_user(username: &str, hasroot: bool, password: &str, do_hash_pass: bool, shell: &str) {
    let shell: &str = shell;
    if do_hash_pass {
        let hashed_pass = &*hash_pass(password).expect("Failed to hash password");
        let _password = &hashed_pass;
    }
    let shell_to_install = match shell {
        "bash" => "bash",
        "csh" => "tcsh",
        "fish" => "fish",
        "tcsh" => "tcsh",
        "zsh" => "zsh",
        &_ => "bash",
    };
    install::install(vec![shell_to_install]);
    let shell_path = match shell {
        "bash" => "/bin/bash",
        "csh" => "/usr/bin/csh",
        "fish" => "/usr/bin/fish",
        "tcsh" => "/usr/bin/tcsh",
        "zsh" => "/usr/bin/zsh",
        &_ => "/usr/bin/bash",
    };
    exec_eval(
        exec_chroot(
            "useradd",
            vec![
                String::from("-m"),
                String::from("-s"),
                String::from(shell_path),
                String::from("-p"),
                String::from(password).replace('\n', ""),
                String::from(username),
            ],
        ),
        format!("Create user {username}").as_str(),
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
            format!("Add user {username} to wheel group").as_str(),
        );
        files_eval(
            fs::sed_file(
                "/mnt/etc/sudoers",
                "# %wheel ALL=(ALL:ALL) ALL",
                "%wheel ALL=(ALL:ALL) ALL",
            ),
            "Add wheel group to sudoers",
        );
        files_eval(
            fs::append_file("/mnt/etc/sudoers", "\nDefaults pwfeedback\n"),
            "Add pwfeedback to sudoers",
        );
        files_eval(
            fs::create_directory("/mnt/var/lib/AccountsService/users/"),
            "Create /mnt/var/lib/AcountsService",
        );
        fs::create_file(&format!("/mnt/var/lib/AccountsService/users/{username}"));
        files_eval(
            fs::append_file(
                &format!("/mnt/var/lib/AccountsService/users/{username}"),
                r#"[User]
                Session=plasma"#,
            ),
            format!("Populate AccountsService user file for {username}").as_str(),
        )
    }
}

pub fn hash_pass(password: &str) -> Result<String, bcrypt::BcryptError> {
    let cost = 12;
    bcrypt::hash(password, cost)
}


pub fn root_pass(root_pass: &str) {
    let hashed_pass =
        hash_pass(root_pass).expect("Failed to hash root password for 'usermod' command");
    exec_eval(
        exec_chroot(
            "usermod",
            vec![
                String::from("--password"),
                hashed_pass,
                String::from("root"),
            ],
        ),
        "set root password",
    );
}