#![allow(dead_code)]
use std::sync::{Arc, Mutex};

#[derive(Debug, Clone)]
pub struct AppState {
    is_checking: bool,
    is_connected: bool,
    check_completed: bool,
    error_message: Option<String>,
}

impl AppState {
    pub fn new() -> Self {
        Self {
            is_checking: false,
            is_connected: false,
            check_completed: false,
            error_message: None,
        }
    }

    pub fn is_checking(&self) -> bool {
        self.is_checking
    }

    pub fn set_checking(&mut self, checking: bool) {
        self.is_checking = checking;
    }

    pub fn is_connected(&self) -> bool {
        self.is_connected
    }

    pub fn set_connected(&mut self, connected: bool) {
        self.is_connected = connected;
    }

    pub fn is_check_completed(&self) -> bool {
        self.check_completed
    }

    pub fn set_check_completed(&mut self, completed: bool) {
        self.check_completed = completed;
    }

    pub fn error_message(&self) -> Option<&String> {
        self.error_message.as_ref()
    }

    pub fn set_error_message(&mut self, message: Option<String>) {
        self.error_message = message;
    }

    pub fn reset(&mut self) {
        self.is_checking = false;
        self.is_connected = false;
        self.check_completed = false;
        self.error_message = None;
    }
}

impl Default for AppState {
    fn default() -> Self {
        Self::new()
    }
}

pub type SharedAppState = Arc<Mutex<AppState>>;

pub fn create_shared_state() -> SharedAppState {
    Arc::new(Mutex::new(AppState::new()))
}
