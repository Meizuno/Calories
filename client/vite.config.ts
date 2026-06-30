import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import ui from "@nuxt/ui/vite";

// Built output goes to ./dist (default). The client is fully separate from the
// server; they are combined only in the Docker image. In dev, proxy /api → Go.
export default defineConfig({
  plugins: [vue(), ui()],
  server: {
    proxy: {
      "/api": "http://localhost:8080",
    },
  },
});
