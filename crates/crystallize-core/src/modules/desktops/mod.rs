use crate::cli::DesktopSetup;

mod desktop;
mod services;

pub fn install_desktop_setup(desktop_setup: DesktopSetup) {
  log::debug!("Installing {desktop_setup:?}");
  services::networkmanager();
  services::ufw();
  if desktop_setup != DesktopSetup::None {
    desktop::packages();
    services::bluetooth();
    services::cups();
    services::tuned_ppd()
  }

  match desktop_setup {
    DesktopSetup::Gnome => desktop::gnome(),
    DesktopSetup::Kde => desktop::kde(),
    DesktopSetup::Cinnamon => desktop::cinnamon(),
    DesktopSetup::None => log::debug!("No desktop setup selected"),
  }
}
