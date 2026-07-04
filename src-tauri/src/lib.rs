mod commands;
mod control;
mod sidecar;

use std::time::Duration;

use tauri::{Emitter, Manager};

/// How often the background poll asks the sidecar for status+logs and emits
/// them to the frontend, matching the old Fyne GUI's poll cadence so the
/// connection badge and log tail feel just as live.
const POLL_INTERVAL: Duration = Duration::from_millis(1500);

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_fs::init())
        .plugin(tauri_plugin_dialog::init())
        .setup(|app| {
            if cfg!(debug_assertions) {
                app.handle().plugin(
                    tauri_plugin_log::Builder::default()
                        .level(log::LevelFilter::Info)
                        .build(),
                )?;
            }

            sidecar::spawn(app.handle())?;

            let app_handle = app.handle().clone();
            tauri::async_runtime::spawn(async move {
                let mut since: u64 = 0;
                loop {
                    tokio::time::sleep(POLL_INTERVAL).await;
                    let state: tauri::State<std::sync::Arc<sidecar::SidecarState>> =
                        app_handle.state();
                    let req = control::Request {
                        cmd: control::CMD_LOGS.into(),
                        since: Some(since),
                        ..Default::default()
                    };
                    match sidecar::send_request(state.inner(), &req).await {
                        Ok(resp) if resp.ok => {
                            if let Some(last) = resp.logs.last() {
                                since = last.seq;
                            }
                            if let Some(status) = &resp.status {
                                let _ = app_handle.emit("status-update", status.clone());
                            }
                            if !resp.logs.is_empty() {
                                let _ = app_handle.emit("logs-update", resp.logs);
                            }
                        }
                        Ok(resp) => {
                            log::warn!("poll: sidecar returned error: {}", resp.error);
                        }
                        Err(err) => {
                            // Expected right after the sidecar exits (e.g. app
                            // closing); the terminated handler in sidecar.rs
                            // already notified the frontend, so just stop
                            // polling instead of spamming this every tick.
                            log::warn!("poll: request failed: {err}");
                            break;
                        }
                    }
                }
            });

            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            commands::connect,
            commands::disconnect,
            commands::status,
            commands::logs,
            commands::parse_config,
            commands::serialize_config,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
