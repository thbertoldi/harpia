export const THEME_STORAGE_KEY = "aiuna-theme";
export const COLOR_SCHEME_STORAGE_KEY = "aiuna-color-scheme";
const LEGACY_THEME_STORAGE_KEY = "harpia-theme";

export const THEMES = ["default", "aiuna", "tenant-base"] as const;
export type ThemeName = (typeof THEMES)[number];

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

export function getTheme(): ThemeName {
  if (typeof localStorage === "undefined") {
    return "default";
  }

  const stored = localStorage.getItem(THEME_STORAGE_KEY);
  if (stored && isThemeName(stored)) {
    return stored;
  }

  return "default";
}

export function applyTheme(name: ThemeName): void {
  document.documentElement.setAttribute("data-theme", name);
  localStorage.setItem(THEME_STORAGE_KEY, name);
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
  localStorage.setItem(COLOR_SCHEME_STORAGE_KEY, scheme);
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

  return "dark";
}

/** Apply named theme package and dark/light class together. */
export function initTheme(options?: { theme?: ThemeName }): void {
  applyTheme(options?.theme ?? getTheme());
  applyColorScheme(resolveColorScheme());
}
