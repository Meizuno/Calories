import { createRouter, createWebHistory } from "vue-router";
import HomeView from "./views/HomeView.vue";
import LogView from "./views/LogView.vue";
import StatsView from "./views/StatsView.vue";
import AssistantView from "./views/AssistantView.vue";
import ProfileView from "./views/ProfileView.vue";
import SharedProfileView from "./views/SharedProfileView.vue";
import LoginView from "./views/LoginView.vue";
import { session, loadSession } from "./lib/session";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    // Home: the diary for authenticated users, the welcome screen for anonymous.
    { path: "/", component: HomeView },
    { path: "/login", component: LoginView, meta: { public: true } },
    { path: "/log", component: LogView },
    { path: "/stats", component: StatsView },
    { path: "/assistant", component: AssistantView },
    { path: "/profiles/me", component: ProfileView },
    { path: "/profile/:uuid", component: SharedProfileView, meta: { public: true } },
  ],
});

router.beforeEach(async (to) => {
  await loadSession();

  // Public: the shared profile view and the sign-in page. A signed-in visitor
  // has no business on /login, so send them on to the app.
  if (to.meta.public) {
    if (to.path === "/login" && session.authenticated) return "/";
    return true;
  }

  // Home is open to all (welcome vs diary), but a signed-in user who hasn't
  // finished onboarding is funnelled to the profile form first.
  if (to.path === "/") {
    if (session.authenticated && !session.profile?.onboarded) return "/profiles/me";
    return true;
  }

  // Everything else requires a session. Remember where they were going so the
  // sign-in lands them back here.
  if (!session.authenticated) {
    return { path: "/login", query: { return: to.fullPath } };
  }
  if (!session.profile?.onboarded && to.path !== "/profiles/me") return "/profiles/me";
  return true;
});

export default router;
