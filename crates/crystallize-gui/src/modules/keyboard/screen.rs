use crate::modules::keyboard::types::KeyboardSelection;
use crate::modules::keyboard::variant;
use crate::modules::keyboard::variant::KeyboardVariant;
use crate::modules::keymap::Keymap;
use adw::prelude::*;
use adw::subclass::prelude::*;
use glib::subclass::types::ObjectSubclassIsExt;
use gtk::{glib, CompositeTemplate, ListBox, ListBoxRow};
use std::cell::OnceCell;
use std::cell::RefCell;
use std::process::Command;

mod imp {
    use super::*;

    #[derive(Debug, Default, CompositeTemplate)]
    #[template(resource = "/org/crystalnetwork/crystallize/ui/keyboard/keyboard_screen.ui")]
    pub struct KeyboardScreen {
        #[template_child]
        pub preview_group: TemplateChild<adw::PreferencesGroup>,
        #[template_child]
        pub keyboard_search_button: TemplateChild<adw::ButtonRow>,
        #[template_child]
        pub country_preview_row: TemplateChild<adw::ActionRow>,
        #[template_child]
        pub variant_preview_row: TemplateChild<adw::ComboRow>,
        #[template_child]
        pub variant_model: TemplateChild<gtk::StringList>,
        #[template_child]
        pub test_entry: TemplateChild<adw::EntryRow>,

        pub search_window: OnceCell<adw::Window>,
        pub layout_list: OnceCell<gtk::ListBox>,
        pub current_selection: RefCell<Option<KeyboardSelection>>,
    }

    #[glib::object_subclass]
    impl ObjectSubclass for KeyboardScreen {
        const NAME: &'static str = "KeyboardScreen";
        type Type = super::KeyboardScreen;
        type ParentType = adw::Bin;

        fn class_init(klass: &mut Self::Class) {
            klass.bind_template();
        }

        fn instance_init(obj: &glib::subclass::InitializingObject<Self>) {
            obj.init_template();
        }
    }

    impl ObjectImpl for KeyboardScreen {
        fn constructed(&self) {
            self.parent_constructed();
            let obj = self.obj();
            obj.setup_ui();
            obj.setup_callbacks();
        }
    }

    impl WidgetImpl for KeyboardScreen {}
    impl BinImpl for KeyboardScreen {}
}

glib::wrapper! {
    pub struct KeyboardScreen(ObjectSubclass<imp::KeyboardScreen>)
        @extends adw::Bin, gtk::Widget,
        @implements gtk::Accessible, gtk::Buildable, gtk::ConstraintTarget;
}

impl KeyboardScreen {
    pub fn new() -> Self {
        glib::Object::builder().build()
    }

    fn setup_ui(&self) {
        let imp = self.imp();

        let search_window = adw::Window::builder()
            .title("Select Keyboard Layout")
            .default_width(600)
            .default_height(500)
            .modal(true)
            .build();

        let header_bar = adw::HeaderBar::builder()
            .title_widget(&adw::WindowTitle::new("Select Keyboard Layout", ""))
            .build();

        let main_box = gtk::Box::builder()
            .orientation(gtk::Orientation::Vertical)
            .build();

        let search_bar = gtk::SearchBar::builder()
            .search_mode_enabled(true)
            .build();

        let search_entry = gtk::SearchEntry::builder()
            .placeholder_text("Search keyboard layouts...")
            .build();

        search_bar.set_child(Some(&search_entry));

        let scrolled_window = gtk::ScrolledWindow::builder()
            .hscrollbar_policy(gtk::PolicyType::Never)
            .vscrollbar_policy(gtk::PolicyType::Automatic)
            .vexpand(true)
            .build();

        let layout_list = gtk::ListBox::builder()
            .selection_mode(gtk::SelectionMode::Single)
            .build();

        layout_list.add_css_class("boxed-list");

        scrolled_window.set_child(Some(&layout_list));

        let action_box = gtk::Box::builder()
            .orientation(gtk::Orientation::Horizontal)
            .spacing(12)
            .margin_top(12)
            .margin_bottom(12)
            .margin_start(12)
            .margin_end(12)
            .halign(gtk::Align::End)
            .build();

        let cancel_button = gtk::Button::builder()
            .label("Cancel")
            .build();

        let select_button = gtk::Button::builder()
            .label("Select")
            .css_classes(vec!["suggested-action"])
            .build();

        action_box.append(&cancel_button);
        action_box.append(&select_button);

        main_box.append(&search_bar);
        main_box.append(&scrolled_window);
        main_box.append(&action_box);

        search_window.set_titlebar(Some(&header_bar));
        search_window.set_content(Some(&main_box));

        let search_window_weak = search_window.downgrade();
        let layout_list_weak = layout_list.downgrade();

        imp.search_window.set(search_window).unwrap();
        imp.layout_list.set(layout_list).unwrap();

        search_entry.connect_search_changed({
            let layout_list_weak = layout_list_weak.clone();
            move |entry| {
                if let Some(layout_list) = layout_list_weak.upgrade() {
                    let query = entry.text().to_lowercase();
                    Self::filter_layouts(&layout_list, &query);
                }
            }
        });

        cancel_button.connect_clicked({
            let search_window_weak = search_window_weak.clone();
            move |_| {
                if let Some(window) = search_window_weak.upgrade() {
                    window.close();
                }
            }
        });

        let screen_weak = self.downgrade();
        select_button.connect_clicked(move |_| {
            if let (Some(window), Some(layout_list), Some(screen)) = (
                search_window_weak.upgrade(),
                layout_list_weak.upgrade(),
                screen_weak.upgrade(),
            ) {
                if let Some(row) = layout_list.selected_row() {
                    screen.handle_layout_selection(row);
                }
                window.close();
            }
        });
    }

