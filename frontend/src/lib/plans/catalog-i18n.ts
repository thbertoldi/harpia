import type { Locale } from "$lib/i18n";
import { translate } from "$lib/i18n";
import { resolveLocalizedContent } from "$lib/i18n/content";
import type {
  PlanTemplate,
  TemplateInputParameter,
} from "$lib/gen/harpia/plans/v1/plans_pb";

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
  return localizedPlanNameByKey(template.key, locale, template.name);
}

export function localizedPlanNameByKey(
  templateKey: string,
  locale: Locale,
  fallback = "",
): string {
  const key = templateKey.trim();
  if (!key) return fallback;
  return resolveCatalogEntry(
    `catalog.plan.${key}.name`,
    locale,
    fallback || key,
  );
}

export function localizedPlanDescription(
  template: PlanTemplate,
  locale: Locale,
): string {
  return localizedPlanDescriptionByKey(
    template.key,
    locale,
    template.description,
  );
}

export function localizedPlanDescriptionByKey(
  templateKey: string,
  locale: Locale,
  fallback = "",
): string {
  const key = templateKey.trim();
  if (!key) return fallback;
  return resolveCatalogEntry(
    `catalog.plan.${key}.description`,
    locale,
    fallback,
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

  return localizedStepTitleByKey(template.key, stepKey, locale, fallback);
}

export function localizedStepTitleByKey(
  templateKey: string,
  stepKey: string,
  locale: Locale,
  fallback = "",
): string {
  const key = templateKey.trim();
  const normalizedStepKey = stepKey.trim();
  const fallbackTitle = fallback || normalizedStepKey;
  if (!key || !normalizedStepKey) return fallbackTitle;
  return resolveCatalogEntry(
    `catalog.plan.${key}.step.${normalizedStepKey}.title`,
    locale,
    fallbackTitle,
  );
}

export function localizedStepDescription(
  template: PlanTemplate,
  stepKey: string,
  locale: Locale,
): string {
  const fallback =
    template.steps.find((step) => step.key === stepKey)?.description ?? "";
  return localizedStepDescriptionByKey(template.key, stepKey, locale, fallback);
}

export function localizedStepDescriptionByKey(
  templateKey: string,
  stepKey: string,
  locale: Locale,
  fallback = "",
): string {
  const key = templateKey.trim();
  const normalizedStepKey = stepKey.trim();
  if (!key || !normalizedStepKey) return fallback;
  return resolveCatalogEntry(
    `catalog.plan.${key}.step.${normalizedStepKey}.description`,
    locale,
    fallback,
  );
}

function resolveInputEntry(
  inputKey: string,
  suffix: "label" | "description",
  locale: Locale,
  fallback: string,
): string {
  const key = inputKey.trim();
  if (!key) return fallback;
  const i18nKey = `plans.inputs.${key}.${suffix}`;
  const resolved = translate(i18nKey, locale);
  return resolved !== i18nKey ? resolved : fallback;
}

export function localizedInputLabel(
  parameter: TemplateInputParameter,
  locale: Locale,
): string {
  return resolveInputEntry(
    parameter.key,
    "label",
    locale,
    parameter.label || parameter.key,
  );
}

export function localizedInputDescription(
  parameter: TemplateInputParameter,
  locale: Locale,
): string {
  return resolveInputEntry(
    parameter.key,
    "description",
    locale,
    parameter.description,
  );
}
