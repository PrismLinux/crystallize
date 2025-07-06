use flexi_logger::{DeferredNow, LogSpecification, Logger, style};
use log::{LevelFilter, Record};
use std::io::Write;

pub fn init(verbose: usize) {
  let log_specification = match verbose {
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
    .format(format_log_message)
    .start()
    .unwrap();
}

pub fn format_log_message(
  w: &mut dyn Write,
  now: &mut DeferredNow,
  record: &Record,
) -> std::io::Result<()> {
  let message = record.args().to_string();
  let level = record.level();
  let timestamp = now.now().format("%H:%M:%S").to_string();
  let styled_level = style(level).paint(level.to_string());

  writeln!(w, "[ {styled_level} ] {timestamp} {message}")
}
