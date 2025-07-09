use std::process::Command;
use tokio::time::{timeout, Duration};
use log::{debug, warn, error};

pub async fn check_internet_connection_async() -> bool {
    debug!("Performing asynchronous internet connection check");

    let endpoints = ["8.8.8.8", "1.1.1.1", "208.67.222.222"];
    let timeout_duration = Duration::from_secs(5);

    for endpoint in &endpoints {
        let endpoint_clone = endpoint.to_string();
        let ping_result = timeout(
            timeout_duration, // Timeout for the blocking ping task
            tokio::task::spawn_blocking(move || {
                // Ping command itself has a short timeout
                Command::new("ping")
                    .arg("-c")
                    .arg("1")
                    .arg("-W") // Timeout for ping command itself (milliseconds)
                    .arg("1000") // 1 second timeout for ping
                    .arg(&endpoint_clone)
                    .output()
            }),
        ).await;

        match ping_result {
            Ok(Ok(Ok(output))) => { // Outer Ok for tokio::timeout, middle Ok for spawn_blocking, inner Ok for output()
                if output.status.success() {
                    debug!("Successfully pinged {endpoint}");
                    return true;
                }
            }
            Ok(Ok(Err(e))) => {
                warn!("Failed to execute ping for {endpoint}: {e}");
            }
            Ok(Err(join_error)) => { // Error from tokio::task::JoinHandle
                error!("Ping blocking task failed to join: {join_error}");
            }
            Err(_) => { // Error from tokio::time::timeout
                warn!("Ping to {endpoint} timed out.");
            }
        }
    }

    // Fallback DNS check
    let dns_result = timeout(
        timeout_duration, // Timeout for the blocking nslookup task
        tokio::task::spawn_blocking(|| {
            Command::new("nslookup")
                .arg("google.com")
                .output()
        })
    ).await;

    match dns_result {
        Ok(Ok(Ok(output))) => {
            debug!("DNS fallback result: {}", output.status.success());
            output.status.success()
        }
        Ok(Ok(Err(e))) => {
            warn!("Failed to execute nslookup: {e}");
            false
        }
        Ok(Err(join_error)) => {
            error!("Nslookup blocking task failed to join: {join_error}");
            false
        }
        Err(_) => {
            warn!("Nslookup timed out.");
            false
        }
    }
}