/**
 * Returns true when the user has requested reduced motion via the
 * `prefers-reduced-motion: reduce` media query. Returns false in any
 * environment without `matchMedia` (e.g., server-side rendering).
 *
 * Consumed by every transition helper in `lib/motion/` to collapse
 * animations to 0ms opacity-only crossfades when reduce is set.
 */
export function prefersReducedMotion(): boolean {
  if (typeof globalThis.matchMedia !== "function") return false;
  return globalThis.matchMedia("(prefers-reduced-motion: reduce)").matches;
}
