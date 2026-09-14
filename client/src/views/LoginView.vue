<script setup lang="ts">
import { computed, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { login, register, loginWithGoogle, session } from "../lib/session";
import { ApiError } from "../lib/http";
import LocaleToggle from "../components/LocaleToggle.vue";
import { t } from "../lib/i18n";
import { useUiSize } from "../composables/useUiSize";

const route = useRoute();
const router = useRouter();
const { control, compact } = useUiSize();

// "login" | "register" — the sign-up form only adds the name field.
const mode = ref<"login" | "register">(route.query.mode === "register" && session.registration ? "register" : "login");
const form = ref({ email: "", password: "", name: "" });
const busy = ref(false);
const showPassword = ref(false);

// Guard the obvious empty submit rather than letting the server answer it.
const canSubmit = computed(() => form.value.email.trim().length > 0 && form.value.password.length > 0 && !busy.value);
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
  <div class="w-full max-w-sm space-y-5">
    <div class="space-y-2 text-center">
      <div class="text-4xl">🥗</div>
      <h1 class="text-xl font-semibold sm:text-2xl">{{ isRegister ? t("auth.createAccount") : t("auth.signIn") }}</h1>
    </div>

    <UCard>
      <form class="space-y-4" @submit.prevent="submit">
        <p
          v-if="error"
          class="flex items-start gap-2 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-200"
          role="alert"
        >
          <UIcon name="i-ui-warning" class="mt-0.5 size-4 shrink-0" />
          <span>{{ error }}</span>
        </p>

        <label v-if="isRegister" class="block">
          <span class="mb-1 flex items-baseline gap-1.5 text-xs text-gray-500">
            {{ t("auth.name") }}
            <span class="text-[11px] text-gray-400">({{ t("auth.nameOptional") }})</span>
          </span>
          <UInput
            v-model="form.name"
            autocomplete="name"
            :size="control"
            class="w-full"
            :placeholder="t('auth.namePlaceholder')"
          />
        </label>

        <label class="block">
          <span class="mb-1 block text-xs text-gray-500">{{ t("auth.email") }}</span>
          <!-- w-full: UInput is inline-flex by default, so without it the field
               sits at roughly half the card width. -->
          <UInput
            v-model="form.email"
            type="email"
            autocomplete="email"
            autofocus
            required
            :size="control"
            class="w-full"
            :placeholder="t('auth.emailPlaceholder')"
          />
        </label>

        <label class="block">
          <span class="mb-1 block text-xs text-gray-500">{{ t("auth.password") }}</span>
          <UInput
            v-model="form.password"
            :type="showPassword ? 'text' : 'password'"
            :autocomplete="isRegister ? 'new-password' : 'current-password'"
            required
            :size="control"
            class="w-full"
            :placeholder="t('auth.passwordMask')"
          >
            <template #trailing>
              <!-- Typing a password blind is the commonest cause of a failed
                   sign-in; let people check what they typed. -->
              <button
                type="button"
                class="grid size-6 place-items-center rounded text-gray-400 outline-none transition hover:text-gray-700 focus-visible:ring-2 focus-visible:ring-emerald-500/60 dark:hover:text-gray-200"
                :title="showPassword ? t('auth.hidePassword') : t('auth.showPassword')"
                :aria-label="showPassword ? t('auth.hidePassword') : t('auth.showPassword')"
                :aria-pressed="showPassword"
                @click="showPassword = !showPassword"
              >
                <UIcon :name="showPassword ? 'i-ui-eye-off' : 'i-ui-eye'" class="size-4" />
              </button>
            </template>
          </UInput>
          <span v-if="isRegister" class="mt-1 block text-[11px] text-gray-400">{{ t("auth.passwordHint") }}</span>
        </label>

        <UButton
          type="submit"
          block
          :size="control"
          :loading="busy"
          :disabled="!canSubmit"
          :label="isRegister ? t('auth.createAccount') : t('auth.signInAction')"
        />
      </form>

      <template v-if="session.google">
        <div class="my-4 flex items-center gap-3 text-xs text-gray-400">
          <span class="h-px flex-1 bg-gray-200 dark:bg-gray-800" />
          {{ t("auth.or") }}
          <span class="h-px flex-1 bg-gray-200 dark:bg-gray-800" />
        </div>
        <UButton
          block
          :size="control"
          color="neutral"
          variant="outline"
          icon="i-simple-icons-google"
          :label="t('auth.google')"
          @click="loginWithGoogle(returnTo)"
        />
      </template>
      <template #footer>
        <div class="flex items-center justify-center">
          <LocaleToggle />
        </div>
      </template>
    </UCard>

    <p v-if="session.registration" class="text-center text-sm text-gray-500">
      <template v-if="isRegister">
        {{ t("auth.haveAccount") }}
        <UButton variant="link" :size="compact" :label="t('auth.signInAction')" @click="switchMode('login')" />
      </template>
      <template v-else>
        {{ t("auth.noAccount") }}
        <UButton variant="link" :size="compact" :label="t('auth.registerLink')" @click="switchMode('register')" />
      </template>
    </p>
  </div>
</template>
