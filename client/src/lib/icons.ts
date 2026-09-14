import { addCollection } from "@iconify/vue";

// Small UI icons, registered locally rather than pulled from api.iconify.design
// at runtime — the same reasoning as lib/flags.ts: no CDN round-trip, nothing to
// fail offline, and they paint on first render instead of popping in.
//
// The prefix is one word on purpose. Nuxt UI strips the leading `i-` and hands
// the rest to Iconify, which splits on the FIRST hyphen, so a prefix containing
// one (`i-my-icons-foo`) never resolves.
//
// Paths are Heroicons outline (MIT).
const stroke = (d: string) =>
  `<g fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="${d}"/></g>`;

addCollection({
  prefix: "ui",
  width: 24,
  height: 24,
  icons: {
    prev: { body: stroke("M15.75 19.5 8.25 12l7.5-7.5") },
    next: { body: stroke("m8.25 4.5 7.5 7.5-7.5 7.5") },
    calendar: {
      body: stroke(
        "M6.75 3v2.25M17.25 3v2.25M3 18.75V7.5a2.25 2.25 0 0 1 2.25-2.25h13.5A2.25 2.25 0 0 1 21 7.5v11.25m-18 0A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75m-18 0v-7.5A2.25 2.25 0 0 1 5.25 9h13.5A2.25 2.25 0 0 1 21 11.25v7.5",
      ),
    },
  },
});
