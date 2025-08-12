use glib_build_tools::compile_resources;

fn main() {
  print!("cargo:rerun-in-changed=assets");
  print!("cargo:rerun-in-changed=assets/ui");
  print!("cargo:rerun-in-changed=assets/icons");

  compile_resources(
    &["assets", "assets/ui", "assets/ui/locale", "assets/icons"],
    "assets/crystallize.gresource.xml",
    "crystallize.gresource",
  );
}
