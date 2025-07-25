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
    Command::Partition(args) => {
      let mut partitions = args.partitions;
      partition::partition(args.device, args.mode, args.efi, &mut partitions);
    }
    Command::SetupKeyring => {
      base::setup_archlinux_keyring().unwrap();
    }
    Command::InstallBase(args) => {
      base::install_base_packages(args.kernel);
    }
    Command::GenFstab => {
      base::genfstab();
    }
    Command::Bootloader { subcommand } => match subcommand {
      BootloaderSubcommand::GrubEfi { efidir } => {
        grub::install_bootloader_efi(efidir);
      }
      BootloaderSubcommand::GrubLegacy { device } => {
        grub::install_bootloader_legacy(device);
      }
    },
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
    Command::Zram { size } => {
      base::install_zram(size);
    }
    Command::CopyLive => {
      base::copy_live_config();
    }
    Command::Nvidia => {
      nvidia::install_nvidia();
    }
    Command::Config { config } => {
      config::read_config(config);
    }
    Command::Desktops { desktop } => {
      desktops::install_desktop_setup(desktop);
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
    Command::Flatpak => {
      base::install_flatpak();
    }
    Command::Nix => {
      base::install_homemgr();
    }
  }
}
