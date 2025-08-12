use adw::prelude::*;
use adw::subclass::prelude::*;
use gtk::{CompositeTemplate, ResponseType, glib};
use std::cell::{OnceCell, RefCell};

mod dialog;
use dialog::LocaleDialog;

mod imp {
  use super::*;

  #[derive(Debug, Default, CompositeTemplate)]
  #[template(resource = "/org/crystalnetwork/crystallize/ui/locale_screen.ui")]
  pub struct LocaleScreen {
    #[template_child]
    pub main_locale_list: TemplateChild<gtk::StringList>,
    #[template_child]
    pub other_locale_list: TemplateChild<adw::ExpanderRow>,
    #[template_child]
    pub locale_search_button: TemplateChild<gtk::Button>,
    #[template_child]
    pub empty_locales: TemplateChild<adw::ActionRow>,

    pub additional_locales_listbox: OnceCell<gtk::ListBox>,
    pub is_valid: RefCell<bool>,
    pub selected_locales: RefCell<Vec<String>>,
  }

  #[glib::object_subclass]
  impl ObjectSubclass for LocaleScreen {
    const NAME: &'static str = "LocaleScreen";
    type Type = super::LocaleScreen;
    type ParentType = adw::Bin;

    fn class_init(klass: &mut Self::Class) {
      klass.bind_template();
      klass.bind_template_callbacks();
    }

    fn instance_init(obj: &glib::subclass::InitializingObject<Self>) {
      obj.init_template();
    }
  }

  #[gtk::template_callbacks]
  impl LocaleScreen {
    #[template_callback]
    fn on_search_button_clicked(&self, _button: &gtk::Button) {
      let obj = self.obj();
      obj.show_locale_dialog();
    }
  }

  impl ObjectImpl for LocaleScreen {
    fn constructed(&self) {
      self.parent_constructed();

      let obj = self.obj();
      obj.initialize_state();
      obj.setup_locales();
    }

    fn dispose(&self) {
      // Clean up any remaining references
      self.selected_locales.borrow_mut().clear();
    }
  }

  impl WidgetImpl for LocaleScreen {}
  impl BinImpl for LocaleScreen {}
}

glib::wrapper! {
    pub struct LocaleScreen(ObjectSubclass<imp::LocaleScreen>)
        @extends adw::Bin, gtk::Widget,
        @implements gtk::Accessible, gtk::Buildable, gtk::ConstraintTarget;
}

impl LocaleScreen {
  /// Creates a new LocaleScreen instance
  pub fn new() -> Self {
    glib::Object::builder().build()
  }

  /// Initialize the widget state
  fn initialize_state(&self) {
    let imp = self.imp();
    imp.is_valid.replace(true);
    imp.selected_locales.replace(Vec::new());
  }

  /// Setup the main locale list with system languages
  fn setup_locales(&self) {
    let imp = self.imp();
    let locales = glib::language_names();
    let locale_strs: Vec<&str> = locales.iter().map(|gstring| gstring.as_ref()).collect();

    // Clear existing items and add new ones
    imp
      .main_locale_list
      .splice(0, imp.main_locale_list.n_items(), &locale_strs);
    self.set_valid(true);
  }

  /// Show the locale selection dialog
  fn show_locale_dialog(&self) {
    let Some(parent_window) = self
      .root()
      .and_then(|root| root.downcast::<gtk::Window>().ok())
    else {
      eprintln!("Could not find parent window for locale dialog");
      return;
    };

    let locale_dialog = LocaleDialog::new(&parent_window);

    locale_dialog.connect_response(glib::clone!(
      #[weak(rename_to = this)]
      self,
      move |dialog, response| {
        match response {
          ResponseType::Accept => {
            if let Some(locale_name) = dialog.selected_locale() {
              this.add_additional_locale(&locale_name);
            }
          }
          _ => {} // User cancelled or closed dialog
        }
        dialog.close();
      }
    ));

    locale_dialog.present();
  }

  /// Set the validity state of the locale screen
  pub fn set_valid(&self, valid: bool) {
    let imp = self.imp();
    imp.is_valid.replace(valid);

    // Emit property change signal if needed
    self.notify("is-valid");
  }

  /// Check if the current locale configuration is valid
  pub fn is_valid(&self) -> bool {
    *self.imp().is_valid.borrow()
  }

  /// Get all selected additional locales
  pub fn additional_locales(&self) -> Vec<String> {
    self.imp().selected_locales.borrow().clone()
  }

  /// Add a new locale to the additional locales list
  pub fn add_additional_locale(&self, locale_name: &str) {
    let imp = self.imp();

    // Check if locale is already added
    if imp
      .selected_locales
      .borrow()
      .contains(&locale_name.to_string())
    {
      return;
    }

    // Initialize the listbox if it doesn't exist
    let listbox = imp.additional_locales_listbox.get_or_init(|| {
      let listbox = gtk::ListBox::builder()
        .css_classes(["boxed-list"])
        .selection_mode(gtk::SelectionMode::None)
        .build();

      imp.other_locale_list.set_child(Some(&listbox));
      listbox
    });

    // Create the locale row
    let row = self.create_locale_row(locale_name);
    listbox.append(&row);

    // Update the selected locales list
    imp
      .selected_locales
      .borrow_mut()
      .push(locale_name.to_string());

    // Update validity if needed
    self.validate_locales();
  }

  /// Remove a locale from the additional locales list
  fn remove_additional_locale(&self, locale_name: &str, row: &adw::ActionRow) {
    let imp = self.imp();

    // Remove from the selected locales list
    imp
      .selected_locales
      .borrow_mut()
      .retain(|l| l != locale_name);

    if let Some(listbox) = imp.additional_locales_listbox.get() {
      listbox.remove(row);

      // If no more additional locales, show empty state
      if imp.selected_locales.borrow().is_empty() {
        imp.other_locale_list.set_child(Some(&*imp.empty_locales));
      }
    }

    self.validate_locales();
  }

  /// Create a new locale row with remove button
  fn create_locale_row(&self, locale_name: &str) -> adw::ActionRow {
    let row = adw::ActionRow::builder().title(locale_name).build();

    let remove_button = gtk::Button::builder()
      .icon_name("user-trash-symbolic")
      .tooltip_text(&format!("Remove {}", locale_name))
      .valign(gtk::Align::Center)
      .css_classes(["flat", "circular"])
      .build();

    // Connect remove button with proper cleanup
    let locale_name_owned = locale_name.to_string();
    remove_button.connect_clicked(glib::clone!(
      #[weak(rename_to = this)]
      self,
      #[weak]
      row,
      move |_| {
        this.remove_additional_locale(&locale_name_owned, &row);
      }
    ));

    row.add_suffix(&remove_button);
    row
  }

  /// Validate the current locale configuration
  fn validate_locales(&self) {
    // Add your validation logic here
    // For example: check if at least one locale is selected
    let is_valid = true; // Replace with actual validation
    self.set_valid(is_valid);
  }

  /// Reset the locale screen to its initial state
  pub fn reset(&self) {
    let imp = self.imp();

    // Clear additional locales
    imp.selected_locales.borrow_mut().clear();

    // Reset UI to empty state
    imp.other_locale_list.set_child(Some(&*imp.empty_locales));

    // Reset validity
    self.set_valid(true);
  }
}

impl Default for LocaleScreen {
  fn default() -> Self {
    Self::new()
  }
}
