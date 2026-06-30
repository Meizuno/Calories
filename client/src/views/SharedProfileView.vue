<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { api } from "../lib/api";
import type { Day, Profile } from "../lib/types";
import DaySummary from "../components/DaySummary.vue";
import MealTable from "../components/MealTable.vue";

const route = useRoute();
const uuid = computed(() => route.params.uuid as string);

const profile = ref<Profile | null>(null);
const day = ref<Day | null>(null);
const error = ref(false);

const pad = (n: number) => String(n).padStart(2, "0");
const todayISO = () => {
  const d = new Date();
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
};
const date = ref(todayISO());
const isToday = computed(() => date.value === todayISO());
function shiftDate(d: string, n: number) {
  const t = new Date(d + "T00:00:00Z");
  t.setUTCDate(t.getUTCDate() + n);
  return t.toISOString().slice(0, 10);
}
const WD = ["ne", "po", "út", "st", "čt", "pá", "so"];
const weekday = (d: string) => WD[new Date(d + "T00:00:00Z").getUTCDay()];
function goto(d: string) {
  if (d > todayISO()) return;
  date.value = d;
}

async function load() {
  error.value = false;
  try {
    profile.value = await api.getShared(uuid.value);
    day.value = await api.getSharedDay(uuid.value, date.value);
  } catch {
    error.value = true;
  }
}
watch([uuid, date], load, { immediate: true });

const k = (n: number) => Math.round(n);
</script>

<template>
  <div v-if="error" class="p-8 text-center text-gray-400">Profil nenalezen nebo není sdílený.</div>

  <div v-else-if="profile && day" class="space-y-5">
    <h1 class="text-xl font-semibold sm:text-2xl">
      {{ profile.name || "Sdílený profil" }}
      <span class="text-sm font-normal text-gray-400">(jen ke čtení)</span>
    </h1>

    <div class="flex items-center justify-between">
      <UButton color="neutral" variant="soft" label="←" @click="goto(shiftDate(date, -1))" />
      <div class="text-base font-semibold tabular-nums sm:text-lg">
        {{ date }} <span class="text-gray-400">({{ weekday(date) }})</span>
      </div>
      <UButton color="neutral" variant="soft" label="→" :disabled="isToday" @click="goto(shiftDate(date, 1))" />
    </div>

    <DaySummary :day="day" />

    <div
      v-for="m in day.meals"
      :key="m.id"
      class="rounded-lg border border-gray-200 p-3 dark:border-gray-800"
    >
      <div class="flex items-center justify-between font-medium">
        <span>{{ m.name }}</span>
        <span class="tabular-nums text-sm font-normal text-gray-500">{{ k(m.total.kcal) }} kcal</span>
      </div>
      <MealTable class="mt-2" :meal="m" />
    </div>
  </div>

  <div v-else class="p-8 text-center text-gray-400">Načítání…</div>
</template>
