<script setup lang="ts">
import { computed, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { login, register, loginWithGoogle, session } from "../lib/session";
import { ApiError } from "../lib/http";

const route = useRoute();
const router = useRouter();

// "login" | "register" — the sign-up form only adds the name field.
const mode = ref<"login" | "register">(route.query.mode === "register" && session.registration ? "register" : "login");
const form = ref({ email: "", password: "", name: "" });
const busy = ref(false);
// Google failures come back as ?error= on a redirect, since the browser arrives
// here by navigation with no fetch waiting to read a response body.
const error = ref(typeof route.query.error === "string" ? route.query.error : "");

// Sign-up only exists when the server allows it; otherwise the form and the
// "create an account" footer are hidden entirely, so nobody meets a 403.
const isRegister = computed(() => session.registration && mode.value === "register");
const returnTo = computed(() => (typeof route.query.return === "string" && route.query.return.startsWith("/") ? route.query.return : "/"));

function switchMode(to: "login" | "register") {
  mode.value = to;
  error.value = "";
}

async function submit() {
  if (busy.value) return;
  error.value = "";
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
    error.value = e instanceof ApiError ? e.message : "Přihlášení se nezdařilo, zkus to prosím znovu.";
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <div class="mx-auto max-w-sm space-y-5 py-6">
    <div class="space-y-2 text-center">
      <div class="text-4xl">🥗</div>
      <h1 class="text-xl font-semibold sm:text-2xl">{{ isRegister ? "Vytvořit účet" : "Přihlášení" }}</h1>
    </div>

    <UCard>
      <form class="space-y-4" @submit.prevent="submit">
        <p v-if="error" class="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-200">
          {{ error }}
        </p>

        <label v-if="isRegister" class="block">
          <span class="mb-1 block text-xs text-gray-500">Jméno</span>
          <UInput v-model="form.name" autocomplete="name" placeholder="Tvé jméno" />
        </label>

        <label class="block">
          <span class="mb-1 block text-xs text-gray-500">E-mail</span>
          <UInput v-model="form.email" type="email" autocomplete="email" required placeholder="ty@example.com" />
        </label>

        <label class="block">
          <span class="mb-1 block text-xs text-gray-500">Heslo</span>
          <UInput
            v-model="form.password"
            type="password"
            :autocomplete="isRegister ? 'new-password' : 'current-password'"
            required
            :placeholder="isRegister ? 'Alespoň 8 znaků' : '••••••••'"
          />
        </label>

        <UButton type="submit" block :loading="busy" :label="isRegister ? 'Vytvořit účet' : 'Přihlásit se'" />
      </form>

      <template v-if="session.google">
        <div class="my-4 flex items-center gap-3 text-xs text-gray-400">
          <span class="h-px flex-1 bg-gray-200 dark:bg-gray-800" />
          nebo
          <span class="h-px flex-1 bg-gray-200 dark:bg-gray-800" />
        </div>
        <UButton
          block
          color="neutral"
          variant="outline"
          icon="i-simple-icons-google"
          label="Pokračovat s Google"
          @click="loginWithGoogle(returnTo)"
        />
      </template>
    </UCard>

    <p v-if="session.registration" class="text-center text-sm text-gray-500">
      <template v-if="isRegister">
        Už máš účet?
        <UButton variant="link" size="sm" label="Přihlásit se" @click="switchMode('login')" />
      </template>
      <template v-else>
        Nemáš účet?
        <UButton variant="link" size="sm" label="Zaregistrovat se" @click="switchMode('register')" />
      </template>
    </p>
  </div>
</template>
