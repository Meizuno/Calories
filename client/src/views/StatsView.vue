<script setup lang="ts">
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { api } from "../lib/api";
import StatsPanel from "../components/StatsPanel.vue";
import { t } from "../lib/i18n";

const pad = (n: number) => String(n).padStart(2, "0");
function todayISO() {
  const d = new Date();
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

// Period type + anchor date live in the URL (?type=week|month & date=YYYY-MM-DD)
// so the view is shareable, bookmarkable, and survives reload / back-forward.
// The URL is the single source of truth — writable computeds over the query,
// bound into StatsPanel's models (no two-way-sync loop).
const route = useRoute();
const router = useRouter();
function setQuery(patch: Record<string, string>) {
  router.replace({ query: { ...route.query, ...patch } });
}
type Gran = "week" | "month" | "year" | "custom";
const GRANS: Gran[] = ["week", "month", "year", "custom"];
const gran = computed<Gran>({
  get: () => {
    const v = route.query.type;
    return typeof v === "string" && (GRANS as string[]).includes(v) ? (v as Gran) : "week";
  },
  set: (v) => setQuery({ type: v }),
});

// A custom range lives in the URL as well, so it survives reload and can be
// shared like any other period.
const isISO = (v: unknown): v is string => typeof v === "string" && /^\d{4}-\d{2}-\d{2}$/.test(v);
const from = computed<string>({
  get: () => (isISO(route.query.from) ? route.query.from : ""),
  set: (v) => setQuery({ from: v }),
});
const to = computed<string>({
  get: () => (isISO(route.query.to) ? route.query.to : ""),
  set: (v) => setQuery({ to: v }),
});
const anchor = computed<string>({
  get: () => {
    const d = route.query.date;
    return typeof d === "string" && /^\d{4}-\d{2}-\d{2}$/.test(d) ? d : todayISO();
  },
  set: (v) => setQuery({ date: v }),
});

const fetchStats = (from: string, to: string) => api.getStats(from, to);
const fetchDays = () => api.getDays();
</script>

<template>
  <StatsPanel
    v-model:gran="gran"
    v-model:anchor="anchor"
    v-model:from="from"
    v-model:to="to"
    :fetch-stats="fetchStats"
    :fetch-days="fetchDays"
  >
    <template #title>
      <h1 class="text-lg font-semibold sm:text-xl">{{ t("nav.stats") }}</h1>
    </template>
  </StatsPanel>
</template>
