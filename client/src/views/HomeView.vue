<script setup lang="ts">
import { session } from "../lib/session";
import DiaryView from "./DiaryView.vue";
import WelcomeView from "./WelcomeView.vue";
</script>

<template>
  <DiaryView v-if="session.authenticated" />
  <WelcomeView v-else />
</template>
