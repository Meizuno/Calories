<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api } from "../lib/api";
import type { Day } from "../lib/types";
import DaySummary from "../components/DaySummary.vue";
import MealTable from "../components/MealTable.vue";

const route = useRoute();
const router = useRouter();

const pad = (n: number) => String(n).padStart(2, "0");
// Local calendar date — not UTC, so "dnes" matches the user's actual day.
const todayISO = () => {
  const d = new Date();
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
};
const date = computed(() => (route.query.date as string) || todayISO());
const isToday = computed(() => date.value === todayISO());

function shiftDate(d: string, n: number) {
  const t = new Date(d + "T00:00:00Z");
  t.setUTCDate(t.getUTCDate() + n);
  return t.toISOString().slice(0, 10);
}
const WD = ["ne", "po", "út", "st", "čt", "pá", "so"];
const weekday = (d: string) => WD[new Date(d + "T00:00:00Z").getUTCDay()];

function goto(d: string) {
  if (d > todayISO()) return; // no future days
  router.push({ query: { date: d } });
}

const day = ref<Day | null>(null);
const open = ref<string[]>([]);
const mealId = ref<number>();
const entry = ref({ name: "", quantity: "", unit: "g", kcal: "0", carb: "0", protein: "0", fat: "0" });
const newMeal = ref("");

// inline-edit state
const editingMeal = ref<number | null>(null);
const mealDraft = ref("");
const noteDraft = ref("");

const units = [
  { label: "g", value: "g" },
  { label: "ml", value: "ml" },
  { label: "ks", value: "ks" },
  { label: "porce", value: "porce" },
];

async function reload() {
  day.value = await api.getDay(date.value);
  mealId.value = day.value?.meals[0]?.id; // always a meal of the loaded day
  cancelEdit();
  expandAll();
}
watch(date, reload, { immediate: true });

const mealSelectItems = computed(() => (day.value?.meals ?? []).map((m) => ({ label: m.name, value: m.id })));
const mealItems = computed(() => (day.value?.meals ?? []).map((m) => ({ label: m.name, value: String(m.id), meal: m })));
const allOpen = computed(() => (day.value?.meals.length ?? 0) > 0 && open.value.length === (day.value?.meals.length ?? 0));

function expandAll() {
  open.value = (day.value?.meals ?? []).map((m) => String(m.id));
}
function collapseAll() {
  open.value = [];
}
function toggleAll() {
  if (allOpen.value) collapseAll();
  else expandAll();
}

const numVal = (s: string) => Math.max(0, parseFloat(s) || 0);

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
async function delEntry(id: number) {
  if (confirm("Smazat položku?")) day.value = await api.deleteEntry(date.value, id);
}
async function addMeal() {
  const n = newMeal.value.trim();
  if (!n) return;
  day.value = await api.addMeal(date.value, n);
  newMeal.value = "";
  if (!mealId.value) mealId.value = day.value.meals[0]?.id;
  expandAll();
}
async function delMeal(id: number) {
  if (confirm("Smazat jídlo i s položkami?")) day.value = await api.deleteMeal(date.value, id);
}

// ── editing ──────────────────────────────────────────────────────────────────
function cancelEdit() {
  editingMeal.value = null;
}
function startEditMeal(m: { id: number; name: string; note: string }) {
  editingMeal.value = m.id;
  mealDraft.value = m.name;
  noteDraft.value = m.note;
  const key = String(m.id);
  if (!open.value.includes(key)) open.value = [...open.value, key];
}
async function saveMeal(id: number) {
  const n = mealDraft.value.trim();
  if (!n) return;
  day.value = await api.updateMeal(date.value, id, n, noteDraft.value.trim());
  editingMeal.value = null;
}
// Entry editing lives in <MealTable>; it emits the new values, we persist them.
async function onUpdateEntry(
  id: number,
  body: { name: string; quantity: number; unit: string; kcal: number; carb: number; protein: number; fat: number },
) {
  day.value = await api.updateEntry(date.value, id, body);
}

