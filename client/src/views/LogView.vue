<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { api } from "../lib/api";
import type { Day } from "../lib/types";

const route = useRoute();

const pad = (n: number) => String(n).padStart(2, "0");
const todayISO = () => {
  const d = new Date();
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
};
const date = computed(() => (route.query.date as string) || todayISO());
const WD = ["ne", "po", "út", "st", "čt", "pá", "so"];
const weekday = (d: string) => WD[new Date(d + "T00:00:00Z").getUTCDay()];

const day = ref<Day | null>(null);
const mealId = ref<number>();
const newMeal = ref("");
const entry = ref({ name: "", quantity: "", unit: "g", kcal: "0", carb: "0", protein: "0", fat: "0" });

const units = [
  { label: "g", value: "g" },
  { label: "ml", value: "ml" },
  { label: "ks", value: "ks" },
  { label: "porce", value: "porce" },
];

async function reload() {
  day.value = await api.getDay(date.value);
  if (!mealId.value || !day.value.meals.some((m) => m.id === mealId.value)) {
    mealId.value = day.value.meals[0]?.id;
  }
}
watch(date, reload, { immediate: true });

const mealSelectItems = computed(() => (day.value?.meals ?? []).map((m) => ({ label: m.name, value: m.id })));
const hasMeals = computed(() => (day.value?.meals.length ?? 0) > 0);
const numVal = (s: string) => Math.max(0, parseFloat(s) || 0);
const k = (n: number) => Math.round(n);

async function addMeal() {
  const n = newMeal.value.trim();
  if (!n) return;
  day.value = await api.addMeal(date.value, n);
  newMeal.value = "";
  if (!mealId.value) mealId.value = day.value.meals[0]?.id;
}
async function addEntry() {
  const q = parseFloat(entry.value.quantity);
  if (!mealId.value || !entry.value.name.trim() || !(q > 0)) return;
  day.value = await api.addEntry({
    date: date.value,
    mealId: mealId.value,
    name: entry.value.name.trim(),
    quantity: q,
    unit: entry.value.unit || "g",
    kcal: numVal(entry.value.kcal),
    carb: numVal(entry.value.carb),
    protein: numVal(entry.value.protein),
    fat: numVal(entry.value.fat),
  });
  entry.value = { name: "", quantity: "", unit: "g", kcal: "0", carb: "0", protein: "0", fat: "0" };
}
</script>

<template>
  <div v-if="day" class="space-y-5">
    <div class="flex items-center justify-between gap-3">
      <div>
        <h1 class="text-lg font-semibold sm:text-xl">Přidat</h1>
        <p class="text-sm text-gray-500 tabular-nums">{{ date }} <span class="text-gray-400">({{ weekday(date) }})</span></p>
      </div>
      <UButton color="neutral" variant="soft" label="← Deník" :to="{ path: '/', query: { date } }" />
    </div>

    <!-- New meal (Snídaně, Oběd…) — there must be a meal before items can be added. -->
    <UCard>
      <template #header><span class="font-medium">Nové jídlo</span></template>
      <form class="flex gap-2" @submit.prevent="addMeal">
        <UInput v-model="newMeal" placeholder="Snídaně, Oběd, Večeře…" class="flex-1" />
        <UButton type="submit" color="neutral" label="+ Jídlo" />
      </form>
      <div v-if="hasMeals" class="mt-3 flex flex-wrap gap-1.5">
        <span
          v-for="m in day.meals"
          :key="m.id"
          class="rounded-full bg-gray-100 px-2.5 py-1 text-xs tabular-nums text-gray-600 dark:bg-gray-800 dark:text-gray-300"
        >{{ m.name }} · {{ k(m.total.kcal) }} kcal</span>
      </div>
    </UCard>

    <!-- New item into a meal. -->
    <UCard v-if="hasMeals">
      <template #header><span class="font-medium">Přidat položku</span></template>
      <form class="grid grid-cols-2 gap-3 sm:grid-cols-4" @submit.prevent="addEntry">
        <label class="flex flex-col gap-1 text-xs text-gray-500">
          Přidat do
          <USelect v-model="mealId" :items="mealSelectItems" />
        </label>
        <label class="flex flex-col gap-1 text-xs text-gray-500">
          Název
          <UInput v-model="entry.name" placeholder="např. Banán" />
        </label>
        <label class="flex flex-col gap-1 text-xs text-gray-500">
          Množství
          <UInput v-model="entry.quantity" type="number" step="any" min="0" />
        </label>
        <label class="flex flex-col gap-1 text-xs text-gray-500">
          Jednotka
          <USelect v-model="entry.unit" :items="units" />
        </label>
        <label class="flex flex-col gap-1 text-xs text-gray-500">
          kcal
          <UInput v-model="entry.kcal" type="number" step="any" min="0" />
        </label>
        <label class="flex flex-col gap-1 text-xs text-gray-500">
          Sach.
          <UInput v-model="entry.carb" type="number" step="any" min="0" />
        </label>
        <label class="flex flex-col gap-1 text-xs text-gray-500">
          Bílk.
          <UInput v-model="entry.protein" type="number" step="any" min="0" />
        </label>
        <label class="flex flex-col gap-1 text-xs text-gray-500">
          Tuky
          <UInput v-model="entry.fat" type="number" step="any" min="0" />
        </label>
        <div class="col-span-2 flex justify-end sm:col-span-4">
          <UButton type="submit" label="Přidat položku" />
        </div>
      </form>
    </UCard>

    <p v-else class="text-center text-sm text-gray-500">Nejdřív přidej jídlo, pak do něj můžeš vkládat položky.</p>
  </div>

  <div v-else class="p-8 text-center text-gray-400">Načítání…</div>
</template>
