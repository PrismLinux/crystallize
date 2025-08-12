use adw::prelude::*;
use adw::subclass::prelude::*;
use gtk::{CompositeTemplate, ResponseType, glib};
use std::cell::{OnceCell, RefCell};

mod imp {
  use super::*;

  #[derive(Default, CompositeTemplate)]
  #[template(resource = "/org/crystalnetwork/crystallize/ui/locale/locale_dialog.ui")]
  pub struct LocaleDialog {
    #[template_child]
    pub search_entry: TemplateChild<gtk::SearchEntry>,
    #[template_child]
    pub locale_list: TemplateChild<gtk::ListBox>,
    #[template_child]
    pub stack: TemplateChild<gtk::Stack>,
    #[template_child]
    pub empty_page: TemplateChild<adw::StatusPage>,
    #[template_child]
    pub cancel_button: TemplateChild<gtk::Button>,
    #[template_child]
    pub add_button: TemplateChild<gtk::Button>,

    pub locales_model: OnceCell<gtk::StringList>,
    pub filter_model: OnceCell<gtk::FilterListModel>,
    pub selected_locale: RefCell<Option<String>>,
    pub response_callback: RefCell<Option<Box<dyn Fn(ResponseType) + 'static>>>,
  }

  #[glib::object_subclass]
  impl ObjectSubclass for LocaleDialog {
    const NAME: &'static str = "LocaleDialog";
    type Type = super::LocaleDialog;
    type ParentType = adw::Dialog;

    fn class_init(klass: &mut Self::Class) {
      klass.bind_template();
      klass.bind_template_callbacks();
    }

    fn instance_init(obj: &glib::subclass::InitializingObject<Self>) {
      obj.init_template();
    }
  }

  #[gtk::template_callbacks]
  impl LocaleDialog {
    #[template_callback]
    fn on_search_changed(&self, entry: &gtk::SearchEntry) {
      let obj = self.obj();
      obj.update_filter(&entry.text());
    }

    #[template_callback]
    fn on_row_activated(&self, _listbox: &gtk::ListBox, row: &gtk::ListBoxRow) {
      let obj = self.obj();
      obj.select_locale_from_row(row);
      obj.emit_response(ResponseType::Accept);
    }

    #[template_callback]
    fn on_locale_selected(&self, _listbox: &gtk::ListBox, row: Option<&gtk::ListBoxRow>) {
      let obj = self.obj();

      if let Some(row) = row {
        obj.select_locale_from_row(row);
      } else {
        obj.clear_selection();
      }

      obj.update_add_button_sensitivity();
    }

    #[template_callback]
    fn on_cancel_clicked(&self, _button: &gtk::Button) {
      let obj = self.obj();
      obj.emit_response(ResponseType::Cancel);
    }

    #[template_callback]
    fn on_add_clicked(&self, _button: &gtk::Button) {
      let obj = self.obj();
      obj.emit_response(ResponseType::Accept);
    }
  }

  impl ObjectImpl for LocaleDialog {
    fn constructed(&self) {
      self.parent_constructed();

      let obj = self.obj();
      obj.setup_dialog();
      obj.setup_locale_list();
      obj.setup_search();
    }

    fn dispose(&self) {
      // Clean up references
      self.selected_locale.replace(None);
      self.response_callback.replace(None);
    }
  }

  impl WidgetImpl for LocaleDialog {}
  impl AdwDialogImpl for LocaleDialog {}
}

glib::wrapper! {
    pub struct LocaleDialog(ObjectSubclass<imp::LocaleDialog>)
        @extends adw::Dialog, gtk::Widget,
        @implements gtk::Accessible, gtk::Buildable, gtk::ConstraintTarget;
}

impl LocaleDialog {
  /// Creates a new LocaleDialog
  pub fn new<I: IsA<gtk::Window> + IsA<gtk::Widget>>(parent: &I) -> Self {
    let dialog: Self = glib::Object::builder().build();
    dialog.set_parent(parent);
    dialog
  }

  /// Setup the dialog buttons and properties
  fn setup_dialog(&self) {
    let imp = self.imp();

    // Set up button states
    imp.add_button.set_sensitive(false);

    // Set dialog properties
    self.set_title("Select Locale");
  }

