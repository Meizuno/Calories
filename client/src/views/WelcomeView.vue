<script setup lang="ts">
import { login } from "../lib/session";
</script>

<template>
  <div class="space-y-6 py-10 text-center">
    <div class="text-5xl">🥗</div>
    <h1 class="text-2xl font-bold sm:text-3xl">Calories</h1>
    <p class="mx-auto max-w-md text-gray-500">Jednoduchý deník kalorií a makroživin — zapiš, co jíš, a sleduj svůj denní cíl.</p>
    <div class="pt-2">
      <UButton size="lg" label="Přihlásit se" @click="login" />
    </div>
  </div>
</template>
