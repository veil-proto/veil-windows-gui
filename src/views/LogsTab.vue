<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref } from "vue";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import * as controlClient from "@/lib/control-client";
import type { LogLine } from "@/lib/control-types";
import type { UnlistenFn } from "@tauri-apps/api/event";

// Same ring-buffer-cursor pattern as the old GUI: track the highest Seq
// we've rendered and only ask for/append lines after it, so re-opening this
// tab or a slow poll tick never re-renders the whole backlog.
const lines = ref<LogLine[]>([]);
const lastSeq = ref(0);
const viewportRef = ref<HTMLElement | null>(null);
let unlisten: UnlistenFn | undefined;

function appendLines(newLines: LogLine[]) {
  if (newLines.length === 0) return;
  lines.value.push(...newLines);
  lastSeq.value = newLines[newLines.length - 1].seq;
  // Cap client-side memory the same way the sidecar's own ring buffer does,
  // so leaving this tab open for a long session doesn't grow unbounded.
  if (lines.value.length > 2000) {
    lines.value.splice(0, lines.value.length - 2000);
  }
  nextTick(() => {
    viewportRef.value?.scrollTo({ top: viewportRef.value.scrollHeight });
  });
}

async function loadInitial() {
  try {
    const { logs } = await controlClient.logs(0);
    appendLines(logs);
  } catch {
    // Sidecar may still be starting; the shared poll (onLogsUpdate) will
    // catch us up once it's up.
  }
}

function levelClass(level?: string): string {
  switch (level?.toLowerCase()) {
    case "error":
      return "text-destructive";
    case "warn":
    case "warning":
      return "text-veil-warning";
    default:
      return "text-foreground";
  }
}

function formatTime(unixSeconds: number): string {
  return new Date(unixSeconds * 1000).toLocaleTimeString();
}

function clear() {
  lines.value = [];
}

onMounted(async () => {
  await loadInitial();
  unlisten = await controlClient.onLogsUpdate((newLines) => {
    // The shared poll tracks its own cursor independently; only take lines
    // we haven't already rendered via loadInitial/a previous tick.
    appendLines(newLines.filter((l) => l.seq > lastSeq.value));
  });
});

onUnmounted(() => {
  unlisten?.();
});
</script>

<template>
  <Card>
    <CardHeader class="flex flex-row items-center justify-between">
      <div>
        <CardTitle>Logs</CardTitle>
        <CardDescription>Live output from the tunnel backend.</CardDescription>
      </div>
      <Button variant="outline" size="sm" @click="clear">Clear</Button>
    </CardHeader>
    <CardContent>
      <div
        ref="viewportRef"
        class="h-[480px] overflow-y-auto rounded-md border border-border bg-input/40 p-3"
      >
        <p v-if="lines.length === 0" class="text-sm text-muted-foreground">
          Waiting for logs…
        </p>
        <div
          v-for="line in lines"
          :key="line.seq"
          class="font-mono text-xs leading-relaxed"
          :class="levelClass(line.level)"
        >
          <span class="text-muted-foreground">{{ formatTime(line.time) }}</span>
          {{ " " }}{{ line.msg }}
        </div>
      </div>
    </CardContent>
  </Card>
</template>
