<script setup lang="ts">
import type { Day } from "../lib/types";

const props = defineProps<{ day: Day }>();

const g = (n: number) => Math.round(n * 10) / 10;
const pct = (eaten: number, target: number) => (target > 0 ? Math.min((eaten / target) * 100, 100) : 0);

const macros = [
  { key: "carb", label: "Sacharidy", color: "#0ea5e9" },
  { key: "protein", label: "Bílkoviny", color: "#10b981" },
  { key: "fat", label: "Tuky", color: "#f59e0b" },
] as const;
</script>

<template>
  <div class="w-full space-y-3">
    <div v-for="m in macros" :key="m.key">
      <div class="mb-1 flex justify-between text-sm sm:text-base">
        <span>{{ m.label }}</span>
        <span class="tabular-nums text-gray-500">{{ g(day.eaten[m.key]) }} / {{ g(day.target[m.key]) }} g</span>
      </div>
      <!-- muted track in the macro's own colour (hex + ~18% alpha) -->
      <div class="h-2.5 w-full rounded-full sm:h-3" :style="{ backgroundColor: m.color + '2e' }">
        <div
          class="h-2.5 rounded-full transition-all duration-500 sm:h-3"
          :style="{ width: pct(day.eaten[m.key], day.target[m.key]) + '%', backgroundColor: m.color }"
        ></div>
      </div>
    </div>
  </div>
</template>
