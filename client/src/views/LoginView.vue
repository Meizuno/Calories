<script setup lang="ts">
import { computed, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { login, register, loginWithGoogle, session } from "../lib/session";
import { ApiError } from "../lib/http";
import { t } from "../lib/i18n";

const route = useRoute();
const router = useRouter();

// "login" | "register" — the sign-up form only adds the name field.
const mode = ref<"login" | "register">(route.query.mode === "register" && session.registration ? "register" : "login");
const form = ref({ email: "", password: "", name: "" });
const busy = ref(false);
// Google failures come back as ?error= on a redirect, since the browser arrives
// here by navigation with no fetch waiting to read a response body.
const errorCode = ref(typeof route.query.error === "string" ? route.query.error : "");
const error = computed(() => (errorCode.value ? t(`errors.${errorCode.value}`) : ""));

// Sign-up only exists when the server allows it; otherwise the form and the
// "create an account" footer are hidden entirely, so nobody meets a 403.
const isRegister = computed(() => session.registration && mode.value === "register");
const returnTo = computed(() => (typeof route.query.return === "string" && route.query.return.startsWith("/") ? route.query.return : "/"));

function switchMode(to: "login" | "register") {
  mode.value = to;
  errorCode.value = "";
}

async function submit() {
  if (busy.value) return;
  errorCode.value = "";
  busy.value = true;
  try {
    if (isRegister.value) {
      await register(form.value.email, form.value.password, form.value.name);
    } else {
      await login(form.value.email, form.value.password);
    }
    // A brand-new account still has to fill in the goal before using the diary.
    router.replace(session.profile?.onboarded ? returnTo.value : "/profiles/me");
  } catch (e) {
    // Translate the API's code; a network failure has none, so fall back.
    errorCode.value = e instanceof ApiError ? e.code : "";
    if (!errorCode.value) errorCode.value = "unknown";
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <div class="mx-auto max-w-sm space-y-5 py-6">
    <div class="space-y-2 text-center">
      <div class="text-4xl">🥗</div>
      <h1 class="text-xl font-semibold sm:text-2xl">{{ isRegister ? t("auth.createAccount") : t("auth.signIn") }}</h1>
    </div>

    <UCard>
      <form class="space-y-4" @submit.prevent="submit">
        <p v-if="error" class="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-200">
          {{ error }}
        </p>

        <label v-if="isRegister" class="block">
          <span class="mb-1 block text-xs text-gray-500">{{ t("auth.name") }}</span>
          <UInput v-model="form.name" autocomplete="name" :placeholder="t('auth.namePlaceholder')" />
        </label>

        <label class="block">
          <span class="mb-1 block text-xs text-gray-500">{{ t("auth.email") }}</span>
          <UInput v-model="form.email" type="email" autocomplete="email" required :placeholder="t('auth.emailPlaceholder')" />
        </label>

        <label class="block">
          <span class="mb-1 block text-xs text-gray-500">{{ t("auth.password") }}</span>
          <UInput
            v-model="form.password"
            type="password"
            :autocomplete="isRegister ? 'new-password' : 'current-password'"
            required
            :placeholder="isRegister ? t('auth.passwordMin') : t('auth.passwordMask')"
          />
        </label>

        <UButton type="submit" block :loading="busy" :label="isRegister ? t('auth.createAccount') : t('auth.signInAction')" />
      </form>

      <template v-if="session.google">
        <div class="my-4 flex items-center gap-3 text-xs text-gray-400">
          <span class="h-px flex-1 bg-gray-200 dark:bg-gray-800" />
          {{ t("auth.or") }}
          <span class="h-px flex-1 bg-gray-200 dark:bg-gray-800" />
        </div>
        <UButton
          block
          color="neutral"
          variant="outline"
          icon="i-simple-icons-google"
          :label="t('auth.google')"
          @click="loginWithGoogle(returnTo)"
        />
      </template>
    </UCard>

    <p v-if="session.registration" class="text-center text-sm text-gray-500">
      <template v-if="isRegister">
        {{ t("auth.haveAccount") }}
        <UButton variant="link" size="sm" :label="t('auth.signInAction')" @click="switchMode('login')" />
      </template>
      <template v-else>
        {{ t("auth.noAccount") }}
        <UButton variant="link" size="sm" :label="t('auth.registerLink')" @click="switchMode('register')" />
      </template>
    </p>
  </div>
</template>
