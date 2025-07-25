use glib_build_tools::compile_resources;

fn main() {
  print!("cargo:rerun-in-changed=assets");
  print!("cargo:rerun-in-changed=assets/ui");
  print!("cargo:rerun-in-changed=assets/icons");

  compile_resources(
    &["assets", "assets/ui", "assets/icons"],
    "assets/crystallize.gresource.xml",
    "crystallize.gresource",
  );
}
