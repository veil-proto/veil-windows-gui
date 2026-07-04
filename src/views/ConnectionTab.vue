<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { open as openDialog } from "@tauri-apps/plugin-dialog";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import * as controlClient from "@/lib/control-client";
import * as configStore from "@/lib/config-store";
import type { ConfigEntry } from "@/lib/config-store";
import * as veilLink from "@/lib/veil-link";
import type { Status } from "@/lib/control-types";
import { applyDisallowed, loadDisallowed } from "@/lib/disallowed";

const props = defineProps<{ status: Status }>();
const emit = defineEmits<{ connected: [Status] }>();

const configs = ref<ConfigEntry[]>([]);
const selectedPath = ref<string>("");
const importLink = ref("");
const busy = ref(false);
const error = ref("");

const isConnected = computed(() => props.status.state === "connected");
const isConnecting = computed(() => props.status.state === "connecting");

async function refreshConfigs() {
  configs.value = await configStore.listConfigs();
  if (!selectedPath.value && configs.value.length > 0) {
    selectedPath.value = configs.value[0].path;
  }
}

onMounted(refreshConfigs);

async function handleImportFile() {
  error.value = "";
  const picked = await openDialog({
    multiple: false,
    filters: [{ name: "VEIL config", extensions: ["conf"] }],
  });
  if (!picked || Array.isArray(picked)) return;
  try {
    const text = await configStore.loadConfig(picked);
    const base = picked.split(/[\\/]/).pop() ?? "veil";
    const name = base.replace(/\.conf$/i, "");
    const entry = await configStore.saveConfig(name, text);
    await refreshConfigs();
    selectedPath.value = entry.path;
  } catch (e) {
    error.value = String(e);
  }
}

async function handleImportLink() {
  error.value = "";
  try {
    const { configText, name } = veilLink.decode(importLink.value);
    const entry = await configStore.saveConfig(name || "Imported", configText);
    await refreshConfigs();
    selectedPath.value = entry.path;
    importLink.value = "";
  } catch (e) {
    error.value = String(e);
  }
}

// Mirrors the old Fyne GUI's effectiveConfigText: reduce each peer's
// AllowedIPs by that peer's Disallowed CIDRs (Split Tunnel tab) before the
// config text ever reaches the sidecar. Any failure here (parse error, no
// Disallowed data) falls back to the raw text unchanged — the Disallowed
// feature only ever narrows AllowedIPs, never blocks a connect.
async function effectiveConfigText(confPath: string, rawText: string): Promise<string> {
  try {
    const doc = await loadDisallowed(confPath);
    if (Object.keys(doc.per_peer).length === 0) return rawText;
    const pc = await controlClient.parseConfig(rawText);
    const updated = applyDisallowed(pc, doc);
    if (updated === pc) return rawText;
    return await controlClient.serializeConfig(updated);
  } catch {
    return rawText;
  }
}

async function handleConnect() {
  if (!selectedPath.value) return;
  error.value = "";
  busy.value = true;
  try {
    const entry = configs.value.find((c) => c.path === selectedPath.value);
    const text = await configStore.loadConfig(selectedPath.value);
    const sendText = await effectiveConfigText(selectedPath.value, text);
    const status = await controlClient.connect(sendText, entry?.name ?? "VEIL");
    emit("connected", status);
  } catch (e) {
    error.value = String(e);
  } finally {
    busy.value = false;
  }
}

async function handleDisconnect() {
  error.value = "";
  busy.value = true;
  try {
    const status = await controlClient.disconnect();
    emit("connected", status);
  } catch (e) {
    error.value = String(e);
  } finally {
    busy.value = false;
  }
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let value = n / 1024;
  let i = 0;
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024;
    i++;
  }
  return `${value.toFixed(1)} ${units[i]}`;
}

function formatHandshake(unixSeconds?: number): string {
  if (!unixSeconds) return "never";
  const seconds = Math.floor(Date.now() / 1000) - unixSeconds;
  if (seconds < 5) return "just now";
  if (seconds < 60) return `${seconds}s ago`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
  return `${Math.floor(seconds / 3600)}h ago`;
}
</script>

<template>
  <div class="grid gap-4 md:grid-cols-2">
    <Card>
      <CardHeader>
        <CardTitle>Connection</CardTitle>
        <CardDescription>Pick a config and connect the tunnel.</CardDescription>
      </CardHeader>
      <CardContent class="flex flex-col gap-4">
        <div class="flex flex-col gap-2">
          <Label>Config</Label>
          <Select v-model="selectedPath" :disabled="isConnected || isConnecting">
            <SelectTrigger class="w-full">
              <SelectValue placeholder="Select a config" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="c in configs" :key="c.path" :value="c.path">
                {{ c.name }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div class="flex gap-2">
          <Button
            v-if="!isConnected"
            :disabled="busy || isConnecting || !selectedPath"
            @click="handleConnect"
          >
            {{ isConnecting ? "Connecting…" : "Connect" }}
          </Button>
          <Button v-else variant="destructive" :disabled="busy" @click="handleDisconnect">
            Disconnect
          </Button>
          <Button variant="outline" @click="handleImportFile">Import file…</Button>
        </div>

        <div class="flex flex-col gap-2 border-t border-border pt-4">
          <Label>Import a veil:// link</Label>
          <div class="flex gap-2">
            <Input v-model="importLink" placeholder="veil://..." />
            <Button variant="secondary" :disabled="!importLink" @click="handleImportLink">
              Import link
            </Button>
          </div>
        </div>

        <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
      </CardContent>
    </Card>

    <Card>
      <CardHeader>
        <CardTitle>Status</CardTitle>
        <CardDescription>{{ status.iface || "No active interface" }}</CardDescription>
      </CardHeader>
      <CardContent>
        <div v-if="!status.peers?.length" class="text-sm text-muted-foreground">
          No peer statistics yet.
        </div>
        <div v-for="peer in status.peers" :key="peer.public_key" class="flex flex-col gap-1 py-2 border-b border-border last:border-0">
          <div class="flex items-center justify-between">
            <span class="font-mono text-xs text-muted-foreground">
              {{ peer.public_key.slice(0, 8) }}…
            </span>
            <span
              class="text-xs"
              :class="peer.connected ? 'text-veil-teal' : 'text-veil-slate'"
            >
              {{ peer.connected ? "connected" : "idle" }}
            </span>
          </div>
          <div class="text-xs text-muted-foreground">{{ peer.endpoint || "—" }}</div>
          <div class="flex justify-between text-xs text-muted-foreground">
            <span>Handshake: {{ formatHandshake(peer.last_handshake) }}</span>
            <span>↑{{ formatBytes(peer.tx_bytes) }} ↓{{ formatBytes(peer.rx_bytes) }}</span>
          </div>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
