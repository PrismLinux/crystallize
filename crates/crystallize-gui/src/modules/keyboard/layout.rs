use crate::modules::{keyboard::variant::KeyboardVariant, keymap::Keymap};
use adw::prelude::{ExpanderRowExt, PreferencesRowExt};
use glib::subclass::types::ObjectSubclassIsExt;
use gtk::glib;
use std::cell::RefCell;

glib::wrapper! {
    pub struct KeyboardLayout(ObjectSubclass<imp::KeyboardLayout>)
        @extends adw::ExpanderRow, adw::PreferencesRow, gtk::ListBoxRow, gtk::Widget,
        @implements gtk::Accessible, gtk::Actionable, gtk::Buildable, gtk::ConstraintTarget;
}

impl KeyboardLayout {
  pub fn new(country: &str, country_shorthand: &str, keymap: &Keymap) -> Self {
    let obj: Self = glib::Object::builder().build();

    obj.set_title(country);
    obj.set_subtitle(country_shorthand);

    let imp = obj.imp();
    imp.country.set(country.to_string()).unwrap();
    imp
      .country_shorthand
      .set(country_shorthand.to_string())
      .unwrap();

    // Create variants
    let mut button_group: Option<gtk::CheckButton> = None;

    for variant in &keymap.variants {
      let variant_widget = KeyboardVariant::new(
        keymap.layout,
        variant,
        country,
        country_shorthand,
        button_group.as_ref(),
      );

      if button_group.is_none() {
        button_group = Some(variant_widget.select_variant().clone());
      }

      obj.add_row(&variant_widget);
      imp.variants.borrow_mut().push(variant_widget);
    }

    obj
  }

  pub fn country(&self) -> Option<&String> {
    self.imp().country.get()
  }

  pub fn country_shorthand(&self) -> Option<&String> {
    self.imp().country_shorthand.get()
  }

  pub fn variants(&self) -> Vec<KeyboardVariant> {
    self.imp().variants.borrow().clone()
  }
}

mod imp {
  use super::*;
  use adw::subclass::prelude::{ExpanderRowImpl, PreferencesRowImpl};
  use glib::subclass::prelude::*;
  use gtk::subclass::prelude::*;
  use std::cell::OnceCell;

  #[derive(Default)]
  pub struct KeyboardLayout {
    pub country: OnceCell<String>,
    pub country_shorthand: OnceCell<String>,
    pub variants: RefCell<Vec<KeyboardVariant>>,
  }

  #[glib::object_subclass]
  impl ObjectSubclass for KeyboardLayout {
    const NAME: &'static str = "KeyboardLayout";
    type Type = super::KeyboardLayout;
    type ParentType = adw::ExpanderRow;
  }

  impl ObjectImpl for KeyboardLayout {
    fn constructed(&self) {
      self.parent_constructed();
    }
  }

  impl WidgetImpl for KeyboardLayout {}
  impl ListBoxRowImpl for KeyboardLayout {}
  impl PreferencesRowImpl for KeyboardLayout {}
  impl ExpanderRowImpl for KeyboardLayout {}
}
