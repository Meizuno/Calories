<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";

// One plotted day: the raw daily total plus its trailing 7-day average.
export interface Point {
  date: string;
  value: number;
  avg: number;
}

const props = withDefaults(
  defineProps<{
    points: Point[];
    goal?: number;
    unit?: string;
    color?: string;
  }>(),
  { goal: 0, unit: "kcal", color: "#10b981" },
);

// Track the container's pixel width so the SVG uses a real px coordinate system
// (crisp text + correct bar widths) instead of a scaled viewBox.
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

const H = 260;
const PAD = { top: 14, right: 14, bottom: 26, left: 44 };
const plotW = computed(() => Math.max(0, width.value - PAD.left - PAD.right));
const plotH = H - PAD.top - PAD.bottom;
const baseline = PAD.top + plotH;

// Round an axis maximum up to a "nice" 1/2/5 × 10ⁿ value.
function niceCeil(v: number) {
  if (v <= 0) return 1;
  const p = Math.pow(10, Math.floor(Math.log10(v)));
  const n = v / p;
  const step = n <= 1 ? 1 : n <= 2 ? 2 : n <= 5 ? 5 : 10;
  return step * p;
}

const yMax = computed(() => {
  const vals = props.points.flatMap((p) => [p.value, p.avg]);
  return niceCeil(Math.max(1, props.goal, ...vals));
});

const n = computed(() => props.points.length);
const slot = computed(() => (n.value > 0 ? plotW.value / n.value : 0));
const barW = computed(() => Math.max(1, Math.min(slot.value * 0.68, 40)));

const y = (v: number) => PAD.top + plotH * (1 - Math.min(v, yMax.value) / yMax.value);
const cx = (i: number) => PAD.left + slot.value * (i + 0.5);

const bars = computed(() =>
  props.points.map((p, i) => ({
    ...p,
    x: PAD.left + slot.value * i + (slot.value - barW.value) / 2,
    yTop: y(p.value),
    h: Math.max(0, baseline - y(p.value)),
  })),
);

// The 7-day average as a single polyline through each slot centre.
const avgLine = computed(() => props.points.map((p, i) => `${cx(i).toFixed(1)},${y(p.avg).toFixed(1)}`).join(" "));

const goalY = computed(() => (props.goal > 0 ? y(props.goal) : null));

// Four horizontal gridlines with rounded value labels.
const grid = computed(() => [0, 0.25, 0.5, 0.75, 1].map((f) => ({ v: Math.round(yMax.value * f), yy: y(yMax.value * f) })));

// Show at most ~8 date labels so they never crowd. Czech D.M. format.
const labelStep = computed(() => Math.max(1, Math.ceil(n.value / 8)));
function shortDate(iso: string) {
  const [, m, d] = iso.split("-");
  return `${Number(d)}.${Number(m)}.`;
}
const xLabels = computed(() =>
  props.points
    .map((p, i) => ({ i, text: shortDate(p.date), xx: cx(i) }))
    .filter((_, i) => i % labelStep.value === 0),
);
</script>

<template>
  <div ref="host" class="w-full">
    <svg v-if="width > 0" :width="width" :height="H" role="img" class="block">
      <!-- gridlines + y labels -->
      <g>
        <line
          v-for="(gl, i) in grid"
          :key="'g' + i"
          :x1="PAD.left"
          :x2="width - PAD.right"
          :y1="gl.yy"
          :y2="gl.yy"
          class="stroke-gray-200 dark:stroke-gray-800"
          stroke-width="1"
        />
        <text
          v-for="(gl, i) in grid"
          :key="'gt' + i"
          :x="PAD.left - 8"
          :y="gl.yy + 3"
          text-anchor="end"
          class="fill-gray-400 tabular-nums"
          font-size="10"
        >{{ gl.v }}</text>
      </g>

      <!-- daily bars -->
      <g>
        <rect
          v-for="b in bars"
          :key="b.date"
          :x="b.x"
          :y="b.yTop"
          :width="barW"
          :height="b.h"
          :rx="Math.min(barW / 2, 3)"
          :fill="color"
          :fill-opacity="b.value > 0 ? 0.85 : 0"
        >
          <title>{{ b.date }}: {{ Math.round(b.value) }} {{ unit }}</title>
        </rect>
      </g>

      <!-- goal reference (faint dashed) -->
      <g v-if="goalY !== null">
        <line
          :x1="PAD.left"
          :x2="width - PAD.right"
          :y1="goalY"
          :y2="goalY"
          class="stroke-gray-400"
          stroke-width="1"
          stroke-dasharray="4 4"
          stroke-opacity="0.7"
        />
        <text :x="width - PAD.right" :y="goalY - 4" text-anchor="end" class="fill-gray-400" font-size="10">cíl</text>
      </g>

      <!-- 7-day rolling average -->
      <polyline
        :points="avgLine"
        fill="none"
        :stroke="color"
        stroke-width="2"
        stroke-linejoin="round"
        stroke-linecap="round"
      />

      <!-- x date labels -->
      <text
        v-for="l in xLabels"
        :key="'x' + l.i"
        :x="l.xx"
        :y="H - 8"
        text-anchor="middle"
        class="fill-gray-400 tabular-nums"
        font-size="10"
      >{{ l.text }}</text>
    </svg>
  </div>
</template>
