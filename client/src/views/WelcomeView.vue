<script setup lang="ts">
import { useRouter } from "vue-router";
import { session } from "../lib/session";
import { t } from "../lib/i18n";
import { useUiSize } from "../composables/useUiSize";

const router = useRouter();
const { hero } = useUiSize();
</script>

<template>
  <div class="space-y-6 py-10 text-center">
    <div class="text-5xl">🥗</div>
    <h1 class="text-2xl font-bold sm:text-3xl">Calories</h1>
    <p class="mx-auto max-w-md text-gray-500">{{ t("welcome.tagline") }}</p>
    <div class="flex justify-center gap-3 pt-2">
      <UButton :size="hero" :label="t('welcome.signIn')" @click="router.push('/login')" />
      <UButton
        v-if="session.registration"
        :size="hero"
        color="neutral"
        variant="soft"
        :label="t('welcome.createAccount')"
        @click="router.push({ path: '/login', query: { mode: 'register' } })"
      />
    </div>
  </div>
</template>
