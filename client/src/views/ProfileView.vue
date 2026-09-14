<script setup lang="ts">
import { computed, ref } from "vue";
import { useRouter } from "vue-router";
import { api } from "../lib/api";
import { session, loadSession, changePassword } from "../lib/session";
import { ApiError } from "../lib/http";
import { t } from "../lib/i18n";
import type { Profile } from "../lib/types";
import { useUiSize } from "../composables/useUiSize";

const router = useRouter();
const { control } = useUiSize();
const profile = ref<Profile | null>(null);
const form = ref({ name: "", kcal: "0", carb: "0", protein: "0", fat: "0", shared: false });
const saving = ref(false);

// Account section: an account created through Google has no password yet, so it
// only needs the new one; changing an existing password requires the current.
const pw = ref({ current: "", next: "" });
const pwBusy = ref(false);
const pwErrorCode = ref("");
const pwError = computed(() => (pwErrorCode.value ? t(`errors.${pwErrorCode.value}`) : ""));
const pwDone = ref(false);
const hasPassword = computed(() => session.user?.hasPassword ?? true);
const googleLinked = computed(() => session.user?.providers.includes("google") ?? false);

async function savePassword() {
  if (pwBusy.value || !pw.value.next) return;
  pwErrorCode.value = "";
  pwDone.value = false;
  pwBusy.value = true;
  try {
    await changePassword(pw.value.current, pw.value.next);
    pw.value = { current: "", next: "" };
    pwDone.value = true;
  } catch (e) {
    pwErrorCode.value = e instanceof ApiError ? e.code : "unknown";
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
      {{ t("profile.onboardingHint") }}
    </div>
    <h1 class="text-xl font-semibold sm:text-2xl">{{ isOnboarding ? t("profile.finishSignup") : t("profile.title") }}</h1>

    <UCard>
      <form class="space-y-4" @submit.prevent="save">
        <label class="block">
          <span class="mb-1 block text-xs text-gray-500">{{ t("profile.name") }}</span>
          <UInput v-model="form.name" :size="control" class="w-full" :placeholder="t('auth.namePlaceholder')" />
        </label>

        <div>
          <span class="mb-1 block text-xs text-gray-500">{{ t("profile.dailyGoal") }}</span>
          <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <label class="flex flex-col gap-1 text-xs text-gray-500">
              kcal
              <UInput v-model="form.kcal" type="number" step="any" min="0" :size="control" class="w-full" />
            </label>
            <label class="flex flex-col gap-1 text-xs text-gray-500">
              {{ t("macros.carb") }} (g)
              <UInput v-model="form.carb" type="number" step="any" min="0" :size="control" class="w-full" />
            </label>
            <label class="flex flex-col gap-1 text-xs text-gray-500">
              {{ t("macros.protein") }} (g)
              <UInput v-model="form.protein" type="number" step="any" min="0" :size="control" class="w-full" />
            </label>
            <label class="flex flex-col gap-1 text-xs text-gray-500">
              {{ t("macros.fat") }} (g)
              <UInput v-model="form.fat" type="number" step="any" min="0" :size="control" class="w-full" />
            </label>
          </div>
        </div>

        <div class="flex items-center justify-between rounded-lg border border-gray-200 px-3 py-2 dark:border-gray-800">
          <div>
            <div class="text-sm font-medium">{{ t("profile.shared") }}</div>
            <div class="text-xs text-gray-500">{{ t("profile.sharedHint") }}</div>
          </div>
          <USwitch v-model="form.shared" :size="control" />
        </div>

        <div v-if="shareUrl" class="flex items-center gap-2">
          <UInput :model-value="shareUrl" readonly :size="control" class="flex-1" />
          <UButton color="neutral" variant="soft" :size="control" :label="t('common.copy')" @click="copyShare" />
        </div>

        <div class="flex justify-end">
          <UButton type="submit" :size="control" :loading="saving" :label="isOnboarding ? t('profile.continue') : t('common.save')" />
        </div>
      </form>
    </UCard>

    <!-- Account & sign-in. Hidden during onboarding so the first run stays to
         one task: the goal. -->
    <UCard v-if="!isOnboarding && session.user">
      <div class="space-y-4">
        <div>
          <h2 class="text-sm font-medium">{{ t("profile.account") }}</h2>
          <p class="text-xs text-gray-500">
            {{ session.user.email }}
            <span v-if="googleLinked"> · {{ t("profile.googleLinked") }}</span>
          </p>
        </div>

        <form class="space-y-3" @submit.prevent="savePassword">
          <p v-if="pwError" class="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-200">
            {{ pwError }}
          </p>
          <p v-else-if="pwDone" class="rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950 dark:text-emerald-200">
            {{ t("profile.passwordChanged") }}
          </p>

          <label v-if="hasPassword" class="block">
            <span class="mb-1 block text-xs text-gray-500">{{ t("profile.currentPassword") }}</span>
            <UInput v-model="pw.current" type="password" autocomplete="current-password" :size="control" class="w-full" />
          </label>
          <label class="block">
            <span class="mb-1 block text-xs text-gray-500">{{ hasPassword ? t("profile.newPassword") : t("profile.setPassword") }}</span>
            <UInput v-model="pw.next" type="password" autocomplete="new-password" :size="control" class="w-full" :placeholder="t('auth.passwordMin')" />
          </label>

          <div class="flex justify-end">
            <UButton type="submit" color="neutral" variant="soft" :size="control" :loading="pwBusy" :label="hasPassword ? t('profile.changePassword') : t('profile.setPassword')" />
          </div>
        </form>
      </div>
    </UCard>
  </div>

  <div v-else class="p-8 text-center text-gray-400">{{ t("common.loading") }}</div>
</template>
