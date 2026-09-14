import { defineConfig, type Plugin } from "vite";
import vue from "@vitejs/plugin-vue";
import ui from "@nuxt/ui/vite";

// Nuxt UI ships two builds of a few components: the Nuxt one (which renders
// <NuxtLink>, a component that only exists inside Nuxt) and a plain-Vue
// replacement under runtime/vue/components. Its own plugin is meant to swap them
// in, but the check it uses — `normalize(importer).includes(runtimeDir)` — does
// not match in this setup, so <UButton> ends up rendering Nuxt's Link.vue. That
// throws "Cannot destructure property 'href' of 'undefined'", because the
// unresolved <NuxtLink> never calls its slot with any props.
//
// Redirect those imports ourselves. Only the two components that actually have a
// Vue-mode replacement are touched; everything else resolves normally.
function nuxtUIVueOverrides(): Plugin {
  const overridden = new Set(["Link", "Icon"]);
  // .../@nuxt/ui/dist/runtime/components/Button.vue  →  captures the runtime dir
  const fromRuntimeComponents = /^(.*[\\/]@nuxt[\\/]ui[\\/]dist[\\/]runtime)[\\/]components[\\/][^\\/]+\.vue$/;

  return {
    name: "calories:nuxt-ui-vue-overrides",
    enforce: "pre",
    resolveId(id, importer) {
      if (!importer) return null;
      const name = /^\.\/(\w+)\.vue$/.exec(id)?.[1];
      if (!name || !overridden.has(name)) return null;
      // Vite appends ?vue&type=… to SFC sub-requests; match on the bare path.
      const match = fromRuntimeComponents.exec(importer.split("?")[0]);
      if (!match) return null;
      return `${match[1]}/vue/components/${name}.vue`;
    },
  };
}

// Built output goes to ./dist (default). The client is fully separate from the
// server; they are combined only in the Docker image. In dev, proxy /api → Go.
export default defineConfig({
  // colorMode: false → Nuxt UI stops managing the theme. index.html follows the
  // OS light/dark preference by toggling the `dark` class on <html> (no in-app
  // switch — it just tracks the system setting).
  plugins: [nuxtUIVueOverrides(), vue(), ui({ colorMode: false })],
  server: {
    proxy: {
      "/api": "http://localhost:8080",
    },
  },
});
