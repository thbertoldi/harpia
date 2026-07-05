import type { Locale } from "$lib/i18n";
import { resolveLocalizedContent } from "$lib/i18n/content";
import type { PlanTemplate } from "$lib/gen/harpia/plans/v1/plans_pb";

/**
 * Catalog-driven localization for plan content (ADR-015: the catalog is
 * content). Display surfaces resolve the localized name/title via the
 * `catalog.plan.<key>.*` content keys, falling back to the template's own
 * English fields when a key is absent (or the template key is unknown). This
 * keeps the resolver total: nothing breaks when a template lacks a content
 * entry.
 */

/**
 * `resolveLocalizedContent` returns the lookup key itself when the entry is
 * missing in both the locale and `en` catalogs. Treat that sentinel — and an
 * empty string — as "no entry" so callers fall back to the template field.
 */
function resolveCatalogEntry(
  contentKey: string,
  locale: Locale,
  fallback: string,
): string {
  const resolved = resolveLocalizedContent(contentKey, locale);
  return resolved && resolved !== contentKey ? resolved : fallback;
}

/**
 * Localized plan display name. Resolves `catalog.plan.<template.key>.name`
 * for the locale, falling back to `template.name` (and finally the key).
 */
export function localizedPlanName(
  template: PlanTemplate,
  locale: Locale,
): string {
  const key = template.key?.trim();
  if (!key) return template.name || "";
  return resolveCatalogEntry(
    `catalog.plan.${key}.name`,
    locale,
    template.name || key,
  );
}

/**
 * Localized step title. Resolves
 * `catalog.plan.<template.key>.step.<stepKey>.title` for the locale, falling
 * back to the matching `template.steps[].title`, and finally to `stepKey`.
 */
export function localizedStepTitle(
  template: PlanTemplate,
  stepKey: string,
  locale: Locale,
): string {
  const fallbackStepTitle =
    template.steps.find((step) => step.key === stepKey)?.title ?? "";
  const fallback = fallbackStepTitle || stepKey;

  const key = template.key?.trim();
  if (!key) return fallback;

  return resolveCatalogEntry(
    `catalog.plan.${key}.step.${stepKey}.title`,
    locale,
    fallback,
  );
}
