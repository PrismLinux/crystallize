use glib_build_tools::compile_resources;

fn main() {
	println!("cargo:rerun-if-changed=ui/");
	println!("cargo:rerun-if-changed=assets/");

    // Compile GLib resources
    compile_resources(
        &["ui", "assets", "assets/locales", "."],
        "crystallize.gresource.xml",
        "crystallize.gresource",
    );
}