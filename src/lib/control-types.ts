// TypeScript mirror of github.com/veil-proto/veil-windows/control's proto.go.
// Keep field names and shapes in exact sync with that file — these types are
// the JSON wire contract with the veil-sidecar process (via the Tauri Rust
// shell's control commands, see control-client.ts), not just documentation.

export const CmdStatus = "status";
export const CmdConnect = "connect";
export const CmdDisconnect = "disconnect";
export const CmdLogs = "logs";
export const CmdParseConfig = "parseConfig";
export const CmdSerializeConfig = "serializeConfig";

export interface Request {
  cmd: string;
  config?: string;
  name?: string;
  since?: number;
  parsedConfig?: ParsedConfig;
}

export interface Response {
  ok: boolean;
  error?: string;
  status?: Status;
  logs?: LogLine[];
  parsedConfig?: ParsedConfig;
  config?: string;
}

export interface LogLine {
  seq: number;
  time: number; // unix seconds
  level?: string;
  msg: string;
}

export type State = "disconnected" | "connecting" | "connected";

export interface Status {
  state: State;
  name?: string;
  iface?: string;
  peers?: PeerStatus[];
}

export interface PeerStatus {
  public_key: string;
  endpoint?: string;
  connected: boolean;
  last_handshake?: number; // unix seconds, 0/absent if none
  last_received?: number;
  rx_bytes: number;
  tx_bytes: number;
  frame_budget: number;
}

export interface ParsedConfig {
  interface: ParsedInterface;
  peers: ParsedPeer[];
}

export interface ParsedInterface {
  privateKey: string; // hex
  address?: string;
  bindAddress?: string;
  listenPort?: number;
  nid: string; // hex
  netSecret?: string; // hex, empty when netSecretInsecure
  netSecretInsecure?: boolean;
  allowInsecureNetSecret?: boolean;
  padding?: string;
  dns?: string;
  fwMark?: number;
}

export interface ParsedPeer {
  publicKey: string; // hex
  allowedIPs?: string[];
  endpoint?: string;
  persistentKeepalive?: number;
  presharedKey?: string; // hex
}
