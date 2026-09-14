<script setup lang="ts">
import { computed } from "vue";
import { t, currentLocale, setLocale, LOCALE_NAMES, LOCALE_FLAGS, type Locale } from "../lib/i18n";

// The switcher shows the language currently in use (flag + code); clicking it
// moves to the next one. With two locales that is a plain toggle, but it walks
// the list rather than hardcoding the pair.
//
// Extracted because it appears three times: twice in the header (signed in and
// out) and once on the bare sign-in shell, which has no header at all.
const LOCALES = Object.keys(LOCALE_NAMES) as Locale[];
const active = computed(() => currentLocale());

function cycle() {
  setLocale(LOCALES[(LOCALES.indexOf(active.value) + 1) % LOCALES.length]);
}
</script>

<template>
  <button
    type="button"
    class="flex size-9 items-center justify-center gap-1.5 rounded-xl text-xs font-semibold uppercase tracking-wide text-gray-500 outline-none transition hover:bg-gray-100 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-emerald-500/60 sm:w-auto sm:px-2.5 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-100"
    :title="LOCALE_NAMES[active]"
    :aria-label="t('nav.language')"
    @click="cycle"
  >
    <UIcon :name="LOCALE_FLAGS[active]" class="size-5 shrink-0" />
    <span class="hidden sm:inline">{{ active }}</span>
  </button>
</template>
