import type { Locale } from "./types";

function toIntlLocale(locale: Locale): string {
  return locale === "pt-BR" ? "pt-BR" : "en-US";
}

export function formatLocaleDate(
  value: string | Date,
  locale: Locale,
  options: Intl.DateTimeFormatOptions = {
    month: "short",
    day: "numeric",
    year: "numeric",
  },
): string {
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) {
    return typeof value === "string" ? value : "";
  }
  return date.toLocaleDateString(toIntlLocale(locale), options);
}

export function formatLocaleDateTime(
  value: string | Date,
  locale: Locale,
  options: Intl.DateTimeFormatOptions = {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  },
): string {
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) {
    return typeof value === "string" ? value : "";
  }
  return date.toLocaleString(toIntlLocale(locale), options);
}

const RELATIVE_UNITS: Array<{
  limitSeconds: number;
  divisorSeconds: number;
  unit: Intl.RelativeTimeFormatUnit;
}> = [
  { limitSeconds: 60, divisorSeconds: 1, unit: "second" },
  { limitSeconds: 3600, divisorSeconds: 60, unit: "minute" },
  { limitSeconds: 86_400, divisorSeconds: 3600, unit: "hour" },
  { limitSeconds: 604_800, divisorSeconds: 86_400, unit: "day" },
];

/**
 * Returns a short relative-time string (e.g. "2 min ago", "1 hr ago",
 * "ontem", "há 3 dias") for an ISO-8601 instant in the past, localised
 * via Intl.RelativeTimeFormat. Falls back to `formatLocaleDate` for ages
 * beyond a week.
 */
export function formatRelativeTime(iso: string, locale: string): string {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "";
  const deltaSeconds = Math.max(0, Math.floor((Date.now() - then) / 1000));
  for (const { limitSeconds, divisorSeconds, unit } of RELATIVE_UNITS) {
    if (deltaSeconds < limitSeconds) {
      const value = Math.max(1, Math.floor(deltaSeconds / divisorSeconds));
      return new Intl.RelativeTimeFormat(locale, { numeric: "auto" }).format(
        -value,
        unit,
      );
    }
  }
  return formatLocaleDate(iso, locale as Locale);
}
