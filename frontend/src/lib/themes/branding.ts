import { translate, type Locale } from "$lib/i18n";
import type { ThemeName } from "./index";

const BRAND_NAME_KEYS: Record<ThemeName, string> = {
  default: "brand.harpia.name",
  aiuna: "brand.aiuna.name",
  "tenant-base": "brand.tenant.name",
};

const BRAND_META_KEYS: Record<ThemeName, string> = {
  default: "brand.harpia.metaDescription",
  aiuna: "brand.aiuna.metaDescription",
  "tenant-base": "brand.tenant.metaDescription",
};

const BRAND_FAVICONS: Record<ThemeName, string> = {
  default: "/favicon.png",
  aiuna: "/brand/aiuna-symbol.png",
  "tenant-base": "/favicon.png",
};

export function brandName(theme: ThemeName, locale: Locale): string {
  return translate(BRAND_NAME_KEYS[theme], locale);
}

export function brandMetaDescription(theme: ThemeName, locale: Locale): string {
  return translate(BRAND_META_KEYS[theme], locale);
}

export function brandFavicon(theme: ThemeName): string {
  return BRAND_FAVICONS[theme];
}

export function brandTranslateParams(
  theme: ThemeName,
  locale: Locale,
): Record<string, string> {
  return { brand: brandName(theme, locale) };
}
