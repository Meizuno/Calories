import { onUnmounted, ref, watch } from "vue";
import { useMediaQuery } from "./useMediaQuery";

// Ease-out cubic: quick off the mark, settling gently. Kept in sync with the
// CSS timing used for the ring and the macro bars so a number and the shape it
// describes move together rather than drifting apart.
export const EASE_OUT_CUBIC = "cubic-bezier(0.33, 1, 0.68, 1)";
export const ANIMATION_MS = 600;

const ease = (t: number) => 1 - Math.pow(1 - t, 3);

/**
 * Tween a number towards its source value on every change.
 *
 * Honours `prefers-reduced-motion`: readers who ask for less movement get the
 * value immediately rather than a count-up, which is exactly the kind of motion
 * that setting exists to suppress.
 */
export function useAnimatedNumber(source: () => number, duration = ANIMATION_MS) {
  const reduced = useMediaQuery("(prefers-reduced-motion: reduce)");
  const current = ref(source());

  let frame = 0;
  let startedAt = 0;
  let from = current.value;
  let to = current.value;

  function step(now: number) {
    if (!startedAt) startedAt = now;
    const progress = Math.min((now - startedAt) / duration, 1);
    current.value = from + (to - from) * ease(progress);
    frame = progress < 1 ? requestAnimationFrame(step) : 0;
  }

  watch(source, (value) => {
    to = value;
    if (reduced.value || duration <= 0) {
      cancelAnimationFrame(frame);
      frame = 0;
      current.value = value;
      return;
    }
    // Start from wherever the previous tween had reached, so rapid changes
    // (holding the day arrows) stay continuous instead of snapping back.
    from = current.value;
    startedAt = 0;
    cancelAnimationFrame(frame);
    frame = requestAnimationFrame(step);
  });

  onUnmounted(() => cancelAnimationFrame(frame));

  return current;
}

/**
 * A 0 → 1 progress that restarts whenever `source` changes, for revealing a
 * freshly rendered shape. Separate from useAnimatedNumber because a reveal has
 * to snap back to zero before it runs; tweening down and up again would show a
 * dip instead of a growth.
 *
 * Runs once on mount, so the first render animates too.
 */
export function useReveal(source: () => unknown, duration = ANIMATION_MS) {
  const reduced = useMediaQuery("(prefers-reduced-motion: reduce)");
  const progress = ref(1);

  let frame = 0;
  let startedAt = 0;

  function step(now: number) {
    if (!startedAt) startedAt = now;
    const elapsed = Math.min((now - startedAt) / duration, 1);
    progress.value = ease(elapsed);
    frame = elapsed < 1 ? requestAnimationFrame(step) : 0;
  }

  watch(
    source,
    () => {
      cancelAnimationFrame(frame);
      frame = 0;
      if (reduced.value || duration <= 0) {
        progress.value = 1;
        return;
      }
      progress.value = 0;
      startedAt = 0;
      frame = requestAnimationFrame(step);
    },
    { immediate: true },
  );

  onUnmounted(() => cancelAnimationFrame(frame));

  return progress;
}
