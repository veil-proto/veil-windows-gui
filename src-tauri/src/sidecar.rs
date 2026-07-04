//! Manages the veil-sidecar child process: spawns it as a Tauri sidecar
//! binary, writes one JSON request line to its stdin per call, and matches
//! each response line read from its stdout back to the caller that's
//! waiting on it.
//!
//! The wire protocol (see control.rs) is strictly request-then-response, one
//! at a time, matching how github.com/veil-proto/veil-windows/control.Client
//! drives it on the Go side — there is no request ID field. This module
//! therefore pairs requests to responses by arrival order (a FIFO queue of
//! one-shot channels): as long as callers don't issue a second request
//! before the first's response arrives, order is preserved. send_request
//! enforces this itself by holding a lock across the whole write+await, so
//! concurrent Tauri commands queue up safely instead of racing.

use std::collections::VecDeque;
use std::sync::Arc;

use tauri::{AppHandle, Emitter, Manager};
use tauri_plugin_shell::process::{CommandChild, CommandEvent};
use tauri_plugin_shell::ShellExt;
use tokio::sync::{oneshot, Mutex};

use crate::control::Response;

pub struct SidecarState {
    child: Mutex<CommandChild>,
    /// FIFO queue of response waiters, one per in-flight request. Locked
    /// together with the write side in send_request so a request is always
    /// enqueued before send_request releases the lock, which is what keeps
    /// the reader task's pop-front-on-each-line logic correctly paired.
    pending: Mutex<VecDeque<oneshot::Sender<Response>>>,
}

/// Spawns the veil-sidecar binary (bundled as a Tauri externalBin) and wires
/// up a background task that routes each stdout line to the oldest pending
/// request. Call once from the app's setup hook.
pub fn spawn(app: &AppHandle) -> anyhow::Result<()> {
    let (mut rx, child) = app.shell().sidecar("veil-sidecar")?.spawn()?;

    let state = Arc::new(SidecarState {
        child: Mutex::new(child),
        pending: Mutex::new(VecDeque::new()),
    });
    app.manage(state.clone());

    let app_handle = app.clone();
    tauri::async_runtime::spawn(async move {
        while let Some(event) = rx.recv().await {
            match event {
                CommandEvent::Stdout(bytes) => {
                    let line = String::from_utf8_lossy(&bytes);
                    match serde_json::from_str::<Response>(line.trim_end()) {
                        Ok(resp) => {
                            let mut pending = state.pending.lock().await;
                            if let Some(waiter) = pending.pop_front() {
                                let _ = waiter.send(resp);
                            }
                            // No waiter means a response arrived with nothing
                            // registered to receive it (shouldn't happen given
                            // send_request's locking) — silently drop rather
                            // than panic, since a corrupted pairing here is
                            // recoverable (the next real request just waits
                            // for its own line) but crashing the reader task
                            // would take down all future control traffic.
                        }
                        Err(err) => {
                            log::warn!("sidecar: malformed response line: {err}: {line}");
                        }
                    }
                }
                CommandEvent::Stderr(bytes) => {
                    // The sidecar's own log.Printf output (stderr, never
                    // stdout — see handler_windows.go's comment on why). Not
                    // the same as the control-protocol Logs command, but
                    // useful for diagnosing a sidecar that fails to start at
                    // all, so forward it to the frontend too.
                    let line = String::from_utf8_lossy(&bytes).trim_end().to_string();
                    log::info!("sidecar stderr: {line}");
                    let _ = app_handle.emit("sidecar-stderr", line);
                }
                CommandEvent::Terminated(payload) => {
                    log::warn!("sidecar process exited: {payload:?}");
                    let _ = app_handle.emit("sidecar-terminated", ());
                    // Fail every still-pending request instead of leaving
                    // callers hanging forever.
                    let mut pending = state.pending.lock().await;
                    while let Some(waiter) = pending.pop_front() {
                        let _ = waiter.send(Response {
                            ok: false,
                            error: "sidecar process exited".into(),
                            status: None,
                            logs: Vec::new(),
                            parsed_config: None,
                            config: String::new(),
                        });
                    }
                }
                _ => {}
            }
        }
    });

    Ok(())
}

/// Sends one request to the sidecar and returns its response. Serializes
/// concurrent callers: the write and the pending-queue push happen under the
/// same lock, so two Tauri commands invoked at nearly the same time still
/// pair correctly with their own responses instead of racing.
pub async fn send_request(
    state: &Arc<SidecarState>,
    req: &crate::control::Request,
) -> Result<Response, String> {
    let mut line = serde_json::to_vec(req).map_err(|e| e.to_string())?;
    line.push(b'\n');

    let (tx, rx) = oneshot::channel();
    {
        // Lock order: pending before child, and hold both across the write,
        // so the reader task can never observe a response line before its
        // waiter is registered.
        let mut pending = state.pending.lock().await;
        let mut child = state.child.lock().await;
        child.write(&line).map_err(|e| e.to_string())?;
        pending.push_back(tx);
    }

    rx.await
        .map_err(|_| "sidecar reader task ended before a response arrived".to_string())
}
