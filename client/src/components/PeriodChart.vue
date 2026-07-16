<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";

type Metric = "kcal" | "carb" | "protein" | "fat";

// One bucket: a day (week view) or a week average (month view). `pct` is each
// metric as a percentage of its daily goal (the common scale that lets calories
// and grams share one chart); `raw` keeps the absolute value for the tooltip.
export interface DayBars {
  label: string;
  logged: boolean;
  pct: Record<Metric, number>;
  raw: Record<Metric, number>;
}

const props = defineProps<{ points: DayBars[] }>();

const METRICS: { key: Metric; label: string; color: string; unit: string }[] = [
  { key: "kcal", label: "Kalorie", color: "#10b981", unit: "kcal" },
  { key: "carb", label: "Sacharidy", color: "#0ea5e9", unit: "g" },
  { key: "protein", label: "Bílkoviny", color: "#8b5cf6", unit: "g" },
  { key: "fat", label: "Tuky", color: "#f59e0b", unit: "g" },
];

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
const PAD = { top: 16, right: 12, bottom: 26, left: 48 };
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
  Math.max(0, ...props.points.filter((p) => p.logged).flatMap((p) => METRICS.map((m) => p.pct[m.key]))),
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

// Flatten to one rect per (bucket, metric); unlogged buckets draw nothing.
const rects = computed(() => {
  const out: { key: string; x: number; y: number; w: number; h: number; color: string; title: string }[] = [];
  props.points.forEach((p, i) => {
    if (!p.logged) return;
    const x0 = PAD.left + slot.value * i + groupPad.value;
    METRICS.forEach((m, j) => {
      const pct = p.pct[m.key];
      const y = yFor(pct);
      out.push({
        key: `${i}-${m.key}`,
        x: x0 + j * (barW.value + GAP),
        y,
        w: barW.value,
        h: Math.max(0, baseline - y),
        color: m.color,
        title: `${p.label} · ${m.label}: ${fmtRaw(p.raw[m.key], m.unit)} ${m.unit} (${Math.round(pct)} %)`,
      });
    });
  });
  return out;
});

const labels = computed(() => props.points.map((p, i) => ({ text: p.label, cx: PAD.left + slot.value * (i + 0.5) })));
const goalY = computed(() => yFor(100));
</script>

<template>
  <div ref="host" class="w-full">
    <svg v-if="width > 0" :width="width" :height="H" role="img" class="block">
      <!-- gridlines + % labels -->
      <g>
        <line
          v-for="v in grid"
          :key="'g' + v"
          :x1="PAD.left"
          :x2="width - PAD.right"
          :y1="yFor(v)"
          :y2="yFor(v)"
          class="stroke-gray-200 dark:stroke-gray-800"
          stroke-width="1"
        />
        <text v-for="v in grid" :key="'t' + v" :x="PAD.left - 6" :y="yFor(v) + 3" text-anchor="end" class="fill-gray-400 tabular-nums" font-size="10">{{ v }} %</text>
      </g>

      <!-- 100% goal line -->
      <line :x1="PAD.left" :x2="width - PAD.right" :y1="goalY" :y2="goalY" class="stroke-gray-400" stroke-width="1" stroke-dasharray="4 4" stroke-opacity="0.8" />
      <text :x="width - PAD.right" :y="goalY - 4" text-anchor="end" class="fill-gray-400" font-size="9">cíl</text>

      <!-- grouped bars: 4 per bucket (kcal / carb / protein / fat), % of goal -->
      <rect
        v-for="r in rects"
        :key="r.key"
        :x="r.x"
        :y="r.y"
        :width="r.w"
        :height="r.h"
        :rx="Math.min(r.w / 2, 2)"
        :fill="r.color"
        fill-opacity="0.85"
      >
        <title>{{ r.title }}</title>
      </rect>

      <!-- x labels -->
      <text
        v-for="(l, i) in labels"
        :key="'x' + i"
        :x="l.cx"
        :y="H - 8"
        text-anchor="middle"
        class="fill-gray-400 tabular-nums"
        font-size="10"
      >{{ l.text }}</text>
    </svg>
  </div>
</template>
