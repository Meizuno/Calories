<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { api } from "../lib/api";
import type { Day, Profile } from "../lib/types";
import DaySummary from "../components/DaySummary.vue";
import DayNav from "../components/DayNav.vue";
import WeekTrend from "../components/WeekTrend.vue";
import MealTable from "../components/MealTable.vue";
import StatsPanel from "../components/StatsPanel.vue";
import { useMediaQuery } from "../composables/useMediaQuery";
import { useUiSize } from "../composables/useUiSize";
import { t } from "../lib/i18n";

// The public, read-only twin of the diary. It shows the same things in the same
// places — the ring and macro bars in a sticky column, the day stepper with the
// day it describes, the meals as an accordion — because a visitor should not
// have to learn a second layout. The only difference is that nothing here can
// be changed: no add, no rename, no delete, no copy.
const route = useRoute();
const uuid = computed(() => route.params.uuid as string);
const isWide = useMediaQuery("(min-width: 1280px)");
const { inline } = useUiSize();

const profile = ref<Profile | null>(null);
const day = ref<Day | null>(null);
const error = ref(false);
const open = ref<string[]>([]);
// Which half is showing: the per-day meals or the period statistics.
const tab = ref<"diary" | "stats">("diary");

const pad = (n: number) => String(n).padStart(2, "0");
const todayISO = () => {
  const d = new Date();
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
};
const date = ref(todayISO());
const k = (n: number) => Math.round(n);

function goto(d: string) {
  if (d > todayISO()) return; // no future days
  date.value = d;
}

// Drilling into a day — from the chart or the trend — switches to this
// profile's own diary for that date. There is no app-wide diary to send an
// anonymous visitor to.
function openDay(d: string) {
  goto(d);
  tab.value = "diary";
}

const mealItems = computed(() => (day.value?.meals ?? []).map((m) => ({ label: m.name, value: String(m.id), meal: m })));
const allOpen = computed(() => (day.value?.meals.length ?? 0) > 0 && open.value.length === (day.value?.meals.length ?? 0));
const expandAll = () => (open.value = (day.value?.meals ?? []).map((m) => String(m.id)));
const toggleAll = () => (allOpen.value ? (open.value = []) : expandAll());

// The profile and its list of days belong to the person, not to the day being
// looked at, so they load once per uuid. Refetching them on every step put a
// second round trip in front of the day itself, which delayed the summary
// updating — and with it the moment its ring and bars animate.
const days = ref<Set<string>>(new Set());
watch(
  uuid,
  async () => {
    error.value = false;
    try {
      profile.value = await api.getShared(uuid.value);
    } catch {
      error.value = true;
      return;
    }
    try {
      days.value = new Set(await api.getSharedDays(uuid.value));
    } catch {
      /* non-fatal — the calendar just offers nothing */
    }
  },
  { immediate: true },
);

// The day is the only thing a step changes. Replacing it (rather than blanking
// it first) keeps DaySummary mounted, which is what lets its reveal re-run and
// the figures tween from the previous day's values instead of appearing.
async function loadDay() {
  try {
    day.value = await api.getSharedDay(uuid.value, date.value);
    expandAll();
  } catch {
    error.value = true;
  }
}
watch([uuid, date], loadDay, { immediate: true });

// Both panels read this profile's public endpoints rather than the viewer's own.
const fetchStats = (from: string, to: string) => api.getSharedStats(uuid.value, from, to);
</script>

<template>
  <div v-if="error" class="p-8 text-center text-gray-400">{{ t("shared.notFound") }}</div>

  <div v-else-if="profile" class="space-y-5">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h1 class="min-w-0 truncate text-xl font-semibold sm:text-2xl">
        {{ profile.name || t("shared.title") }}
        <span class="text-sm font-normal text-gray-400">{{ t("shared.readOnly") }}</span>
      </h1>

      <!-- The same segmented control the statistics toolbar uses, rather than
           two buttons that happen to sit together. -->
      <div class="flex items-center gap-0.5 rounded-xl bg-gray-100/70 p-0.5 dark:bg-gray-900/70">
        <button
          v-for="v in (['diary', 'stats'] as const)"
          :key="v"
          type="button"
          class="rounded-lg px-2.5 py-1.5 text-sm font-medium outline-none transition focus-visible:ring-2 focus-visible:ring-emerald-500/60 sm:px-3 sm:py-2 sm:text-base"
          :class="tab === v
            ? 'bg-white text-gray-900 shadow-sm dark:bg-gray-800 dark:text-gray-100'
            : 'text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-100'"
          @click="tab = v"
        >{{ v === 'diary' ? t("shared.diary") : t("shared.stats") }}</button>
      </div>
    </div>

    <!-- Diary: the private layout exactly — summary in a sticky column from xl
         up, meals beside it, the trend underneath where there is room. -->
    <template v-if="tab === 'diary'">
      <div v-if="day" class="grid gap-5 xl:grid-cols-[minmax(0,1fr)_21rem] xl:items-start">
        <aside class="order-1 xl:order-2 xl:sticky xl:top-20">
          <DaySummary :day="day" sidebar>
            <template #header>
              <DayNav :date="date" :days="days" @update:date="goto" />
            </template>
          </DaySummary>

          <WeekTrend
            v-if="isWide"
            :date="date"
            :fetch-stats="fetchStats"
            class="mt-5 block"
            @pick-day="goto"
          />
        </aside>

        <div class="order-2 space-y-4 xl:order-1">
          <div class="flex items-center justify-between">
            <h2 class="text-base font-medium sm:text-lg">{{ t("diary.title") }}</h2>
            <UButton
              v-if="day.meals.length"
              :size="inline"
              color="neutral"
              variant="soft"
              :label="allOpen ? t('diary.collapseAll') : t('diary.expandAll')"
              @click="toggleAll"
            />
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
                <span class="shrink-0 tabular-nums text-sm font-normal text-gray-500">
                  {{ k(item.meal.total.kcal) }} kcal
                </span>
              </div>
            </template>
            <template #content="{ item }">
              <!-- The #content slot bypasses Nuxt UI's body padding, so add our
                   own below the table to keep the item's border clear. -->
              <div class="pb-3">
                <MealTable :meal="item.meal" />
              </div>
            </template>
          </UAccordion>

          <div v-else class="rounded-lg border border-dashed border-gray-200 p-8 text-center dark:border-gray-800">
            <p class="text-sm text-gray-500">{{ t("diary.noMeals") }}</p>
          </div>
        </div>
      </div>
      <div v-else class="p-8 text-center text-gray-400">{{ t("common.loading") }}</div>
    </template>

    <!-- Statistics: keyed by uuid so switching profiles remounts the panel. -->
    <StatsPanel
      v-else
      :key="uuid"
      :fetch-stats="fetchStats"
      :fetch-days="() => api.getSharedDays(uuid)"
      @pick-day="openDay"
    />
  </div>

  <div v-else class="p-8 text-center text-gray-400">{{ t("common.loading") }}</div>
</template>
