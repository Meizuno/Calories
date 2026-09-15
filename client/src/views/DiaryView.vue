<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { type DateValue, getLocalTimeZone, parseDate, today } from "@internationalized/date";
import { api } from "../lib/api";
import type { Day } from "../lib/types";
import DaySummary from "../components/DaySummary.vue";
import WeekTrend from "../components/WeekTrend.vue";
import { useMediaQuery } from "../composables/useMediaQuery";
import { useUiSize } from "../composables/useUiSize";
import MealTable from "../components/MealTable.vue";
import { t, weekdayShort } from "../lib/i18n";

const route = useRoute();
const router = useRouter();
// xl is the point at which the summary becomes a sticky column with room to
// spare beneath it; below that the trend has nowhere useful to live.
const isWide = useMediaQuery("(min-width: 1280px)");
const { control, compact, inline } = useUiSize();

const pad = (n: number) => String(n).padStart(2, "0");
// Local calendar date — not UTC, so "dnes" matches the user's actual day.
const todayISO = () => {
  const d = new Date();
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
};
const date = computed(() => {
  const raw = route.query.date;
  const iso = typeof raw === "string" && /^\d{4}-\d{2}-\d{2}$/.test(raw) ? raw : todayISO();
  // goto() already refuses to step forward past today; this applies the same
  // rule to a date arriving from outside — a link, a bookmark, a chart click.
  return iso > todayISO() ? todayISO() : iso;
});
const isToday = computed(() => date.value === todayISO());

function shiftDate(d: string, n: number) {
  const t = new Date(d + "T00:00:00Z");
  t.setUTCDate(t.getUTCDate() + n);
  return t.toISOString().slice(0, 10);
}
const weekday = (d: string) => weekdayShort(d);
const k = (n: number) => Math.round(n);

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
  copiedTo.value = {};
  copyFailed.value = null;
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
// ── copying a meal forward ───────────────────────────────────────────────────
// Eating the same breakfast most mornings should not mean retyping it. Only
// offered while looking at some other day: on today there is nothing to copy
// forward to.
//
// What happened is reported next to the button that was pressed, not at the top
// of the page — the meals are a long list, and a confirmation above the fold is
// a confirmation you never see. Keyed by source meal so each row speaks only
// for itself.
const copying = ref<number | null>(null);
// source meal id → the meal it created on today, which is what undo removes and
// what stops the same meal being copied twice by an impatient second click.
const copiedTo = ref<Record<number, number>>({});
const copyFailed = ref<number | null>(null);

async function copyToToday(meal: { id: number; name: string }) {
  if (copying.value !== null || copiedTo.value[meal.id]) return;
  copying.value = meal.id;
  copyFailed.value = null;
  try {
    // The copy is appended to the end of the target day, so it is the last meal
    // in the day we get back. Holding its id is what makes undo possible.
    const day = await api.copyMeal(meal.id, todayISO());
    const created = day.meals[day.meals.length - 1];
    if (created) copiedTo.value[meal.id] = created.id;
    // Today has data now, so the calendar should let you back into it.
    loadDays();
  } catch {
    copyFailed.value = meal.id;
  } finally {
    copying.value = null;
  }
}

// The action is a single control that flips: copy, then take it back. A second
// click cannot produce a second copy by accident, which matters now that it
// sits next to edit and delete.
function toggleCopy(meal: { id: number; name: string }) {
  if (copiedTo.value[meal.id]) return undoCopy(meal.id);
  return copyToToday(meal);
}

// Undo removes the meal the copy created, rather than asking someone to travel
// to today and delete it by hand. No confirmation: it only ever takes back
// something added seconds ago.
async function undoCopy(sourceID: number) {
  const created = copiedTo.value[sourceID];
  if (created === undefined) return;
  delete copiedTo.value[sourceID];
  try {
    await api.deleteMeal(todayISO(), created);
    loadDays();
  } catch {
    copyFailed.value = sourceID;
  }
}

// Entry editing lives in <MealTable>; it emits the new values, we persist them.
async function onUpdateEntry(
  id: number,
  body: { name: string; quantity: number; unit: string; kcal: number; carb: number; protein: number; fat: number },
) {
  day.value = await api.updateEntry(date.value, id, body);
}

</script>

