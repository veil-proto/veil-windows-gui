// Thin wrapper around Tauri's invoke/listen for the control protocol
// commands exposed by src-tauri/src/commands.rs. Every tab view should go
// through this module rather than calling `invoke` directly, so the
// request/response shapes stay centralized in one place next to
// control-types.ts.

import { invoke } from "@tauri-apps/api/core";
import { listen, type UnlistenFn } from "@tauri-apps/api/event";
import type { LogLine, ParsedConfig, Status } from "./control-types";

export async function connect(config: string, name: string): Promise<Status> {
  return invoke<Status>("connect", { config, name });
}

export async function disconnect(): Promise<Status> {
  return invoke<Status>("disconnect");
}

export async function status(): Promise<Status> {
  return invoke<Status>("status");
}

export interface StatusAndLogs {
  status: Status;
  logs: LogLine[];
}

export async function logs(since: number): Promise<StatusAndLogs> {
  return invoke<StatusAndLogs>("logs", { since });
}

export async function parseConfig(config: string): Promise<ParsedConfig> {
  return invoke<ParsedConfig>("parse_config", { config });
}

export async function serializeConfig(parsedConfig: ParsedConfig): Promise<string> {
  return invoke<string>("serialize_config", { parsedConfig });
}

// Subscribes to the Rust shell's background poll (see src-tauri/src/lib.rs),
// which emits these on the same ~1.5s cadence the old Fyne GUI polled on.
// Prefer this over each view running its own poll timer, so there is one
// shared request stream to the sidecar rather than several racing ones.
export function onStatusUpdate(cb: (status: Status) => void): Promise<UnlistenFn> {
  return listen<Status>("status-update", (event) => cb(event.payload));
}

export function onLogsUpdate(cb: (logs: LogLine[]) => void): Promise<UnlistenFn> {
  return listen<LogLine[]>("logs-update", (event) => cb(event.payload));
}

// Fires once if the sidecar process exits unexpectedly (e.g. it crashed);
// a clean app shutdown doesn't need this since nothing is listening by then.
export function onSidecarTerminated(cb: () => void): Promise<UnlistenFn> {
  return listen<void>("sidecar-terminated", () => cb());
}
