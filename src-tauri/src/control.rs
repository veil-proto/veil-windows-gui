//! Rust mirror of github.com/veil-proto/veil-windows/control's proto.go and
//! src/lib/control-types.ts. Keep all three in sync — this is the JSON wire
//! contract with the veil-sidecar child process.

use serde::{Deserialize, Serialize};

pub const CMD_STATUS: &str = "status";
pub const CMD_CONNECT: &str = "connect";
pub const CMD_DISCONNECT: &str = "disconnect";
pub const CMD_LOGS: &str = "logs";
pub const CMD_PARSE_CONFIG: &str = "parseConfig";
pub const CMD_SERIALIZE_CONFIG: &str = "serializeConfig";

#[derive(Serialize, Default)]
pub struct Request {
    pub cmd: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub config: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub since: Option<u64>,
    #[serde(rename = "parsedConfig", skip_serializing_if = "Option::is_none")]
    pub parsed_config: Option<ParsedConfig>,
}

#[derive(Deserialize, Clone)]
pub struct Response {
    pub ok: bool,
    #[serde(default)]
    pub error: String,
    pub status: Option<Status>,
    #[serde(default)]
    pub logs: Vec<LogLine>,
    #[serde(rename = "parsedConfig")]
    pub parsed_config: Option<ParsedConfig>,
    #[serde(default)]
    pub config: String,
}

#[derive(Deserialize, Serialize, Clone)]
pub struct LogLine {
    pub seq: u64,
    pub time: i64,
    #[serde(default)]
    pub level: String,
    pub msg: String,
}

#[derive(Deserialize, Serialize, Clone)]
pub struct Status {
    pub state: String,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub iface: String,
    #[serde(default)]
    pub peers: Vec<PeerStatus>,
}

#[derive(Deserialize, Serialize, Clone)]
pub struct PeerStatus {
    pub public_key: String,
    #[serde(default)]
    pub endpoint: String,
    pub connected: bool,
    #[serde(default)]
    pub last_handshake: i64,
    #[serde(default)]
    pub last_received: i64,
    pub rx_bytes: u64,
    pub tx_bytes: u64,
    pub frame_budget: i64,
}

#[derive(Deserialize, Serialize, Clone, Default)]
pub struct ParsedConfig {
    pub interface: ParsedInterface,
    #[serde(default)]
    pub peers: Vec<ParsedPeer>,
}

#[derive(Deserialize, Serialize, Clone, Default)]
pub struct ParsedInterface {
    #[serde(rename = "privateKey")]
    pub private_key: String,
    #[serde(default)]
    pub address: String,
    #[serde(rename = "bindAddress", default)]
    pub bind_address: String,
    #[serde(rename = "listenPort", default)]
    pub listen_port: i64,
    pub nid: String,
    #[serde(rename = "netSecret", default)]
    pub net_secret: String,
    #[serde(rename = "netSecretInsecure", default)]
    pub net_secret_insecure: bool,
    #[serde(rename = "allowInsecureNetSecret", default)]
    pub allow_insecure_net_secret: bool,
    #[serde(default)]
    pub padding: String,
    #[serde(default)]
    pub dns: String,
    #[serde(rename = "fwMark", default)]
    pub fw_mark: i64,
}

#[derive(Deserialize, Serialize, Clone, Default)]
pub struct ParsedPeer {
    #[serde(rename = "publicKey")]
    pub public_key: String,
    #[serde(rename = "allowedIPs", default)]
    pub allowed_ips: Vec<String>,
    #[serde(default)]
    pub endpoint: String,
    #[serde(rename = "persistentKeepalive", default)]
    pub persistent_keepalive: i64,
    #[serde(rename = "presharedKey", default)]
    pub preshared_key: String,
}
