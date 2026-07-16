<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { api } from "../lib/api";
import type { Stats } from "../lib/types";
import PeriodChart, { type Point } from "../components/PeriodChart.vue";

// ── date helpers (local calendar, matching DiaryView) ────────────────────────
const pad = (n: number) => String(n).padStart(2, "0");
function todayISO() {
  const d = new Date();
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}
function addDaysISO(iso: string, n: number) {
  const t = new Date(iso + "T00:00:00Z");
  t.setUTCDate(t.getUTCDate() + n);
  return t.toISOString().slice(0, 10);
}

// ── period selection ─────────────────────────────────────────────────────────
type PeriodKey = "7" | "30" | "90" | "custom";
const periodItems = [
  { label: "Posledních 7 dní", value: "7" },
  { label: "Posledních 30 dní", value: "30" },
  { label: "Posledních 90 dní", value: "90" },
  { label: "Vlastní období", value: "custom" },
];
const period = ref<PeriodKey>("30");

// Custom bounds, defaulted to the last 30 days so switching to "Vlastní" starts
// from a sensible window rather than an empty one.
const customTo = ref(todayISO());
const customFrom = ref(addDaysISO(todayISO(), -29));

const range = computed<{ from: string; to: string }>(() => {
  if (period.value === "custom") return { from: customFrom.value, to: customTo.value };
  const days = Number(period.value);
  const to = todayISO();
  return { from: addDaysISO(to, -(days - 1)), to };
});

// ── metric selection ─────────────────────────────────────────────────────────
type MetricKey = "kcal" | "carb" | "protein" | "fat";
const METRICS = [
  { key: "kcal", label: "Kalorie", unit: "kcal", color: "#10b981" },
  { key: "carb", label: "Sacharidy", unit: "g", color: "#0ea5e9" },
  { key: "protein", label: "Bílkoviny", unit: "g", color: "#8b5cf6" },
  { key: "fat", label: "Tuky", unit: "g", color: "#f59e0b" },
] as const;
const metricKey = ref<MetricKey>("kcal");
const active = computed(() => METRICS.find((m) => m.key === metricKey.value)!);
const goal = computed(() => (stats.value ? stats.value.goal[metricKey.value] : 0));

// ── data ─────────────────────────────────────────────────────────────────────
const stats = ref<Stats | null>(null);
const loading = ref(false);
const error = ref(false);

async function load() {
  loading.value = true;
  error.value = false;
  try {
    stats.value = await api.getStats(range.value.from, range.value.to);
  } catch {
    error.value = true;
    stats.value = null;
  } finally {
    loading.value = false;
  }
}
// Refetch only when the window changes; switching metric is derived client-side.
watch(() => [range.value.from, range.value.to], load, { immediate: true });

// Fill every calendar day in [from, to] (missing days = 0) and attach a trailing
// 7-day rolling average, so the chart has a continuous axis and a smooth line.
const series = computed<Point[]>(() => {
  const s = stats.value;
  if (!s) return [];
  const byDate = new Map(s.days.map((d) => [d.date, d]));
  const raw: { date: string; value: number }[] = [];
  let cur = s.from;
  while (cur <= s.to && raw.length < 800) {
    raw.push({ date: cur, value: byDate.get(cur)?.[metricKey.value] ?? 0 });
    cur = addDaysISO(cur, 1);
  }
  return raw.map((p, i) => {
    const win = raw.slice(Math.max(0, i - 6), i + 1);
    return { date: p.date, value: p.value, avg: win.reduce((a, b) => a + b.value, 0) / win.length };
  });
});

const hasData = computed(() => series.value.some((p) => p.value > 0));

const fmt = (v: number) => (metricKey.value === "kcal" ? Math.round(v) : Math.round(v * 10) / 10);

const summary = computed(() => {
  const pts = series.value;
  const logged = pts.filter((p) => p.value > 0);
  const total = pts.reduce((a, b) => a + b.value, 0);
  return {
    avg: logged.length ? total / logged.length : 0,
    total,
    loggedCount: logged.length,
    totalDays: pts.length,
  };
});
</script>

<template>
  <div class="space-y-5">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h1 class="text-lg font-semibold sm:text-xl">Statistika</h1>
      <USelect v-model="period" :items="periodItems" size="sm" class="w-48" />
    </div>

    <!-- custom range -->
    <div v-if="period === 'custom'" class="flex flex-wrap items-center gap-2 text-sm">
      <UInput v-model="customFrom" type="date" size="sm" :max="customTo" />
      <span class="text-gray-400">–</span>
      <UInput v-model="customTo" type="date" size="sm" :min="customFrom" :max="todayISO()" />
    </div>

    <!-- metric toggle -->
    <div class="flex flex-wrap gap-1.5">
      <UButton
        v-for="m in METRICS"
        :key="m.key"
        size="xs"
        :color="metricKey === m.key ? 'primary' : 'neutral'"
        :variant="metricKey === m.key ? 'solid' : 'soft'"
        :label="m.label"
        @click="metricKey = m.key"
      />
    </div>

    <!-- summary -->
    <div class="grid grid-cols-3 gap-2 sm:gap-3">
      <UCard :ui="{ body: 'p-3 sm:p-4' }">
        <div class="text-xs text-gray-500">Průměr / den</div>
        <div class="mt-0.5 text-lg font-semibold tabular-nums sm:text-xl">
          {{ fmt(summary.avg) }} <span class="text-sm font-normal text-gray-400">{{ active.unit }}</span>
        </div>
      </UCard>
      <UCard :ui="{ body: 'p-3 sm:p-4' }">
        <div class="text-xs text-gray-500">Celkem</div>
        <div class="mt-0.5 text-lg font-semibold tabular-nums sm:text-xl">
          {{ fmt(summary.total) }} <span class="text-sm font-normal text-gray-400">{{ active.unit }}</span>
        </div>
      </UCard>
      <UCard :ui="{ body: 'p-3 sm:p-4' }">
        <div class="text-xs text-gray-500">Zapsané dny</div>
        <div class="mt-0.5 text-lg font-semibold tabular-nums sm:text-xl">
          {{ summary.loggedCount }} <span class="text-sm font-normal text-gray-400">/ {{ summary.totalDays }}</span>
        </div>
      </UCard>
    </div>

    <!-- chart -->
    <UCard :ui="{ body: 'p-3 sm:p-4' }">
      <div v-if="loading" class="grid h-[260px] place-items-center text-sm text-gray-400">Načítání…</div>
      <div v-else-if="error" class="grid h-[260px] place-items-center text-sm text-red-500">Nepodařilo se načíst data.</div>
      <div v-else-if="!hasData" class="grid h-[260px] place-items-center text-center text-sm text-gray-500">
        V tomto období nejsou žádná data.
      </div>
      <template v-else>
        <div class="mb-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-gray-500">
          <span class="flex items-center gap-1.5">
            <span class="inline-block h-2.5 w-2.5 rounded-sm" :style="{ backgroundColor: active.color, opacity: 0.85 }"></span>
            denní {{ active.label.toLowerCase() }}
          </span>
          <span class="flex items-center gap-1.5">
            <span class="inline-block h-0.5 w-4 rounded" :style="{ backgroundColor: active.color }"></span>
            7denní průměr
          </span>
          <span v-if="goal > 0" class="flex items-center gap-1.5">
            <span class="inline-block h-0 w-4 border-t border-dashed border-gray-400"></span>
            denní cíl
          </span>
        </div>
        <PeriodChart :points="series" :goal="goal" :unit="active.unit" :color="active.color" />
      </template>
    </UCard>
  </div>
</template>
