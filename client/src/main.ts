import { createApp } from "vue";
import ui from "@nuxt/ui/vue-plugin";
import router from "./router";
import App from "./App.vue";
import "./assets/main.css";
import { initLocale } from "./lib/i18n";

initLocale();
createApp(App).use(router).use(ui).mount("#app");
