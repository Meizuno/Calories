<script setup lang="ts">
import { computed } from "vue";
import { ANIMATION_MS, EASE_OUT_CUBIC } from "../composables/useAnimatedNumber";

const props = withDefaults(
  defineProps<{
    value: number;
    max: number;
    size?: number;
    thickness?: number;
    color?: string;
    trackOpacity?: number;
    /** The caller already tweens `value`; skip the CSS transition so the arc is
        not smoothed twice and left lagging behind the figure it mirrors. */
    animated?: boolean;
  }>(),
  { size: 150, thickness: 14, color: "#10b981", trackOpacity: 0.18, animated: false },
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
        :style="{ transition: `stroke 0.3s ease${animated ? '' : `, stroke-dashoffset ${ANIMATION_MS}ms ${EASE_OUT_CUBIC}`}` }"
      />
    </svg>
    <div class="absolute inset-0 grid place-items-center text-center leading-tight">
      <slot />
    </div>
  </div>
</template>
