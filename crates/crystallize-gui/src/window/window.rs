use adw::prelude::AdwDialogExt;
use adw::subclass::prelude::*;
use gtk::prelude::*;
use gtk::{gio, glib, CompositeTemplate};
use std::cell::RefCell;

use crate::modules::welcome::WelcomeScreen;

mod imp {
	use super::*;

	#[derive(Debug, Default, CompositeTemplate)]
	#[template(resource = "/org/crystallinux/crystallize/ui/window.ui")]
	pub struct CrystallizeWindow {
		#[template_child]
		pub carousel: TemplateChild<adw::Carousel>,
		#[template_child]
		pub next_button: TemplateChild<gtk::Button>,
		#[template_child]
		pub back_button: TemplateChild<gtk::Button>,
		#[template_child]
		pub revealer: TemplateChild<gtk::Revealer>,
		#[template_child]
		pub about_button: TemplateChild<gtk::Button>,

		pub current_page: RefCell<usize>,
		pub pages: RefCell<Vec<gtk::Widget>>,
	}

	#[glib::object_subclass]
	impl ObjectSubclass for CrystallizeWindow {
		const NAME: &'static str = "CrystallizeWindow";
		type Type = super::CrystallizeWindow;
		type ParentType = gtk::ApplicationWindow;

		fn class_init(klass: &mut Self::Class) {
			klass.bind_template();
		}

		fn instance_init(obj: &glib::subclass::InitializingObject<Self>) {
			obj.init_template();
		}
	}

	impl ObjectImpl for CrystallizeWindow {
		fn constructed(&self) {
			self.parent_constructed();
			let obj = self.obj();
			obj.setup_callbacks();
			obj.setup_pages();
		}
	}

	impl WidgetImpl for CrystallizeWindow {}
	impl WindowImpl for CrystallizeWindow {}
	impl ApplicationWindowImpl for CrystallizeWindow {}
}

glib::wrapper! {
    pub struct CrystallizeWindow(ObjectSubclass<imp::CrystallizeWindow>)
        @extends adw::ApplicationWindow, gtk::ApplicationWindow, gtk::Window, gtk::Widget,
        @implements gio::ActionGroup, gio::ActionMap, gtk::Accessible, gtk::Buildable,
                    gtk::ConstraintTarget, gtk::Native, gtk::Root, gtk::ShortcutManager;
}

impl CrystallizeWindow {
	pub fn new(app: &adw::Application) -> Self {
		glib::Object::builder().property("application", app).build()
	}

	fn setup_callbacks(&self) {
		let imp = self.imp();

		// Connect next button
		let weak_self = glib::object::WeakRef::new();
		weak_self.set(Some(self));
		imp.next_button.connect_clicked(move |_| {
			if let Some(window) = weak_self.upgrade() {
				window.next_page();
			}
		});

		// Connect back button
		let weak_self = glib::object::WeakRef::new();
		weak_self.set(Some(self));
		imp.back_button.connect_clicked(move |_| {
			if let Some(window) = weak_self.upgrade() {
				window.previous_page();
			}
		});

		// Connect about button
		let weak_self = glib::object::WeakRef::new();
		weak_self.set(Some(self));
		imp.about_button.connect_clicked(move |_| {
			if let Some(window) = weak_self.upgrade() {
				window.show_about_dialog();
			}
		});
	}

	fn setup_pages(&self) {
		let imp = self.imp();

		// Create welcome screen
		let welcome_screen = WelcomeScreen::new();

		// Connect welcome screen next button
		let weak_self = glib::object::WeakRef::new();
		weak_self.set(Some(self));
		welcome_screen.connect_next_button(move || {
			if let Some(window) = weak_self.upgrade() {
				window.next_page();
			}
		});

		// Start internet checking
		welcome_screen.start_internet_check();

		// Add pages to carousel
		imp.carousel.append(&welcome_screen);

		// Store pages
		let mut pages = imp.pages.borrow_mut();
		pages.push(welcome_screen.upcast());
		drop(pages); // Release the mutable borrow

		// Set initial page state
		self.update_navigation_buttons();
	}

	fn next_page(&self) {
		let imp = self.imp();
		let current_page = *imp.current_page.borrow();
		let pages = imp.pages.borrow();

		if current_page < pages.len() - 1 {
			let next_page = current_page + 1;
			if let Some(page) = pages.get(next_page) {
				imp.carousel.scroll_to(page, true);
				imp.current_page.replace(next_page);
				self.update_navigation_buttons();
			}
		}
	}

	fn previous_page(&self) {
		let imp = self.imp();
		let current_page = *imp.current_page.borrow();
		let pages = imp.pages.borrow();

		if current_page > 0 {
			let prev_page = current_page - 1;
			if let Some(page) = pages.get(prev_page) {
				imp.carousel.scroll_to(page, true);
				imp.current_page.replace(prev_page);
				self.update_navigation_buttons();
			}
		}
	}

	fn update_navigation_buttons(&self) {
		let imp = self.imp();

		// Step 1: Extract data from RefCells
		let current_page = *imp.current_page.borrow();
		let pages_len = imp.pages.borrow().len();

		// Step 2: Derive the visibility states
		let is_first_page = current_page == 0;
		let is_last_page = current_page == pages_len.saturating_sub(1);
		let show_navigation = !is_first_page && !is_last_page;

		// Step 3: Apply UI updates (Scoped Mutable Borrows)
		{
			imp.back_button.set_visible(!is_first_page);
		}
		{
			imp.next_button.set_visible(!is_last_page);
		}
		{
			imp.revealer.set_reveal_child(show_navigation);
		}
	}

	pub fn set_page_valid(&self, valid: bool) {
		let imp = self.imp();
		imp.next_button.set_sensitive(valid);
	}

	fn show_about_dialog(&self) {
		let about = adw::AboutDialog::builder()
				.application_name("Crystallize")
				.application_icon("org.crystallinux.crystallize")
				.developer_name("CrystalNetwork Studio")
				.version("0.1.0")
				.comments("CrystalLinux installer")
				.license_type(gtk::License::Gpl30)
				.build();

		about.present(Some(self));
	}
}