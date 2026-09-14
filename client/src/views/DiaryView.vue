<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { type DateValue, getLocalTimeZone, parseDate, today } from "@internationalized/date";
import { api } from "../lib/api";
import type { Day } from "../lib/types";
import DaySummary from "../components/DaySummary.vue";
import MealTable from "../components/MealTable.vue";
import { t, weekdayShort } from "../lib/i18n";

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
const weekday = (d: string) => weekdayShort(d);

function goto(d: string) {
  if (d > todayISO()) return; // no future days
  router.push({ query: { date: d } });
}

const day = ref<Day | null>(null);
const open = ref<string[]>([]);

// Calendar: which days are navigable (have data), and popover state.
const days = ref<Set<string>>(new Set());
const calOpen = ref(false);
const calValue = computed(() => parseDate(date.value));
const maxDate = today(getLocalTimeZone()); // no future days
function isUnavailable(d: DateValue) {
  return !days.value.has(d.toString());
}
function pickDate(value: DateValue | undefined) {
  if (!value) return;
  calOpen.value = false;
  goto(value.toString());
}

// inline-edit state
const editingMeal = ref<number | null>(null);
const mealDraft = ref("");
const noteDraft = ref("");

async function reload() {
  day.value = await api.getDay(date.value);
  cancelEdit();
  expandAll();
}
// The set of days-with-data only changes when entries are added/removed, so load
// it once and refresh after mutations — not on every date change.
async function loadDays() {
  try {
    days.value = new Set(await api.getDays());
  } catch {
    /* non-fatal — calendar just shows no enabled days */
  }
}
watch(date, reload, { immediate: true });
loadDays();

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

async function delEntry(id: number) {
  if (confirm(t("diary.confirmDeleteEntry"))) {
    day.value = await api.deleteEntry(date.value, id);
    loadDays();
  }
}
async function delMeal(id: number) {
  if (confirm(t("diary.confirmDeleteMeal"))) {
    day.value = await api.deleteMeal(date.value, id);
    loadDays();
  }
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
    <div class="flex items-center justify-between gap-2">
      <!-- jump to today; disabled when today is already selected -->
      <UButton size="sm" color="neutral" variant="soft" :label="t('common.today')" :disabled="isToday" @click="goto(todayISO())" />

      <!-- prev / next arrows hugging the date -->
      <div class="flex items-center gap-1">
        <UButton size="xs" color="neutral" variant="soft" label="←" @click="goto(shiftDate(date, -1))" />
        <span class="px-1 text-base font-semibold tabular-nums sm:text-lg">{{ date }} <span class="text-gray-400">({{ weekday(date) }})</span></span>
        <UButton size="xs" color="neutral" variant="soft" label="→" :disabled="isToday" @click="goto(shiftDate(date, 1))" />
      </div>

      <!-- calendar; only days that have data are selectable -->
      <UPopover v-model:open="calOpen">
        <UButton size="sm" color="neutral" variant="soft" label="📅" :aria-label="t('diary.openCalendar')" />
        <template #content>
          <UCalendar
            :model-value="calValue"
            :max-value="maxDate"
            :is-date-unavailable="isUnavailable"
            class="p-2"
            @update:model-value="pickDate"
          />
        </template>
      </UPopover>
    </div>

    <DaySummary :day="day" />

    <div class="flex items-center justify-between">
      <h2 class="text-base font-medium sm:text-lg">{{ t("diary.title") }}</h2>
      <div class="flex items-center gap-2">
        <UButton
          v-if="day.meals.length"
          size="xs"
          color="neutral"
          variant="soft"
          ::label="allOpen ? t('diary.collapseAll') : t('diary.expandAll')"
          @click="toggleAll"
        />
        <UButton size="xs" :label="t('diary.addEntry')" :to="{ path: '/log', query: { date } }" />
      </div>
    </div>

    <UAccordion
      v-if="day.meals.length"
      type="multiple"
      v-model="open"
      :items="mealItems"
      :ui="{ item: 'mb-2 rounded-lg border last:border-b border-gray-200 px-3 dark:border-gray-700' }"
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
        <!-- The #content slot bypasses Nuxt UI's `body` padding, so add our own
             bottom padding to keep each item's border visible below the table. -->
        <div class="pb-3">
          <div v-if="editingMeal === item.meal.id" class="mb-3 space-y-2">
            <div class="flex items-center gap-2">
              <UInput
                v-model="mealDraft"
                size="sm"
                class="flex-1"
                :placeholder="t('diary.mealNamePlaceholder')"
                @keydown.enter.prevent="saveMeal(item.meal.id)"
                @keydown.esc="cancelEdit"
              />
              <UButton size="xs" :label="t('common.save')" @click="saveMeal(item.meal.id)" />
              <UButton size="xs" color="neutral" variant="ghost" :label="t('common.cancel')" @click="cancelEdit" />
            </div>
            <UTextarea v-model="noteDraft" :rows="2" autoresize class="w-full" :placeholder="t('diary.notePlaceholder')" />
          </div>
          <MealTable
            editable
            :meal="item.meal"
            :show-note="editingMeal !== item.meal.id"
            @update-entry="onUpdateEntry"
            @delete-entry="delEntry"
          />
        </div>
      </template>
    </UAccordion>

    <div v-else class="rounded-lg border border-dashed border-gray-200 p-8 text-center dark:border-gray-800">
      <p class="text-sm text-gray-500">{{ t("diary.noMeals") }}</p>
      <UButton class="mt-3" size="sm" :label="t('diary.addMeal')" :to="{ path: '/log', query: { date } }" />
    </div>
  </div>

  <div v-else class="p-8 text-center text-gray-400">{{ t("common.loading") }}</div>
</template>
