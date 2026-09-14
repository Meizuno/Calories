<script setup lang="ts">
import { computed, ref } from "vue";
import { useRouter } from "vue-router";
import { api } from "../lib/api";
import { session, loadSession, changePassword } from "../lib/session";
import { ApiError } from "../lib/http";
import type { Profile } from "../lib/types";

const router = useRouter();
const profile = ref<Profile | null>(null);
const form = ref({ name: "", kcal: "0", carb: "0", protein: "0", fat: "0", shared: false });
const saving = ref(false);

// Account section: an account created through Google has no password yet, so it
// only needs the new one; changing an existing password requires the current.
const pw = ref({ current: "", next: "" });
const pwBusy = ref(false);
const pwError = ref("");
const pwDone = ref(false);
const hasPassword = computed(() => session.user?.hasPassword ?? true);
const googleLinked = computed(() => session.user?.providers.includes("google") ?? false);

async function savePassword() {
  if (pwBusy.value || !pw.value.next) return;
  pwError.value = "";
  pwDone.value = false;
  pwBusy.value = true;
  try {
    await changePassword(pw.value.current, pw.value.next);
    pw.value = { current: "", next: "" };
    pwDone.value = true;
  } catch (e) {
    pwError.value = e instanceof ApiError ? e.message : "Heslo se nepodařilo změnit.";
  } finally {
    pwBusy.value = false;
  }
}

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
    if (wasOnboarding) router.push("/");
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

    <!-- Account & sign-in. Hidden during onboarding so the first run stays to
         one task: the goal. -->
    <UCard v-if="!isOnboarding && session.user">
      <div class="space-y-4">
        <div>
          <h2 class="text-sm font-medium">Účet</h2>
          <p class="text-xs text-gray-500">
            {{ session.user.email }}
            <span v-if="googleLinked"> · propojeno s Google</span>
          </p>
        </div>

        <form class="space-y-3" @submit.prevent="savePassword">
          <p v-if="pwError" class="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-200">
            {{ pwError }}
          </p>
          <p v-else-if="pwDone" class="rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950 dark:text-emerald-200">
            Heslo bylo změněno. Ostatní zařízení byla odhlášena.
          </p>

          <label v-if="hasPassword" class="block">
            <span class="mb-1 block text-xs text-gray-500">Současné heslo</span>
            <UInput v-model="pw.current" type="password" autocomplete="current-password" />
          </label>
          <label class="block">
            <span class="mb-1 block text-xs text-gray-500">{{ hasPassword ? "Nové heslo" : "Nastavit heslo" }}</span>
            <UInput v-model="pw.next" type="password" autocomplete="new-password" placeholder="Alespoň 8 znaků" />
          </label>

          <div class="flex justify-end">
            <UButton type="submit" color="neutral" variant="soft" :loading="pwBusy" :label="hasPassword ? 'Změnit heslo' : 'Nastavit heslo'" />
          </div>
        </form>
      </div>
    </UCard>
  </div>

  <div v-else class="p-8 text-center text-gray-400">Načítání…</div>
</template>
