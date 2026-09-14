import { computed } from "vue";
import { useMediaQuery } from "./useMediaQuery";

export type UiSize = "xs" | "sm" | "md" | "lg" | "xl";

const SCALE: UiSize[] = ["xs", "sm", "md", "lg", "xl"];
const step = (base: UiSize, by: number) =>
  SCALE[Math.min(SCALE.length - 1, Math.max(0, SCALE.indexOf(base) + by))];

/**
 * Nuxt UI control sizes that follow the viewport.
 *
 * `size` is a prop, not a class, so there is no `sm:size-lg`: a component gets
 * one size for every screen. Everything was therefore sized for the phone and
 * stayed that small on a desktop, where the same button sits in a row with a
 * great deal of room around it.
 *
 * The roles below are named rather than raw sizes so the whole app steps
 * together — and so a control's size says what it is for:
 *
 *  - `control`  primary form fields and the button that submits them
 *  - `compact`  dense inline editing inside a list row, where a full-size field
 *               would push the row apart
 *  - `inline`   toolbar and row actions, deliberately subordinate to content
 *  - `hero`     the one or two calls to action on a landing screen
 *
 * One step at the `sm` breakpoint (640px), which is the split the layout
 * already uses everywhere else. A second step is one more line here if the
 * desktop ever wants it.
 */
export function useUiSize() {
  const roomy = useMediaQuery("(min-width: 640px)");
  const role = (base: UiSize) => computed<UiSize>(() => (roomy.value ? step(base, 1) : base));

  return {
    control: role("md"),
    compact: role("sm"),
    inline: role("xs"),
    hero: role("lg"),
  };
}