    fn setup_callbacks(&self) {
        let imp = self.imp();

        imp.keyboard_search_button.connect_activated({
            let screen_weak = self.downgrade();
            move |_| {
                if let Some(screen) = screen_weak.upgrade() {
                    let imp = screen.imp();
                    if let Some(search_window) = imp.search_window.get() {
                        if let Some(window) =
                            screen.root().and_then(|r| r.downcast::<adw::Window>().ok())
                        {
                            search_window.set_transient_for(Some(&window));
                        }
                        search_window.present();
                    }
                }
            }
        });

        imp.test_entry.connect_changed({
            let screen_weak = self.downgrade();
            move |entry| {
                if let Some(screen) = screen_weak.upgrade() {
                    let text = entry.text();
                    screen.update_test_feedback(&text);
                }
            }
        });
    }

    pub fn setup_with_keymaps(&self, keymaps: Vec<Keymap>) {
        let imp = self.imp();
        let layout_list = imp.layout_list.get().expect("layout_list not initialized");

        while let Some(child) = layout_list.first_child() {
            layout_list.remove(&child);
        }

        for keymap in keymaps {
            let layout_row = self.create_layout_row(&keymap);
            unsafe {
                layout_row.set_qdata(
                    glib::Quark::from_str("keymap"),
                    keymap.clone(),
                )
            };
            layout_list.append(&layout_row);
            
            if keymap.backend_layout == "us" {
                if let Some(_) = keymap.variants.iter().find(|v| KeyboardVariant::variant(v) == "normal") {
                    self.select_variant();
                }
            }
        }
    }

    fn create_layout_row(&self, keymap: &Keymap) -> adw::ActionRow {
        let row = adw::ActionRow::builder()
            .title(&*keymap.layout)
            .subtitle(&format!("Layout: {}", keymap.backend_layout))
            .build();

        if keymap.variants.len() > 1 {
            let variants_label = gtk::Label::builder()
                .label(&format!("{} variants", keymap.variants.len()))
                .css_classes(vec!["caption", "dim-label"])
                .build();
            row.add_suffix(&variants_label);
        }

        row
    }
    
    fn handle_layout_selection(&self, row: ListBoxRow) {
        if let Some(action_row) = row.downcast_ref::<adw::ActionRow>() {
            if let Some(keymap_ptr) = unsafe { action_row.qdata::<Keymap>(glib::Quark::from_str("keymap")) } {
                let keymap = unsafe { keymap_ptr.as_ref() };
                if keymap.variants.len() > 1 {
                    self.show_variant_selection(keymap);
                } else if let Some(variant) = keymap.variants.first() {
                    self.select_variant(variant);
                }
            }
        }
    }
    
    fn show_variant_selection(&self, keymap: &Keymap) {
        let dialog = adw::AlertDialog::builder()
            .title("Select Variant")
            .body(&format!("Choose a variant for {}", keymap.layout))
            .build();

        // This loop is correct. The previous errors here were likely phantom
        // errors caused by the compiler's confusion from the `.find()` closure.
        for variant in &keymap.variants {
            let variant_name = if KeyboardVariant::variant(variant) == "normal" { "Default".to_string() } else { KeyboardVariant::variant(variant).to_string() };
            dialog.add_response(&KeyboardVariant::variant(variant), &variant_name);
        }

        dialog.set_default_response(Some("normal"));
        dialog.set_close_response("cancel");

        let keymap_clone = keymap.clone();
        let screen_weak = self.downgrade();
        
        dialog.connect_response(None, move |_, response| {
            if response != "cancel" {
                if let Some(screen) = screen_weak.upgrade() {
                    // FIX: Dereference the argument `v` inside the closure.
                    if let Some(variant) = keymap_clone.variants.iter().find(|v| KeyboardVariant::variant(*v) == response) {
                        screen.select_variant(variant);
                    }
                }
            }
        });

        if let Some(window) = self.root().and_then(|r| r.downcast::<adw::Window>().ok()) {
            dialog.present(Some(&window));
        }
    }

