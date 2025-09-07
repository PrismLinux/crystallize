use std::result::Result;

use crate::utils::crash;

pub fn exec_eval(return_code: Result<std::process::ExitStatus, std::io::Error>, logmsg: &str) {
  match return_code {
    Ok(_) => {
      log::info!("{logmsg}");
    }
    Err(e) => {
      let exit_code = e.raw_os_error().unwrap_or(-1);
      crash(format!("{logmsg} ERROR: {e}"), exit_code);
    }
  }
}

pub fn files_eval(return_code: Result<(), std::io::Error>, logmsg: &str) {
  match return_code {
    Ok(()) => {
      log::info!("{logmsg}");
    }
    Err(e) => {
      let exit_code = e.raw_os_error().unwrap_or(-1);
      crash(format!("{logmsg} ERROR: {e}"), exit_code);
    }
  }
}
