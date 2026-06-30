<script setup lang="ts">
import { computed, ref } from "vue";
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
const hasRows = computed(() => props.meal.entries.length > 0);

// Identical column geometry (table-fixed + fixed widths) so every meal table
// lines up the same; macro columns carry their own accent colour.
const columns = computed(() => {
  const foot = hasRows.value ? { footer: "" } : {};
  const cols: Record<string, unknown>[] = [
    { accessorKey: "name", header: "Položka", ...foot, meta: { class: { th: "text-left", td: "text-default whitespace-normal wrap-break-word" } } },
    { accessorKey: "quantity", header: "Množství", ...foot, meta: { class: { th: "w-16 text-right sm:w-24", td: "text-right tabular-nums text-muted" } } },
    { accessorKey: "kcal", header: "kcal", ...foot, meta: { class: { th: "w-12 text-right sm:w-16", td: "text-right font-medium tabular-nums text-highlighted" } } },
    { accessorKey: "carb", header: "Sach.", ...foot, meta: { class: { th: "w-12 text-right text-sky-400 sm:w-14", td: "text-right tabular-nums text-sky-400" } } },
    { accessorKey: "protein", header: "Bílk.", ...foot, meta: { class: { th: "w-12 text-right text-emerald-400 sm:w-14", td: "text-right tabular-nums text-emerald-400" } } },
    { accessorKey: "fat", header: "Tuky", ...foot, meta: { class: { th: "w-12 text-right text-amber-400 sm:w-14", td: "text-right tabular-nums text-amber-400" } } },
  ];
  if (props.editable) {
    cols.push({ id: "actions", header: "", ...foot, meta: { class: { th: "w-16 sm:w-20", td: "text-right" } } });
  }
  return cols;
});

const tableUi = {
  base: "min-w-full table-fixed",
  thead: "[&>tr]:after:hidden",
  th: "px-2 py-1 text-xs font-medium sm:text-sm",
  td: "px-2 py-1.5 align-top text-sm whitespace-normal sm:text-base",
  tbody: "divide-y divide-gray-100 dark:divide-gray-800",
  tfoot: "border-t border-gray-200 font-medium dark:border-gray-700",
};
</script>

<template>
  <div>
    <UTable :data="meal.entries" :columns="columns" :ui="tableUi">
      <template #empty><span class="text-gray-400">—</span></template>

      <!-- name -->
      <template #name-cell="{ row }">
        <UInput v-if="editing(row.original.id)" v-model="draft.name" size="xs" class="w-full" />
        <template v-else>{{ row.original.name }}</template>
      </template>
      <template #name-footer>Celkem</template>

      <!-- quantity + unit -->
      <template #quantity-cell="{ row }">
        <div v-if="editing(row.original.id)" class="flex gap-1">
          <UInput v-model="draft.quantity" type="number" step="any" min="0" size="xs" class="min-w-0 flex-1" />
          <USelect v-model="draft.unit" :items="units" size="xs" class="min-w-0 flex-1" />
        </div>
        <template v-else>{{ g(row.original.quantity) }} {{ row.original.unit }}</template>
      </template>

      <!-- macros -->
      <template #kcal-cell="{ row }">
        <UInput v-if="editing(row.original.id)" v-model="draft.kcal" type="number" step="any" min="0" size="xs" class="w-full" />
        <template v-else>{{ k(row.original.kcal) }}</template>
      </template>
      <template #kcal-footer>{{ k(meal.total.kcal) }}</template>

      <template #carb-cell="{ row }">
        <UInput v-if="editing(row.original.id)" v-model="draft.carb" type="number" step="any" min="0" size="xs" class="w-full" />
        <template v-else>{{ g(row.original.carb) }}</template>
      </template>
      <template #carb-footer>{{ g(meal.total.carb) }}</template>

      <template #protein-cell="{ row }">
        <UInput v-if="editing(row.original.id)" v-model="draft.protein" type="number" step="any" min="0" size="xs" class="w-full" />
        <template v-else>{{ g(row.original.protein) }}</template>
      </template>
      <template #protein-footer>{{ g(meal.total.protein) }}</template>

      <template #fat-cell="{ row }">
        <UInput v-if="editing(row.original.id)" v-model="draft.fat" type="number" step="any" min="0" size="xs" class="w-full" />
        <template v-else>{{ g(row.original.fat) }}</template>
      </template>
      <template #fat-footer>{{ g(meal.total.fat) }}</template>

      <!-- actions -->
      <template #actions-cell="{ row }">
        <div class="flex justify-end">
          <template v-if="editing(row.original.id)">
            <UButton size="xs" color="primary" variant="ghost" label="✓" class="size-6 justify-center p-0 sm:size-7" @click="save(row.original.id)" />
            <UButton size="xs" color="neutral" variant="ghost" label="✕" class="size-6 justify-center p-0 sm:size-7" @click="cancel" />
          </template>
          <template v-else>
            <UButton size="xs" color="neutral" variant="ghost" label="✎" class="size-6 justify-center p-0 sm:size-7" @click="startEdit(row.original)" />
            <UButton size="xs" color="error" variant="ghost" label="✕" class="size-6 justify-center p-0 sm:size-7" @click="emit('delete-entry', row.original.id)" />
          </template>
        </div>
      </template>
    </UTable>

    <p
      v-if="meal.note && showNote()"
      class="mt-3 whitespace-pre-line rounded-md border-l-2 border-amber-300 bg-amber-50/60 px-3 py-2 text-xs italic text-gray-600 dark:border-amber-700/60 dark:bg-amber-950/30 dark:text-gray-300 sm:text-sm"
    >
      {{ meal.note }}
    </p>
  </div>
</template>
