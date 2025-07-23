{
  description = "Rust development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    rust-overlay.url = "github:oxalica/rust-overlay";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
      rust-overlay,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        overlays = [ (import rust-overlay) ];
        pkgs = import nixpkgs { inherit system overlays; };

        rustToolchain = pkgs.rust-bin.stable.latest.default.override {
          extensions = [
            "rust-src"
            "rust-analyzer"
            "clippy"
            "rustfmt"
          ];
        };
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            gcc
            openssl
            gtk4
            libadwaita
            glib
            gdk-pixbuf
            pango
            cairo
            librsvg
          ];

          nativeBuildInputs = with pkgs; [
            rustToolchain
            pkg-config
            cargo-watch
            cargo-edit
          ];

          shellHook = ''
            echo "Rust toolchain: $(rustc --version)"
            echo "Rust-analyzer: $(rust-analyzer --version)"
            export PATH=$PATH:${rustToolchain}/bin
          '';
        };
      }
    );
}
