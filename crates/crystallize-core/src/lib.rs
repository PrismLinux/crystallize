pub mod cli;
pub mod config;
pub mod modules;
pub mod utils;

pub fn main() {
  std::panic::set_hook(Box::new(|info| {
    eprintln!("Panic occurred: {info:?}");
    if let Some(location) = info.location() {
      eprintln!(
        "Location: {}:{}:{}",
        location.file(),
        location.line(),
        location.column()
      );
    }
    if let Some(payload) = info.payload().downcast_ref::<&str>() {
      eprintln!("Message: {payload}");
    }
  }));
}
