<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { type DateValue, getLocalTimeZone, parseDate, today } from "@internationalized/date";
import type { Stats } from "../lib/types";
import PeriodChart, { type DayBars } from "./PeriodChart.vue";
import { t, weekdayShort, formatMonth, formatDate } from "../lib/i18n";
import { useAnimatedNumber } from "../composables/useAnimatedNumber";
import { useUiSize } from "../composables/useUiSize";

// Reusable stats UI: period stepper + combined %-of-goal bar chart + summary.
// Data comes from an injected `fetchStats` so the same panel serves both the
// private stats page and a public shared profile. `gran`/`anchor` are models so
// the parent can bind them to the URL (StatsView) or leave them local (shared).
const { inline } = useUiSize();

const props = defineProps<{
  fetchStats: (from: string, to: string) => Promise<Stats>;
  // Optional: dates with data, to disable stepping back past the earliest one.
  fetchDays?: () => Promise<string[]>;
}>();

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
const firstOfYear = (iso: string) => `${iso.slice(0, 4)}-01-01`;
const lastOfYear = (iso: string) => `${iso.slice(0, 4)}-12-31`;
function addYearsISO(iso: string, n: number) {
  return `${Number(iso.slice(0, 4)) + n}-01-01`;
}
function daysBetween(from: string, to: string) {
  return Math.round((Date.parse(`${to}T00:00:00Z`) - Date.parse(`${from}T00:00:00Z`)) / 86400000);
}

// Weekday and month names come from Intl, so they follow the chosen language.
const dayOfMonth = (iso: string) => Number(iso.split("-")[2]);

// ── period stepping ──────────────────────────────────────────────────────────
type Gran = "week" | "month" | "year" | "custom";
// defineModel's own `default` can't reference local setup functions (todayISO),
// so the models are declared bare and defaults applied via wrapper computeds.
// Unbound (shared view) → local value with default; bound (StatsView) → the
// parent's URL-backed value drives them.
const granModel = defineModel<Gran>("gran");
const anchorModel = defineModel<string>("anchor");
const gran = computed<Gran>({
  get: () => granModel.value ?? "week",
  set: (v) => (granModel.value = v),
});
const anchor = computed<string>({
  get: () => anchorModel.value ?? todayISO(),
  set: (v) => (anchorModel.value = v),
});

// A custom range needs two dates rather than an anchor. They are models too, so
// StatsView can keep them in the URL and a hand-picked range stays shareable.
const fromModel = defineModel<string>("from");
const toModel = defineModel<string>("to");
const DEFAULT_CUSTOM_SPAN = 29; // a month-ish, inclusive of today
const customFrom = computed<string>({
  get: () => fromModel.value || addDaysISO(todayISO(), -DEFAULT_CUSTOM_SPAN),
  set: (v) => (fromModel.value = v),
});
const customTo = computed<string>({
  get: () => toModel.value || todayISO(),
  set: (v) => (toModel.value = v),
});

const range = computed<{ from: string; to: string }>(() => {
  switch (gran.value) {
    case "week": {
      const from = mondayOf(anchor.value);
      return { from, to: addDaysISO(from, 6) };
    }
    case "year":
      return { from: firstOfYear(anchor.value), to: lastOfYear(anchor.value) };
    case "custom": {
      // Tolerate a reversed pair rather than fetching an empty window.
      const a = customFrom.value;
      const b = customTo.value;
      return a <= b ? { from: a, to: b } : { from: b, to: a };
    }
    default:
      return { from: firstOfMonth(anchor.value), to: lastOfMonth(anchor.value) };
  }
});

// The current week/month contains today, so there's nothing newer to step to.
const atCurrent = computed(() => todayISO() >= range.value.from && todayISO() <= range.value.to);

// Earliest day with data (when a source is given): stepping back past it is
// pointless. With no source, don't restrict.
const hasDaysSource = !!props.fetchDays;
const firstLoggedDate = ref<string | null>(null);
if (props.fetchDays) {
  props
    .fetchDays()
    .then((days) => (firstLoggedDate.value = days[0] ?? null))
    .catch(() => {});
}
const canPrev = computed(() => (!hasDaysSource ? true : firstLoggedDate.value !== null && firstLoggedDate.value < range.value.from));

