<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
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
import { loadDisallowed, saveDisallowed, subtractCIDRs, type DisallowedDoc } from "@/lib/disallowed";
import type { ParsedConfig } from "@/lib/control-types";

// Structured per-peer Allowed/Disallowed CIDR editor. AllowedIPs come from
// the parsed .conf (the on-disk source of truth, same file the Advanced tab
// edits as raw text); Disallowed is a GUI-only carve-out stored alongside
// the config (see lib/disallowed.ts) and only ever narrows what's actually
// sent to the sidecar at Connect time — it never touches the on-disk
// AllowedIPs or the config file's own validation.
interface PeerEdit {
  publicKey: string;
  allowedIPs: string[];
  disallowedIPs: string[];
  newAllowed: string;
  newDisallowed: string;
}

const configs = ref<ConfigEntry[]>([]);
const selectedPath = ref("");
const parsedConfig = ref<ParsedConfig | null>(null);
const peerEdits = ref<PeerEdit[]>([]);
const error = ref("");
const info = ref("");
const busy = ref(false);

async function refreshConfigs() {
  configs.value = await configStore.listConfigs();
  if (!selectedPath.value && configs.value.length > 0) {
    selectedPath.value = configs.value[0].path;
  }
}

async function load() {
  error.value = "";
  info.value = "";
  parsedConfig.value = null;
  peerEdits.value = [];
  if (!selectedPath.value) return;
  busy.value = true;
  try {
    const text = await configStore.loadConfig(selectedPath.value);
    const pc = await controlClient.parseConfig(text);
    const doc: DisallowedDoc = await loadDisallowed(selectedPath.value);
    parsedConfig.value = pc;
    peerEdits.value = pc.peers.map((peer) => ({
      publicKey: peer.publicKey,
      allowedIPs: [...(peer.allowedIPs ?? [])],
      disallowedIPs: [...(doc.per_peer[peer.publicKey] ?? [])],
      newAllowed: "",
      newDisallowed: "",
    }));
  } catch (e) {
    error.value = String(e);
  } finally {
    busy.value = false;
  }
}

function effectiveFor(edit: PeerEdit): string[] {
  return subtractCIDRs(edit.allowedIPs, edit.disallowedIPs);
}

function addAllowed(edit: PeerEdit) {
  const value = edit.newAllowed.trim();
  if (!value || edit.allowedIPs.includes(value)) return;
  edit.allowedIPs.push(value);
  edit.newAllowed = "";
}

function removeAllowed(edit: PeerEdit, cidr: string) {
  edit.allowedIPs = edit.allowedIPs.filter((c) => c !== cidr);
}

function addDisallowed(edit: PeerEdit) {
  const value = edit.newDisallowed.trim();
  if (!value || edit.disallowedIPs.includes(value)) return;
  edit.disallowedIPs.push(value);
  edit.newDisallowed = "";
}

function removeDisallowed(edit: PeerEdit, cidr: string) {
  edit.disallowedIPs = edit.disallowedIPs.filter((c) => c !== cidr);
}

function peerLabel(publicKey: string): string {
  return `${publicKey.slice(0, 12)}…`;
}

async function save() {
  error.value = "";
  info.value = "";
  if (!parsedConfig.value || !selectedPath.value) return;
  const entry = configs.value.find((c) => c.path === selectedPath.value);
  if (!entry) return;
  busy.value = true;
  try {
    const updated: ParsedConfig = {
      ...parsedConfig.value,
      peers: parsedConfig.value.peers.map((peer, i) => ({
        ...peer,
        allowedIPs: [...peerEdits.value[i].allowedIPs],
      })),
    };
    const text = await controlClient.serializeConfig(updated);
    await configStore.saveConfig(entry.name, text);

    const doc: DisallowedDoc = { per_peer: {} };
    for (const edit of peerEdits.value) {
      if (edit.disallowedIPs.length > 0) {
        doc.per_peer[edit.publicKey] = [...edit.disallowedIPs];
      }
    }
    await saveDisallowed(selectedPath.value, doc);

    parsedConfig.value = updated;
    info.value = "Saved.";
  } catch (e) {
    error.value = String(e);
  } finally {
    busy.value = false;
  }
}

