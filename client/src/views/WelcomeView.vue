<script setup lang="ts">
import { useRouter } from "vue-router";
import { session } from "../lib/session";

const router = useRouter();
</script>

<template>
  <div class="space-y-6 py-10 text-center">
    <div class="text-5xl">🥗</div>
    <h1 class="text-2xl font-bold sm:text-3xl">Calories</h1>
    <p class="mx-auto max-w-md text-gray-500">Jednoduchý deník kalorií a makroživin — zapiš, co jíš, a sleduj svůj denní cíl.</p>
    <div class="flex justify-center gap-3 pt-2">
      <UButton size="lg" label="Přihlásit se" @click="router.push('/login')" />
      <UButton
        v-if="session.registration"
        size="lg"
        color="neutral"
        variant="soft"
        label="Vytvořit účet"
        @click="router.push({ path: '/login', query: { mode: 'register' } })"
      />
    </div>
  </div>
</template>
