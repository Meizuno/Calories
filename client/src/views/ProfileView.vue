<script setup lang="ts">
import { computed, ref } from "vue";
import { useRouter } from "vue-router";
import { api } from "../lib/api";
import { session, loadSession } from "../lib/session";
import type { Profile } from "../lib/types";

const router = useRouter();
const profile = ref<Profile | null>(null);
const form = ref({ name: "", kcal: "0", carb: "0", protein: "0", fat: "0", shared: false });
const saving = ref(false);

const isOnboarding = computed(() => profile.value !== null && !profile.value.onboarded);
const shareUrl = computed(() =>
  profile.value?.shared ? `${location.origin}/profile/${profile.value.publicId}` : "",
);

api.getProfile().then((p) => {
  profile.value = p;
  form.value = {
    name: p.name,
    kcal: String(p.goal.kcal),
    carb: String(p.goal.carb),
    protein: String(p.goal.protein),
    fat: String(p.goal.fat),
    shared: p.shared,
  };
});

const num = (s: string) => Math.max(0, parseFloat(s) || 0);

async function save() {
  if (!form.value.name.trim()) return;
  const wasOnboarding = isOnboarding.value;
  saving.value = true;
  try {
    profile.value = await api.saveProfile({
      name: form.value.name.trim(),
      kcal: num(form.value.kcal),
      carb: num(form.value.carb),
      protein: num(form.value.protein),
      fat: num(form.value.fat),
      shared: form.value.shared,
    });
    await loadSession(true); // refresh name/onboarded in the shell
    if (wasOnboarding) router.push("/diary");
  } finally {
    saving.value = false;
  }
}

async function copyShare() {
  if (shareUrl.value) await navigator.clipboard?.writeText(shareUrl.value);
}
</script>

<template>
  <div v-if="profile" class="mx-auto max-w-lg space-y-5">
    <div v-if="isOnboarding" class="rounded-lg border border-sky-200 bg-sky-50 p-4 text-sm text-sky-800 dark:border-sky-900 dark:bg-sky-950 dark:text-sky-200">
      Vítej! Než začneš, vyplň prosím svůj profil a denní cíl.
    </div>
    <h1 class="text-xl font-semibold sm:text-2xl">{{ isOnboarding ? "Dokončit registraci" : "Můj profil" }}</h1>

    <UCard>
      <form class="space-y-4" @submit.prevent="save">
        <label class="block">
          <span class="mb-1 block text-xs text-gray-500">Jméno</span>
          <UInput v-model="form.name" placeholder="Tvé jméno" />
        </label>

        <div>
          <span class="mb-1 block text-xs text-gray-500">Denní cíl</span>
          <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <label class="flex flex-col gap-1 text-xs text-gray-500">
              kcal
              <UInput v-model="form.kcal" type="number" step="any" min="0" />
            </label>
            <label class="flex flex-col gap-1 text-xs text-gray-500">
              Sacharidy (g)
              <UInput v-model="form.carb" type="number" step="any" min="0" />
            </label>
            <label class="flex flex-col gap-1 text-xs text-gray-500">
              Bílkoviny (g)
              <UInput v-model="form.protein" type="number" step="any" min="0" />
            </label>
            <label class="flex flex-col gap-1 text-xs text-gray-500">
              Tuky (g)
              <UInput v-model="form.fat" type="number" step="any" min="0" />
            </label>
          </div>
        </div>

        <div class="flex items-center justify-between rounded-lg border border-gray-200 px-3 py-2 dark:border-gray-800">
          <div>
            <div class="text-sm font-medium">Sdílený profil</div>
            <div class="text-xs text-gray-500">Zpřístupní deník komukoli přes veřejný odkaz (jen ke čtení).</div>
          </div>
          <USwitch v-model="form.shared" />
        </div>

        <div v-if="shareUrl" class="flex items-center gap-2">
          <UInput :model-value="shareUrl" readonly class="flex-1" />
          <UButton color="neutral" variant="soft" label="Kopírovat" @click="copyShare" />
        </div>

        <div class="flex justify-end">
          <UButton type="submit" :loading="saving" :label="isOnboarding ? 'Pokračovat' : 'Uložit'" />
        </div>
      </form>
    </UCard>
  </div>

  <div v-else class="p-8 text-center text-gray-400">Načítání…</div>
</template>
