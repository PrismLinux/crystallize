use clap::{Args, Parser, Subcommand, ValueEnum};
use serde::{Deserialize, Serialize};
use std::path::PathBuf;

#[derive(Debug, Parser)]
#[clap(name="crystallize-cli", version=env!("CARGO_PKG_VERSION"), about=env!("CARGO_PKG_DESCRIPTION"), author=env!("CARGO_PKG_AUTHORS"))]
pub struct Opt {
  #[clap(subcommand)]
  pub command: Command,

  #[arg(short, long, action = clap::ArgAction::Count)]
  pub verbose: u8,
}

#[derive(Debug, Subcommand)]
pub enum Command {
  /// Read Crystallize installation config
  #[clap(name = "config")]
  Config {
    /// The config file to read
    config: PathBuf,
  },
}

#[derive(Debug, Args)]
pub struct InstallBaseArgs {
  #[clap(long)]
  pub kernel: String,
}

#[derive(Debug, Clone)]
pub struct Partition {
  pub mountpoint: String,
  pub blockdevice: String,
  pub filesystem: String,
}

impl Partition {
  pub fn new(mountpoint: String, blockdevice: String, filesystem: String) -> Self {
    Self {
      mountpoint,
      blockdevice,
      filesystem,
    }
  }
}

pub fn parse_partition(s: &str) -> Result<Partition, &'static str> {
  println!("{s}");
  Ok(Partition::new(
    s.split(':').collect::<Vec<&str>>()[0].to_string(),
    s.split(':').collect::<Vec<&str>>()[1].to_string(),
    s.split(':').collect::<Vec<&str>>()[2].to_string(),
  ))
}

#[derive(Debug, ValueEnum, Copy, Clone, Ord, PartialOrd, Eq, PartialEq, Serialize, Deserialize)]
pub enum PartitionMode {
  #[clap(name = "auto")]
  Auto,
  #[clap(name = "manual")]
  Manual,
}

#[derive(Debug, ValueEnum, Copy, Clone, Ord, PartialOrd, Eq, PartialEq, Serialize, Deserialize)]
pub enum DesktopSetup {
  #[clap(name = "gnome")]
  Gnome,
  #[clap(name = "kde", aliases = ["plasma"])]
  Kde,
  #[clap(name = "cinnamon")]
  Cinnamon,
  #[clap(name = "hyprland")]
  Hyprland,
  #[clap(name = "None/DIY")]
  None,
}
