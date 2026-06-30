<script setup lang="ts">
import { computed } from "vue";

const props = withDefaults(
  defineProps<{
    value: number;
    max: number;
    size?: number;
    thickness?: number;
    color?: string;
    trackOpacity?: number;
  }>(),
  { size: 150, thickness: 14, color: "#10b981", trackOpacity: 0.18 },
);

const center = computed(() => props.size / 2);
const radius = computed(() => (props.size - props.thickness) / 2);
const circ = computed(() => 2 * Math.PI * radius.value);
const pct = computed(() => (props.max > 0 ? Math.min(props.value / props.max, 1) : 0));
const offset = computed(() => circ.value * (1 - pct.value));
</script>

<template>
  <div class="relative inline-grid place-items-center" :style="{ width: size + 'px', height: size + 'px' }">
    <svg :width="size" :height="size" class="-rotate-90">
      <circle :cx="center" :cy="center" :r="radius" fill="none" :stroke="color" :stroke-opacity="trackOpacity" :stroke-width="thickness" />
      <circle
        :cx="center"
        :cy="center"
        :r="radius"
        fill="none"
        :stroke="color"
        :stroke-width="thickness"
        stroke-linecap="round"
        :stroke-dasharray="circ"
        :stroke-dashoffset="offset"
        style="transition: stroke-dashoffset 0.5s ease, stroke 0.3s ease"
      />
    </svg>
    <div class="absolute inset-0 grid place-items-center text-center leading-tight">
      <slot />
    </div>
  </div>
</template>
