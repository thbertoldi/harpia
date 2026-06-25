import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

/**
 * Regression guard (M6): every per-mode surface/text token must be defined in
 * BOTH the `:not(.dark)` (light) and `.dark` (dark) blocks of each theme. A
 * token defined in only one block — or in a mode-agnostic `:root[data-theme]`
 * block — silently fails to respect the active color scheme.
 *
 * This caught the bug where --token-surface-deep/-hover/-pop were added once
 * with dark values and stayed dark in light mode.
 */
const THEMES = ["default", "aiuna", "tenant-base"] as const;

const THEME_DIR = import.meta.dirname;

function modeBlock(css: string, theme: string, mode: "light" | "dark"): string {
  // Match `[data-theme="<theme>"]:not(.dark) { ... }` or `...,.dark { ... }`.
  const selector =
    mode === "light"
      ? `[data-theme="${theme}"]:not(.dark)`
      : `[data-theme="${theme}"].dark`;
  const start = css.indexOf(selector);
  if (start === -1) return "";
  const open = css.indexOf("{", start);
  const close = css.indexOf("}", open);
  return css.slice(open + 1, close);
}

function tokenNames(block: string): Set<string> {
  return new Set(
    [...block.matchAll(/(--token-[a-z0-9-]+)\s*:/g)].map((m) => m[1]),
  );
}

// Tokens whose value depends on the color scheme: they MUST be set in both the
// light and dark blocks of every theme. (Brand tokens like --token-primary may
// legitimately be a base value with a dark-only override, so they're excluded.)
const MODE_SENSITIVE_TOKENS = [
  "--token-surface",
  "--token-surface-elevated",
  "--token-surface-deep",
  "--token-surface-hover",
  "--token-surface-pop",
  "--token-text",
  "--token-text-muted",
  "--token-text-muted-dark",
  "--token-border",
];

describe("theme light/dark token parity", () => {
  for (const theme of THEMES) {
    it(`${theme}: mode-sensitive tokens are defined in both light and dark`, () => {
      const css = readFileSync(resolve(THEME_DIR, `${theme}.css`), "utf8");
      const light = tokenNames(modeBlock(css, theme, "light"));
      const dark = tokenNames(modeBlock(css, theme, "dark"));

      expect(light.size).toBeGreaterThan(0);
      expect(dark.size).toBeGreaterThan(0);

      const missingLight = MODE_SENSITIVE_TOKENS.filter((t) => !light.has(t));
      const missingDark = MODE_SENSITIVE_TOKENS.filter((t) => !dark.has(t));

      expect(missingLight, `missing from ${theme} light block`).toEqual([]);
      expect(missingDark, `missing from ${theme} dark block`).toEqual([]);
    });
  }
});
