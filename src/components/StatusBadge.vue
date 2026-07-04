<script setup lang="ts">
import { computed } from "vue";
import { Badge } from "@/components/ui/badge";
import type { Status } from "@/lib/control-types";

const props = defineProps<{ status: Status }>();

// Mirrors the old Fyne GUI's statusTone mapping (widgets.go): neutral/slate
// for disconnected, progress/violet for connecting, positive/teal once
// connected.
const label = computed(() => {
  switch (props.status.state) {
    case "connected":
      return props.status.name ? `Connected · ${props.status.name}` : "Connected";
    case "connecting":
      return "Connecting…";
    default:
      return "Disconnected";
  }
});

const toneClass = computed(() => {
  switch (props.status.state) {
    case "connected":
      return "bg-veil-teal/15 text-veil-teal border-veil-teal/40";
    case "connecting":
      return "bg-accent/15 text-accent border-accent/40";
    default:
      return "bg-veil-slate/15 text-veil-slate border-veil-slate/40";
  }
});

const dotClass = computed(() => {
  switch (props.status.state) {
    case "connected":
      return "bg-veil-teal";
    case "connecting":
      return "bg-accent";
    default:
      return "bg-veil-slate";
  }
});
</script>

<template>
  <Badge variant="outline" class="gap-2 rounded-full px-3 py-1" :class="toneClass">
    <span class="h-2 w-2 rounded-full" :class="dotClass" />
    {{ label }}
  </Badge>
</template>
