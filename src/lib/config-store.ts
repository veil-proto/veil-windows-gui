// TypeScript port of the old Fyne GUI's store.go: imported configs are kept
// as plain VEIL .conf files under the user's roaming app-data directory. The
// path deliberately matches what the old GUI used (%APPDATA%\VEIL\configs on
// Windows) rather than Tauri's per-app-identifier default, so configs a user
// already imported before this rewrite are still found.

import { dataDir, join } from "@tauri-apps/api/path";
import {
  exists,
  mkdir,
  readDir,
  readTextFile,
  remove,
  writeTextFile,
} from "@tauri-apps/plugin-fs";

export interface ConfigEntry {
  name: string; // display name (file base, no extension)
  path: string;
}

async function storeDir(): Promise<string> {
  return join(await dataDir(), "VEIL", "configs");
}

// sanitizeName reduces a display name to a safe file base (no separators or
// other characters that would escape the store directory), mirroring
// store.go's sanitizeName exactly.
export function sanitizeName(name: string): string {
  let out = name.trim();
  out = Array.from(out)
    .map((ch) => ("/\\:*?\"<>|".includes(ch) ? "-" : ch))
    .join("");
  out = out.replace(/^[.\s]+|[.\s]+$/g, "");
  return out === "" ? "veil" : out;
}

export async function listConfigs(): Promise<ConfigEntry[]> {
  const dir = await storeDir();
  if (!(await exists(dir))) return [];
  const entries = await readDir(dir);
  const out: ConfigEntry[] = [];
  for (const entry of entries) {
    if (entry.isDirectory) continue;
    if (!entry.name.toLowerCase().endsWith(".conf")) continue;
    const base = entry.name.slice(0, -".conf".length);
    if (base.trim() === "") continue;
    out.push({ name: base, path: await join(dir, entry.name) });
  }
  out.sort((a, b) => a.name.localeCompare(b.name));
  return out;
}

export async function saveConfig(name: string, text: string): Promise<ConfigEntry> {
  const dir = await storeDir();
  await mkdir(dir, { recursive: true });
  const base = sanitizeName(name);
  const path = await join(dir, `${base}.conf`);
  await writeTextFile(path, text);
  return { name: base, path };
}

export async function loadConfig(path: string): Promise<string> {
  return readTextFile(path);
}

export async function deleteConfig(path: string): Promise<void> {
  await remove(path);
}
