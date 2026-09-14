import { addCollection } from "@iconify/vue";
import { svgBody } from "./svg";

import cs from "../assets/flags/cs.svg?raw";
import en from "../assets/flags/en.svg?raw";

// Nuxt UI resolves `i-<collection>-<name>` icons through Iconify, which by
// default fetches them from api.iconify.design at runtime. The two flags in the
// language picker are ~1.3 KB together, so they are registered locally instead:
// no CDN round-trip, nothing to fail offline, and they paint on first render
// rather than popping in once a request returns.
//
// The artwork lives in ../assets/flags as real .svg files and is inlined here by
// Vite's `?raw`, which keeps that property — `public/` would mean fetching them.
//
// The prefix is one word on purpose. Nuxt UI strips the leading `i-` and hands
// the rest to Iconify, which splits on the FIRST hyphen — so `i-circle-flags-cz`
// would be read as collection "circle", icon "flags-cz", and never resolve.
//
// Artwork from the `circle-flags` collection (CC0). The mask ids are the one
// edit: upstream ships both with the same id, which would collide when two flags
// render at once.
addCollection({
  prefix: "locale",
  width: 512,
  height: 512,
  icons: {
    cs: { body: svgBody(cs) },
    en: { body: svgBody(en) },
  },
});