const k = (n: number) => Math.round(n);
</script>

<template>
  <div v-if="day" class="space-y-5">
    <div class="flex items-center justify-between">
      <UButton color="neutral" variant="soft" label="←" @click="goto(shiftDate(date, -1))" />
      <div class="flex flex-wrap items-center justify-center gap-x-2">
        <span class="text-base font-semibold tabular-nums sm:text-lg">{{ date }} <span class="text-gray-400">({{ weekday(date) }})</span></span>
        <span v-if="isToday" class="text-xs font-medium text-emerald-500">dnes</span>
        <UButton v-else size="xs" variant="link" label="↩ na dnes" @click="goto(todayISO())" />
      </div>
      <UButton color="neutral" variant="soft" label="→" :disabled="isToday" @click="goto(shiftDate(date, 1))" />
    </div>

    <DaySummary :day="day" />

    <UCard v-if="day.meals.length">
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
          <UButton type="submit" label="Přidat" />
        </div>
      </form>
    </UCard>

    <div class="flex items-center justify-between">
      <h2 class="text-base font-medium sm:text-lg">Jídelníček</h2>
      <UButton
        size="xs"
        color="neutral"
        variant="soft"
        :label="allOpen ? 'Sbalit vše' : 'Rozbalit vše'"
        @click="toggleAll"
      />
    </div>

    <UAccordion
      type="multiple"
      v-model="open"
      :items="mealItems"
      :ui="{ item: 'mb-2 rounded-lg border border-gray-200 px-3 dark:border-gray-800' }"
    >
      <template #default="{ item }">
        <div class="flex grow items-center justify-between gap-3 pr-3">
          <span class="font-medium">{{ item.meal.name }}</span>
          <span class="flex items-center gap-3">
            <span class="tabular-nums text-sm font-normal text-gray-500">{{ k(item.meal.total.kcal) }} kcal</span>
            <span
              role="button"
              tabindex="0"
              class="cursor-pointer text-sm font-normal text-sky-500 hover:text-sky-600"
              @click.stop="startEditMeal(item.meal)"
              @keydown.enter.stop.prevent="startEditMeal(item.meal)"
            >upravit</span>
            <span
              role="button"
              tabindex="0"
              class="cursor-pointer text-sm font-normal text-red-500 hover:text-red-600"
              @click.stop="delMeal(item.meal.id)"
              @keydown.enter.stop.prevent="delMeal(item.meal.id)"
            >smazat</span>
          </span>
        </div>
      </template>
      <template #content="{ item }">
        <div v-if="editingMeal === item.meal.id" class="mb-3 space-y-2">
          <div class="flex items-center gap-2">
            <UInput
              v-model="mealDraft"
              size="sm"
              class="flex-1"
              placeholder="Název jídla"
              @keydown.enter.prevent="saveMeal(item.meal.id)"
              @keydown.esc="cancelEdit"
            />
            <UButton size="xs" label="Uložit" @click="saveMeal(item.meal.id)" />
            <UButton size="xs" color="neutral" variant="ghost" label="Zrušit" @click="cancelEdit" />
          </div>
          <UTextarea v-model="noteDraft" :rows="2" autoresize class="w-full" placeholder="Poznámka (komentář)…" />
        </div>
        <MealTable
          editable
          :meal="item.meal"
          :show-note="editingMeal !== item.meal.id"
          @update-entry="onUpdateEntry"
          @delete-entry="delEntry"
        />
      </template>
    </UAccordion>

    <form class="flex gap-2" @submit.prevent="addMeal">
      <UInput v-model="newMeal" placeholder="Nové jídlo (Snídaně, Oběd…)" class="flex-1" />
      <UButton type="submit" color="neutral" label="+ Jídlo" />
    </form>
  </div>

  <div v-else class="p-8 text-center text-gray-400">Načítání…</div>
</template>
