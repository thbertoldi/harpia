import { writable } from "svelte/store";

export const THEME_CACHE_KEY = "aiuna-theme";
export const COLOR_SCHEME_STORAGE_KEY = "aiuna-color-scheme";
const LEGACY_THEME_STORAGE_KEY = "harpia-theme";

export const THEMES = ["default", "aiuna", "tenant-base"] as const;
export type ThemeName = (typeof THEMES)[number];

/** Platform default for tenants without an assigned theme package. */
export const PLATFORM_DEFAULT_THEME: ThemeName = "aiuna";

export const activeTheme = writable<ThemeName>(PLATFORM_DEFAULT_THEME);

export function isThemeName(value: string): value is ThemeName {
  return (THEMES as readonly string[]).includes(value);
}

function readLegacyColorScheme(): "dark" | "light" | null {
  const legacy = localStorage.getItem(LEGACY_THEME_STORAGE_KEY);
  if (legacy === "dark" || legacy === "light") {
    return legacy;
  }
  return null;
}

/**
 * Resolve tenant theme package. White-label tenants get one theme; users only pick light/dark.
 */
export function resolveTheme(tenantThemeKey?: string | null): ThemeName {
  if (tenantThemeKey && isThemeName(tenantThemeKey)) {
    return tenantThemeKey;
  }
  return PLATFORM_DEFAULT_THEME;
}

export function getActiveTheme(): ThemeName {
  if (typeof document === "undefined") {
    return PLATFORM_DEFAULT_THEME;
  }

  const current = document.documentElement.getAttribute("data-theme");
  if (current && isThemeName(current)) {
    return current;
  }

  return PLATFORM_DEFAULT_THEME;
}

/** Cache the active tenant theme for first paint; not a user preference override. */
export function cacheTheme(name: ThemeName): void {
  if (typeof localStorage !== "undefined") {
    localStorage.setItem(THEME_CACHE_KEY, name);
  }
}

export function readCachedTheme(): ThemeName | null {
  if (typeof localStorage === "undefined") {
    return null;
  }

  const cached = localStorage.getItem(THEME_CACHE_KEY);
  if (cached && isThemeName(cached)) {
    return cached;
  }

  return null;
}

export function applyTheme(name: ThemeName): void {
  document.documentElement.setAttribute("data-theme", name);
  activeTheme.set(name);
  cacheTheme(name);
}

export function getColorScheme(): "dark" | "light" | null {
  if (typeof localStorage === "undefined") {
    return null;
  }

  const stored = localStorage.getItem(COLOR_SCHEME_STORAGE_KEY);
  if (stored === "dark" || stored === "light") {
    return stored;
  }

  return readLegacyColorScheme();
}

export function applyColorScheme(scheme: "dark" | "light"): void {
  document.documentElement.classList.toggle("dark", scheme === "dark");
  if (typeof localStorage !== "undefined") {
    localStorage.setItem(COLOR_SCHEME_STORAGE_KEY, scheme);
  }
}

export function resolveColorScheme(): "dark" | "light" {
  const stored = getColorScheme();
  if (stored) {
    return stored;
  }

  if (typeof window !== "undefined") {
    return window.matchMedia("(prefers-color-scheme: dark)").matches
      ? "dark"
      : "light";
  }

  return "light";
}

/** Apply tenant theme package and user color-scheme preference. */
export function initTheme(options?: {
  theme?: ThemeName;
  tenantThemeKey?: string | null;
}): void {
  applyTheme(options?.theme ?? resolveTheme(options?.tenantThemeKey));
  applyColorScheme(resolveColorScheme());
}

/** @deprecated Use resolveTheme with tenant theme key. */
export function getDefaultThemeName(): ThemeName {
  return PLATFORM_DEFAULT_THEME;
}

/** @deprecated Use resolveTheme with tenant theme key. */
export function getTheme(): ThemeName {
  return getActiveTheme();
}
