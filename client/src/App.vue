<script setup lang="ts">
import { computed } from "vue";
import { RouterLink, RouterView, useRoute, useRouter } from "vue-router";
import { session, logout } from "./lib/session";
import { t, currentLocale } from "./lib/i18n";
import { cs as uiCs, en as uiEn } from "@nuxt/ui/locale";
import { usePullToRefresh } from "./composables/usePullToRefresh";
import LocaleToggle from "./components/LocaleToggle.vue";

const router = useRouter();
const route = useRoute();
// Sign-in and sign-up render on a bare shell: no nav, no profile, no sign-out,
// because none of it is reachable yet. Only the language toggle survives — it
// is the one control that still matters before you are signed in.
const isAuthPage = computed(() => route.path === "/login");
const profileName = computed(() => session.profile?.name?.trim() || "");
const initial = computed(() => (profileName.value ? profileName.value.charAt(0).toUpperCase() : "🙂"));

// Pull down from the top of the page to reload — a full "refresh everything"
// on mobile (the app re-fetches from the API on boot). Default onTrigger is
// window.location.reload().
const { distance: pullDistance, pulling: isPulling, ready: pullReady } = usePullToRefresh();

// Hand Nuxt UI the matching locale so its own components (the calendar, menus)
// speak the same language as our strings.
const uiLocale = computed(() => (currentLocale() === "cs" ? uiCs : uiEn));
// Primary destinations. Icon paths are inline so they render without the
// Iconify CDN round-trip, the same reasoning as the locale flags.
const navItems = computed(() => [
  {
    to: "/",
    label: t("diary.title"),
    icon: "M12 6.04A8.97 8.97 0 0 0 6 3.75c-1.05 0-2.06.18-3 .51v14.25A8.99 8.99 0 0 1 6 18c2.3 0 4.4.87 6 2.29m0-14.25a8.97 8.97 0 0 1 6-2.29c1.05 0 2.06.18 3 .51v14.25A8.99 8.99 0 0 0 18 18a8.97 8.97 0 0 0-6 2.29m0-14.25v14.25",
  },
  {
    to: "/stats",
    label: t("nav.stats"),
    icon: "M3.75 20.25h16.5M7.5 20.25v-6.75m4.5 6.75V8.25m4.5 12V4.5",
  },
]);

</script>

<template>
  <UApp :locale="uiLocale">
    <!-- Bare shell for sign-in / sign-up: the card sits in the middle of the
         viewport with nothing around it. The language switcher lives inside the
         card itself, so there is no floating chrome left to place. -->
    <div v-if="isAuthPage" class="grid min-h-dvh place-items-center px-4 py-10">
      <RouterView />
    </div>

    <template v-else>
    <header
      class="sticky top-0 z-30 border-b border-gray-200/60 bg-white/80 backdrop-blur-md supports-[backdrop-filter]:bg-white/65 dark:border-gray-800/60 dark:bg-gray-950/80 dark:supports-[backdrop-filter]:bg-gray-950/65"
    >
      <div class="mx-auto flex h-14 max-w-6xl items-center gap-2 px-3 sm:gap-3 sm:px-6">
        <!-- Brand. Shrinks before anything else, and the wordmark drops away on
             the narrowest screens so the controls always keep their room. -->
        <RouterLink
          to="/"
          class="flex min-w-0 shrink items-center gap-2 rounded-xl font-semibold outline-none focus-visible:ring-2 focus-visible:ring-emerald-500/60"
        >
          <span class="grid size-8 shrink-0 place-items-center rounded-xl bg-emerald-500/15 text-lg">🥗</span>
          <span class="truncate text-base tracking-tight sm:text-lg">Calories</span>
        </RouterLink>

        <template v-if="session.authenticated">
          <!-- Primary navigation, as one segmented group so the two destinations
               read as a pair rather than as loose links. Labels collapse to
               icons on small screens — the diary used to be reachable only by
               clicking the logo. -->
          <nav class="ml-1 flex items-center gap-0.5 rounded-xl bg-gray-100/70 p-0.5 dark:bg-gray-900/70">
            <RouterLink
              v-for="item in navItems"
              :key="item.to"
              :to="item.to"
              class="flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm font-medium text-gray-500 outline-none transition hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-emerald-500/60 dark:text-gray-400 dark:hover:text-gray-100"
              active-class="bg-white text-gray-900 shadow-sm dark:bg-gray-800 dark:text-gray-100"
              :aria-label="item.label"
            >
              <svg viewBox="0 0 24 24" class="size-4 shrink-0" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                <path :d="item.icon" />
              </svg>
              <span class="hidden sm:inline">{{ item.label }}</span>
            </RouterLink>
          </nav>

          <!-- Account cluster, pushed to the far edge. -->
          <div class="ml-auto flex items-center gap-1">
            <LocaleToggle />

            <RouterLink
              to="/profiles/me"
              class="flex max-w-[9rem] items-center gap-2 rounded-xl p-1 outline-none transition hover:bg-gray-100 focus-visible:ring-2 focus-visible:ring-emerald-500/60 sm:pr-2.5 dark:hover:bg-gray-800"
              active-class="bg-gray-100 dark:bg-gray-800"
              :title="profileName || t('nav.profile')"
            >
              <span class="grid size-7 shrink-0 place-items-center rounded-full bg-gradient-to-br from-emerald-400 to-emerald-600 text-xs font-semibold text-white">{{ initial }}</span>
              <span class="hidden truncate text-sm text-gray-700 sm:inline dark:text-gray-200">{{ profileName || t("nav.profile") }}</span>
            </RouterLink>

            <!-- Signing out is not navigation, so it reads as a quiet icon
                 rather than a third link of equal weight. -->
            <button
              type="button"
              class="grid size-9 shrink-0 place-items-center rounded-xl text-gray-400 outline-none transition hover:bg-red-50 hover:text-red-600 focus-visible:ring-2 focus-visible:ring-red-500/60 dark:hover:bg-red-950/50 dark:hover:text-red-400"
              :title="t('nav.logout')"
              :aria-label="t('nav.logout')"
              @click="logout"
            >
              <svg viewBox="0 0 24 24" class="size-5" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                <path d="M15.75 9V5.25A2.25 2.25 0 0 0 13.5 3h-6a2.25 2.25 0 0 0-2.25 2.25v13.5A2.25 2.25 0 0 0 7.5 21h6a2.25 2.25 0 0 0 2.25-2.25V15M12 9l-3 3m0 0 3 3m-3-3h12.75" />
              </svg>
            </button>
          </div>
        </template>

        <div v-else class="ml-auto flex items-center gap-1">
          <LocaleToggle />

          <button
            type="button"
            class="rounded-xl bg-emerald-500 px-3.5 py-2 text-sm font-medium text-white shadow-sm outline-none transition hover:bg-emerald-600 focus-visible:ring-2 focus-visible:ring-emerald-500/60 active:bg-emerald-700"
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

    <main class="mx-auto max-w-6xl px-4 py-6 sm:px-6 sm:py-8">
      <RouterView />
    </main>
    </template>
  </UApp>
</template>