// Stepping a custom range moves the whole window by its own length, which is
// the only reading of "previous" that makes sense for an arbitrary span.
function step(direction: -1 | 1) {
  switch (gran.value) {
    case "week":
      anchor.value = addDaysISO(mondayOf(anchor.value), 7 * direction);
      break;
    case "month":
      anchor.value = addMonthsISO(anchor.value, direction);
      break;
    case "year":
      anchor.value = addYearsISO(anchor.value, direction);
      break;
    case "custom": {
      const span = daysBetween(range.value.from, range.value.to) + 1;
      customFrom.value = addDaysISO(range.value.from, span * direction);
      customTo.value = addDaysISO(range.value.to, span * direction);
      break;
    }
  }
}
function prev() {
  if (canPrev.value) step(-1);
}
function next() {
  if (!atCurrent.value) step(1);
}
function jumpNow() {
  if (gran.value === "custom") {
    // Slide the window forward so it ends today, keeping its length — moving
    // only the anchor would leave a custom range visibly unchanged.
    const span = daysBetween(range.value.from, range.value.to);
    customTo.value = todayISO();
    customFrom.value = addDaysISO(todayISO(), -span);
    return;
  }
  anchor.value = todayISO();
}

// A period that starts after today has not happened yet: it is drawn so the
// week or year stays whole, but it is neither openable nor hoverable.
const isFuture = (start: string) => start > todayISO();

const fmtDM = (iso: string) => formatDate(iso, { day: "numeric", month: "numeric" });
const capitalise = (v: string) => v.charAt(0).toUpperCase() + v.slice(1);
const periodLabel = computed(() => {
  switch (gran.value) {
    case "week":
      return `${fmtDM(range.value.from)} – ${fmtDM(range.value.to)} ${range.value.to.slice(0, 4)}`;
    case "year":
      return range.value.from.slice(0, 4);
    case "custom":
      return `${fmtDM(range.value.from)} – ${fmtDM(range.value.to)} ${range.value.to.slice(0, 4)}`;
    default:
      return capitalise(formatMonth(range.value.from));
  }
});

// ── data ─────────────────────────────────────────────────────────────────────
const stats = ref<Stats | null>(null);
const loading = ref(false);
const error = ref(false);

