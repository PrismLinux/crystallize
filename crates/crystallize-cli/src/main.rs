use clap::Parser;
use crystallize_core::{
  cli::{Command, Opt},
  config,
  utils::logging,
};

fn main() {
  human_panic::setup_panic!();
  let opt: Opt = Opt::parse();
  logging::init(opt.verbose.into());

  match opt.command {
    Command::Config { config } => {
      if let Err(e) = config::read_config(config) {
        eprintln!("Error reading config: {e}");
        std::process::exit(1);
      }
    }
  }
}
