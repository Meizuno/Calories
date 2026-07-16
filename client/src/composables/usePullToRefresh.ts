import { ref, computed, onMounted, onBeforeUnmount } from "vue";

type Options = {
  threshold?: number;
  max?: number;
  // Returns false to skip the gesture entirely (e.g. while an overlay is open).
  canPull?: () => boolean;
  // What to do when the user releases past the threshold. Defaults to a full
  // page reload — the whole app re-fetches from the API on boot, so a reload
  // is the simplest "refresh everything" for the mobile path.
  onTrigger?: () => void;
};

// Mobile pull-to-refresh on the window scroll. The handlers only engage while
// the page is scrolled to the very top (window.scrollY === 0), so a normal
// scroll-up gesture is unaffected. The reactive `distance` / `ready` are
// surfaced for the visual indicator App.vue renders under the header.
export function usePullToRefresh(options: Options = {}) {
  const THRESHOLD = options.threshold ?? 70;
  const MAX = options.max ?? 120;
  const distance = ref(0);
  const pulling = ref(false);
  const ready = computed(() => distance.value >= THRESHOLD);
  let startY: number | null = null;

  function reset() {
    distance.value = 0;
    startY = null;
    pulling.value = false;
  }

  function onStart(e: TouchEvent) {
    if (window.scrollY > 0) return;
    if (options.canPull && !options.canPull()) return;
    startY = e.touches[0]?.clientY ?? null;
    pulling.value = startY !== null;
  }

  function onMove(e: TouchEvent) {
    if (startY === null) return;
    // The page scrolled out from under the gesture (elastic bounce) — abandon.
    if (window.scrollY > 0) {
      reset();
      return;
    }
    const delta = (e.touches[0]?.clientY ?? startY) - startY;
    if (delta <= 0) {
      distance.value = 0;
      return;
    }
    // Dampen the travel and cap it, then swallow the native scroll so the page
    // doesn't rubber-band while the indicator is showing.
    distance.value = Math.min(delta * 0.5, MAX);
    if (distance.value > 0) e.preventDefault();
  }

  function onEnd() {
    if (ready.value) {
      (options.onTrigger ?? (() => window.location.reload()))();
      return;
    }
    reset();
  }

  onMounted(() => {
    window.addEventListener("touchstart", onStart, { passive: true });
    window.addEventListener("touchmove", onMove, { passive: false });
    window.addEventListener("touchend", onEnd, { passive: true });
    window.addEventListener("touchcancel", onEnd, { passive: true });
  });

  onBeforeUnmount(() => {
    window.removeEventListener("touchstart", onStart);
    window.removeEventListener("touchmove", onMove);
    window.removeEventListener("touchend", onEnd);
    window.removeEventListener("touchcancel", onEnd);
  });

  return { distance, pulling, ready };
}
