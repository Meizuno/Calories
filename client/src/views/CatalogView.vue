<script setup lang="ts">
import { computed, ref } from "vue";
import { api } from "../lib/api";
import type { Food } from "../lib/types";

const foods = ref<Food[]>([]);
const open = ref<string[]>(["foods"]);
const items = computed(() => [{ label: `Seznam potravin (${foods.value.length})`, value: "foods" }]);
const allOpen = computed(() => open.value.includes("foods"));
function toggleAll() {
  open.value = allOpen.value ? [] : ["foods"];
}
const form = ref({ name: "", basisUnit: "g", basisAmount: "100", kcal: "0", carb: "0", protein: "0", fat: "0" });
const units = [
  { label: "g", value: "g" },
  { label: "ml", value: "ml" },
  { label: "ks", value: "ks" },
  { label: "porce", value: "porce" },
];

api.listFoods().then((x) => (foods.value = x));

const num = (s: string) => Math.max(0, parseFloat(s) || 0);

async function add() {
  if (!form.value.name.trim()) return;
  foods.value = await api.createFood({
    name: form.value.name.trim(),
    basisUnit: form.value.basisUnit,
    basisAmount: num(form.value.basisAmount) || 100,
    kcal: num(form.value.kcal),
    carb: num(form.value.carb),
    protein: num(form.value.protein),
    fat: num(form.value.fat),
  });
  form.value.name = "";
}
async function del(id: number) {
  if (confirm("Smazat?")) foods.value = await api.deleteFood(id);
}

const k = (n: number) => Math.round(n);
const g = (n: number) => Math.round(n * 10) / 10;
</script>

<template>
  <div class="space-y-5">
    <div class="flex items-center justify-between">
      <h1 class="text-lg font-semibold sm:text-xl">Potraviny</h1>
      <UButton size="xs" color="neutral" variant="soft" :label="allOpen ? 'Sbalit vše' : 'Rozbalit vše'" @click="toggleAll" />
    </div>

    <UCard>
      <form class="flex flex-wrap items-end gap-3" @submit.prevent="add">
        <label class="flex flex-col gap-1 text-xs text-gray-500">Název<UInput v-model="form.name" class="w-48" /></label>
        <label class="flex flex-col gap-1 text-xs text-gray-500">Jednotka<USelect v-model="form.basisUnit" :items="units" class="w-24" /></label>
        <label class="flex flex-col gap-1 text-xs text-gray-500">Na kolik<UInput v-model="form.basisAmount" type="number" step="any" min="0" class="w-24" /></label>
        <label class="flex flex-col gap-1 text-xs text-gray-500">kcal<UInput v-model="form.kcal" type="number" step="any" min="0" class="w-20" /></label>
        <label class="flex flex-col gap-1 text-xs text-gray-500">Sach.<UInput v-model="form.carb" type="number" step="any" min="0" class="w-20" /></label>
        <label class="flex flex-col gap-1 text-xs text-gray-500">Bílk.<UInput v-model="form.protein" type="number" step="any" min="0" class="w-20" /></label>
        <label class="flex flex-col gap-1 text-xs text-gray-500">Tuky<UInput v-model="form.fat" type="number" step="any" min="0" class="w-20" /></label>
        <UButton type="submit" label="Přidat potravinu" />
      </form>
    </UCard>

    <UAccordion type="multiple" v-model="open" :items="items">
      <template #content>
      <table class="w-full text-sm sm:text-base">
        <thead class="text-gray-500">
          <tr>
            <th class="py-1 text-left font-normal text-gray-500">Název</th>
            <th class="py-1 text-right font-normal text-gray-500">Základ</th>
            <th class="py-1 text-right font-medium text-gray-600 dark:text-gray-300">kcal</th>
            <th class="py-1 text-right font-medium text-sky-600 dark:text-sky-400">Sach.</th>
            <th class="py-1 text-right font-medium text-emerald-600 dark:text-emerald-400">Bílk.</th>
            <th class="py-1 text-right font-medium text-amber-600 dark:text-amber-400">Tuky</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="x in foods" :key="x.id" class="border-t border-gray-100 hover:bg-gray-50/70 dark:border-gray-800 dark:hover:bg-gray-900/40">
            <td class="py-1.5">{{ x.name }}</td>
            <td class="py-1.5 text-right tabular-nums text-gray-500">{{ g(x.basisAmount) }} {{ x.basisUnit }}</td>
            <td class="py-1.5 text-right font-medium tabular-nums">{{ k(x.kcal) }}</td>
            <td class="py-1.5 text-right tabular-nums text-sky-600 dark:text-sky-400">{{ g(x.carb) }}</td>
            <td class="py-1.5 text-right tabular-nums text-emerald-600 dark:text-emerald-400">{{ g(x.protein) }}</td>
            <td class="py-1.5 text-right tabular-nums text-amber-600 dark:text-amber-400">{{ g(x.fat) }}</td>
            <td class="py-1.5 text-right"><UButton size="xs" color="error" variant="ghost" label="✕" @click="del(x.id)" /></td>
          </tr>
          <tr v-if="!foods.length"><td colspan="7" class="py-3 text-center text-gray-400">Zatím prázdné</td></tr>
        </tbody>
      </table>
      </template>
    </UAccordion>
  </div>
</template>
