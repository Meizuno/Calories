<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { t } from "../lib/i18n";
import { useReveal } from "../composables/useAnimatedNumber";

type Metric = "kcal" | "carb" | "protein" | "fat";

// One bucket: a day, a week average or a month average. `pct` is each metric as
// a percentage of its daily goal (the common scale that lets calories and grams
// share one chart); `raw` keeps the absolute value for the readout. `key` names
// the period the bucket stands for, so a click can drill into it.
//
// `future` marks a bucket that has not happened yet. Such a bucket is drawn, so
// the period still reads as a whole week or year, but it is inert: nothing to
// open and nothing to read out.
export interface DayBars {
  key?: string;
  label: string;
  logged: boolean;
  future?: boolean;
  pct: Record<Metric, number>;
  raw: Record<Metric, number>;
}

const props = withDefaults(defineProps<{ points: DayBars[]; interactive?: boolean }>(), { interactive: false });
const emit = defineEmits<{ (e: "select", point: DayBars): void }>();

const METRICS = computed<{ key: Metric; label: string; color: string; unit: string }[]>(() => [
  { key: "kcal", label: t("macros.calories"), color: "#8b5cf6", unit: "kcal" },
  { key: "carb", label: t("macros.carb"), color: "#0ea5e9", unit: "g" },
  { key: "protein", label: t("macros.protein"), color: "#10b981", unit: "g" },
  { key: "fat", label: t("macros.fat"), color: "#f59e0b", unit: "g" },
]);

// Track pixel width so the SVG uses a real px coordinate system (crisp text).
const host = ref<HTMLElement | null>(null);
const width = ref(0);
let ro: ResizeObserver | null = null;
onMounted(() => {
  ro = new ResizeObserver((entries) => {
    width.value = entries[0]?.contentRect.width ?? 0;
  });
  if (host.value) ro.observe(host.value);
});
onBeforeUnmount(() => ro?.disconnect());

const H = 264;
const PAD = { top: 16, right: 12, bottom: 28, left: 44 };
const GAP = 2; // gap between the 4 bars within a bucket
const plotW = computed(() => Math.max(0, width.value - PAD.left - PAD.right));
const plotH = H - PAD.top - PAD.bottom;
const baseline = PAD.top + plotH;

const n = computed(() => props.points.length);
const slot = computed(() => (n.value > 0 ? plotW.value / n.value : 0));
const groupPad = computed(() => slot.value * 0.12);
const barW = computed(() => Math.max(1, (slot.value - 2 * groupPad.value - 3 * GAP) / 4));

// Axis tops out at the goal (100%) or higher if a bucket overshoots; rounded up
// to a clean step so the 100% goal line lands on the grid.
const maxPct = computed(() =>
  Math.max(0, ...props.points.filter((p) => p.logged).flatMap((p) => METRICS.value.map((m) => p.pct[m.key]))),
);
const yMax = computed(() => {
  const step = 25;
  return Math.max(100, Math.ceil(maxPct.value / step) * step);
});
const gridStep = computed(() => (yMax.value <= 150 ? 25 : yMax.value <= 300 ? 50 : 100));
const yFor = (pct: number) => baseline - plotH * (Math.min(pct, yMax.value) / yMax.value);

const grid = computed(() => {
  const lines: number[] = [];
  for (let v = 0; v <= yMax.value; v += gridStep.value) lines.push(v);
  return lines;
});

const fmtRaw = (v: number, unit: string) => (unit === "kcal" ? Math.round(v) : Math.round(v * 10) / 10);

// Bars grow up from the baseline when the data changes, so a new period reads
// as the chart being redrawn rather than a different picture appearing.
const reveal = useReveal(() => props.points);

// One hit area per bucket, spanning the full plot height: aiming at a 6px bar is
// fiddly, and an empty bucket has no bar to aim at yet still needs to be
// reachable when it is a drill-down target.
const slots = computed(() =>
  props.points.map((p, i) => ({
    point: p,
    index: i,
    x: PAD.left + slot.value * i,
    cx: PAD.left + slot.value * (i + 0.5),
  })),
);

const bars = computed(() =>
  props.points.flatMap((p, i) => {
    if (!p.logged) return [];
    const x0 = PAD.left + slot.value * i + groupPad.value;
    return METRICS.value.map((m, j) => {
      const full = Math.max(0, baseline - yFor(p.pct[m.key]));
      const h = full * reveal.value;
      return {
        key: `${i}-${m.key}`,
        index: i,
        x: x0 + j * (barW.value + GAP),
        y: baseline - h,
        w: barW.value,
        h,
        color: m.color,
      };
    });
  }),
);

const goalY = computed(() => yFor(100));

const hovered = ref<number | null>(null);
const active = computed(() => (hovered.value === null ? null : (props.points[hovered.value] ?? null)));
const activeSlot = computed(() => (hovered.value === null ? null : (slots.value[hovered.value] ?? null)));

