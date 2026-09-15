<script setup lang="ts">
import { computed } from "vue";

// A heading. Everything is flattened towards the body size: these appear inside
// a chat turn, not on a page of their own, and an h1-sized line in the middle
// of a conversation reads as shouting.
const props = defineProps<{ depth: number }>();
const tag = computed(() => `h${Math.min(Math.max(props.depth, 1), 6)}`);
const size = computed(() =>
  props.depth <= 2 ? "text-base font-semibold" : "text-sm font-semibold",
);
</script>

<template>
  <component :is="tag" :class="['mb-1 mt-3 first:mt-0 text-highlighted', size]">
    <slot />
  </component>
</template>
