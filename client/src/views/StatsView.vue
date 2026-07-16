<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api } from "../lib/api";
import type { Stats } from "../lib/types";
import PeriodChart, { type DayBars } from "../components/PeriodChart.vue";

// ── date helpers (local calendar, UTC math to avoid DST drift) ───────────────
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
// Monday of the week containing `iso` (weeks are Mon–Sun).
function mondayOf(iso: string) {
  const t = new Date(iso + "T00:00:00Z");
  return addDaysISO(iso, -((t.getUTCDay() + 6) % 7));
}
function firstOfMonth(iso: string) {
  return iso.slice(0, 8) + "01";
}
function lastOfMonth(iso: string) {
  const [y, m] = iso.split("-").map(Number);
  return new Date(Date.UTC(y, m, 0)).toISOString().slice(0, 10); // day 0 of next month
}
function addMonthsISO(iso: string, n: number) {
  const [y, m] = iso.split("-").map(Number);
  return new Date(Date.UTC(y, m - 1 + n, 1)).toISOString().slice(0, 10);
}

const WD = ["Ne", "Po", "Út", "St", "Čt", "Pá", "So"]; // getUTCDay: 0 = Sunday
const weekdayShort = (iso: string) => WD[new Date(iso + "T00:00:00Z").getUTCDay()];
const dayOfMonth = (iso: string) => Number(iso.split("-")[2]);
const CZ_MONTHS = ["leden", "únor", "březen", "duben", "květen", "červen", "červenec", "srpen", "září", "říjen", "listopad", "prosinec"];

// ── period stepping ──────────────────────────────────────────────────────────
// Period type + anchor date live in the URL (?type=week|month & date=YYYY-MM-DD)
// so the view is shareable, bookmarkable, and survives reload / back-forward.
// The URL is the single source of truth — these are writable computeds over it,
// so setting them just replaces the query (no two-way-sync loop).
const route = useRoute();
const router = useRouter();
function setQuery(patch: Record<string, string>) {
  router.replace({ query: { ...route.query, ...patch } });
}

type Gran = "week" | "month";
const gran = computed<Gran>({
  get: () => (route.query.type === "month" ? "month" : "week"),
  set: (v) => setQuery({ type: v }),
});
const anchor = computed<string>({
  get: () => {
    const d = route.query.date;
    return typeof d === "string" && /^\d{4}-\d{2}-\d{2}$/.test(d) ? d : todayISO();
  },
  set: (v) => setQuery({ date: v }),
});

const range = computed<{ from: string; to: string }>(() => {
  if (gran.value === "week") {
    const from = mondayOf(anchor.value);
    return { from, to: addDaysISO(from, 6) };
  }
  return { from: firstOfMonth(anchor.value), to: lastOfMonth(anchor.value) };
});

// The current week/month contains today, so there's nothing newer to step to.
const atCurrent = computed(() => todayISO() >= range.value.from && todayISO() <= range.value.to);

// Earliest day that has any logged data (from /api/days, sorted ascending).
// Stepping back is pointless once the window reaches past it — nothing before.
const firstLoggedDate = ref<string | null>(null);
api
  .getDays()
  .then((days) => (firstLoggedDate.value = days[0] ?? null))
  .catch(() => {});
const canPrev = computed(() => firstLoggedDate.value !== null && firstLoggedDate.value < range.value.from);

function prev() {
  if (!canPrev.value) return;
  anchor.value = gran.value === "week" ? addDaysISO(mondayOf(anchor.value), -7) : addMonthsISO(anchor.value, -1);
}
function next() {
  if (atCurrent.value) return;
  anchor.value = gran.value === "week" ? addDaysISO(mondayOf(anchor.value), 7) : addMonthsISO(anchor.value, 1);
}
function jumpNow() {
  anchor.value = todayISO();
}

