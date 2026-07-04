<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import StatusBadge from "@/components/StatusBadge.vue";
import ConnectionTab from "@/views/ConnectionTab.vue";
import SplitTunnelTab from "@/views/SplitTunnelTab.vue";
import LogsTab from "@/views/LogsTab.vue";
import AdvancedTab from "@/views/AdvancedTab.vue";
import { onStatusUpdate, status as fetchStatus } from "@/lib/control-client";
import type { Status } from "@/lib/control-types";
import type { UnlistenFn } from "@tauri-apps/api/event";

const status = ref<Status>({ state: "disconnected" });
let unlisten: UnlistenFn | undefined;

onMounted(async () => {
  try {
    status.value = await fetchStatus();
  } catch {
    // Sidecar may still be starting up; the background poll's first tick
    // will populate this shortly.
  }
  unlisten = await onStatusUpdate((s) => {
    status.value = s;
  });
});

onUnmounted(() => {
  unlisten?.();
});
</script>

<template>
  <div class="min-h-screen bg-background text-foreground flex flex-col">
    <header class="flex items-center justify-between border-b border-border px-6 py-4">
      <div class="flex items-center gap-3">
        <img src="/veil-mark.png" alt="VEIL" class="h-7 w-7" />
        <span class="text-lg font-semibold tracking-tight">VEIL</span>
      </div>
      <StatusBadge :status="status" />
    </header>

    <main class="flex-1 p-6">
      <Tabs default-value="connection" class="w-full">
        <TabsList class="grid w-full grid-cols-4">
          <TabsTrigger value="connection">Connection</TabsTrigger>
          <TabsTrigger value="split-tunnel">Split Tunnel</TabsTrigger>
          <TabsTrigger value="logs">Logs</TabsTrigger>
          <TabsTrigger value="advanced">Advanced</TabsTrigger>
        </TabsList>
        <TabsContent value="connection">
          <ConnectionTab :status="status" @connected="status = $event" />
        </TabsContent>
        <TabsContent value="split-tunnel">
          <SplitTunnelTab />
        </TabsContent>
        <TabsContent value="logs">
          <LogsTab />
        </TabsContent>
        <TabsContent value="advanced">
          <AdvancedTab />
        </TabsContent>
      </Tabs>
    </main>
  </div>
</template>
