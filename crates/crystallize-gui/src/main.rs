mod modules;
mod utils;
mod window;

use gtk::prelude::*;
use gtk::{gio, glib};

use window::CrystallizeWindow;

const APP_ID: &str = "org.crystallinux.crystallize";

#[tokio::main]
async fn main() -> glib::ExitCode {
  // Initialize logger
  env_logger::init();

  // Create app
  let app = adw::Application::builder().application_id(APP_ID).build();

  // Connect activate
  app.connect_activate(build_ui);

  // Run the app
  app.run()
}

fn build_ui(app: &adw::Application) {
  // Load resources
  let resources_bytes = include_bytes!(concat!(env!("OUT_DIR"), "/crystallize.gresource"));
  let data = glib::Bytes::from(&resources_bytes[..]);
  let resources = gio::Resource::from_data(&data).expect("Failed to load resources");
  gio::resources_register(&resources);

  // Create window
  let window = CrystallizeWindow::new(app);
  window.present();
}
