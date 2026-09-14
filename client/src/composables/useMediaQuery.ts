import { onUnmounted, ref } from "vue";

/**
 * Reactive `window.matchMedia`. Used to keep work off small screens entirely —
 * a component guarded with this never mounts, so it never fetches either.
 */
export function useMediaQuery(query: string) {
  const matches = ref(false);
  if (typeof window !== "undefined" && typeof window.matchMedia === "function") {
    const mq = window.matchMedia(query);
    matches.value = mq.matches;
    const sync = () => (matches.value = mq.matches);
    mq.addEventListener("change", sync);
    onUnmounted(() => mq.removeEventListener("change", sync));
  }
  return matches;
}
