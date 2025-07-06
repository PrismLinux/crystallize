use clap::{Args, Parser, Subcommand, ValueEnum};
use serde::{Deserialize, Serialize};
use std::path::PathBuf;

/// Disk partitioning mode for installation
#[derive(Debug, ValueEnum, Copy, Clone, Ord, PartialOrd, Eq, PartialEq, Serialize, Deserialize)]
pub enum PartitionMode {
  #[value(name = "auto")]
  Auto,
  #[value(name = "manual")]
  Manual,
}

/// Description of a single partition (mountpoint:blockdevice:filesystem)
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
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

/// Parser for Partition from string "mountpoint:blockdevice:filesystem"
pub fn parse_partition(s: &str) -> Result<Partition, &'static str> {
  let parts: Vec<&str> = s.split(':').collect();
  if parts.len() != 3 {
    return Err("Partition must be in format mountpoint:blockdevice:filesystem");
  }
  Ok(Partition::new(
    parts[0].to_string(),
    parts[1].to_string(),
    parts[2].to_string(),
  ))
}

/// Main command-line option parser
#[derive(Parser)]
#[command(name = "crystallize-cli")]
#[command(about = "A CLI tool for system management")]
pub struct Opt {
  /// Increase verbosity level (e.g., -v, -vv, -vvv)
  #[arg(short, long, action = clap::ArgAction::Count)]
  pub verbose: u8,

  #[command(subcommand)]
  pub command: Command,
}

/// All supported CLI commands
#[derive(Debug, Subcommand)]
pub enum Command {
  /// Configure disk partitions.
  #[clap(name = "partition")]
  Partition(PartitionArgs),

  /// Install the base system packages.
  #[clap(name = "install-base")]
  InstallBase(InstallBaseArgs),

  /// Set up the package manager keyring.
  #[clap(name = "setup-keyring")]
  SetupKeyring,

  /// Generate the /etc/fstab file.
  #[clap(name = "genfstab")]
  GenFstab,

  /// Configure the system bootloader.
  #[clap(name = "bootloader")]
  Bootloader {
    #[clap(subcommand)]
    subcommand: BootloaderSubcommand,
  },

  /// Set up system locale, keyboard layout, and timezone.
  #[clap(name = "locale")]
  Locale(LocaleArgs),

  /// Configure network settings.
  #[clap(name = "networking")]
  Networking(NetworkingArgs),

  /// Configure swap space.
  #[clap(name = "swap")]
  Swap {
    #[arg(value_parser)]
    size: u64,
  },

  /// Copy configuration from the live system.
  #[clap(name = "copy-live-config")]
  CopyLive,

  /// Install Nvidia graphics drivers.
  #[clap(name = "nvidia")]
  Nvidia,

  /// Process a configuration file.
  #[clap(name = "config")]
  Config { config: PathBuf },

  /// Set up a desktop environment.
  #[clap(name = "desktops")]
  Desktops {
    #[arg(value_enum)]
    desktop: DesktopSetup,
  },

  /// Manage system users.
  #[clap(name = "users")]
  Users {
    #[clap(subcommand)]
    subcommand: UsersSubcommand,
  },
}

/// Arguments for the partition command
#[derive(Debug, Args)]
pub struct PartitionArgs {
  /// Partitioning mode (auto/manual)
  #[arg(value_enum)]
  pub mode: PartitionMode,

  /// Device for partitioning (required for auto)
  #[arg(required_if_eq("mode", "auto"))]
  pub device: Option<PathBuf>,

  /// Whether to use EFI
  #[arg(long)]
  pub efi: bool,

  /// Partitions for manual mode (required for manual)
  #[arg(
    required_if_eq("mode", "manual"),
    value_parser = parse_partition
    )]
  pub partitions: Vec<Partition>,
}

/// Arguments for the install-base command
#[derive(Debug, Args)]
pub struct InstallBaseArgs {
  #[arg(long)]
  pub kernel: String,
}

/// Subcommands for bootloader
#[derive(Debug, Subcommand)]
pub enum BootloaderSubcommand {
  #[clap(name = "grub-efi")]
  GrubEfi { efidir: PathBuf },
  #[clap(name = "grub-legacy")]
  GrubLegacy { device: PathBuf },
}

/// Arguments for the locale command
#[derive(Debug, Args)]
pub struct LocaleArgs {
  pub keyboard: String,
  pub timezone: String,
  pub locales: Vec<String>,
}

/// Arguments for the networking command
#[derive(Debug, Args)]
pub struct NetworkingArgs {
  pub hostname: String,
  #[arg(long)]
  pub ipv6: bool,
}

/// Supported desktops
#[derive(Debug, ValueEnum, Copy, Clone, Ord, PartialOrd, Eq, PartialEq, Serialize, Deserialize)]
pub enum DesktopSetup {
  #[value(name = "kde", aliases = ["plasma"])]
  Kde,
  #[value(name = "gnome", aliases = ["gnome"])]
  Gnome,
  #[value(name = "None/DIY")]
  None,
}

/// Arguments for creating a new user
#[derive(Debug, Args)]
pub struct NewUserArgs {
  pub username: String,
  #[arg(long, aliases=&["has-root", "sudoer", "root"])]
  pub hasroot: bool,
  pub password: String,
  pub shell: String,
}

/// Subcommands for users
#[derive(Debug, Subcommand)]
pub enum UsersSubcommand {
  #[clap(name="new-user", aliases=&["newUser"])]
  NewUser(NewUserArgs),
  #[clap(name="root-password", aliases=&["root-pass", "rootPass"])]
  RootPass { password: String },
}
