use chrono::Timelike;
use flexi_logger::{DeferredNow, LogSpecification, Logger, style};
use log::LevelFilter;
use std::io::Write;

pub fn init(verbosity: usize) -> Result<(), Box<dyn std::error::Error>> {
  let log_specification = match verbosity {
    0 => LogSpecification::builder()
      .default(LevelFilter::Info)
      .build(),
    1 => LogSpecification::builder()
      .default(LevelFilter::Debug)
      .build(),
    _ => LogSpecification::builder()
      .default(LevelFilter::Trace)
      .build(),
  };

  Logger::with(log_specification)
    .format(format_log_entry)
    .start()
    .map_err(|e| -> Box<dyn std::error::Error> {
      format!("Failed to initialize logger: {e}").into()
    })?;

  Ok(())
}

/// Formats a log entry with color
fn format_log_entry(
  w: &mut dyn Write,
  now: &mut DeferredNow,
  record: &log::Record,
) -> std::io::Result<()> {
  let level = record.level();
  let time = now.now();
  let (h, m, s) = (time.hour(), time.minute(), time.second());
  write!(
    w,
    "[ {} ] {:02}:{:02}:{:02} {}",
    style(level).paint(level.to_string()),
    h,
    m,
    s,
    record.args()
  )
}
