<script setup lang="ts">
import { computed } from "vue";
import { RouterLink, RouterView, useRouter } from "vue-router";
import { session, logout } from "./lib/session";
import { t, currentLocale, setLocale, LOCALE_NAMES, LOCALE_FLAGS, type Locale } from "./lib/i18n";
import { cs as uiCs, en as uiEn } from "@nuxt/ui/locale";
import { usePullToRefresh } from "./composables/usePullToRefresh";

const router = useRouter();
const profileName = computed(() => session.profile?.name?.trim() || "");
const initial = computed(() => (profileName.value ? profileName.value.charAt(0).toUpperCase() : "🙂"));

// Pull down from the top of the page to reload — a full "refresh everything"
// on mobile (the app re-fetches from the API on boot). Default onTrigger is
// window.location.reload().
const { distance: pullDistance, pulling: isPulling, ready: pullReady } = usePullToRefresh();

// Hand Nuxt UI the matching locale so its own components (the calendar, menus)
// speak the same language as our strings.
const uiLocale = computed(() => (currentLocale() === "cs" ? uiCs : uiEn));
// The switcher shows the language currently in use (flag + code); clicking it
// moves to the next one. With two locales that is a plain toggle.
const LOCALES = Object.keys(LOCALE_NAMES) as Locale[];
const activeLocale = computed(() => currentLocale());
function cycleLocale() {
  const next = LOCALES[(LOCALES.indexOf(activeLocale.value) + 1) % LOCALES.length];
  setLocale(next);
}
</script>

<template>
  <UApp :locale="uiLocale">
    <header
      class="sticky top-0 z-30 border-b border-gray-200/70 bg-white/75 backdrop-blur dark:border-gray-800/70 dark:bg-gray-950/70"
    >
      <div class="mx-auto flex max-w-3xl items-center justify-between gap-3 px-4 py-2.5 sm:px-6">
        <RouterLink to="/" class="flex items-center gap-2 font-semibold">
          <span class="grid h-8 w-8 place-items-center rounded-xl bg-emerald-500/15 text-lg">🥗</span>
          <span class="text-base tracking-tight sm:text-lg">Calories</span>
        </RouterLink>

        <div class="flex items-center gap-1">
        <UButton
          color="neutral"
          variant="ghost"
          size="sm"
          :icon="LOCALE_FLAGS[activeLocale]"
          :label="activeLocale.toUpperCase()"
          :title="LOCALE_NAMES[activeLocale]"
          :aria-label="t('nav.language')"
          class="gap-1.5 text-xs font-semibold tracking-wide text-gray-500 dark:text-gray-400"
          @click="cycleLocale"
        />

        <nav v-if="session.authenticated" class="flex items-center gap-1">
          <RouterLink
            to="/stats"
            class="rounded-lg px-2.5 py-1.5 text-sm text-gray-500 transition hover:bg-gray-100 hover:text-gray-800 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-100"
            active-class="bg-gray-100 text-gray-900 dark:bg-gray-800 dark:text-gray-100"
          >
            {{ t("nav.stats") }}
          </RouterLink>

          <RouterLink
            to="/profiles/me"
            class="ml-1 flex items-center gap-2 rounded-full py-1 pl-1 pr-3 transition hover:bg-gray-100 dark:hover:bg-gray-800"
            active-class="bg-gray-100 dark:bg-gray-800"
          >
            <span class="grid h-6 w-6 place-items-center rounded-full bg-emerald-500 text-xs font-semibold text-white">{{ initial }}</span>
            <span class="max-w-28 truncate text-sm text-gray-700 dark:text-gray-200">{{ profileName || t("nav.profile") }}</span>
          </RouterLink>

          <button
            type="button"
            class="ml-0.5 rounded-lg px-2.5 py-1.5 text-sm text-gray-400 transition hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-gray-800 dark:hover:text-gray-200"
            @click="logout"
          >
            {{ t("nav.logout") }}
          </button>
        </nav>

        <button
          v-else
          type="button"
          class="rounded-lg bg-emerald-500 px-3 py-1.5 text-sm font-medium text-white transition hover:bg-emerald-600"
          @click="router.push('/login')"
        >
          {{ t("nav.login") }}
        </button>
        </div>
      </div>
    </header>

    <div
      class="flex items-center justify-center overflow-hidden"
      :class="{ 'transition-[height] duration-200 ease-out': !isPulling }"
      :style="{ height: pullDistance + 'px' }"
    >
      <div
        class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400"
        :class="{ 'text-emerald-500 dark:text-emerald-400': pullReady }"
      >
        <svg
          viewBox="0 0 24 24"
          class="size-4 transition-transform duration-200"
          :class="{ 'rotate-180': pullReady }"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <path d="M12 5v14M5 12l7 7 7-7" />
        </svg>
        <span>{{ pullReady ? t("pull.release") : t("pull.pull") }}</span>
      </div>
    </div>

    <main class="mx-auto max-w-3xl px-4 py-6 sm:px-6 sm:py-8">
      <RouterView />
    </main>
  </UApp>
</template>
