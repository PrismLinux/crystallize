use clap::Parser;
use crystallize_core::{
  cli::{BootloaderSubcommand, Command, Opt, UsersSubcommand},
  config,
  modules::{
    base::{self, grub, nvidia},
    desktops, locale, network, partition, users,
  },
  utils::logging,
};

fn main() {
  human_panic::setup_panic!();
  let opt: Opt = Opt::parse();
  logging::init(opt.verbose.into());

  match opt.command {
    Command::Bootloader { subcommand } => match subcommand {
      BootloaderSubcommand::GrubEfi { efidir } => {
        grub::install_bootloader_efi(efidir);
      }
      BootloaderSubcommand::GrubLegacy { device } => {
        grub::install_bootloader_legacy(device);
      }
    },
    Command::Config { config } => {
      if let Err(e) = config::read_config(config) {
        eprintln!("Error reading config: {e}");
        std::process::exit(1);
      }
    }
    Command::CopyLive => {
      base::copy_live_config();
    }
    Command::Desktops { desktop } => {
      desktops::install_desktop_setup(desktop);
    }
    Command::Flatpak => {
      base::install_flatpak();
    }
    Command::GenFstab => {
      base::genfstab();
    }
    Command::InstallBase(args) => {
      base::install_base_packages(args.kernel);
    }
    Command::Locale(args) => {
      locale::set_locale(args.locales.join(" "));
      locale::set_keyboard(&args.keyboard);
      locale::set_timezone(&args.timezone);
    }
    Command::Networking(args) => {
      if args.ipv6 {
        network::create_hosts();
        network::enable_ipv6()
      } else {
        network::create_hosts();
      }
      network::set_hostname(&args.hostname);
    }
    Command::Nix => {
      base::install_homemgr();
    }
    Command::Nvidia => {
      nvidia::install_nvidia();
    }
    Command::Partition(args) => {
      let mut partitions = args.partitions;
      partition::partition(args.device, args.mode, args.efi, &mut partitions);
    }
    Command::SetupKeyring => {
      base::setup_archlinux_keyring().unwrap();
    }
    Command::Users { subcommand } => match subcommand {
      UsersSubcommand::NewUser(args) => {
        users::new_user(
          &args.username,
          args.hasroot,
          &args.password,
          true,
          &args.shell,
        );
      }
      UsersSubcommand::RootPass { password } => {
        users::root_pass(&password);
      }
    },
    Command::Zram { size } => {
      base::install_zram(size);
    }
  }
}
