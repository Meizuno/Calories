<script setup lang="ts">
import { computed } from "vue";

// A link in the assistant's reply.
//
// The href is checked here rather than trusted: model output can carry anything
// a tool result carried, and javascript: or data: URLs are the obvious way to
// turn a rendered reply into an attack. Anything that is not plainly http,
// https or mailto is rendered as text with no link at all.
const props = defineProps<{ href: string; title?: string }>();

const safe = computed(() => {
  try {
    const url = new URL(props.href, window.location.origin);
    return ["http:", "https:", "mailto:"].includes(url.protocol) ? url.href : null;
  } catch {
    return null;
  }
});
</script>

<template>
  <a
    v-if="safe"
    :href="safe"
    :title="title"
    target="_blank"
    rel="noopener noreferrer"
    class="text-emerald-600 underline underline-offset-2 hover:text-emerald-700 dark:text-emerald-400 dark:hover:text-emerald-300"
  ><slot /></a>
  <span v-else><slot /></span>
</template>