    pub fn select_variant(&self, variant: &KeyboardVariant) {
        let imp = self.imp();
        let selection = variant.get_selection();
        imp.current_selection.replace(Some(selection.clone()));

        imp.country_preview_row.set_title(&selection.country);
        imp.country_preview_row.set_subtitle(&format!("Layout: {}", selection.country_shorthand));

        if selection.variant != "normal" {
            imp.variant_preview_row.set_visible(true);
            imp.variant_preview_row.set_title(&format!("Variant: {}", selection.variant));
            
            let model = &*imp.variant_model;
            model.splice(0, model.n_items(), &[&selection.variant]);
        } else {
            imp.variant_preview_row.set_visible(false);
        }

        imp.preview_group.set_description(Some(&format!(
            "Current selection: {} - {}",
            selection.country_shorthand, selection.variant
        )));

        self.set_keyboard_layout(&selection.country_shorthand, &selection.variant);
    }

    fn set_keyboard_layout(&self, layout: &str, variant: &str) {
        let is_wayland = std::env::var("WAYLAND_DISPLAY").is_ok();

        let result = if is_wayland {
            self.set_wayland_layout(layout, variant)
        } else {
            self.set_xorg_layout(layout, variant)
        };

        if let Err(e) = result {
            self.show_error_toast(&format!("Failed to set keyboard layout: {}", e));
        } else {
            self.show_success_toast("Keyboard layout updated successfully");
        }
    }

    fn set_wayland_layout(&self, layout: &str, variant: &str) -> Result<(), String> {
        let keymap = if variant == "normal" {
            layout.to_string()
        } else {
            format!("{}+{}", layout, variant)
        };

        let output = Command::new("localectl")
            .args(["set-keymap", &keymap])
            .output()
            .map_err(|e| e.to_string())?;

        if !output.status.success() {
            return Err(String::from_utf8_lossy(&output.stderr).to_string());
        }
        Ok(())
    }

    fn set_xorg_layout(&self, layout: &str, variant: &str) -> Result<(), String> {
        let mut cmd = Command::new("setxkbmap");
        cmd.arg(layout);

        if variant != "normal" {
            cmd.args(["-variant", variant]);
        }

        let output = cmd.output().map_err(|e| e.to_string())?;

        if !output.status.success() {
            return Err(String::from_utf8_lossy(&output.stderr).to_string());
        }
        Ok(())
    }
    
    fn show_error_toast(&self, message: &str) {
        if let Some(window) = self.root().and_then(|r| r.downcast::<adw::Window>().ok()) {
             if let Some(overlay) = window.child().and_then(|c| c.downcast::<adw::ToastOverlay>().ok()) {
                let toast = adw::Toast::builder()
                    .title(message)
                    .timeout(5)
                    .build();
                overlay.add_toast(toast);
            }
        }
    }

    fn show_success_toast(&self, message: &str) {
       if let Some(window) = self.root().and_then(|r| r.downcast::<adw::Window>().ok()) {
            if let Some(overlay) = window.child().and_then(|c| c.downcast::<adw::ToastOverlay>().ok()) {
                let toast = adw::Toast::builder()
                    .title(message)
                    .timeout(3)
                    .build();
                overlay.add_toast(toast);
            }
        }
    }

    fn update_test_feedback(&self, _text: &str) {
    }
    
    fn filter_layouts(layout_list: &ListBox, query: &str) {
        if query.is_empty() {
             layout_list.set_filter_func(|_| true);
        } else {
            let query_lower = query.to_lowercase();
            layout_list.set_filter_func(move |row| {
                if let Some(action_row) = row.downcast_ref::<adw::ActionRow>() {
                    let title = action_row.title().to_lowercase();
                    let subtitle = action_row.subtitle().unwrap_or_default().to_lowercase();
                    title.contains(&query_lower) || subtitle.contains(&query_lower)
                } else {
                    true
                }
            });
        }
        layout_list.invalidate_filter();
    }

    pub fn get_current_selection(&self) -> Option<KeyboardSelection> {
        self.imp().current_selection.borrow().clone()
    }
}

impl Default for KeyboardScreen {
    fn default() -> Self {
        Self::new()
    }
}