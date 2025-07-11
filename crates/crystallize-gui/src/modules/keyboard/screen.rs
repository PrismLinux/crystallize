use crate::modules::keyboard::layout::KeyboardLayout;
use crate::modules::keyboard::types::KeyboardSelection;
use crate::modules::keyboard::variant::KeyboardVariant;
use crate::modules::keymap::Keymap;
use adw;
use adw::prelude::PreferencesGroupExt;
use glib::subclass::types::ObjectSubclassIsExt;
use gtk::CompositeTemplate;
use gtk::{glib, prelude::*};
use log::error;
use std::cell::RefCell;
use std::process::Command;

glib::wrapper! {
    pub struct KeyboardScreen(ObjectSubclass<imp::KeyboardScreen>)
        @extends adw::Bin, gtk::Widget,
        @implements gtk::Accessible, gtk::Buildable, gtk::ConstraintTarget;
}

impl KeyboardScreen {
  pub fn new() -> Self {
    glib::Object::builder().build()
  }

  pub fn setup_with_keymaps(&self, keymaps: Vec<Keymap>) {
    let imp = self.imp();

    // Clear existing layouts
    imp.layout_list.remove_all();

    // Add new layouts
    for keymap in keymaps {
      let layout = KeyboardLayout::new(&keymap.layout, &keymap.backend_layout, &keymap);

      imp.layout_list.append(&layout);

      // Set default to US layout if available
      if keymap.backend_layout == "us" {
        for variant in layout.variants() {
          if variant.variant() == "normal" {
            self.select_variant(&variant);
            break;
          }
        }
      }
    }
  }

  pub fn select_variant(&self, variant: &KeyboardVariant) {
    let imp = self.imp();

    let selection = variant.get_selection();
    imp.current_selection.replace(Some(selection.clone()));

    // Update preview
    if imp.country_preview_list.n_items() > 0 {
      imp
        .country_preview_list
        .splice(0, imp.country_preview_list.n_items(), &[&selection.country]);
    } else {
      imp.country_preview_list.append(&selection.country);
    }

    if selection.variant != "normal" {
      imp.variant_preview.set_visible(true);
      if imp.variant_preview_list.n_items() > 0 {
        imp.variant_preview_list.splice(
          0,
          imp.variant_preview_list.n_items(),
          &[&selection.variant],
        );
      } else {
        imp.variant_preview_list.append(&selection.variant);
      }
    } else {
      imp.variant_preview.set_visible(false);
    }

    imp.preview.set_description(Some(&format!(
      "Test \"{} - {}\"",
      selection.country_shorthand, selection.variant
    )));

    // Set keyboard layout
    self.set_xkbmap(&selection.country_shorthand, &selection.variant);
  }

  fn set_xkbmap(&self, layout: &str, variant: &str) {
    let is_wayland = std::env::var("WAYLAND_DISPLAY").is_ok();
    let is_sleex = std::env::var("XDG_CURRENT_DESKTOP")
      .map(|v| v == "Sleex")
      .unwrap_or(false);

    let cmd_result = if is_sleex {
      Command::new("hyprctl")
        .args(["keyword", "input:kb_layout", layout])
        .output()
    } else if is_wayland {
      let keymap = if variant == "normal" {
        layout.to_string()
      } else {
        format!("{layout}+{variant}")
      };

      Command::new("localectl")
        .args(["set-keymap", &keymap])
        .output()
    } else {
      // Xorg
      let mut cmd = Command::new("setxkbmap");
      cmd.arg(layout);

      if variant != "normal" {
        cmd.args(["-variant", variant]);
      }

      cmd.output()
    };

    match cmd_result {
      Ok(output) => {
        if !output.status.success() {
          error!(
            "Failed to set keyboard layout. Command exited with status {}: {}",
            output.status,
            String::from_utf8_lossy(&output.stderr)
          );
        }
      }
      Err(e) => {
        error!("Failed to execute command to set keyboard layout: {e}");
      }
    }
  }

  pub fn get_current_selection(&self) -> Option<KeyboardSelection> {
    self.imp().current_selection.borrow().clone()
  }
}

mod imp {
  use super::*;
  use adw::subclass::prelude::*;

  #[derive(Debug, CompositeTemplate)]
  #[template(resource = "/org/crystalnetwork/crystallize/ui/keyboard/keyboard_screen.ui")]
  pub struct KeyboardScreen {
    #[template_child]
    pub preview: TemplateChild<adw::PreferencesGroup>,
    #[template_child]
    pub keyboard_search_button: TemplateChild<gtk::Button>,
    #[template_child]
    pub country_preview_list: TemplateChild<gtk::StringList>,
    #[template_child]
    pub variant_preview: TemplateChild<adw::ComboRow>,
    #[template_child]
    pub variant_preview_list: TemplateChild<gtk::StringList>,

    pub search_dialog: gtk::Dialog,
    pub layout_list: gtk::ListBox,
    pub select_variant_button: gtk::Button,

    pub current_selection: RefCell<Option<KeyboardSelection>>,
  }

  #[glib::object_subclass]
  impl ObjectSubclass for KeyboardScreen {
    const NAME: &'static str = "KeyboardScreen";
    type Type = super::KeyboardScreen;
    type Interfaces = ();
    type ParentType = adw::Bin;

    fn class_init(klass: &mut Self::Class) {
      klass.bind_template();
    }

    fn instance_init(obj: &glib::subclass::InitializingObject<Self>) {
      obj.init_template();
    }
  }

  // TODO: Refactor to new Version over Deprecated
  impl ObjectImpl for KeyboardScreen {
    fn constructed(&self) {
      self.parent_constructed();

      // Connect search button to show dialog
      self
        .keyboard_search_button
        .connect_clicked(glib::clone!(@weak self as imp => move |_| {
            imp.search_dialog.present();
        }));

      // Connect select button to hide dialog
      self
        .select_variant_button
        .connect_clicked(glib::clone!(@weak self as imp => move |_| {
            imp.search_dialog.close();
        }));
    }
  }

  impl Default for KeyboardScreen {
    fn default() -> Self {
      let builder =
        gtk::Builder::from_resource("/org/crystalnetwork/crystallize/ui/keyboard/keyboard_dialog.ui");
      Self {
        preview: Default::default(),
        keyboard_search_button: Default::default(),
        country_preview_list: Default::default(),
        variant_preview: Default::default(),
        variant_preview_list: Default::default(),
        search_dialog: builder
          .object("search_dialog")
          .expect("Could not get search_dialog"),
        layout_list: builder
          .object("layout_list")
          .expect("Could not get layout_list"),
        select_variant_button: builder
          .object("select_variant_button")
          .expect("Could not get select_variant_button"),
        current_selection: RefCell::new(None),
      }
    }
  }

  impl WidgetImpl for KeyboardScreen {}
  impl BinImpl for KeyboardScreen {}
}
