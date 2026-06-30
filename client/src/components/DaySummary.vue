<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import type { Day } from "../lib/types";
import RingChart from "./RingChart.vue";
import MacroBars from "./MacroBars.vue";

const props = defineProps<{ day: Day }>();

const k = (n: number) => Math.round(n);
const over = computed(() => props.day.eaten.kcal > props.day.target.kcal);

// Smaller ring on phones, full size from the `sm` breakpoint up.
const wide = ref(false);
let mq: MediaQueryList | undefined;
const sync = () => (wide.value = !!mq?.matches);
onMounted(() => {
  mq = window.matchMedia("(min-width: 640px)");
  sync();
  mq.addEventListener("change", sync);
});
onUnmounted(() => mq?.removeEventListener("change", sync));
</script>

<template>
  <UCard>
    <div class="grid items-center gap-6 sm:grid-cols-2">
      <!-- kcal progress ring (track is a muted shade of the same colour) -->
      <div class="flex items-center justify-center gap-4 sm:justify-start">
        <RingChart
          :value="day.eaten.kcal"
          :max="day.target.kcal"
          :size="wide ? 150 : 104"
          :thickness="wide ? 14 : 10"
          :color="over ? '#f43f5e' : '#10b981'"
        >
          <div>
            <div class="text-xl font-semibold tabular-nums sm:text-3xl">{{ k(day.remaining.kcal) }}</div>
            <div class="text-xs text-gray-500 sm:text-sm">kcal zbývá</div>
          </div>
        </RingChart>
        <div class="text-sm sm:text-base">
          <div class="tabular-nums"><b>{{ k(day.eaten.kcal) }}</b> / {{ k(day.target.kcal) }} kcal</div>
          <div class="text-gray-500">snědeno / cíl</div>
        </div>
      </div>

      <!-- per-macro progress bars -->
      <MacroBars :day="day" />
    </div>
  </UCard>
</template>
