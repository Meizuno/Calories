<script setup lang="ts">
import { RouterLink, RouterView } from "vue-router";
import { session, login, logout } from "./lib/session";
</script>

<template>
  <UApp>
    <header class="border-b border-gray-200 dark:border-gray-800">
      <div class="mx-auto flex max-w-3xl items-center justify-between px-4 py-3 sm:px-6 sm:py-4">
        <RouterLink to="/" class="text-base font-semibold sm:text-lg">🥗 Calories</RouterLink>
        <nav class="flex items-center gap-4 text-sm text-gray-600 sm:gap-6 sm:text-base dark:text-gray-300">
          <template v-if="session.authenticated">
            <RouterLink to="/diary" class="hover:text-gray-900 dark:hover:text-white">Deník</RouterLink>
            <RouterLink to="/catalog" class="hover:text-gray-900 dark:hover:text-white">Potraviny</RouterLink>
            <RouterLink to="/profiles/me" class="hover:text-gray-900 dark:hover:text-white">Profil</RouterLink>
            <button type="button" class="hover:text-gray-900 dark:hover:text-white" @click="logout">Odhlásit</button>
          </template>
          <template v-else>
            <button type="button" class="hover:text-gray-900 dark:hover:text-white" @click="login">Přihlásit</button>
          </template>
        </nav>
      </div>
    </header>
    <main class="mx-auto max-w-3xl px-4 py-6 sm:px-6 sm:py-8">
      <RouterView />
    </main>
  </UApp>
</template>