// Keep the readout inside the chart rather than letting it hang off the edge.
const readoutLeft = computed(() => {
  const cx = activeSlot.value?.cx ?? 0;
  const half = 90;
  return Math.min(Math.max(cx, PAD.left + half), Math.max(PAD.left + half, width.value - half));
});

function choose(point: DayBars) {
  if (props.interactive && point.key) emit("select", point);
}
</script>

<template>
  <div ref="host" class="relative w-full">
    <svg
      v-if="width > 0"
      :width="width"
      :height="H"
      role="img"
      class="block select-none"
      @pointerleave="hovered = null"
    >
      <!-- gridlines + % labels -->
      <g>
        <line
          v-for="v in grid"
          :key="'g' + v"
          :x1="PAD.left"
          :x2="width - PAD.right"
          :y1="yFor(v)"
          :y2="yFor(v)"
          class="stroke-gray-200/70 dark:stroke-gray-800/70"
          stroke-width="1"
        />
        <text
          v-for="v in grid"
          :key="'t' + v"
          :x="PAD.left - 8"
          :y="yFor(v) + 3"
          text-anchor="end"
          class="fill-gray-400 tabular-nums"
          font-size="10"
        >{{ v }}%</text>
      </g>

      <!-- the hovered column, behind everything -->
      <rect
        v-if="activeSlot"
        :x="activeSlot.x"
        :y="PAD.top"
        :width="slot"
        :height="plotH"
        rx="6"
        class="fill-gray-900/5 dark:fill-white/5"
      />

      <!-- the daily goal -->
      <line
        :x1="PAD.left"
        :x2="width - PAD.right"
        :y1="goalY"
        :y2="goalY"
        class="stroke-gray-400 dark:stroke-gray-500"
        stroke-width="1"
        stroke-dasharray="4 4"
      />
      <text :x="width - PAD.right" :y="goalY - 5" text-anchor="end" class="fill-gray-400" font-size="9">
        {{ t("macros.goal") }}
      </text>

      <!-- grouped bars: kcal / carb / protein / fat, each as % of its goal -->
      <rect
        v-for="b in bars"
        :key="b.key"
        :x="b.x"
        :y="b.y"
        :width="b.w"
        :height="b.h"
        :rx="Math.min(b.w / 2, 3)"
        :fill="b.color"
        opacity="0.9"
      />

      <!-- x labels -->
      <text
        v-for="s in slots"
        :key="'x' + s.index"
        :x="s.cx"
        :y="H - 9"
        text-anchor="middle"
        class="tabular-nums transition-colors"
        :class="
          hovered === s.index
            ? 'fill-gray-700 dark:fill-gray-200'
            : s.point.future
              ? 'fill-gray-300 dark:fill-gray-700'
              : 'fill-gray-400'
        "
        font-size="10"
      >{{ s.point.label }}</text>

      <!-- full-height hit areas, last so they sit on top -->
      <rect
        v-for="s in slots"
        :key="'hit' + s.index"
        :x="s.x"
        :y="PAD.top"
        :width="slot"
        :height="plotH"
        fill="transparent"
        :class="interactive && s.point.key ? 'cursor-pointer' : ''"
        :tabindex="interactive && s.point.key ? 0 : undefined"
        :role="interactive && s.point.key ? 'button' : undefined"
        :aria-label="s.point.label"
        @pointerenter="hovered = s.point.future ? null : s.index"
        @focus="hovered = s.point.future ? null : s.index"
        @blur="hovered = null"
        @click="choose(s.point)"
        @keydown.enter.prevent="choose(s.point)"
        @keydown.space.prevent="choose(s.point)"
      />
    </svg>

    <!-- Readout for the hovered bucket. Replaces the native <title> tooltips,
         which appear only after a delay and cannot show four metrics legibly. -->
    <div
      v-if="active"
      class="pointer-events-none absolute top-1 z-10 -translate-x-1/2 rounded-lg border border-gray-200 bg-white/95 px-2.5 py-1.5 text-xs shadow-lg backdrop-blur dark:border-gray-700 dark:bg-gray-900/95"
      :style="{ left: readoutLeft + 'px' }"
    >
      <div class="mb-1 font-medium">{{ active.label }}</div>
      <div v-if="active.logged" class="space-y-0.5">
        <div v-for="m in METRICS" :key="m.key" class="flex items-center gap-1.5 whitespace-nowrap">
          <span class="inline-block size-2 shrink-0 rounded-sm" :style="{ backgroundColor: m.color }" />
          <span class="text-gray-500">{{ m.label }}</span>
          <span class="ml-auto tabular-nums font-medium">{{ fmtRaw(active.raw[m.key], m.unit) }} {{ m.unit }}</span>
        </div>
      </div>
      <div v-else class="text-gray-400">{{ t("stats.noData") }}</div>
    </div>
  </div>
</template>
