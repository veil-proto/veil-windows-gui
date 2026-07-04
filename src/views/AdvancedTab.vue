<script setup lang="ts">
import { onMounted, ref } from "vue";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import * as configStore from "@/lib/config-store";
import type { ConfigEntry } from "@/lib/config-store";

// The raw .conf editor: still the on-disk source of truth, same philosophy
// as the old Fyne GUI's Advanced tab. Structured editing (Split Tunnel tab)
// round-trips through this same text via parseConfig/serializeConfig — there
// is only ever one on-disk representation of a config.
const configs = ref<ConfigEntry[]>([]);
const selectedPath = ref("");
const text = ref("");
const saved = ref(true);
const error = ref("");
const info = ref("");

async function refresh() {
  configs.value = await configStore.listConfigs();
  if (!selectedPath.value && configs.value.length > 0) {
    selectedPath.value = configs.value[0].path;
  }
}

async function loadSelected() {
  error.value = "";
  info.value = "";
  if (!selectedPath.value) {
    text.value = "";
    return;
  }
  try {
    text.value = await configStore.loadConfig(selectedPath.value);
    saved.value = true;
  } catch (e) {
    error.value = String(e);
  }
}

async function save() {
  error.value = "";
  info.value = "";
  const entry = configs.value.find((c) => c.path === selectedPath.value);
  if (!entry) return;
  try {
    await configStore.saveConfig(entry.name, text.value);
    saved.value = true;
    info.value = "Saved.";
  } catch (e) {
    error.value = String(e);
  }
}

onMounted(async () => {
  await refresh();
  await loadSelected();
});
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle>Advanced</CardTitle>
      <CardDescription>
        Edit the raw .conf text directly. This is the file actually used to
        connect — the Split Tunnel tab edits and saves the same file.
      </CardDescription>
    </CardHeader>
    <CardContent class="flex flex-col gap-4">
      <div class="flex items-center gap-2">
        <Select v-model="selectedPath" @update:model-value="loadSelected">
          <SelectTrigger class="w-64">
            <SelectValue placeholder="Select a config" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="c in configs" :key="c.path" :value="c.path">
              {{ c.name }}
            </SelectItem>
          </SelectContent>
        </Select>
        <Button :disabled="!selectedPath" @click="save">Save</Button>
        <span v-if="info" class="text-sm text-muted-foreground">{{ info }}</span>
      </div>

      <Textarea
        v-model="text"
        class="min-h-[420px] font-mono text-xs"
        spellcheck="false"
        @input="saved = false"
      />

      <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
    </CardContent>
  </Card>
</template>
