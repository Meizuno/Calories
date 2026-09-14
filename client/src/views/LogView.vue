<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { api } from "../lib/api";
import type { Day } from "../lib/types";
import { t, weekdayShort } from "../lib/i18n";
import { useUiSize } from "../composables/useUiSize";

const route = useRoute();
const { control, inline } = useUiSize();

const pad = (n: number) => String(n).padStart(2, "0");
const todayISO = () => {
  const d = new Date();
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
};
const date = computed(() => (route.query.date as string) || todayISO());
const weekday = (d: string) => weekdayShort(d);

const day = ref<Day | null>(null);
const mealId = ref<number>();
const newMeal = ref("");
// Macro fields start empty rather than at "0": a zero has to be selected and
// overwritten for every field, and it also reads as a real value.
const entry = ref({ name: "", quantity: "", unit: "g", kcal: "", carb: "", protein: "", fat: "" });

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

const hasMeals = computed(() => (day.value?.meals.length ?? 0) > 0);
const numVal = (s: string) => Math.max(0, parseFloat(s) || 0);
const k = (n: number) => Math.round(n);

// kcal implied by the macros (4/4/9 per gram). Offered as a one-tap fill rather
// than written automatically: packaging often disagrees slightly with the
// arithmetic, and silently overwriting what someone typed is worse than a
// button they can ignore.
const impliedKcal = computed(() =>
  Math.round(numVal(entry.value.carb) * 4 + numVal(entry.value.protein) * 4 + numVal(entry.value.fat) * 9),
);
const canFillKcal = computed(() => impliedKcal.value > 0 && impliedKcal.value !== numVal(entry.value.kcal));
const applyImpliedKcal = () => (entry.value.kcal = String(impliedKcal.value));

const canAddEntry = computed(
  () => !!mealId.value && entry.value.name.trim().length > 0 && numVal(entry.value.quantity) > 0,
);

// A short confirmation of what was just added: the form clears on submit, so
// without it there is no sign anything happened.
const justAdded = ref("");
let addedTimer: ReturnType<typeof setTimeout> | undefined;
function noteAdded(name: string) {
  justAdded.value = name;
  clearTimeout(addedTimer);
  addedTimer = setTimeout(() => (justAdded.value = ""), 4000);
}

async function addMeal() {
  const n = newMeal.value.trim();
  if (!n) return;
  day.value = await api.addMeal(date.value, n);
  newMeal.value = "";
  // Target the meal just created — it is almost always the one being filled.
  mealId.value = day.value.meals.find((m) => m.name === n)?.id ?? day.value.meals[0]?.id;
  noteAdded(n);
}
async function addEntry() {
  const q = parseFloat(entry.value.quantity);
  if (!canAddEntry.value) return;
  const name = entry.value.name.trim();
  day.value = await api.addEntry({
    date: date.value,
    mealId: mealId.value,
    name,
    quantity: q,
    unit: entry.value.unit || "g",
    kcal: numVal(entry.value.kcal),
    carb: numVal(entry.value.carb),
    protein: numVal(entry.value.protein),
    fat: numVal(entry.value.fat),
  });
  // Keep the unit: the next item is usually measured the same way.
  const unit = entry.value.unit;
  entry.value = { name: "", quantity: "", unit, kcal: "", carb: "", protein: "", fat: "" };
  noteAdded(name);
}
</script>

