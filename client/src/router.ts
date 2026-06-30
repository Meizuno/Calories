import { createRouter, createWebHistory } from "vue-router";
import WelcomeView from "./views/WelcomeView.vue";
import DiaryView from "./views/DiaryView.vue";
import CatalogView from "./views/CatalogView.vue";
import ProfileView from "./views/ProfileView.vue";
import SharedProfileView from "./views/SharedProfileView.vue";
import { session, loadSession, redirectToLogin } from "./lib/session";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", component: WelcomeView, meta: { public: true } },
    { path: "/diary", component: DiaryView },
    { path: "/catalog", component: CatalogView },
    { path: "/profiles/me", component: ProfileView },
    { path: "/profile/:uuid", component: SharedProfileView, meta: { public: true } },
  ],
});

router.beforeEach(async (to) => {
  await loadSession();
  if (to.meta.public) return true;
  if (!session.authenticated) {
    redirectToLogin();
    return false;
  }
  // A freshly-registered profile must complete onboarding before using the app.
  if (!session.profile?.onboarded && to.path !== "/profiles/me") {
    return "/profiles/me";
  }
  return true;
});

export default router;
