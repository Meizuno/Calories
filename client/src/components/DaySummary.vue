<script setup lang="ts">
import { computed } from "vue";
import type { Day } from "../lib/types";
import RingChart from "./RingChart.vue";
import MacroBars from "./MacroBars.vue";

const props = defineProps<{ day: Day }>();

const k = (n: number) => Math.round(n);
const over = computed(() => props.day.eaten.kcal > props.day.target.kcal);
</script>

<template>
  <UCard>
    <div class="grid items-center gap-6 sm:grid-cols-2">
      <!-- kcal progress ring (track is a muted shade of the same colour) -->
      <div class="flex items-center gap-4">
        <RingChart :value="day.eaten.kcal" :max="day.target.kcal" :color="over ? '#f43f5e' : '#10b981'">
          <div>
            <div class="text-2xl font-semibold tabular-nums sm:text-3xl">{{ k(day.remaining.kcal) }}</div>
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