<template>
  <div v-if="day" class="mx-auto max-w-3xl space-y-5">
    <div class="flex items-center justify-between gap-3">
      <div>
        <h1 class="text-lg font-semibold sm:text-xl">{{ t("log.title") }}</h1>
        <p class="text-sm tabular-nums text-gray-500">{{ date }} <span class="text-gray-400">({{ weekday(date) }})</span></p>
      </div>
      <UButton color="neutral" variant="soft" :size="inline" icon="i-ui-prev" :label="t('log.back')" :to="{ path: '/', query: { date } }" />
    </div>

    <p
      v-if="justAdded"
      class="flex items-center gap-2 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950 dark:text-emerald-200"
      role="status"
    >
      <UIcon name="i-ui-check" class="size-4 shrink-0" />
      {{ t("log.added", { name: justAdded }) }}
    </p>

    <!-- Adding an item is the reason this page exists, so it leads. Creating a
         meal is occasional and sits underneath — unless there are no meals yet,
         in which case there is nothing to add to and the order flips. -->
    <UCard v-if="hasMeals">
      <template #header><span class="font-medium">{{ t("log.addEntry") }}</span></template>

      <form class="space-y-4" @submit.prevent="addEntry">
        <!-- The meal chips are the target selector. They already listed every
             meal with its total; making them selectable removes a separate
             dropdown and shows the choice rather than hiding it. -->
        <div>
          <span class="mb-1.5 block text-xs text-gray-500">{{ t("log.pickMeal") }}</span>
          <div class="flex flex-wrap gap-1.5">
            <button
              v-for="m in day.meals"
              :key="m.id"
              type="button"
              class="rounded-full px-3 py-1.5 text-sm outline-none transition focus-visible:ring-2 focus-visible:ring-emerald-500/60"
              :class="mealId === m.id
                ? 'bg-emerald-500 font-medium text-white'
                : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-800 dark:text-gray-300 dark:hover:bg-gray-700'"
              :aria-pressed="mealId === m.id"
              @click="mealId = m.id"
            >
              {{ m.name }}
              <span class="tabular-nums" :class="mealId === m.id ? 'text-emerald-50' : 'text-gray-400'">· {{ k(m.total.kcal) }} kcal</span>
            </button>
          </div>
        </div>

        <div class="grid gap-3 sm:grid-cols-[1fr_7rem_7rem]">
          <label class="flex flex-col gap-1 text-xs text-gray-500">
            {{ t("common.name") }}
            <UInput v-model="entry.name" :size="control" class="w-full" :placeholder="t('log.foodPlaceholder')" />
          </label>
          <label class="flex flex-col gap-1 text-xs text-gray-500">
            {{ t("common.quantity") }}
            <UInput v-model="entry.quantity" type="number" step="any" min="0" inputmode="decimal" :size="control" class="w-full" placeholder="0" />
          </label>
          <label class="flex flex-col gap-1 text-xs text-gray-500">
            {{ t("common.unit") }}
            <USelect v-model="entry.unit" :items="units" :size="control" class="w-full" />
          </label>
        </div>

        <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <label class="flex flex-col gap-1 text-xs text-gray-500">
            kcal
            <UInput v-model="entry.kcal" type="number" step="any" min="0" inputmode="decimal" :size="control" class="w-full" placeholder="0" />
          </label>
          <label class="flex flex-col gap-1 text-xs text-sky-500">
            {{ t("macros.carbShort") }}.
            <UInput v-model="entry.carb" type="number" step="any" min="0" inputmode="decimal" :size="control" class="w-full" placeholder="0" />
          </label>
          <label class="flex flex-col gap-1 text-xs text-emerald-500">
            {{ t("macros.proteinShort") }}.
            <UInput v-model="entry.protein" type="number" step="any" min="0" inputmode="decimal" :size="control" class="w-full" placeholder="0" />
          </label>
          <label class="flex flex-col gap-1 text-xs text-amber-500">
            {{ t("macros.fatShort") }}
            <UInput v-model="entry.fat" type="number" step="any" min="0" inputmode="decimal" :size="control" class="w-full" placeholder="0" />
          </label>
        </div>

        <div class="flex flex-wrap items-center justify-end gap-2">
          <UButton
            v-if="canFillKcal"
            :size="inline"
            color="neutral"
            variant="soft"
            :label="t('log.fromMacros', { n: impliedKcal })"
            @click="applyImpliedKcal"
          />
          <UButton type="submit" :size="control" :disabled="!canAddEntry" :label="t('log.addEntry')" />
        </div>
      </form>
    </UCard>

    <UCard>
      <template #header>
        <span class="font-medium">{{ t("log.newMeal") }}</span>
      </template>
      <form class="flex gap-2" @submit.prevent="addMeal">
        <UInput v-model="newMeal" :size="control" class="flex-1" :placeholder="t('log.mealPlaceholder')" />
        <UButton type="submit" color="neutral" :size="control" :disabled="!newMeal.trim()" :label="t('log.addMeal')" />
      </form>
      <p v-if="!hasMeals" class="mt-2 text-xs text-gray-500">{{ t("log.addMealFirst") }}</p>
    </UCard>
  </div>

  <div v-else class="p-8 text-center text-gray-400">{{ t("common.loading") }}</div>
</template>
