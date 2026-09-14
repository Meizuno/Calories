<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import type { Day } from "../lib/types";
import RingChart from "./RingChart.vue";
import { useAnimatedNumber, useReveal } from "../composables/useAnimatedNumber";
import MacroBars from "./MacroBars.vue";
import { t } from "../lib/i18n";

// `sidebar` marks the instance that moves into the diary's xl column: it keeps
// the normal two-column ring/bars layout at medium widths and collapses back to
// one column only once it is actually narrow.
const props = defineProps<{ day: Day; sidebar?: boolean }>();

const k = (n: number) => Math.round(n);

// The ring and bars already glide between states; the figures used to jump.
// Tweening them makes a logged meal read as a change to today rather than a
// different screen appearing.
const remaining = useAnimatedNumber(() => props.day.remaining.kcal);
const eaten = useAnimatedNumber(() => props.day.eaten.kcal);

// Grow in on first render and whenever the day changes, the same reveal the
// statistics chart uses — a different day is a different dataset. Editing the
// day you are already on keeps the tween-between behaviour instead, so adding
// a meal nudges the figures rather than rebuilding the card.
const reveal = useReveal(() => props.day.date);
const shownRemaining = computed(() => remaining.value * reveal.value);
const shownEaten = computed(() => eaten.value * reveal.value);
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
  <UCard :ui="{ header: 'p-3 sm:px-4' }">
    <template v-if="$slots.header" #header>
      <slot name="header" />
    </template>

    <div class="grid items-center gap-6 sm:grid-cols-2" :class="sidebar ? 'xl:grid-cols-1' : ''">
      <!-- kcal progress ring (track is a muted shade of the same colour) -->
      <div class="flex items-center justify-center gap-4 sm:justify-start">
        <RingChart
          :value="shownEaten"
          animated
          :max="day.target.kcal"
          :size="wide ? 150 : 104"
          :thickness="wide ? 14 : 10"
          :color="over ? '#f43f5e' : '#10b981'"
        >
          <div>
            <div class="text-xl font-semibold tabular-nums sm:text-3xl">{{ k(shownRemaining) }}</div>
            <div class="text-xs text-gray-500 sm:text-sm">{{ t("day.remaining") }}</div>
          </div>
        </RingChart>
        <div class="text-sm sm:text-base">
          <div class="tabular-nums"><b>{{ k(shownEaten) }}</b> / {{ k(day.target.kcal) }} kcal</div>
          <div class="text-gray-500">{{ t("day.eatenOfGoal") }}</div>
        </div>
      </div>

      <!-- per-macro progress bars -->
      <MacroBars :day="day" :reveal="reveal" />
    </div>
  </UCard>
</template>
