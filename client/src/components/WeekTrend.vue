<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { api } from "../lib/api";
import { t, weekdayShort } from "../lib/i18n";
import { useAnimatedNumber, useReveal } from "../composables/useAnimatedNumber";
import type { Stats } from "../lib/types";

// Seven days ending on the day being viewed, so browsing back through the diary
// carries its own context rather than always showing the current week.
const props = defineProps<{ date: string }>();
const router = useRouter();

const stats = ref<Stats | null>(null);
const failed = ref(false);

function shift(iso: string, days: number) {
  const d = new Date(`${iso}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + days);
  return d.toISOString().slice(0, 10);
}

async function load(date: string) {
  failed.value = false;
  try {
    stats.value = await api.getStats(shift(date, -6), date);
  } catch {
    stats.value = null;
    failed.value = true;
  }
}
watch(() => props.date, load, { immediate: true });

const goal = computed(() => stats.value?.goal.kcal ?? 0);

// The API returns only days that have entries; the gaps are real information
// ("nothing logged"), so they are filled in rather than skipped.
const days = computed(() => {
  const logged = new Map((stats.value?.days ?? []).map((d) => [d.date, d.kcal]));
  return Array.from({ length: 7 }, (_, i) => {
    const iso = shift(props.date, i - 6);
    const kcal = logged.get(iso);
    return {
      iso,
      label: weekdayShort(iso),
      kcal: kcal ?? 0,
      logged: kcal !== undefined,
      current: iso === props.date,
    };
  });
});

// Bars are scaled against the goal, capped so one huge day cannot flatten the
// rest. The dashed line marks 100%.
const CEILING = 150;
const scale = (kcal: number) => (goal.value > 0 ? Math.min((kcal / goal.value) * 100, CEILING) : 0);

// Grow the bars up from the axis, restarting whenever a new week arrives —
// keyed on the data rather than the date so the reveal begins when there is
// something to reveal, not while the card is still empty.
const reveal = useReveal(() => stats.value);

// Each bar starts a little after the one before it, so the week reads left to
// right as it draws. On bars this short a simultaneous grow is easy to miss;
// the cascade is what makes the motion legible.
const STAGGER = 0.07; // share of the timeline between neighbouring bars
const WINDOW = 1 - STAGGER * 6; // six gaps across seven bars
const barProgress = (index: number) =>
  Math.min(Math.max((reveal.value - index * STAGGER) / WINDOW, 0), 1);
const barHeight = (kcal: number, index: number) =>
  `${(scale(kcal) / CEILING) * 100 * barProgress(index)}%`;
const goalLine = `${(100 / CEILING) * 100}%`;

const average = computed(() => {
  const withData = days.value.filter((d) => d.logged);
  if (!withData.length) return 0;
  return withData.reduce((sum, d) => sum + d.kcal, 0) / withData.length;
});
const animatedAverage = useAnimatedNumber(() => average.value);
const shownAverage = computed(() => animatedAverage.value * reveal.value);
const hasData = computed(() => days.value.some((d) => d.logged));

// Hovering a day lifts its column, matching the statistics chart; clicking one
// opens it in the diary, the same gesture the chart's day level uses.
const hovered = ref<string | null>(null);
const openDay = (iso: string) => router.push({ path: "/", query: { date: iso } });
</script>

<template>
  <UCard :ui="{ header: 'p-3 sm:px-4', body: 'p-3 sm:p-4' }">
    <template #header>
      <div class="flex items-center justify-between gap-2">
        <h2 class="text-sm font-medium">{{ t("stats.lastWeek") }}</h2>
        <RouterLink
          to="/stats"
          class="rounded-lg px-2 py-1 text-xs text-gray-500 outline-none transition hover:bg-gray-100 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-emerald-500/60 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-100"
        >
          {{ t("stats.seeAll") }}
        </RouterLink>
      </div>
    </template>

    <p v-if="failed" class="py-4 text-center text-xs text-gray-500">{{ t("stats.loadFailed") }}</p>
    <p v-else-if="!hasData" class="py-4 text-center text-xs text-gray-500">{{ t("stats.noData") }}</p>

    <template v-else>
      <div class="relative h-24">
        <!-- the daily goal -->
        <div
          class="pointer-events-none absolute inset-x-0 border-t border-dashed border-gray-300 dark:border-gray-600"
          :style="{ bottom: goalLine }"
        />
        <div class="flex h-full items-end gap-1" @pointerleave="hovered = null">
          <button
            v-for="(d, i) in days"
            :key="d.iso"
            type="button"
            class="group flex h-full flex-1 items-end rounded-md px-0.5 outline-none transition focus-visible:ring-2 focus-visible:ring-emerald-500/60"
            :class="hovered === d.iso ? 'bg-gray-900/5 dark:bg-white/5' : ''"
            :aria-label="d.logged ? `${d.label}: ${Math.round(d.kcal)} kcal` : d.label"
            @pointerenter="hovered = d.iso"
            @focus="hovered = d.iso"
            @blur="hovered = null"
            @click="openDay(d.iso)"
          >
            <!-- min-height keeps a logged-but-tiny day visible rather than
                 indistinguishable from an empty one -->
            <div
              v-if="d.logged"
              class="w-full rounded-t-md"
              :class="[
                d.kcal > goal ? 'bg-rose-400 dark:bg-rose-500' : 'bg-emerald-400 dark:bg-emerald-500',
                d.current ? 'ring-2 ring-gray-900/70 dark:ring-white/70' : '',
              ]"
              :style="{ height: barProgress(i) > 0 ? `max(3px, ${barHeight(d.kcal, i)})` : '0px' }"
            />
            <div
              v-else
              class="h-1 w-full rounded-full bg-gray-200 dark:bg-gray-700"
              :class="d.current ? 'ring-2 ring-gray-900/70 dark:ring-white/70' : ''"
            />
          </button>
        </div>
      </div>

      <div class="mt-1.5 flex gap-1">
        <span
          v-for="d in days"
          :key="d.iso"
          class="flex-1 truncate text-center text-[10px] transition-colors"
          :class="hovered === d.iso || d.current ? 'font-semibold text-gray-700 dark:text-gray-200' : 'text-gray-400'"
        >{{ d.label }}</span>
      </div>

      <div class="mt-3 flex items-baseline justify-between border-t border-gray-100 pt-2.5 text-xs dark:border-gray-800">
        <span class="text-gray-500">{{ t("stats.avgPerDay") }}</span>
        <span class="tabular-nums font-semibold">{{ Math.round(shownAverage) }} <span class="font-normal text-gray-400">/ {{ Math.round(goal) }} kcal</span></span>
      </div>
    </template>
  </UCard>
</template>