const hasPeers = computed(() => peerEdits.value.length > 0);

onMounted(async () => {
  await refreshConfigs();
  await load();
});
</script>

<template>
  <Card>
    <CardHeader class="flex flex-row items-center justify-between">
      <div>
        <CardTitle>Split Tunnel</CardTitle>
        <CardDescription>
          Carve subnets out of a peer's Allowed IPs without touching the config file.
        </CardDescription>
      </div>
      <div class="flex items-center gap-2">
        <Select v-model="selectedPath" @update:model-value="load">
          <SelectTrigger class="w-64">
            <SelectValue placeholder="Select a config" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="c in configs" :key="c.path" :value="c.path">
              {{ c.name }}
            </SelectItem>
          </SelectContent>
        </Select>
        <Button :disabled="busy || !parsedConfig" @click="save">Save</Button>
      </div>
    </CardHeader>
    <CardContent class="flex flex-col gap-6">
      <p v-if="!hasPeers && !error" class="text-sm text-muted-foreground">
        No peers in this config.
      </p>
      <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
      <p v-if="info" class="text-sm text-muted-foreground">{{ info }}</p>

      <div
        v-for="(edit, i) in peerEdits"
        :key="edit.publicKey"
        class="flex flex-col gap-4"
      >
        <Separator v-if="i > 0" />
        <div class="font-mono text-xs text-muted-foreground">{{ peerLabel(edit.publicKey) }}</div>

        <div class="grid gap-4 md:grid-cols-2">
          <div class="flex flex-col gap-2">
            <Label>Allowed IPs</Label>
            <div class="flex flex-wrap gap-2">
              <Badge v-for="cidr in edit.allowedIPs" :key="cidr" variant="secondary" class="gap-1">
                {{ cidr }}
                <button
                  type="button"
                  class="ml-1 text-muted-foreground hover:text-destructive"
                  @click="removeAllowed(edit, cidr)"
                >
                  ×
                </button>
              </Badge>
              <span v-if="edit.allowedIPs.length === 0" class="text-xs text-muted-foreground">
                none
              </span>
            </div>
            <div class="flex gap-2">
              <Input
                v-model="edit.newAllowed"
                placeholder="10.0.0.0/24"
                @keyup.enter="addAllowed(edit)"
              />
              <Button variant="outline" size="sm" @click="addAllowed(edit)">Add</Button>
            </div>
          </div>

          <div class="flex flex-col gap-2">
            <Label>Disallowed (carved out)</Label>
            <div class="flex flex-wrap gap-2">
              <Badge v-for="cidr in edit.disallowedIPs" :key="cidr" variant="outline" class="gap-1">
                {{ cidr }}
                <button
                  type="button"
                  class="ml-1 text-muted-foreground hover:text-destructive"
                  @click="removeDisallowed(edit, cidr)"
                >
                  ×
                </button>
              </Badge>
              <span v-if="edit.disallowedIPs.length === 0" class="text-xs text-muted-foreground">
                none
              </span>
            </div>
            <div class="flex gap-2">
              <Input
                v-model="edit.newDisallowed"
                placeholder="192.168.1.0/24"
                @keyup.enter="addDisallowed(edit)"
              />
              <Button variant="outline" size="sm" @click="addDisallowed(edit)">Add</Button>
            </div>
          </div>
        </div>

        <div class="flex flex-col gap-1">
          <Label class="text-muted-foreground">Effective (sent at connect)</Label>
          <div class="flex flex-wrap gap-2">
            <Badge v-for="cidr in effectiveFor(edit)" :key="cidr" class="bg-veil-teal/20 text-veil-teal">
              {{ cidr }}
            </Badge>
            <span v-if="effectiveFor(edit).length === 0" class="text-xs text-muted-foreground">
              nothing routed
            </span>
          </div>
        </div>
      </div>
    </CardContent>
  </Card>
</template>
