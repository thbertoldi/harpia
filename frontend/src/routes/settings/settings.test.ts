import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { canManageTenantSettings } from "$lib/auth-roles";

// ---------------------------------------------------------------------------
// AuthZ: manageTenantSettings permission
// ---------------------------------------------------------------------------

describe("manageTenantSettings AuthZ", () => {
  it("grants access to Leader role", () => {
    expect(canManageTenantSettings({ role: "Leader" })).toBe(true);
  });

  it("grants access to admin alias (maps to Leader)", () => {
    expect(canManageTenantSettings({ role: "admin" })).toBe(true);
    expect(canManageTenantSettings({ role: "owner" })).toBe(true);
  });

  it("grants access to Engineer role", () => {
    expect(canManageTenantSettings({ role: "Engineer" })).toBe(true);
  });

  it("denies access to Overseer role", () => {
    expect(canManageTenantSettings({ role: "Overseer" })).toBe(false);
  });

  it("denies access when user is null", () => {
    expect(canManageTenantSettings(null)).toBe(false);
    expect(canManageTenantSettings(undefined)).toBe(false);
  });

  it("denies access when role is missing", () => {
    expect(canManageTenantSettings({})).toBe(false);
    expect(canManageTenantSettings({ role: "" })).toBe(false);
  });

  it("denies access for unknown roles", () => {
    expect(canManageTenantSettings({ role: "member" })).toBe(false);
    expect(canManageTenantSettings({ role: "guest" })).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// Settings page i18n copy guard
// The settings page must not contain hard-coded user-visible English strings.
// All copy must go through translate(). This test scans the markup portion only.
// ---------------------------------------------------------------------------

const FRONTEND_ROOT = resolve(import.meta.dirname, "../../..");

function markupOnly(content: string): string {
  return content.replace(/<script[\s\S]*?<\/script>/gi, "");
}

const SETTINGS_PAGE = resolve(
  FRONTEND_ROOT,
  "src/routes/settings/+page.svelte",
);

const BANNED_LITERALS_IN_SETTINGS = [
  "LLM Providers",
  "Tenant key",
  "Platform key",
  "Save key",
  "Rotate key",
  "Remove key",
  "Coming soon",
  "API key",
];

describe("settings page copy guard", () => {
  it("avoids hard-coded English literals in settings page markup", () => {
    const content = markupOnly(readFileSync(SETTINGS_PAGE, "utf8"));
    for (const literal of BANNED_LITERALS_IN_SETTINGS) {
      expect(
        content,
        `settings/+page.svelte still contains hard-coded "${literal}"`,
      ).not.toContain(literal);
    }
  });
});

// ---------------------------------------------------------------------------
// i18n catalog completeness: all settings.* keys must exist in en.json
// ---------------------------------------------------------------------------

const EN_CATALOG = JSON.parse(
  readFileSync(resolve(FRONTEND_ROOT, "src/lib/i18n/en.json"), "utf8"),
) as Record<string, string>;

const EXPECTED_SETTINGS_KEYS = [
  "nav.settings",
  "settings.nav.ariaLabel",
  "settings.heading",
  "settings.subheading",
  "settings.nav.tenant",
  "settings.nav.llmProviders",
  "settings.nav.quotas",
  "settings.nav.billing",
  "settings.tenant.heading",
  "settings.tenant.subheading",
  "settings.tenant.name",
  "settings.tenant.namePlaceholder",
  "settings.tenant.slug",
  "settings.tenant.slugHint",
  "settings.tenant.managedByPlatform",
  "settings.tenant.managedByPlatformHint",
  "settings.tenant.todoRef",
  "settings.llmProviders.heading",
  "settings.llmProviders.subheading",
  "settings.llmProviders.empty.title",
  "settings.llmProviders.empty.description",
  "settings.llmProviders.empty.fallbackExplain",
  "settings.llmProviders.badge.platformKey",
  "settings.llmProviders.badge.tenantKey",
  "settings.llmProviders.badge.blocked",
  "settings.llmProviders.keyInput.label",
  "settings.llmProviders.keyInput.placeholder",
  "settings.llmProviders.keyInput.masked",
  "settings.llmProviders.keyInput.maskedJustNow",
  "settings.llmProviders.keyInput.neverSet",
  "settings.llmProviders.actions.save",
  "settings.llmProviders.actions.rotate",
  "settings.llmProviders.actions.remove",
  "settings.llmProviders.actions.saving",
  "settings.llmProviders.actions.removing",
  "settings.llmProviders.defaultModel.label",
  "settings.llmProviders.allowedModels.label",
  "settings.llmProviders.allowedModels.hint",
  "settings.llmProviders.transparency",
  "settings.llmProviders.removeConfirm.title",
  "settings.llmProviders.removeConfirm.description",
  "settings.llmProviders.removeConfirm.cancel",
  "settings.llmProviders.removeConfirm.confirm",
  "settings.llmProviders.toast.saved",
  "settings.llmProviders.toast.rotated",
  "settings.llmProviders.toast.removed",
  "settings.llmProviders.error.noProviderConfigured",
  "settings.llmProviders.error.blockedByPlatform",
  "settings.llmProviders.error.keyDecryptionFailed",
  "settings.llmProviders.error.unknown",
  "settings.quotas.heading",
  "settings.quotas.subheading",
  "settings.quotas.comingSoon",
  "settings.quotas.comingSoonDescription",
  "settings.quotas.monthlyBudget",
  "settings.quotas.hardCap",
  "settings.quotas.usageGauge",
  "settings.billing.heading",
  "settings.billing.placeholder",
];

describe("i18n catalog completeness", () => {
  it("has all expected settings.* keys in en.json", () => {
    for (const key of EXPECTED_SETTINGS_KEYS) {
      expect(
        EN_CATALOG,
        `Missing i18n key: "${key}" in en.json`,
      ).toHaveProperty(key);
    }
  });

  it("has non-empty values for all settings.* keys", () => {
    for (const key of EXPECTED_SETTINGS_KEYS) {
      const value = EN_CATALOG[key];
      expect(value, `Empty value for key "${key}"`).toBeTruthy();
    }
  });
});

// ---------------------------------------------------------------------------
// pt-BR catalog: all settings.* keys must also exist in pt-BR.json
// ---------------------------------------------------------------------------

const PT_BR_CATALOG = JSON.parse(
  readFileSync(resolve(FRONTEND_ROOT, "src/lib/i18n/pt-BR.json"), "utf8"),
) as Record<string, string>;

describe("pt-BR catalog completeness", () => {
  it("has all expected settings.* keys in pt-BR.json", () => {
    for (const key of EXPECTED_SETTINGS_KEYS) {
      expect(
        PT_BR_CATALOG,
        `Missing i18n key: "${key}" in pt-BR.json`,
      ).toHaveProperty(key);
    }
  });
});

// ---------------------------------------------------------------------------
// nav sections: settings must be registered for eligible roles
// ---------------------------------------------------------------------------

import { filterNavSections } from "$lib/nav/sections";

describe("nav sections for settings", () => {
  it("includes /settings for Leader role", () => {
    const sections = filterNavSections("Leader");
    expect(sections.some((s) => s.href === "/settings")).toBe(true);
  });

  it("includes /settings for Engineer role", () => {
    const sections = filterNavSections("Engineer");
    expect(sections.some((s) => s.href === "/settings")).toBe(true);
  });

  it("excludes /settings for Overseer role", () => {
    const sections = filterNavSections("Overseer");
    expect(sections.some((s) => s.href === "/settings")).toBe(false);
  });
});