async function load() {
  loading.value = true;
  error.value = false;
  try {
    stats.value = await props.fetchStats(range.value.from, range.value.to);
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
  while (cur <= s.to && out.length < 800) {
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
function toBars(label: string, logged: boolean, vals: Record<Metric, number>, start: string): DayBars {
  const pct = {} as Record<Metric, number>;
  for (const k of METRIC_KEYS) pct[k] = goals.value[k] > 0 ? (vals[k] / goals.value[k]) * 100 : 0;
  const future = isFuture(start);
  return { key: future ? undefined : start, label, logged, future, raw: { ...vals }, pct };
}

// How the window is cut into bars. Week shows each day; anything longer would
// be unreadable day-by-day, so longer spans average into weeks or months. A
// custom range picks whichever keeps the bar count sane.
type Bucket = "day" | "week" | "month";
const bucketing = computed<Bucket>(() => {
  switch (gran.value) {
    case "week":
      return "day";
    case "month":
      return "week";
    case "year":
      return "month";
    default: {
      const span = daysBetween(range.value.from, range.value.to) + 1;
      if (span <= 35) return "day";
      return span <= 186 ? "week" : "month";
    }
  }
});

const points = computed<DayBars[]>(() => {
  const days = dailyFilled.value;
  if (bucketing.value === "day") {
    return days.map((d) =>
      toBars(
        weekdayShort(d.date),
        d.logged,
        { kcal: d.kcal, carb: d.carb, protein: d.protein, fat: d.fat },
        d.date,
      ),
    );
  }

  const keyOf = bucketing.value === "week" ? mondayOf : (iso: string) => iso.slice(0, 7);
  const groups = new Map<string, DayPoint[]>();
  for (const d of days) {
    const k = keyOf(d.date);
    const arr = groups.get(k);
    if (arr) arr.push(d);
    else groups.set(k, [d]);
  }
  return [...groups.entries()]
    .sort((a, b) => (a[0] < b[0] ? -1 : 1))
    .map(([, group]) => {
      const label =
        bucketing.value === "month"
          ? formatDate(group[0].date, { month: "short" })
          : `${dayOfMonth(group[0].date)}.–${dayOfMonth(group[group.length - 1].date)}.`;
      return toBars(
        label,
        group.some((d) => d.logged),
        {
          kcal: meanOverLogged(group, "kcal"),
          carb: meanOverLogged(group, "carb"),
          protein: meanOverLogged(group, "protein"),
          fat: meanOverLogged(group, "fat"),
        },
        // A week or month is reachable once it has started, even if it runs
        // past today — the period view itself caps what it shows.
        group[0].date,
      );
    });
});

const hasData = computed(() => dailyFilled.value.some((d) => d.logged));

const summary = computed(() => {
  const days = dailyFilled.value;
  return {
    loggedCount: days.filter((d) => d.logged).length,
    totalDays: days.length,
    kcal: meanOverLogged(days, "kcal"),
    carb: meanOverLogged(days, "carb"),
    protein: meanOverLogged(days, "protein"),
    fat: meanOverLogged(days, "fat"),
  };
});

const emit = defineEmits<{ (e: "pick-day", date: string): void }>();

function drillInto(point: DayBars) {
  if (!point.key) return;
  switch (bucketing.value) {
    case "day":
      emit("pick-day", point.key);
      break;
    case "week":
      gran.value = "week";
      anchor.value = point.key;
      break;
    case "month":
      gran.value = "month";
      anchor.value = point.key;
      break;
  }
}

const GRANS = computed(() => [
  { key: "week" as Gran, label: t("stats.week") },
  { key: "month" as Gran, label: t("stats.month") },
  { key: "year" as Gran, label: t("stats.year") },
  { key: "custom" as Gran, label: t("stats.custom") },
]);

// Range picker. UCalendar speaks CalendarDate, the rest of the app speaks ISO,
// so this converts in both directions and only commits once both ends are set.
const rangeOpen = ref(false);
const maxCalendarDate = today(getLocalTimeZone());
const calendarRange = computed({
  get: () => ({ start: parseDate(range.value.from), end: parseDate(range.value.to) }),
  set: (v: { start: DateValue | null; end: DateValue | null } | null) => {
    if (!v?.start) return;
    customFrom.value = v.start.toString();
    // Picking a start clears the end until the second click lands.
    customTo.value = (v.end ?? v.start).toString();
    if (v.end) rangeOpen.value = false;
  },
});

const fmt = (v: number, m: Metric) => (m === "kcal" ? Math.round(v) : Math.round(v * 10) / 10);
const SUMMARY = computed(() => [
  { key: "kcal" as Metric, label: t("macros.calories"), unit: "kcal", color: "#8b5cf6" },
  { key: "carb" as Metric, label: t("macros.carb"), unit: "g", color: "#0ea5e9" },
  { key: "protein" as Metric, label: t("macros.protein"), unit: "g", color: "#10b981" },
  { key: "fat" as Metric, label: t("macros.fat"), unit: "g", color: "#f59e0b" },
]);
// What a click on a bar does, which changes with the bucket and which nothing
// about a bar chart otherwise advertises.
const drillHint = computed(() => {
  switch (bucketing.value) {
    case "day":
      return t("stats.hintDay");
    case "week":
      return t("stats.hintWeek");
    default:
      return t("stats.hintMonth");
  }
});

const avgLabel = computed(() => {
  switch (bucketing.value) {
    case "day":
      return t("stats.avgPerDay");
    case "week":
      return t("stats.avgPerDayInWeek");
    default:
      return t("stats.avgPerDayInMonth");
  }
});

// dailyFilled walks the window a day at a time, so a year needs more headroom
// than the old 400-day cap allowed.
const animated = {
  kcal: useAnimatedNumber(() => summary.value.kcal),
  carb: useAnimatedNumber(() => summary.value.carb),
  protein: useAnimatedNumber(() => summary.value.protein),
  fat: useAnimatedNumber(() => summary.value.fat),
} as const;
const goalPct = (key: Metric) =>
  goals.value[key] > 0 ? Math.min((animated[key].value / goals.value[key]) * 100, 100) : 0;
</script>

<template>
  <div class="space-y-5">
    <slot name="title"><span></span></slot>

    <!-- One control bar: what window, then which window. Previously these were
         two separate rows, which read as unrelated settings. -->
    <div class="flex flex-wrap items-center gap-3">
      <div class="flex items-center gap-0.5 rounded-xl bg-gray-100/70 p-0.5 dark:bg-gray-900/70">
        <button
          v-for="g in GRANS"
          :key="g.key"
          type="button"
          class="rounded-lg px-2.5 py-1.5 text-sm font-medium outline-none transition focus-visible:ring-2 focus-visible:ring-emerald-500/60 sm:px-3 sm:py-2 sm:text-base"
          :class="gran === g.key
            ? 'bg-white text-gray-900 shadow-sm dark:bg-gray-800 dark:text-gray-100'
            : 'text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-100'"
          @click="gran = g.key"
        >{{ g.label }}</button>
      </div>

      <div class="ml-auto flex items-center gap-1">
        <UButton
          :size="inline"
          color="neutral"
          variant="ghost"
          icon="i-ui-prev"
          :aria-label="t('common.previous')"
          :disabled="!canPrev"
          @click="prev"
        />

        <!-- For a custom window the label itself opens the range picker. -->
        <UPopover v-if="gran === 'custom'" v-model:open="rangeOpen">
          <button
            type="button"
            class="flex items-center gap-1.5 rounded-lg px-2 py-1 text-sm font-semibold tabular-nums outline-none transition hover:bg-gray-100 focus-visible:ring-2 focus-visible:ring-emerald-500/60 sm:text-base dark:hover:bg-gray-800"
          >
            {{ periodLabel }}
            <UIcon name="i-ui-calendar" class="size-4 text-gray-400" />
          </button>
          <template #content>
            <UCalendar
              v-model="calendarRange"
              range
              :max-value="maxCalendarDate"
              class="p-2"
            />
          </template>
        </UPopover>
        <span v-else class="min-w-40 px-1 text-center text-sm font-semibold tabular-nums sm:text-base">{{ periodLabel }}</span>

        <UButton
          :size="inline"
          color="neutral"
          variant="ghost"
          icon="i-ui-next"
          :aria-label="t('common.next')"
          :disabled="atCurrent"
          @click="next"
        />
        <UButton
          class="ml-1"
          :size="inline"
          color="neutral"
          variant="soft"
          :label="t('stats.jumpNow')"
          :disabled="atCurrent"
          @click="jumpNow"
        />
      </div>
    </div>

    <!-- period-wide averages. The bar makes "how close to goal" readable at a
         glance; the numbers alone needed arithmetic. -->
    <div class="grid grid-cols-2 gap-2 sm:grid-cols-4 sm:gap-3">
      <UCard v-for="s in SUMMARY" :key="s.key" :ui="{ body: 'p-3 sm:p-4' }">
        <div class="flex items-center gap-1.5 text-xs text-gray-500">
          <span class="inline-block h-2 w-2 rounded-full" :style="{ backgroundColor: s.color }"></span>
          {{ s.label }}
        </div>
        <div class="mt-0.5 tabular-nums">
          <span class="text-lg font-semibold sm:text-xl">{{ fmt(animated[s.key].value, s.key) }}</span>
          <span class="text-sm font-normal text-gray-400">/ {{ fmt(goals[s.key], s.key) }} {{ s.unit }}</span>
        </div>
        <div class="mt-2 h-1.5 w-full overflow-hidden rounded-full" :style="{ backgroundColor: s.color + '2e' }">
          <div class="h-full rounded-full" :style="{ width: goalPct(s.key) + '%', backgroundColor: s.color }"></div>
        </div>
        <div class="mt-1.5 text-[11px] text-gray-400">{{ avgLabel }}</div>
      </UCard>
    </div>

    <!-- combined chart -->
    <UCard :ui="{ body: 'p-3 sm:p-4' }">
      <div v-if="loading" class="grid h-[264px] place-items-center text-sm text-gray-400">{{ t("common.loading") }}</div>
      <div v-else-if="error" class="grid h-[264px] place-items-center text-sm text-red-500">{{ t("stats.loadFailed") }}</div>
      <div v-else-if="!hasData" class="grid h-[264px] place-items-center text-center text-sm text-gray-500">
        {{ t("stats.noData") }}
      </div>
      <template v-else>
        <div class="mb-2 flex flex-wrap items-center justify-between gap-x-4 gap-y-1 text-xs text-gray-500">
          <span class="flex flex-wrap items-center gap-x-4 gap-y-1">
          <span v-for="s in SUMMARY" :key="s.key" class="flex items-center gap-1.5">
            <span class="inline-block h-2.5 w-2.5 rounded-sm" :style="{ backgroundColor: s.color, opacity: 0.85 }"></span>
            {{ s.label }}
          </span>
          </span>
          <span class="tabular-nums text-gray-400">
            {{ t("stats.loggedDays", { n: summary.loggedCount, total: summary.totalDays }) }}
          </span>
        </div>
        <PeriodChart :points="points" interactive @select="drillInto" />
        <p class="mt-2 text-center text-xs text-gray-400">{{ drillHint }}</p>
      </template>
    </UCard>
  </div>
</template>