const fmtDM = (iso: string) => `${dayOfMonth(iso)}. ${Number(iso.split("-")[1])}.`;
const periodLabel = computed(() => {
  if (gran.value === "week") return `${fmtDM(range.value.from)} – ${fmtDM(range.value.to)} ${range.value.to.slice(0, 4)}`;
  const [y, m] = range.value.from.split("-").map(Number);
  const name = CZ_MONTHS[m - 1];
  return `${name[0].toUpperCase()}${name.slice(1)} ${y}`;
});

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
watch(() => [range.value.from, range.value.to], load, { immediate: true });

const goals = computed(() => stats.value?.goal ?? { kcal: 0, carb: 0, protein: 0, fat: 0 });

interface DayPoint {
  date: string;
  logged: boolean;
  kcal: number;
  carb: number;
  protein: number;
  fat: number;
}

// Every calendar day in the window; missing days become zeroed, unlogged buckets.
const dailyFilled = computed<DayPoint[]>(() => {
  const s = stats.value;
  if (!s) return [];
  const byDate = new Map(s.days.map((d) => [d.date, d]));
  const out: DayPoint[] = [];
  let cur = s.from;
  while (cur <= s.to && out.length < 400) {
    const d = byDate.get(cur);
    out.push({ date: cur, logged: !!d, kcal: d?.kcal ?? 0, carb: d?.carb ?? 0, protein: d?.protein ?? 0, fat: d?.fat ?? 0 });
    cur = addDaysISO(cur, 1);
  }
  return out;
});

type Metric = "kcal" | "carb" | "protein" | "fat";
const METRIC_KEYS: Metric[] = ["kcal", "carb", "protein", "fat"];
const meanOverLogged = (days: DayPoint[], k: Metric) => {
  const logged = days.filter((d) => d.logged);
  return logged.length ? logged.reduce((a, b) => a + b[k], 0) / logged.length : 0;
};

// Turn absolute per-metric values into a bucket of 4 bars, each a percentage of
// that metric's daily goal — the shared scale that lets kcal + grams coexist.
function toBars(label: string, logged: boolean, vals: Record<Metric, number>): DayBars {
  const pct = {} as Record<Metric, number>;
  for (const k of METRIC_KEYS) pct[k] = goals.value[k] > 0 ? (vals[k] / goals.value[k]) * 100 : 0;
  return { label, logged, raw: { ...vals }, pct };
}

// Week → one bucket per day. Month → one bucket per Mon–Sun week, each the
// average over that week's logged days ("weekly average", as requested).
const points = computed<DayBars[]>(() => {
  if (gran.value === "week") {
    return dailyFilled.value.map((d) =>
      toBars(weekdayShort(d.date), d.logged, { kcal: d.kcal, carb: d.carb, protein: d.protein, fat: d.fat }),
    );
  }
  const groups = new Map<string, DayPoint[]>();
  for (const d of dailyFilled.value) {
    const k = mondayOf(d.date);
    const arr = groups.get(k);
    if (arr) arr.push(d);
    else groups.set(k, [d]);
  }
  return [...groups.entries()]
    .sort((a, b) => (a[0] < b[0] ? -1 : 1))
    .map(([, days]) =>
      toBars(`${dayOfMonth(days[0].date)}.–${dayOfMonth(days[days.length - 1].date)}.`, days.some((d) => d.logged), {
        kcal: meanOverLogged(days, "kcal"),
        carb: meanOverLogged(days, "carb"),
        protein: meanOverLogged(days, "protein"),
        fat: meanOverLogged(days, "fat"),
      }),
    );
});

const hasData = computed(() => dailyFilled.value.some((d) => d.logged));

// Period-wide averages per logged day for the summary strip.
const summary = computed(() => {
  const days = dailyFilled.value;
  const loggedCount = days.filter((d) => d.logged).length;
  return {
    loggedCount,
    totalDays: days.length,
    kcal: meanOverLogged(days, "kcal"),
    carb: meanOverLogged(days, "carb"),
    protein: meanOverLogged(days, "protein"),
    fat: meanOverLogged(days, "fat"),
  };
});