  /// Setup the locale list with available locales
  fn setup_locale_list(&self) {
    let imp = self.imp();

    // Create string list model with locales
    let locales = glib::language_names();
    let locale_strs: Vec<&str> = locales.iter().map(|s| s.as_str()).collect();
    let string_list = gtk::StringList::new(&locale_strs);
    imp.locales_model.set(string_list.clone()).unwrap();

    // Create custom filter for search
    let filter = gtk::CustomFilter::new(|_| true);
    let filter_model = gtk::FilterListModel::new(Some(string_list), Some(filter.clone()));
    imp.filter_model.set(filter_model.clone()).unwrap();

    // Bind the model to the listbox
    imp.locale_list.bind_model(
      Some(&filter_model),
      glib::clone!(move |item| {
        let locale_name = item.downcast_ref::<gtk::StringObject>().unwrap().string();

        Self::create_locale_row(&locale_name)
      }),
    );

    // Update stack visibility based on model
    self.update_empty_state();
  }

  /// Create a locale row widget
  fn create_locale_row(locale_name: &str) -> gtk::Widget {
    let row = adw::ActionRow::builder()
      .title(locale_name)
      .activatable(true)
      .build();

    // Add locale flag or icon if available
    if let Some(icon) = Self::get_locale_icon(locale_name) {
      let image = gtk::Image::from_icon_name(&icon);
      row.add_prefix(&image);
    }

    row.upcast()
  }

  /// Get an appropriate icon for a locale (if available)
  fn get_locale_icon(_locale_name: &str) -> Option<String> {
    // You could implement locale-to-flag mapping here
    // For now, just return a generic icon
    Some("preferences-desktop-locale-symbolic".to_string())
  }

  /// Setup search functionality
  fn setup_search(&self) {
    let imp = self.imp();

    // Set search entry as initial focus
    imp.search_entry.grab_focus();

    // Enable search delay to avoid filtering on every keystroke
    imp.search_entry.set_search_delay(300);
  }

  /// Update the filter based on search text
  fn update_filter(&self, search_text: &str) {
    let imp = self.imp();

    if let Some(filter_model) = imp.filter_model.get() {
      if let Some(filter) = filter_model.filter() {
        let custom_filter = filter.downcast::<gtk::CustomFilter>().unwrap();

        if search_text.is_empty() {
          custom_filter.set_filter_func(|_| true);
        } else {
          let search_text_lower = search_text.to_lowercase();
          custom_filter.set_filter_func(move |item| {
            let locale_name = item
              .downcast_ref::<gtk::StringObject>()
              .unwrap()
              .string()
              .to_lowercase();
            locale_name.contains(&search_text_lower)
          });
        }
      }
    }

    self.update_empty_state();
  }

  /// Select a locale from a list row
  fn select_locale_from_row(&self, row: &gtk::ListBoxRow) {
    if let Some(action_row) = row
      .child()
      .and_then(|c| c.downcast::<adw::ActionRow>().ok())
    {
      let locale_name = action_row.title().to_string();
      self.set_selected_locale(Some(locale_name));
    }
  }

  /// Clear the current selection
  fn clear_selection(&self) {
    self.set_selected_locale(None);
  }

  /// Set the selected locale
  fn set_selected_locale(&self, locale: Option<String>) {
    let imp = self.imp();
    imp.selected_locale.replace(locale);
    self.update_add_button_sensitivity();
  }

  /// Update the sensitivity of the add button
  fn update_add_button_sensitivity(&self) {
    let imp = self.imp();
    let has_selection = imp.selected_locale.borrow().is_some();
    imp.add_button.set_sensitive(has_selection);
  }

  /// Update the empty state visibility
  fn update_empty_state(&self) {
    let imp = self.imp();

    if let Some(filter_model) = imp.filter_model.get() {
      let has_items = filter_model.n_items() > 0;

      if has_items {
        imp.stack.set_visible_child(&*imp.locale_list);
      } else {
        imp.stack.set_visible_child(&*imp.empty_page);
      }
    }
  }

  /// Get the currently selected locale
  pub fn selected_locale(&self) -> Option<String> {
    self.imp().selected_locale.borrow().clone()
  }

  /// Present the dialog
  pub fn present(&self) {
    AdwDialogExt::present(self, None::<&gtk::Widget>);
  }

  /// Connect to response events (simplified version)
  pub fn connect_response<F>(&self, f: F)
  where
    F: Fn(&Self, ResponseType) + 'static,
  {
    let imp = self.imp();
    let callback = Box::new(glib::clone!(
      #[weak(rename_to = dialog)]
      self,
      move |response| {
        f(&dialog, response);
      }
    ));
    imp.response_callback.replace(Some(callback));
  }

  /// Emit a response
  fn emit_response(&self, response_type: ResponseType) {
    let imp = self.imp();
    if let Some(callback) = imp.response_callback.borrow().as_ref() {
      callback(response_type);
    }
  }

  /// Close the dialog (convenience method)
  pub fn close(&self) {
    AdwDialogExt::close(self);
  }
}

impl Default for LocaleDialog {
  fn default() -> Self {
    glib::Object::builder().build()
  }
}
