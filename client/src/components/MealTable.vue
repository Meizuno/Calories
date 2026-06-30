<script setup lang="ts">
import { ref } from "vue";
import type { Meal, Entry } from "../lib/types";

const props = defineProps<{ meal: Meal; editable?: boolean; showNote?: boolean }>();
const emit = defineEmits<{
  (
    e: "update-entry",
    id: number,
    body: { name: string; quantity: number; unit: string; kcal: number; carb: number; protein: number; fat: number },
  ): void;
  (e: "delete-entry", id: number): void;
}>();

const units = [
  { label: "g", value: "g" },
  { label: "ml", value: "ml" },
  { label: "ks", value: "ks" },
  { label: "porce", value: "porce" },
];

const editingId = ref<number | null>(null);
const draft = ref({ name: "", quantity: "", unit: "g", kcal: "0", carb: "0", protein: "0", fat: "0" });
const numVal = (s: string) => Math.max(0, parseFloat(s) || 0);

function startEdit(e: Entry) {
  editingId.value = e.id;
  draft.value = {
    name: e.name,
    quantity: String(e.quantity),
    unit: e.unit,
    kcal: String(e.kcal),
    carb: String(e.carb),
    protein: String(e.protein),
    fat: String(e.fat),
  };
}
function cancel() {
  editingId.value = null;
}
function save(id: number) {
  const q = parseFloat(draft.value.quantity);
  if (!draft.value.name.trim() || !(q > 0)) return;
  emit("update-entry", id, {
    name: draft.value.name.trim(),
    quantity: q,
    unit: draft.value.unit || "g",
    kcal: numVal(draft.value.kcal),
    carb: numVal(draft.value.carb),
    protein: numVal(draft.value.protein),
    fat: numVal(draft.value.fat),
  });
  editingId.value = null;
}
const editing = (id: number) => props.editable && editingId.value === id;

const showNote = () => props.showNote !== false;
const k = (n: number) => Math.round(n);
const g = (n: number) => Math.round(n * 10) / 10;

// donut showing the entry's macro split (sky carbs / emerald protein / amber fat)
function ringStyle(e: { carb: number; protein: number; fat: number }) {
  const t = e.carb + e.protein + e.fat;
  if (!t) return { background: "#3f4654" };
  const c = (e.carb / t) * 100;
  const cp = c + (e.protein / t) * 100;
  return { background: `conic-gradient(#38bdf8 0 ${c}%, #34d399 ${c}% ${cp}%, #fbbf24 ${cp}% 100%)` };
}
</script>

