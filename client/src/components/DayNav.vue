<script setup lang="ts">
import { computed, ref } from "vue";
import { type DateValue, getLocalTimeZone, parseDate, today } from "@internationalized/date";
import { t, weekdayShort } from "../lib/i18n";
import { useUiSize } from "../composables/useUiSize";

// Day selection: step a day at a time, jump to today, or pick from a calendar
// that only offers days with something in them.
//
// It lives in DaySummary's header slot, with the day it describes, so in the
// diary's sticky column it stays reachable while the meals scroll. Extracted
// because the shared profile needs exactly the same control — it used to have a
// pair of "←" / "→" buttons and nothing else, which is how the two views drifted
// apart in the first place.
const props = withDefaults(
  defineProps<{
    date: string;
    /** Days that have data. Empty means "do not restrict the calendar". */
    days?: Set<string>;
  }>(),
  { days: undefined },
);
const emit = defineEmits<{ (e: "update:date", iso: string): void }>();

const { inline } = useUiSize();

const pad = (n: number) => String(n).padStart(2, "0");
// Local calendar date, not UTC, so "today" matches the user's actual day.
const todayISO = () => {
  const d = new Date();
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
};
const isToday = computed(() => props.date === todayISO());
const weekday = (d: string) => weekdayShort(d);

function shift(iso: string, n: number) {
  const d = new Date(`${iso}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + n);
  return d.toISOString().slice(0, 10);
}
function go(iso: string) {
  if (iso > todayISO()) return; // there is no future to look at
  emit("update:date", iso);
}

const calOpen = ref(false);
const calValue = computed(() => parseDate(props.date));
const maxDate = today(getLocalTimeZone());
const isUnavailable = (d: DateValue) => !!props.days && !props.days.has(d.toString());
function pickDate(value: DateValue | undefined) {
  if (!value) return;
  calOpen.value = false;
  go(value.toString());
}
</script>

<template>
  <!-- One line, always. It used to wrap, and in the diary's 21rem sidebar the
       date and its arrows came to within a few pixels of the full width, so
       Today and the calendar dropped onto a second row — a two-line control
       above a card that is otherwise tightly packed.
       @container rather than a breakpoint: this sits in a wide header on one
       page and a narrow column on another, both on the same large screen, so
       the viewport says nothing useful about how much room it actually has.
       The date takes the full size where it fits and one step down where it
       does not, which is the ~20px that made the difference. -->
  <div class="@container flex items-center gap-x-2">
    <div class="flex min-w-0 items-center gap-0.5">
      <UButton
        :size="inline"
        color="neutral"
        variant="ghost"
        icon="i-ui-prev"
        :aria-label="t('common.previous')"
        @click="go(shift(date, -1))"
      />
      <!-- truncate is the floor, not the plan: at any width the two groups fit
           by the numbers, but a longer locale or a bigger root font should cost
           the tail of a date rather than the layout. -->
      <span class="truncate px-1 text-sm font-semibold tabular-nums @xs:text-base">
        {{ date }} <span class="font-normal text-gray-400">({{ weekday(date) }})</span>
      </span>
      <UButton
        :size="inline"
        color="neutral"
        variant="ghost"
        icon="i-ui-next"
        :disabled="isToday"
        :aria-label="t('common.next')"
        @click="go(shift(date, 1))"
      />
    </div>

    <div class="ml-auto flex shrink-0 items-center gap-1">
      <UButton
        :size="inline"
        color="neutral"
        variant="soft"
        :label="t('common.today')"
        :disabled="isToday"
        @click="go(todayISO())"
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
