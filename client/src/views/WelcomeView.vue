<script setup lang="ts">
import { session, login } from "../lib/session";
</script>

<template>
  <div class="space-y-6 py-10 text-center">
    <div class="text-5xl">🥗</div>
    <h1 class="text-2xl font-bold sm:text-3xl">Calories</h1>
    <p class="mx-auto max-w-md text-gray-500">Jednoduchý deník kalorií a makroživin — zapiš, co jíš, a sleduj svůj denní cíl.</p>

    <div v-if="session.authenticated" class="space-y-4">
      <p class="text-lg">Vítej{{ session.profile?.name ? ", " + session.profile.name : "" }}! 👋</p>
      <div class="flex justify-center gap-3">
        <UButton to="/diary" size="lg" label="Otevřít deník" />
        <UButton to="/profiles/me" size="lg" color="neutral" variant="soft" label="Můj profil" />
      </div>
    </div>

    <div v-else class="pt-2">
      <UButton size="lg" label="Přihlásit se" @click="login" />
    </div>
  </div>
</template>
