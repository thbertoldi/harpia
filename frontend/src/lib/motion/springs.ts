import { tweened, type Tweened } from "svelte/motion";
import { spring, type Spring } from "svelte/motion";
import { cubicOut, cubicInOut } from "svelte/easing";
import { prefersReducedMotion } from "./reducedMotion";

/** 200ms cubic-out tween for sidebar count badges (3 → 2 animates). */
export function createCountTween(initial: number): Tweened<number> {
  return tweened(initial, {
    duration: prefersReducedMotion() ? 0 : 200,
    easing: cubicOut,
  });
}

/** Spring for cost-pill amount changes. Settles softly as bindings shift. */
export function createProgressSpring(initial: number): Spring<number> {
  return spring(initial, {
    stiffness: 0.15,
    damping: 0.6,
  });
}

/** Looping 0 → 1 → 0 tween over 1400ms for `running` node opacity pulse. */
export function createNodeStatePulse(): Tweened<number> {
  return tweened(0, {
    duration: prefersReducedMotion() ? 0 : 1400,
    easing: cubicInOut,
  });
}