const fmt = (v: number, m: Metric) => (m === "kcal" ? Math.round(v) : Math.round(v * 10) / 10);
const SUMMARY = [
  { key: "kcal" as Metric, label: "Kalorie", unit: "kcal", color: "#8b5cf6" },
  { key: "carb" as Metric, label: "Sacharidy", unit: "g", color: "#0ea5e9" },
  { key: "protein" as Metric, label: "Bílkoviny", unit: "g", color: "#10b981" },
  { key: "fat" as Metric, label: "Tuky", unit: "g", color: "#f59e0b" },
];
const avgLabel = computed(() => (gran.value === "week" ? "Ø / den" : "Ø / den (v týdnu)"));
</script>

<template>
  <div class="space-y-5">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h1 class="text-lg font-semibold sm:text-xl">Statistika</h1>
      <div class="flex gap-1.5">
        <UButton size="xs" :color="gran === 'week' ? 'primary' : 'neutral'" :variant="gran === 'week' ? 'solid' : 'soft'" label="Týden" @click="gran = 'week'" />
        <UButton size="xs" :color="gran === 'month' ? 'primary' : 'neutral'" :variant="gran === 'month' ? 'solid' : 'soft'" label="Měsíc" @click="gran = 'month'" />
      </div>
    </div>

    <!-- period stepper -->
    <div class="flex items-center justify-between gap-2">
      <UButton size="sm" color="neutral" variant="soft" :label="gran === 'week' ? 'Tento týden' : 'Tento měsíc'" :disabled="atCurrent" @click="jumpNow" />
      <div class="flex items-center gap-1">
        <UButton size="xs" color="neutral" variant="soft" label="←" aria-label="Předchozí" :disabled="!canPrev" @click="prev" />
        <span class="min-w-40 px-1 text-center text-sm font-semibold tabular-nums sm:text-base">{{ periodLabel }}</span>
        <UButton size="xs" color="neutral" variant="soft" label="→" aria-label="Další" :disabled="atCurrent" @click="next" />
      </div>
    </div>

    <!-- period-wide averages -->
    <div class="grid grid-cols-2 gap-2 sm:grid-cols-4 sm:gap-3">
      <UCard v-for="s in SUMMARY" :key="s.key" :ui="{ body: 'p-3 sm:p-4' }">
        <div class="flex items-center gap-1.5 text-xs text-gray-500">
          <span class="inline-block h-2 w-2 rounded-full" :style="{ backgroundColor: s.color }"></span>
          {{ s.label }}
        </div>
        <div class="mt-0.5 tabular-nums">
          <span class="text-lg font-semibold sm:text-xl">{{ fmt(summary[s.key], s.key) }}</span>
          <span class="text-sm font-normal text-gray-400">/ {{ fmt(goals[s.key], s.key) }} {{ s.unit }}</span>
        </div>
        <div class="text-[11px] text-gray-400">{{ avgLabel }}</div>
      </UCard>
    </div>

    <!-- combined chart -->
    <UCard :ui="{ body: 'p-3 sm:p-4' }">
      <div v-if="loading" class="grid h-[264px] place-items-center text-sm text-gray-400">Načítání…</div>
      <div v-else-if="error" class="grid h-[264px] place-items-center text-sm text-red-500">Nepodařilo se načíst data.</div>
      <div v-else-if="!hasData" class="grid h-[264px] place-items-center text-center text-sm text-gray-500">
        V tomto období nejsou žádná data.
      </div>
      <template v-else>
        <div class="mb-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-gray-500">
          <span v-for="s in SUMMARY" :key="s.key" class="flex items-center gap-1.5">
            <span class="inline-block h-2.5 w-2.5 rounded-sm" :style="{ backgroundColor: s.color, opacity: 0.85 }"></span>
            {{ s.label }}
          </span>
        </div>
        <PeriodChart :points="points" />
      </template>
    </UCard>
  </div>
</template>
