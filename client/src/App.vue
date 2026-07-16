<script setup lang="ts">
import { computed } from "vue";
import { RouterLink, RouterView } from "vue-router";
import { session, login, logout } from "./lib/session";
import { usePullToRefresh } from "./composables/usePullToRefresh";

const profileName = computed(() => session.profile?.name?.trim() || "");
const initial = computed(() => (profileName.value ? profileName.value.charAt(0).toUpperCase() : "🙂"));

// Pull down from the top of the page to reload — a full "refresh everything"
// on mobile (the app re-fetches from the API on boot). Default onTrigger is
// window.location.reload().
const { distance: pullDistance, pulling: isPulling, ready: pullReady } = usePullToRefresh();
</script>

<template>
  <UApp>
    <header
      class="sticky top-0 z-30 border-b border-gray-200/70 bg-white/75 backdrop-blur dark:border-gray-800/70 dark:bg-gray-950/70"
    >
      <div class="mx-auto flex max-w-3xl items-center justify-between gap-3 px-4 py-2.5 sm:px-6">
        <RouterLink to="/" class="flex items-center gap-2 font-semibold">
          <span class="grid h-8 w-8 place-items-center rounded-xl bg-emerald-500/15 text-lg">🥗</span>
          <span class="text-base tracking-tight sm:text-lg">Calories</span>
        </RouterLink>

        <nav v-if="session.authenticated" class="flex items-center gap-1">
          <RouterLink
            to="/stats"
            class="rounded-lg px-2.5 py-1.5 text-sm text-gray-500 transition hover:bg-gray-100 hover:text-gray-800 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-100"
            active-class="bg-gray-100 text-gray-900 dark:bg-gray-800 dark:text-gray-100"
          >
            Statistika
          </RouterLink>

          <RouterLink
            to="/profiles/me"
            class="ml-1 flex items-center gap-2 rounded-full py-1 pl-1 pr-3 transition hover:bg-gray-100 dark:hover:bg-gray-800"
            active-class="bg-gray-100 dark:bg-gray-800"
          >
            <span class="grid h-6 w-6 place-items-center rounded-full bg-emerald-500 text-xs font-semibold text-white">{{ initial }}</span>
            <span class="max-w-28 truncate text-sm text-gray-700 dark:text-gray-200">{{ profileName || "Profil" }}</span>
          </RouterLink>

          <button
            type="button"
            class="ml-0.5 rounded-lg px-2.5 py-1.5 text-sm text-gray-400 transition hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-gray-800 dark:hover:text-gray-200"
            @click="logout"
          >
            Odhlásit
          </button>
        </nav>

        <button
          v-else
          type="button"
          class="rounded-lg bg-emerald-500 px-3 py-1.5 text-sm font-medium text-white transition hover:bg-emerald-600"
          @click="login"
        >
          Přihlásit
        </button>
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
        <span>{{ pullReady ? "Uvolněte pro obnovení" : "Táhněte pro obnovení" }}</span>
      </div>
    </div>

    <main class="mx-auto max-w-3xl px-4 py-6 sm:px-6 sm:py-8">
      <RouterView />
    </main>
  </UApp>
</template>
