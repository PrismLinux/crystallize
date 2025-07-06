use crate::utils::error::crash;

pub fn exec_eval(return_code: Result<std::process::ExitStatus, std::io::Error>, logmsg: &str) {
    match &return_code {
        Ok(_) => {
            log::info!("{logmsg}");
        }
        Err(e) => {
            crash(
                format!("{logmsg} ERROR: {e}"),
                return_code.unwrap_err().raw_os_error().unwrap(),
            );
        }
    }
}

pub fn files_eval(return_code: Result<(), std::io::Error>, logmsg: &str) {
    match &return_code {
        Ok(_) => {
            log::info!("{logmsg}");
        }
        Err(e) => {
            crash(
                format!("{logmsg} ERROR: {e}"),
                return_code.unwrap_err().raw_os_error().unwrap(),
            );
        }
    }
}
