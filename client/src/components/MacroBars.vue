<script setup lang="ts">
import { computed } from "vue";
import type { Day } from "../lib/types";
import { t } from "../lib/i18n";
import { useAnimatedNumber } from "../composables/useAnimatedNumber";

// `reveal` is owned by DaySummary so the ring, the bars and every figure share
// one grow-in rather than each running its own, a frame apart.
const props = withDefaults(defineProps<{ day: Day; reveal?: number }>(), { reveal: 1 });

const g = (n: number) => Math.round(n * 10) / 10;
const pct = (eaten: number, target: number) => (target > 0 ? Math.min((eaten / target) * 100, 100) : 0);

// One tween per macro. The keys are fixed, so these are declared up front rather
// than created inside the render loop — composables cannot be called there.
const tweened = {
  carb: useAnimatedNumber(() => props.day.eaten.carb),
  protein: useAnimatedNumber(() => props.day.eaten.protein),
  fat: useAnimatedNumber(() => props.day.eaten.fat),
} as const;
const shown = (key: "carb" | "protein" | "fat") => tweened[key].value * props.reveal;

const macros = computed(
  () =>
    [
      { key: "carb", label: t("macros.carb"), color: "#0ea5e9" },
      { key: "protein", label: t("macros.protein"), color: "#10b981" },
      { key: "fat", label: t("macros.fat"), color: "#f59e0b" },
    ] as const,
);
</script>

<template>
  <div class="w-full space-y-3">
    <div v-for="m in macros" :key="m.key">
      <div class="mb-1 flex justify-between text-sm sm:text-base">
        <span>{{ m.label }}</span>
        <span class="tabular-nums text-gray-500">{{ g(shown(m.key)) }} / {{ g(day.target[m.key]) }} g</span>
      </div>
      <!-- muted track in the macro's own colour (hex + ~18% alpha) -->
      <div class="h-2.5 w-full rounded-full sm:h-3" :style="{ backgroundColor: m.color + '2e' }">
        <!-- Width comes from the same tween as the figure above it, so the bar
             and the number are one motion rather than two CSS/JS timelines that
             only roughly agree. -->
        <div
          class="h-2.5 rounded-full sm:h-3"
          :style="{ width: pct(shown(m.key), day.target[m.key]) + '%', backgroundColor: m.color }"
        ></div>
      </div>
    </div>
  </div>
</template>
