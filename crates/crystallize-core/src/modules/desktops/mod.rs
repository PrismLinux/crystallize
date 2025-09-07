use crate::cli::DesktopSetup;

mod desktop;
mod services;

pub fn install_desktop_setup(
  desktop_setup: DesktopSetup,
) -> Result<(), Box<dyn std::error::Error>> {
  log::debug!("Installing {desktop_setup:?}");
  services::networkmanager()?;
  services::ufw()?;

  if desktop_setup != DesktopSetup::None {
    desktop::graphics()?;
    desktop::packages()?;

    // Services
    services::bluetooth()?;
    services::cups()?;
    services::tuned_ppd()?;
  }

  match desktop_setup {
    DesktopSetup::Gnome => desktop::gnome()?,
    DesktopSetup::Plasma => desktop::plasma()?,
    DesktopSetup::Hyprland => desktop::hyprland()?,
    DesktopSetup::None => log::debug!("No desktop setup selected"),
  }

  Ok(())
}
