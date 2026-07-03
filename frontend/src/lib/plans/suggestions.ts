import type { Locale } from "$lib/i18n";
import { translate } from "$lib/i18n";
import { resolveLocalizedContent } from "$lib/i18n/content";
import { loadPlanTemplates } from "$lib/plans/plan-catalog";
import type { PlanTemplate } from "$lib/gen/harpia/plans/v1/plans_pb";

/**
 * Catalog-driven suggestion chips (ADR-015: the catalog is content). Replaces
 * the hardcoded home examples with chips derived from the live PlanTemplate
 * catalog. See openspec change `catalog-driven-suggestions`.
 */
export interface CatalogSuggestion {
  templateKey: string;
  /** Stable icon key derived from the template vertical (component maps to icon). */
  iconKey: string;
  /** Localized chip label (template name). */
  label: string;
  /** Send-ready prompt submitted as the thread's opening message. */
  prompt: string;
}

export const MAX_SUGGESTIONS = 4;
export const DEFAULT_ICON_KEY = "sparkles";

/**
 * Deterministic vertical -> icon key, with an explicit default so unknown
 * verticals always render a sensible icon.
 */
export function verticalIconKey(vertical: string | undefined): string {
  const v = (vertical ?? "").trim().toLowerCase();
  if (!v) return DEFAULT_ICON_KEY;
  if (v.includes("social") || v.includes("linkedin")) return "share";
  if (v.includes("news") || v.includes("rss")) return "newspaper";
  if (v.includes("content") || v.includes("writ")) return "edit";
  if (v.includes("analy") || v.includes("data") || v.includes("chart"))
    return "chart";
  if (v.includes("market")) return "megaphone";
  return DEFAULT_ICON_KEY;
}

/**
 * Build a single suggestion from a localized template. The prompt resolves from
 * `catalog.plan.<key>.suggestion`, falling back to a composed
 * "Create a {name}" string when the content key is absent.
 */
export function suggestionForTemplate(
  template: PlanTemplate,
  locale: Locale,
): CatalogSuggestion {
  const key = template.key;
  const label = template.name?.trim() || key;
  const suggestionKey = `catalog.plan.${key}.suggestion`;
  const resolved = resolveLocalizedContent(suggestionKey, locale);
  const prompt =
    resolved && resolved !== suggestionKey
      ? resolved
      : translate("home.suggestion.compose", locale, { name: label });

  return {
    templateKey: key,
    iconKey: verticalIconKey(template.vertical),
    label,
    prompt,
  };
}

/**
 * Load up to `max` catalog-derived suggestions in stable catalog order. Throws
 * only when the catalog cannot be reached and no mock fallback is available;
 * callers fall back to a generic prompt set on rejection.
 */
export async function loadCatalogSuggestions(
  locale: Locale = "en",
  max: number = MAX_SUGGESTIONS,
): Promise<CatalogSuggestion[]> {
  const templates = await loadPlanTemplates(locale);
  return templates.slice(0, max).map((t) => suggestionForTemplate(t, locale));
}
