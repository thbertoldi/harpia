import { env } from "$env/dynamic/public";

export type FlagName = "uxRealignment.m1";

/**
 * Returns whether the given feature flag is enabled in the current environment.
 *
 * Flags map to PUBLIC_* env vars; only the literal string "true" enables a flag.
 */
export function hasFeature(flag: FlagName): boolean {
  switch (flag) {
    case "uxRealignment.m1":
      return env.PUBLIC_FEATURE_UX_REALIGNMENT_M1 === "true";
    default:
      return false;
  }
}
