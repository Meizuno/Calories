<script setup lang="ts">
import type { Tokens } from "marked";

// A table. The header and body are rendered here rather than as six more
// components: the parts are never used apart, and keeping them together means
// the column alignment marked gives us is applied in one place.
defineProps<{ header: Tokens.TableCell[]; rows: Tokens.TableCell[][]; align: (string | null)[] }>();

const alignClass = (a: string | null) =>
  a === "center" ? "text-center" : a === "right" ? "text-right" : "text-left";
</script>

<template>
  <!-- Wrapped so a wide table scrolls inside itself; the conversation column
       must not widen to fit one. -->
  <div class="my-3 overflow-x-auto">
    <table class="w-full border-collapse text-sm">
      <thead>
        <tr class="border-b border-muted">
          <th
            v-for="(cell, i) in header"
            :key="i"
            :class="['px-2 py-1.5 font-semibold text-highlighted', alignClass(align[i] ?? null)]"
          >
            <slot name="cell" :tokens="cell.tokens" />
          </th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(row, r) in rows" :key="r" class="border-b border-muted/60 last:border-0">
          <td
            v-for="(cell, i) in row"
            :key="i"
            :class="['px-2 py-1.5 align-top tabular-nums', alignClass(align[i] ?? null)]"
          >
            <slot name="cell" :tokens="cell.tokens" />
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
