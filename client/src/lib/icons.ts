import { addCollection } from "@iconify/vue";
import { svgBody } from "./svg";

import calendar from "../assets/icons/calendar.svg?raw";
import check from "../assets/icons/check.svg?raw";
import copy from "../assets/icons/copy.svg?raw";
import eye from "../assets/icons/eye.svg?raw";
import eyeOff from "../assets/icons/eye-off.svg?raw";
import next from "../assets/icons/next.svg?raw";
import prev from "../assets/icons/prev.svg?raw";
import warning from "../assets/icons/warning.svg?raw";

// Small UI icons, registered locally rather than pulled from api.iconify.design
// at runtime — the same reasoning as lib/flags.ts: no CDN round-trip, nothing to
// fail offline, and they paint on first render instead of popping in.
//
// The artwork lives in ../assets/icons as real .svg files and is inlined here by
// Vite's `?raw`, so it is still part of the bundle — editable as SVG, with no
// request at runtime. `public/` would mean fetching them instead.
//
// The prefix is one word on purpose. Nuxt UI strips the leading `i-` and hands
// the rest to Iconify, which splits on the FIRST hyphen, so a prefix containing
// one (`i-my-icons-foo`) never resolves.
//
// Paths are Heroicons outline (MIT).
addCollection({
  prefix: "ui",
  width: 24,
  height: 24,
  icons: {
    prev: { body: svgBody(prev) },
    next: { body: svgBody(next) },
    check: { body: svgBody(check) },
    copy: { body: svgBody(copy) },
    warning: { body: svgBody(warning) },
    eye: { body: svgBody(eye) },
    "eye-off": { body: svgBody(eyeOff) },
    calendar: { body: svgBody(calendar) },
  },
});
