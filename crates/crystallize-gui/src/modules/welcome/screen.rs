use adw::subclass::prelude::*;
use gtk::prelude::*;
use gtk::{glib, CompositeTemplate};
use std::cell::RefCell;
use tokio::time::Duration;

use crate::utils::network::check_internet_connection_async;

mod imp {
	use super::*;

	#[derive(Debug, Default, CompositeTemplate)]
	#[template(resource = "/org/crystallinux/crystallize/ui/welcome_screen.ui")]
	pub struct WelcomeScreen {
		#[template_child]
		pub next_button: TemplateChild<gtk::Button>,
		#[template_child]
		pub no_internet: TemplateChild<gtk::Label>,

		pub is_valid: RefCell<bool>,
		pub do_check_internet: RefCell<bool>,
	}

	#[glib::object_subclass]
	impl ObjectSubclass for WelcomeScreen {
		const NAME: &'static str = "WelcomeScreen";
		type Type = super::WelcomeScreen;
		type ParentType = adw::Bin;

		fn class_init(klass: &mut Self::Class) {
			klass.bind_template();
		}

		fn instance_init(obj: &glib::subclass::InitializingObject<Self>) {
			obj.init_template();
		}
	}

	impl ObjectImpl for WelcomeScreen {
		fn constructed(&self) {
			self.parent_constructed();
			self.is_valid.replace(true);
			self.do_check_internet.replace(true);
		}
	}

	impl WidgetImpl for WelcomeScreen {}
	impl BinImpl for WelcomeScreen {}
}

glib::wrapper! {
    pub struct WelcomeScreen(ObjectSubclass<imp::WelcomeScreen>)
        @extends adw::Bin, gtk::Widget,
        @implements gtk::Accessible, gtk::Buildable, gtk::ConstraintTarget;
}

impl WelcomeScreen {
	pub fn new() -> Self {
		glib::Object::builder().build()
	}

	pub fn connect_next_button<F>(&self, callback: F)
	where
			F: Fn() + 'static,
	{
		let imp = self.imp();
		imp.next_button.connect_clicked(move |_| {
			callback();
		});
	}

	pub fn set_valid(&self, valid: bool) {
		let imp = self.imp();
		imp.is_valid.replace(valid);
		imp.next_button.set_sensitive(valid);
		imp.no_internet.set_visible(!valid);
	}

	pub fn is_valid(&self) -> bool {
		*self.imp().is_valid.borrow()
	}

	pub fn start_internet_check(&self) {
		let weak_self = glib::object::WeakRef::new();
		weak_self.set(Some(self));

		// Use glib::timeout_add_local to check internet connection periodically
		glib::timeout_add_local(Duration::from_secs(2), move || {
			let weak_self = weak_self.clone();

			glib::spawn_future_local(async move {
				let has_internet = check_internet_connection_async().await;

				if let Some(welcome_screen) = weak_self.upgrade() {
					welcome_screen.set_valid(has_internet);

					if has_internet {
						log::info!("Internet connection available");
						// Return glib::ControlFlow::Break to stop the timeout
						return glib::ControlFlow::Break;
					} else {
						log::info!("No internet connection, retrying...");
					}
				}

				glib::ControlFlow::Continue
			});

			glib::ControlFlow::Continue
		});
	}

	pub fn stop_internet_check(&self) {
		self.imp().do_check_internet.replace(false);
	}
}

impl Default for WelcomeScreen {
	fn default() -> Self {
		Self::new()
	}
}