<template>
  <div class="space-y-2">
    <p v-if="!meal.entries.length" class="py-3 text-center text-gray-400">—</p>

    <div
      v-for="e in meal.entries"
      :key="e.id"
      class="rounded-xl border border-gray-200 bg-gray-50 px-3 py-2.5 dark:border-gray-800 dark:bg-gray-900"
    >
      <!-- edit form -->
      <div v-if="editing(e.id)" class="space-y-2">
        <UInput v-model="draft.name" size="sm" class="w-full" placeholder="Název" />
        <div class="grid grid-cols-2 gap-2 sm:grid-cols-3">
          <label class="text-xs text-gray-500">
            Množství
            <div class="mt-0.5 flex gap-1">
              <UInput v-model="draft.quantity" type="number" step="any" min="0" size="sm" class="min-w-0 flex-1" />
              <USelect v-model="draft.unit" :items="units" size="sm" class="w-20 min-w-0" />
            </div>
          </label>
          <label class="text-xs text-gray-500">kcal<UInput v-model="draft.kcal" type="number" step="any" min="0" size="sm" class="mt-0.5 w-full" /></label>
          <label class="text-xs text-sky-400">Sach.<UInput v-model="draft.carb" type="number" step="any" min="0" size="sm" class="mt-0.5 w-full" /></label>
          <label class="text-xs text-emerald-400">Bílk.<UInput v-model="draft.protein" type="number" step="any" min="0" size="sm" class="mt-0.5 w-full" /></label>
          <label class="text-xs text-amber-400">Tuky<UInput v-model="draft.fat" type="number" step="any" min="0" size="sm" class="mt-0.5 w-full" /></label>
        </div>
        <div class="flex justify-end gap-2">
          <UButton size="xs" label="Uložit" @click="save(e.id)" />
          <UButton size="xs" color="neutral" variant="ghost" label="Zrušit" @click="cancel" />
        </div>
      </div>

      <!-- display -->
      <template v-else>
        <div class="flex items-center gap-3">
          <div class="relative size-9 shrink-0 rounded-full" :style="ringStyle(e)">
            <div class="absolute inset-1.5 rounded-full bg-gray-50 dark:bg-gray-900" />
          </div>
          <div class="min-w-0 flex-1">
            <div class="font-medium leading-tight wrap-break-word">{{ e.name }}</div>
            <div class="text-xs text-gray-500">{{ g(e.quantity) }} {{ e.unit }}</div>
          </div>
          <div class="shrink-0 text-right leading-none">
            <div class="text-base font-bold tabular-nums">{{ k(e.kcal) }}</div>
            <div class="text-[11px] text-gray-500">kcal</div>
          </div>
        </div>
        <div class="mt-2 flex items-center justify-between gap-2 border-t border-gray-200 pt-2 dark:border-gray-800">
          <div class="flex flex-wrap gap-x-4 gap-y-1 text-sm tabular-nums sm:text-base">
            <span><span class="text-gray-500">Sach</span> <b class="text-sky-600 dark:text-sky-400">{{ g(e.carb) }} g</b></span>
            <span><span class="text-gray-500">Bílk</span> <b class="text-emerald-600 dark:text-emerald-400">{{ g(e.protein) }} g</b></span>
            <span><span class="text-gray-500">Tuky</span> <b class="text-amber-600 dark:text-amber-400">{{ g(e.fat) }} g</b></span>
          </div>
          <div v-if="editable" class="-mr-1 flex shrink-0">
            <UButton size="xs" color="neutral" variant="ghost" label="✎" class="size-7 justify-center p-0" @click="startEdit(e)" />
            <UButton size="xs" color="error" variant="ghost" label="✕" class="size-7 justify-center p-0" @click="emit('delete-entry', e.id)" />
          </div>
        </div>
      </template>
    </div>

    <!-- meal total -->
    <div v-if="meal.entries.length" class="flex flex-wrap items-center gap-x-4 gap-y-1 px-3 pt-1.5">
      <span class="font-semibold">Celkem</span>
      <div class="flex flex-wrap gap-x-4 gap-y-1 text-sm tabular-nums sm:text-base">
        <span><span class="text-gray-500">Sach</span> <b class="text-sky-600 dark:text-sky-400">{{ g(meal.total.carb) }} g</b></span>
        <span><span class="text-gray-500">Bílk</span> <b class="text-emerald-600 dark:text-emerald-400">{{ g(meal.total.protein) }} g</b></span>
        <span><span class="text-gray-500">Tuky</span> <b class="text-amber-600 dark:text-amber-400">{{ g(meal.total.fat) }} g</b></span>
      </div>
      <span class="ml-auto"><b class="text-base tabular-nums">{{ k(meal.total.kcal) }}</b> <span class="text-[11px] text-gray-500">kcal</span></span>
    </div>

    <p
      v-if="meal.note && showNote()"
      class="mt-2 whitespace-pre-line rounded-md border-l-2 border-amber-300 bg-amber-50/60 px-3 py-2 text-xs italic text-gray-600 dark:border-amber-700/60 dark:bg-amber-950/30 dark:text-gray-300 sm:text-sm"
    >
      {{ meal.note }}
    </p>
  </div>
</template>
