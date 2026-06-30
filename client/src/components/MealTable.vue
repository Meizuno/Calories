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

const showNote = () => props.showNote !== false;
const k = (n: number) => Math.round(n);
const g = (n: number) => Math.round(n * 10) / 10;
</script>

<template>
  <div>
    <table class="w-full text-sm sm:text-base">
      <thead class="text-xs sm:text-sm">
        <tr>
          <th class="py-1 text-left font-normal text-gray-500">Položka</th>
          <th class="py-1 text-right font-normal text-gray-500">Množství</th>
          <th class="py-1 text-right font-medium text-gray-600 dark:text-gray-300">kcal</th>
          <th class="py-1 text-right font-medium text-sky-600 dark:text-sky-400">Sach.</th>
          <th class="py-1 text-right font-medium text-emerald-600 dark:text-emerald-400">Bílk.</th>
          <th class="py-1 text-right font-medium text-amber-600 dark:text-amber-400">Tuky</th>
          <th v-if="editable"></th>
        </tr>
      </thead>
      <tbody>
        <template v-for="e in meal.entries" :key="e.id">
          <tr v-if="editable && editingId === e.id" class="border-t border-gray-100 dark:border-gray-800">
            <td class="py-1.5 pr-2"><UInput v-model="draft.name" size="xs" /></td>
            <td class="py-1.5">
              <div class="flex justify-end gap-1">
                <UInput v-model="draft.quantity" type="number" step="any" min="0" size="xs" class="w-16" />
                <USelect v-model="draft.unit" :items="units" size="xs" class="w-20" />
              </div>
            </td>
            <td class="py-1.5 pl-2"><UInput v-model="draft.kcal" type="number" step="any" min="0" size="xs" class="w-16" /></td>
            <td class="py-1.5 pl-2"><UInput v-model="draft.carb" type="number" step="any" min="0" size="xs" class="w-16" /></td>
            <td class="py-1.5 pl-2"><UInput v-model="draft.protein" type="number" step="any" min="0" size="xs" class="w-16" /></td>
            <td class="py-1.5 pl-2"><UInput v-model="draft.fat" type="number" step="any" min="0" size="xs" class="w-16" /></td>
            <td class="py-1.5 text-right whitespace-nowrap">
              <UButton size="xs" color="primary" variant="ghost" label="✓" @click="save(e.id)" />
              <UButton size="xs" color="neutral" variant="ghost" label="✕" @click="cancel" />
            </td>
          </tr>
          <tr v-else class="border-t border-gray-100 hover:bg-gray-50/70 dark:border-gray-800 dark:hover:bg-gray-900/40">
            <td class="py-1.5">{{ e.name }}</td>
            <td class="py-1.5 whitespace-nowrap text-right tabular-nums text-gray-500">{{ g(e.quantity) }} {{ e.unit }}</td>
            <td class="py-1.5 text-right font-medium tabular-nums">{{ k(e.kcal) }}</td>
            <td class="py-1.5 text-right tabular-nums text-sky-600 dark:text-sky-400">{{ g(e.carb) }}</td>
            <td class="py-1.5 text-right tabular-nums text-emerald-600 dark:text-emerald-400">{{ g(e.protein) }}</td>
            <td class="py-1.5 text-right tabular-nums text-amber-600 dark:text-amber-400">{{ g(e.fat) }}</td>
            <td v-if="editable" class="py-1.5 text-right whitespace-nowrap">
              <UButton size="xs" color="neutral" variant="ghost" label="✎" @click="startEdit(e)" />
              <UButton size="xs" color="error" variant="ghost" label="✕" @click="emit('delete-entry', e.id)" />
            </td>
          </tr>
        </template>
        <tr v-if="!meal.entries.length"><td :colspan="editable ? 7 : 6" class="py-3 text-center text-gray-400">—</td></tr>
      </tbody>
      <tfoot v-if="meal.entries.length">
        <tr class="border-t border-gray-200 font-medium dark:border-gray-700">
          <td class="py-1.5">Celkem</td>
          <td></td>
          <td class="py-1.5 text-right tabular-nums">{{ k(meal.total.kcal) }}</td>
          <td class="py-1.5 text-right tabular-nums text-sky-600 dark:text-sky-400">{{ g(meal.total.carb) }}</td>
          <td class="py-1.5 text-right tabular-nums text-emerald-600 dark:text-emerald-400">{{ g(meal.total.protein) }}</td>
          <td class="py-1.5 text-right tabular-nums text-amber-600 dark:text-amber-400">{{ g(meal.total.fat) }}</td>
          <td v-if="editable"></td>
        </tr>
      </tfoot>
    </table>
    <p
      v-if="meal.note && showNote()"
      class="mt-3 whitespace-pre-line rounded-md border-l-2 border-amber-300 bg-amber-50/60 px-3 py-2 text-xs italic text-gray-600 dark:border-amber-700/60 dark:bg-amber-950/30 dark:text-gray-300 sm:text-sm"
    >
      {{ meal.note }}
    </p>
  </div>
</template>
