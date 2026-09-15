<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { api } from "../lib/api";
import type { Day, Food } from "../lib/types";
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
const g = (n: number) => Math.round(n * 10) / 10;

// ── remembered foods ─────────────────────────────────────────────────────────
// Everything logged is remembered, so the second time you eat something you
// pick it and give a quantity instead of retyping four macro fields. Held per
// basis (100 g, 1 ks) on the server, scaled here to whatever is being eaten.
const foods = ref<Food[]>([]);
// The food the macro fields currently come from. Set by picking a suggestion,
// cleared the moment anything is typed over them by hand — a value someone
// corrected must never be silently recomputed away.
const linked = ref<Food | null>(null);
const suggestOpen = ref(false);

async function loadFoods() {
  try {
    foods.value = await api.getFoods();
  } catch {
    /* non-fatal: suggestions are a convenience, the form still works */
  }
}
loadFoods();

// Match without diacritics, so "banan" finds "Banán" — the accents are the
// first thing anyone skips when typing quickly.
const fold = (s: string) =>
  s
    .normalize("NFD")
    .replace(/\p{Diacritic}/gu, "")
    .toLowerCase();

const suggestions = computed(() => {
  const q = fold(entry.value.name.trim());
  const pool = q ? foods.value.filter((f) => fold(f.name).includes(q)) : foods.value;
  return pool.slice(0, 6);
});

// Rewrite the macro fields for the current quantity of the linked food.
function rescale() {
  const f = linked.value;
  if (!f) return;
  const q = numVal(entry.value.quantity);
  if (q <= 0 || f.basisAmount <= 0) return;
  const factor = q / f.basisAmount;
  entry.value.kcal = String(Math.round(f.kcal * factor));
  entry.value.carb = String(g(f.carb * factor));
  entry.value.protein = String(g(f.protein * factor));
  entry.value.fat = String(g(f.fat * factor));
}

function pickFood(f: Food) {
  linked.value = f;
  entry.value.name = f.name;
  entry.value.unit = f.basisUnit;
  // An empty quantity becomes the basis itself: "100 g" is the commonest thing
  // to want, and it gives the macro fields something to scale from.
  if (numVal(entry.value.quantity) <= 0) entry.value.quantity = String(f.basisAmount);
  rescale();
  suggestOpen.value = false;
}

// Typing over the name, the unit or any macro means the line is no longer that
// remembered food, so it stops being rescaled.
const unlink = () => (linked.value = null);
watch(() => entry.value.quantity, rescale);
watch(() => entry.value.unit, () => {
  if (linked.value && entry.value.unit !== linked.value.basisUnit) unlink();
});

async function forgetFood(f: Food) {
  if (linked.value?.id === f.id) unlink();
  try {
    foods.value = await api.forgetFood(f.id);
  } catch {
    /* leave the list as it is; nothing was lost from the diary */
  }
}

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
  unlink();
  // The line just logged is now remembered (or corrected), so pick that up.
  loadFoods();
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
          <label class="relative flex flex-col gap-1 text-xs text-gray-500">
            {{ t("common.name") }}
            <UInput
              v-model="entry.name"
              :size="control"
              class="w-full"
              autocomplete="off"
              :placeholder="t('log.foodPlaceholder')"
              @update:model-value="unlink"
              @focus="suggestOpen = true"
              @blur="suggestOpen = false"
            />
            <!-- Remembered foods, filtered by what has been typed. mousedown is
                 prevented so choosing one does not blur the field and close the
                 list before the click lands. -->
            <ul
              v-if="suggestOpen && suggestions.length"
              class="absolute inset-x-0 top-full z-20 mt-1 overflow-hidden rounded-lg border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-900"
            >
              <li v-for="f in suggestions" :key="f.id" class="flex items-stretch">
                <button
                  type="button"
                  class="flex min-w-0 flex-1 items-baseline gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-gray-100 dark:hover:bg-gray-800"
                  @mousedown.prevent="pickFood(f)"
                >
                  <span class="truncate text-gray-900 dark:text-gray-100">{{ f.name }}</span>
                  <span class="ml-auto shrink-0 tabular-nums text-xs text-gray-400">
                    {{ k(f.kcal) }} kcal / {{ g(f.basisAmount) }} {{ f.basisUnit }}
                  </span>
                </button>
                <button
                  type="button"
                  class="px-2 text-gray-300 transition-colors hover:text-red-500 dark:text-gray-600"
                  :title="t('log.forgetFood')"
                  :aria-label="t('log.forgetFood')"
                  @mousedown.prevent="forgetFood(f)"
                >✕</button>
              </li>
            </ul>
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

        <p v-if="linked" class="-mb-1 text-xs text-gray-400">
          {{ t("log.scaledFrom", { name: linked.name, kcal: k(linked.kcal), amount: g(linked.basisAmount), unit: linked.basisUnit }) }}
        </p>

        <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <label class="flex flex-col gap-1 text-xs text-gray-500">
            kcal
            <UInput v-model="entry.kcal" type="number" step="any" min="0" inputmode="decimal" :size="control" class="w-full" placeholder="0" @update:model-value="unlink" />
          </label>
          <label class="flex flex-col gap-1 text-xs text-sky-500">
            {{ t("macros.carbShort") }}.
            <UInput v-model="entry.carb" type="number" step="any" min="0" inputmode="decimal" :size="control" class="w-full" placeholder="0" @update:model-value="unlink" />
          </label>
          <label class="flex flex-col gap-1 text-xs text-emerald-500">
            {{ t("macros.proteinShort") }}.
            <UInput v-model="entry.protein" type="number" step="any" min="0" inputmode="decimal" :size="control" class="w-full" placeholder="0" @update:model-value="unlink" />
          </label>
          <label class="flex flex-col gap-1 text-xs text-amber-500">
            {{ t("macros.fatShort") }}
            <UInput v-model="entry.fat" type="number" step="any" min="0" inputmode="decimal" :size="control" class="w-full" placeholder="0" @update:model-value="unlink" />
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
