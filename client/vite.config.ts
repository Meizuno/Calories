import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import ui from "@nuxt/ui/vite";

// Built output goes to ./dist (default). The client is fully separate from the
// server; they are combined only in the Docker image. In dev, proxy /api → Go.
export default defineConfig({
  // colorMode: false → Nuxt UI stops managing the theme. index.html follows the
  // OS light/dark preference by toggling the `dark` class on <html> (no in-app
  // switch — it just tracks the system setting).
  plugins: [vue(), ui({ colorMode: false })],
  server: {
    proxy: {
      "/api": "http://localhost:8080",
    },
  },
});
