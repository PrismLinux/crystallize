use crate::cli::DesktopSetup;

mod desktop;
mod services;

pub fn install_desktop_setup(
  desktop_setup: DesktopSetup,
) -> Result<(), Box<dyn std::error::Error>> {
  log::debug!("Installing {desktop_setup:?}");
  services::networkmanager()?;
  services::firewalld()?;

  // Install only the selected desktop environment and its specific packages
  match desktop_setup {
    DesktopSetup::Gnome => {
      desktop::graphics()?;
      desktop::packages()?;
      desktop::gnome()?;
    }
    DesktopSetup::Plasma => {
      desktop::graphics()?;
      desktop::packages()?;
      desktop::plasma()?;
    }
    DesktopSetup::Cosmic => {
      desktop::graphics()?;
      desktop::packages()?;
      desktop::cosmic()?;
    }
    DesktopSetup::Cinnamon => {
      desktop::graphics()?;
      desktop::packages()?;
      desktop::cinnamon()?;
    }
    DesktopSetup::None => {
      log::debug!("No desktop setup selected");
      return Ok(());
    }
  }

  // Common services for all desktop environments
  services::bluetooth()?;
  services::cups()?;
  services::tuned_ppd()?;

  Ok(())
}
