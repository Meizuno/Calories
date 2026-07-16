import { createRouter, createWebHistory } from "vue-router";
import HomeView from "./views/HomeView.vue";
import LogView from "./views/LogView.vue";
import StatsView from "./views/StatsView.vue";
import ProfileView from "./views/ProfileView.vue";
import SharedProfileView from "./views/SharedProfileView.vue";
import { session, loadSession, redirectToLogin } from "./lib/session";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    // Home: the diary for authenticated users, the welcome screen for anonymous.
    { path: "/", component: HomeView },
    { path: "/log", component: LogView },
    { path: "/stats", component: StatsView },
    { path: "/profiles/me", component: ProfileView },
    { path: "/profile/:uuid", component: SharedProfileView, meta: { public: true } },
  ],
});

router.beforeEach(async (to) => {
  await loadSession();

  // Public shared profile — anyone, no onboarding gate.
  if (to.meta.public) return true;

  // Home is open to all (welcome vs diary), but a signed-in user who hasn't
  // finished onboarding is funnelled to the profile form first.
  if (to.path === "/") {
    if (session.authenticated && !session.profile?.onboarded) return "/profiles/me";
    return true;
  }

  // Everything else requires a session.
  if (!session.authenticated) {
    redirectToLogin();
    return false;
  }
  if (!session.profile?.onboarded && to.path !== "/profiles/me") return "/profiles/me";
  return true;
});

export default router;
