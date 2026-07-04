//! Tauri commands the Vue frontend calls via `invoke(...)`. Each one builds a
//! control.Request, sends it through the sidecar, and maps the response back
//! to something the frontend can use directly (an Ok status/parsed config, or
//! an Err with the sidecar's error string).

use std::sync::Arc;

use tauri::State;

use crate::control::{
    ParsedConfig, Request, Status, CMD_CONNECT, CMD_DISCONNECT, CMD_LOGS, CMD_PARSE_CONFIG,
    CMD_SERIALIZE_CONFIG, CMD_STATUS,
};
use crate::sidecar::{send_request, SidecarState};

type SidecarHandle<'a> = State<'a, Arc<SidecarState>>;

#[tauri::command]
pub async fn connect(
    state: SidecarHandle<'_>,
    config: String,
    name: String,
) -> Result<Status, String> {
    let resp = send_request(
        state.inner(),
        &Request {
            cmd: CMD_CONNECT.into(),
            config: Some(config),
            name: Some(name),
            ..Default::default()
        },
    )
    .await?;
    if !resp.ok {
        return Err(resp.error);
    }
    resp.status
        .ok_or_else(|| "connect: response missing status".to_string())
}

#[tauri::command]
pub async fn disconnect(state: SidecarHandle<'_>) -> Result<Status, String> {
    let resp = send_request(
        state.inner(),
        &Request {
            cmd: CMD_DISCONNECT.into(),
            ..Default::default()
        },
    )
    .await?;
    if !resp.ok {
        return Err(resp.error);
    }
    resp.status
        .ok_or_else(|| "disconnect: response missing status".to_string())
}

#[tauri::command]
pub async fn status(state: SidecarHandle<'_>) -> Result<Status, String> {
    let resp = send_request(
        state.inner(),
        &Request {
            cmd: CMD_STATUS.into(),
            ..Default::default()
        },
    )
    .await?;
    if !resp.ok {
        return Err(resp.error);
    }
    resp.status
        .ok_or_else(|| "status: response missing status".to_string())
}

/// Combined status+logs fetch, mirroring the old GUI's single poll tick that
/// asked for both at once (CmdLogs already returns Status alongside Logs on
/// the Go side, see control/server.go's dispatch).
#[derive(serde::Serialize)]
pub struct StatusAndLogs {
    pub status: Status,
    pub logs: Vec<crate::control::LogLine>,
}

#[tauri::command]
pub async fn logs(state: SidecarHandle<'_>, since: u64) -> Result<StatusAndLogs, String> {
    let resp = send_request(
        state.inner(),
        &Request {
            cmd: CMD_LOGS.into(),
            since: Some(since),
            ..Default::default()
        },
    )
    .await?;
    if !resp.ok {
        return Err(resp.error);
    }
    let status = resp
        .status
        .ok_or_else(|| "logs: response missing status".to_string())?;
    Ok(StatusAndLogs {
        status,
        logs: resp.logs,
    })
}

#[tauri::command]
pub async fn parse_config(
    state: SidecarHandle<'_>,
    config: String,
) -> Result<ParsedConfig, String> {
    let resp = send_request(
        state.inner(),
        &Request {
            cmd: CMD_PARSE_CONFIG.into(),
            config: Some(config),
            ..Default::default()
        },
    )
    .await?;
    if !resp.ok {
        return Err(resp.error);
    }
    resp.parsed_config
        .ok_or_else(|| "parseConfig: response missing parsedConfig".to_string())
}

#[tauri::command]
pub async fn serialize_config(
    state: SidecarHandle<'_>,
    parsed_config: ParsedConfig,
) -> Result<String, String> {
    let resp = send_request(
        state.inner(),
        &Request {
            cmd: CMD_SERIALIZE_CONFIG.into(),
            parsed_config: Some(parsed_config),
            ..Default::default()
        },
    )
    .await?;
    if !resp.ok {
        return Err(resp.error);
    }
    Ok(resp.config)
}