<template>
  <div v-if="day" class="space-y-5">
    <!-- From xl up the day summary becomes a column of its own and stays put
         while the meals scroll, so the ring and macro bars remain visible
         instead of disappearing off the top. Below xl it stacks exactly as
         before: summary first, then meals. -->
    <div class="grid gap-5 xl:grid-cols-[minmax(0,1fr)_21rem] xl:items-start">
      <aside class="order-1 xl:order-2 xl:sticky xl:top-20">
        <DaySummary :day="day" sidebar>
          <!-- Day selection sits with the day it describes, so in the sticky
               column it stays reachable while the meals scroll. Wraps to a
               second line in the narrow sidebar. -->
          <template #header>
            <div class="flex flex-wrap items-center justify-between gap-x-2 gap-y-1">
              <div class="flex min-w-0 items-center gap-0.5">
                <UButton
                  :size="inline"
                  color="neutral"
                  variant="ghost"
                  icon="i-ui-prev"
                  :aria-label="t('common.previous')"
                  @click="goto(shiftDate(date, -1))"
                />
                <span class="truncate px-1 text-base font-semibold tabular-nums">
                  {{ date }} <span class="font-normal text-gray-400">({{ weekday(date) }})</span>
                </span>
                <UButton
                  :size="inline"
                  color="neutral"
                  variant="ghost"
                  icon="i-ui-next"
                  :disabled="isToday"
                  :aria-label="t('common.next')"
                  @click="goto(shiftDate(date, 1))"
                />
              </div>

              <div class="ml-auto flex items-center gap-1">
                <UButton
                  :size="inline"
                  color="neutral"
                  variant="soft"
                  :label="t('common.today')"
                  :disabled="isToday"
                  @click="goto(todayISO())"
                />
                <UPopover v-model:open="calOpen">
                  <UButton
                    :size="inline"
                    color="neutral"
                    variant="soft"
                    icon="i-ui-calendar"
                    :aria-label="t('diary.openCalendar')"
                  />
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
            </div>
          </template>
        </DaySummary>

        <WeekTrend v-if="isWide" :date="date" class="mt-5 block" />
      </aside>

      <div class="order-2 space-y-4 xl:order-1">
    <div class="flex items-center justify-between">
      <h2 class="text-base font-medium sm:text-lg">{{ t("diary.title") }}</h2>
      <div class="flex items-center gap-2">
        <UButton
          v-if="day.meals.length"
          :size="inline"
          color="neutral"
          variant="soft"
          :label="allOpen ? t('diary.collapseAll') : t('diary.expandAll')"
          @click="toggleAll"
        />
        <UButton :size="inline" :label="t('diary.addEntry')" :to="{ path: '/log', query: { date } }" />
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
        <div class="flex grow items-center justify-between gap-2 pr-3 sm:gap-3">
          <span class="min-w-0 truncate font-medium">{{ item.meal.name }}</span>
          <span class="flex shrink-0 items-center gap-2 sm:gap-3">
            <span class="tabular-nums text-sm font-normal text-gray-500">{{ k(item.meal.total.kcal) }} kcal</span>
            <!-- Repeating a meal is something you do TO the meal, so it belongs
                 with rename and delete rather than under the food. One control
                 that flips: copy, then click again to take it back. -->
            <span
              v-if="!isToday"
              role="button"
              tabindex="0"
              class="inline-flex cursor-pointer items-center gap-1 text-sm font-normal transition-colors"
              :class="[
                copying === item.meal.id ? 'pointer-events-none opacity-50' : '',
                copyFailed === item.meal.id
                  ? 'text-red-500'
                  : copiedTo[item.meal.id]
                    ? 'text-gray-400 hover:text-gray-600 dark:hover:text-gray-300'
                    : 'text-emerald-600 hover:text-emerald-700 dark:text-emerald-400 dark:hover:text-emerald-300',
              ]"
              :title="copiedTo[item.meal.id] ? t('diary.undoCopyHint') : t('diary.copyToToday')"
              @click.stop="toggleCopy(item.meal)"
              @keydown.enter.stop.prevent="toggleCopy(item.meal)"
            >
              <UIcon :name="copiedTo[item.meal.id] ? 'i-ui-check' : 'i-ui-copy'" class="size-3.5 shrink-0" />
              <span class="hidden sm:inline">{{
                copyFailed === item.meal.id
                  ? t("diary.copyFailed")
                  : copiedTo[item.meal.id]
                    ? t("diary.copied")
                    : t("common.copy")
              }}</span>
            </span>
            <span
              role="button"
              tabindex="0"
              class="cursor-pointer text-sm font-normal text-sky-500 hover:text-sky-600"
              @click.stop="startEditMeal(item.meal)"
              @keydown.enter.stop.prevent="startEditMeal(item.meal)"
            >{{ t("common.edit") }}</span>
            <span
              role="button"
              tabindex="0"
              class="cursor-pointer text-sm font-normal text-red-500 hover:text-red-600"
              @click.stop="delMeal(item.meal.id)"
              @keydown.enter.stop.prevent="delMeal(item.meal.id)"
            >{{ t("common.delete") }}</span>
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
                :size="compact"
                class="flex-1"
                :placeholder="t('diary.mealNamePlaceholder')"
                @keydown.enter.prevent="saveMeal(item.meal.id)"
                @keydown.esc="cancelEdit"
              />
              <UButton :size="inline" :label="t('common.save')" @click="saveMeal(item.meal.id)" />
              <UButton :size="inline" color="neutral" variant="ghost" :label="t('common.cancel')" @click="cancelEdit" />
            </div>
            <UTextarea v-model="noteDraft" :rows="2" autoresize :size="compact" class="w-full" :placeholder="t('diary.notePlaceholder')" />
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
          <UButton class="mt-3" :size="control" :label="t('diary.addMeal')" :to="{ path: '/log', query: { date } }" />
        </div>
      </div>
    </div>
  </div>

  <div v-else class="p-8 text-center text-gray-400">{{ t("common.loading") }}</div>
</template>
