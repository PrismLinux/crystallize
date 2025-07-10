use crate::modules::keyboard::types::KeyboardSelection;
use glib::prelude::ObjectExt;
use glib::subclass::prelude::ObjectSubclassIsExt;
use gtk::prelude::CheckButtonExt;
use gtk::{CompositeTemplate, glib};
use std::cell::OnceCell;

glib::wrapper! {
    pub struct KeyboardVariant(ObjectSubclass<imp::KeyboardVariant>)
        @extends adw::PreferencesRow, adw::ActionRow, gtk::ListBoxRow, gtk::Widget,
        @implements gtk::Accessible, gtk::Actionable, gtk::Buildable, gtk::ConstraintTarget;
}

impl KeyboardVariant {
  pub fn new(
    layout: &str,
    variant: &str,
    country: &str,
    country_shorthand: &str,
    button_group: Option<&gtk::CheckButton>,
  ) -> Self {
    let obj: Self = glib::Object::builder().build();

    obj.set_property("title", variant);
    obj.set_property("subtitle", format!("{country} - {country_shorthand}"));

    let imp = obj.imp();
    imp.layout.set(layout.to_string()).unwrap();
    imp.variant.set(variant.to_string()).unwrap();
    imp.country.set(country.to_string()).unwrap();
    imp
      .country_shorthand
      .set(country_shorthand.to_string())
      .unwrap();

    if let Some(group) = button_group {
      imp.select_variant.set_group(Some(group));
    }

    obj
  }

  pub fn layout(&self) -> String {
    self.imp().layout.get().unwrap().clone()
  }

  pub fn variant(&self) -> String {
    self.imp().variant.get().unwrap().clone()
  }

  pub fn country(&self) -> String {
    self.imp().country.get().unwrap().clone()
  }

  pub fn country_shorthand(&self) -> String {
    self.imp().country_shorthand.get().unwrap().clone()
  }

  pub fn select_variant(&self) -> &gtk::CheckButton {
    &self.imp().select_variant
  }

  pub fn get_selection(&self) -> KeyboardSelection {
    KeyboardSelection {
      country: self.country(),
      country_shorthand: self.country_shorthand(),
      variant: self.variant(),
    }
  }
}

mod imp {
  use super::*;
  use adw::subclass::prelude::*;

  #[derive(Debug, CompositeTemplate)]
  #[template(resource = "/org/crystallinux/crystallize/ui/keyboard/keyboard_variant.ui")]
  pub struct KeyboardVariant {
    #[template_child]
    pub select_variant: TemplateChild<gtk::CheckButton>,

    pub layout: OnceCell<String>,
    pub variant: OnceCell<String>,
    pub country: OnceCell<String>,
    pub country_shorthand: OnceCell<String>,
  }

  #[glib::object_subclass]
  impl ObjectSubclass for KeyboardVariant {
    const NAME: &'static str = "KeyboardVariant";
    type Type = super::KeyboardVariant;
    type ParentType = adw::ActionRow;
    type Interfaces = ();

    fn class_init(klass: &mut Self::Class) {
      klass.bind_template();
    }

    fn instance_init(obj: &glib::subclass::InitializingObject<Self>) {
      obj.init_template();
    }

    fn new() -> Self {
      Self {
        select_variant: TemplateChild::default(),
        layout: OnceCell::new(),
        variant: OnceCell::new(),
        country: OnceCell::new(),
        country_shorthand: OnceCell::new(),
      }
    }
  }

  impl ObjectImpl for KeyboardVariant {
    fn constructed(&self) {
      self.parent_constructed();
    }
  }

  impl WidgetImpl for KeyboardVariant {}
  impl ListBoxRowImpl for KeyboardVariant {}
  impl adw::subclass::preferences_row::PreferencesRowImpl for KeyboardVariant {}
  impl adw::subclass::action_row::ActionRowImpl for KeyboardVariant {}
}
