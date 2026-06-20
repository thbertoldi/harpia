# Harpia UX Realignment — M1 Foundation Cleanup — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Scaffold the new IA surfaces (routes, sidebar, persona mode) behind a feature flag, with no user-visible behavior change when the flag is off. Sets the foundation for M2-M6 to fill in.

**Architecture:** Add a tiny feature-flag helper, a persona-mode storage helper (operator/admin), a parallel M1 nav module that lives alongside the existing `sections.ts`, and stub `+page.svelte` files at every new route from the spec. The layout component picks the nav source (legacy vs M1) based on the flag and renders a persona toggle in the user menu when the user has admin-side permissions. Zero deletions in M1 — legacy routes and the original sidebar continue to work unchanged when the flag is off.

**Tech Stack:** SvelteKit 2.16 + Svelte 5 (runes), TypeScript 5.7, Tailwind v4 with the Harpia design-token system (`talon-gold`, `obsidian`, `obsidian-light`, `cream`, `crown-ash`), Vitest for unit tests, Lucide icons.

## Global Constraints

- **Vocabulary is canonical** per the design spec (`docs/superpowers/specs/2026-06-19-harpia-ux-realignment-design.md` §2). User-facing copy uses *Task / Plan / Agent / Integration / Executor / Overseer / Elicitation / Approval / Feedback / Artifact*. Do not introduce variants like "Elicitation Request" or "Step" in UI strings.
- **No legacy route deletions in M1.** Every existing `+page.svelte` under `/oversee`, `/elicitations`, `/approvals`, `/plans/*`, `/integrations`, `/agents`, `/audit`, `/settings`, `/tasks` continues to exist and function unchanged.
- **Flag-off behavior is byte-identical to today.** When `PUBLIC_FEATURE_UX_REALIGNMENT_M1` is unset or anything other than `'true'`, the layout renders exactly today's sidebar and no persona toggle appears.
- **Operator persona sees zero admin items in the sidebar** — even if the underlying user has `manageIntegrations` etc. The sidebar is a *persona* surface, not a *permission* surface.
- **Design tokens are locked** (per project memory). Use existing token classes (`bg-obsidian-light`, `text-talon-gold`, `text-crown-ash`, `text-cream`). Do not introduce new colors.
- **Tests live next to source** as `<name>.test.ts` files. Run with `pnpm --filter harpia-frontend test` (or `npm run test` from `frontend/`).
- **Lockstep i18n.** Every new key added to `en.json` MUST be added to `pt-BR.json` in the same commit.
- **Commit per task.** Use Conventional Commits with the `feat(ux-m1):` / `chore(ux-m1):` / `test(ux-m1):` scope.
- **`/plans/[id]` is intentionally untouched in M1.** The route already exists as the legacy plan detail page; M3 will repurpose it into the chat thread. Modifying it in M1 would break existing flag-off behavior, which violates the global "flag-off is byte-identical" constraint.

---

## File map (all changes in M1)

**Create:**
- `frontend/src/lib/flags/index.ts` — `hasFeature(flag): boolean`
- `frontend/src/lib/flags/flags.test.ts`
- `frontend/src/lib/personas/storage.ts` — `PersonaMode` type, `readPersonaMode`, `writePersonaMode`, `canSwitchToAdminPersona`
- `frontend/src/lib/personas/storage.test.ts`
- `frontend/src/lib/nav/sections-m1.ts` — `resolveNavSectionsM1(personaMode, role, translate)`
- `frontend/src/lib/nav/sections-m1.test.ts`
- `frontend/src/routes/inbox/+page.svelte` — stub
- `frontend/src/routes/new/+page.svelte` — stub
- `frontend/src/routes/discover/+page.svelte` — stub
- `frontend/src/routes/plans/[templateId]/canvas/+page.svelte` — stub
- `frontend/src/routes/admin/integrations/+page.ts` — redirect loader
- `frontend/src/routes/admin/agents/+page.ts` — redirect loader
- `frontend/src/routes/admin/audit/+page.ts` — redirect loader
- `frontend/src/routes/admin/settings/+page.ts` — redirect loader

**Modify:**
- `frontend/src/lib/i18n/en.json` — add 11 new `nav.*` keys
- `frontend/src/lib/i18n/pt-BR.json` — add the same 11 keys
- `frontend/src/routes/+layout.svelte` — read flag, derive persona state, pick nav source, render persona toggle in header
- `frontend/.env.example` (create if absent) — document `PUBLIC_FEATURE_UX_REALIGNMENT_M1`

---

## Task 1: Feature-flag helper

**Files:**
- Create: `frontend/src/lib/flags/index.ts`
- Test: `frontend/src/lib/flags/flags.test.ts`

**Interfaces:**
- Produces: `type FlagName = 'uxRealignment.m1'`, `function hasFeature(flag: FlagName): boolean`
- Consumes: `env` from `$env/dynamic/public` (SvelteKit-provided)

- [ ] **Step 1: Write the failing test**

Create `frontend/src/lib/flags/flags.test.ts`:

```ts
import { describe, expect, it, vi, beforeEach } from "vitest";

const envMock = vi.hoisted(() => ({ env: {} as Record<string, string> }));
vi.mock("$env/dynamic/public", () => envMock);

describe("hasFeature", () => {
  beforeEach(() => {
    envMock.env = {};
  });

  it("returns false for uxRealignment.m1 when env var unset", async () => {
    const { hasFeature } = await import("./index");
    expect(hasFeature("uxRealignment.m1")).toBe(false);
  });

  it("returns true for uxRealignment.m1 when env var is 'true'", async () => {
    envMock.env.PUBLIC_FEATURE_UX_REALIGNMENT_M1 = "true";
    vi.resetModules();
    const { hasFeature } = await import("./index");
    expect(hasFeature("uxRealignment.m1")).toBe(true);
  });

  it("returns false for uxRealignment.m1 when env var is non-'true' string", async () => {
    envMock.env.PUBLIC_FEATURE_UX_REALIGNMENT_M1 = "1";
    vi.resetModules();
    const { hasFeature } = await import("./index");
    expect(hasFeature("uxRealignment.m1")).toBe(false);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd frontend && npx vitest run src/lib/flags/flags.test.ts
```

Expected: FAIL — `Cannot find module './index'`.

- [ ] **Step 3: Write minimal implementation**

Create `frontend/src/lib/flags/index.ts`:

```ts
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
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd frontend && npx vitest run src/lib/flags/flags.test.ts
```

Expected: 3 tests pass.

- [ ] **Step 5: Document the env var**

If `frontend/.env.example` exists, append the line. If not, create it:

```bash
# UX Realignment milestones — see docs/superpowers/specs/2026-06-19-harpia-ux-realignment-design.md
PUBLIC_FEATURE_UX_REALIGNMENT_M1=false
```

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/flags/ frontend/.env.example
git commit -m "feat(ux-m1): add feature-flag helper"
```

---

## Task 2: Persona-mode storage helpers

**Files:**
- Create: `frontend/src/lib/personas/storage.ts`
- Test: `frontend/src/lib/personas/storage.test.ts`

**Interfaces:**
- Consumes: `hasPermission` from `$lib/auth-roles`
- Produces:
  - `type PersonaMode = 'operator' | 'admin'`
  - `const PERSONA_STORAGE_KEY = 'harpia.personaMode'`
  - `function readPersonaMode(storage: Pick<Storage, 'getItem'>): PersonaMode`
  - `function writePersonaMode(storage: Pick<Storage, 'setItem'>, mode: PersonaMode): void`
  - `function canSwitchToAdminPersona(role: string | undefined): boolean`

- [ ] **Step 1: Write the failing test**

Create `frontend/src/lib/personas/storage.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import {
  PERSONA_STORAGE_KEY,
  canSwitchToAdminPersona,
  readPersonaMode,
  writePersonaMode,
} from "./storage";

function makeStorage(initial: Record<string, string> = {}) {
  const state = { ...initial };
  return {
    getItem: (k: string) => state[k] ?? null,
    setItem: (k: string, v: string) => {
      state[k] = v;
    },
    snapshot: () => ({ ...state }),
  };
}

describe("readPersonaMode", () => {
  it("defaults to operator when nothing is stored", () => {
    expect(readPersonaMode(makeStorage())).toBe("operator");
  });

  it("returns admin when the stored value is 'admin'", () => {
    const s = makeStorage({ [PERSONA_STORAGE_KEY]: "admin" });
    expect(readPersonaMode(s)).toBe("admin");
  });

  it("returns operator for any non-'admin' stored value", () => {
    const s = makeStorage({ [PERSONA_STORAGE_KEY]: "garbage" });
    expect(readPersonaMode(s)).toBe("operator");
  });
});

describe("writePersonaMode", () => {
  it("persists the mode under the storage key", () => {
    const s = makeStorage();
    writePersonaMode(s, "admin");
    expect(s.snapshot()[PERSONA_STORAGE_KEY]).toBe("admin");
  });
});

describe("canSwitchToAdminPersona", () => {
  it("returns true for Leader (has all admin permissions)", () => {
    expect(canSwitchToAdminPersona("Leader")).toBe(true);
  });

  it("returns true for Engineer (has all admin permissions)", () => {
    expect(canSwitchToAdminPersona("Engineer")).toBe(true);
  });

  it("returns true for Overseer (holds viewAudit, which is admin-side)", () => {
    expect(canSwitchToAdminPersona("Overseer")).toBe(true);
  });

  it("returns false for undefined role", () => {
    expect(canSwitchToAdminPersona(undefined)).toBe(false);
  });

  it("returns false for unknown role", () => {
    expect(canSwitchToAdminPersona("member")).toBe(false);
  });
});
```

Note: the Overseer test asserts `true` because `viewAudit` is an admin-side permission and the admin sidebar shows `/admin/audit`. If a user has any one of the four admin permissions, they can switch.

- [ ] **Step 2: Run test to verify it fails**

```bash
cd frontend && npx vitest run src/lib/personas/storage.test.ts
```

Expected: FAIL — `Cannot find module './storage'`.

- [ ] **Step 3: Write minimal implementation**

Create `frontend/src/lib/personas/storage.ts`:

```ts
import { hasPermission, type HarpiaPermission } from "$lib/auth-roles";

export type PersonaMode = "operator" | "admin";

export const PERSONA_STORAGE_KEY = "harpia.personaMode";

const ADMIN_PERMISSIONS: HarpiaPermission[] = [
  "manageIntegrations",
  "manageAgents",
  "viewAudit",
  "manageTenantSettings",
];

export function readPersonaMode(
  storage: Pick<Storage, "getItem">,
): PersonaMode {
  return storage.getItem(PERSONA_STORAGE_KEY) === "admin" ? "admin" : "operator";
}

export function writePersonaMode(
  storage: Pick<Storage, "setItem">,
  mode: PersonaMode,
): void {
  storage.setItem(PERSONA_STORAGE_KEY, mode);
}

/**
 * A user can switch into the admin persona when they hold at least one
 * admin-side permission. Operator persona is always available.
 */
export function canSwitchToAdminPersona(role: string | undefined): boolean {
  if (!role) return false;
  return ADMIN_PERMISSIONS.some((p) => hasPermission({ role }, p));
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd frontend && npx vitest run src/lib/personas/storage.test.ts
```

Expected: 7 tests pass.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/personas/
git commit -m "feat(ux-m1): add persona-mode storage helpers"
```

---

## Task 3: M1 navigation module

**Files:**
- Create: `frontend/src/lib/nav/sections-m1.ts`
- Test: `frontend/src/lib/nav/sections-m1.test.ts`

**Interfaces:**
- Consumes:
  - `PersonaMode` from `$lib/personas/storage`
  - `hasPermission` from `$lib/auth-roles`
  - `ResolvedNavSection`, `NavIcon` from `./types`
- Produces: `function resolveNavSectionsM1(personaMode: PersonaMode, role: string | undefined, translateKey: (k: string) => string): ResolvedNavSection[]`

- [ ] **Step 1: Write the failing test**

Create `frontend/src/lib/nav/sections-m1.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { resolveNavSectionsM1 } from "./sections-m1";

const translate = (key: string) => `t:${key}`;

describe("resolveNavSectionsM1 — operator persona", () => {
  it("returns the operator sections regardless of role", () => {
    const hrefs = resolveNavSectionsM1("operator", "Leader", translate).map(
      (s) => s.href,
    );
    expect(hrefs).toEqual(["/inbox", "/discover", "/new"]);
  });

  it("returns operator sections even for an unknown role", () => {
    const hrefs = resolveNavSectionsM1("operator", "member", translate).map(
      (s) => s.href,
    );
    expect(hrefs).toEqual(["/inbox", "/discover", "/new"]);
  });

  it("translates labels via the callback", () => {
    const labels = resolveNavSectionsM1("operator", "Leader", translate).map(
      (s) => s.label,
    );
    expect(labels).toEqual(["t:nav.needsYou", "t:nav.discover", "t:nav.newPlan"]);
  });
});

describe("resolveNavSectionsM1 — admin persona", () => {
  it("returns all admin sections for Leader", () => {
    const hrefs = resolveNavSectionsM1("admin", "Leader", translate).map(
      (s) => s.href,
    );
    expect(hrefs).toEqual([
      "/admin/integrations",
      "/admin/agents",
      "/admin/audit",
      "/admin/settings",
    ]);
  });

  it("returns all admin sections for Engineer", () => {
    const hrefs = resolveNavSectionsM1("admin", "Engineer", translate).map(
      (s) => s.href,
    );
    expect(hrefs).toEqual([
      "/admin/integrations",
      "/admin/agents",
      "/admin/audit",
      "/admin/settings",
    ]);
  });

  it("returns only audit for Overseer (viewAudit only)", () => {
    const hrefs = resolveNavSectionsM1("admin", "Overseer", translate).map(
      (s) => s.href,
    );
    expect(hrefs).toEqual(["/admin/audit"]);
  });

  it("returns empty array for unknown role", () => {
    expect(resolveNavSectionsM1("admin", "member", translate)).toEqual([]);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd frontend && npx vitest run src/lib/nav/sections-m1.test.ts
```

Expected: FAIL — `Cannot find module './sections-m1'`.

- [ ] **Step 3: Write minimal implementation**

Create `frontend/src/lib/nav/sections-m1.ts`:

```ts
import { Bot, Compass, Inbox, Plug, Plus, ScrollText, Settings } from "lucide-svelte";
import { hasPermission, type HarpiaPermission } from "$lib/auth-roles";
import type { PersonaMode } from "$lib/personas/storage";
import type { NavIcon, ResolvedNavSection } from "./types";

interface M1Section {
  i18nKey: string;
  href: string;
  icon: NavIcon;
}

const OPERATOR_SECTIONS: M1Section[] = [
  { i18nKey: "nav.needsYou", href: "/inbox", icon: Inbox },
  { i18nKey: "nav.discover", href: "/discover", icon: Compass },
  { i18nKey: "nav.newPlan", href: "/new", icon: Plus },
];

const ADMIN_SECTIONS: Array<M1Section & { requiredPermission: HarpiaPermission }> = [
  {
    i18nKey: "nav.adminIntegrations",
    href: "/admin/integrations",
    icon: Plug,
    requiredPermission: "manageIntegrations",
  },
  {
    i18nKey: "nav.adminAgents",
    href: "/admin/agents",
    icon: Bot,
    requiredPermission: "manageAgents",
  },
  {
    i18nKey: "nav.adminAudit",
    href: "/admin/audit",
    icon: ScrollText,
    requiredPermission: "viewAudit",
  },
  {
    i18nKey: "nav.adminSettings",
    href: "/admin/settings",
    icon: Settings,
    requiredPermission: "manageTenantSettings",
  },
];

export function resolveNavSectionsM1(
  personaMode: PersonaMode,
  role: string | undefined,
  translateKey: (key: string) => string,
): ResolvedNavSection[] {
  if (personaMode === "operator") {
    return OPERATOR_SECTIONS.map(({ i18nKey, href, icon }) => ({
      href,
      icon,
      label: translateKey(i18nKey),
    }));
  }
  return ADMIN_SECTIONS.filter((def) =>
    hasPermission({ role }, def.requiredPermission),
  ).map(({ i18nKey, href, icon }) => ({
    href,
    icon,
    label: translateKey(i18nKey),
  }));
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd frontend && npx vitest run src/lib/nav/sections-m1.test.ts
```

Expected: 8 tests pass.

- [ ] **Step 5: Run the full test suite to confirm no regressions**

```bash
cd frontend && npm run test
```

Expected: all existing tests still pass (the existing `nav.test.ts` was not modified).

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/nav/sections-m1.ts frontend/src/lib/nav/sections-m1.test.ts
git commit -m "feat(ux-m1): add M1 navigation module"
```

---

## Task 4: i18n keys for new navigation

**Files:**
- Modify: `frontend/src/lib/i18n/en.json` (after the existing `nav.*` block, ~line 16)
- Modify: `frontend/src/lib/i18n/pt-BR.json` (same position)

**Interfaces:**
- Produces: new translatable keys `nav.needsYou`, `nav.discover`, `nav.newPlan`, `nav.adminIntegrations`, `nav.adminAgents`, `nav.adminAudit`, `nav.adminSettings`, `nav.personaToggle`, `nav.personaOperator`, `nav.personaAdmin`, `nav.stubComingSoon`

- [ ] **Step 1: Check existing i18n contract test**

Inspect `frontend/src/lib/i18n/hardcoded-copy.test.ts` to confirm what it asserts. If it asserts the two locales have identical key sets, this task is implicitly tested by Step 4.

```bash
cd frontend && cat src/lib/i18n/hardcoded-copy.test.ts
```

If the contract test does NOT enforce key-set parity, write one in this step before adding keys:

```ts
// at the end of frontend/src/lib/i18n/content.test.ts (or new file)
import en from "./en.json";
import pt from "./pt-BR.json";

it("en.json and pt-BR.json have identical key sets", () => {
  expect(Object.keys(pt).sort()).toEqual(Object.keys(en).sort());
});
```

- [ ] **Step 2: Add keys to en.json**

Open `frontend/src/lib/i18n/en.json` and insert after line 16 (after `"nav.language"`):

```json
  "nav.needsYou": "Needs you",
  "nav.discover": "Discover",
  "nav.newPlan": "New plan",
  "nav.adminIntegrations": "Integrations",
  "nav.adminAgents": "Agents",
  "nav.adminAudit": "Audit log",
  "nav.adminSettings": "Tenant settings",
  "nav.personaToggle": "Switch mode",
  "nav.personaOperator": "Operator",
  "nav.personaAdmin": "Platform Engineer",
  "nav.stubComingSoon": "Coming in a later milestone",
```

- [ ] **Step 3: Add the same keys to pt-BR.json**

Open `frontend/src/lib/i18n/pt-BR.json` and insert the same block in the same position with Portuguese values:

```json
  "nav.needsYou": "Sua atenção",
  "nav.discover": "Descobrir",
  "nav.newPlan": "Novo plano",
  "nav.adminIntegrations": "Integrações",
  "nav.adminAgents": "Agentes",
  "nav.adminAudit": "Registro de auditoria",
  "nav.adminSettings": "Configurações do tenant",
  "nav.personaToggle": "Trocar modo",
  "nav.personaOperator": "Operador",
  "nav.personaAdmin": "Engenheiro de plataforma",
  "nav.stubComingSoon": "Disponível em um marco futuro",
```

- [ ] **Step 4: Run the i18n test suite**

```bash
cd frontend && npx vitest run src/lib/i18n/
```

Expected: all i18n tests pass, including the en/pt parity check from Step 1.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/i18n/
git commit -m "feat(ux-m1): add nav i18n keys for new IA"
```

---

## Task 5: Operator route stubs

**Files:**
- Create: `frontend/src/routes/inbox/+page.svelte`
- Create: `frontend/src/routes/new/+page.svelte`
- Create: `frontend/src/routes/discover/+page.svelte`
- Create: `frontend/src/routes/plans/[templateId]/canvas/+page.svelte`

**Interfaces:**
- Each stub renders a minimal "Coming in M{N}" placeholder using the `nav.stubComingSoon` key from Task 4. No data loading. No external dependencies.

- [ ] **Step 1: Add a shared stub-page snippet**

Create `frontend/src/lib/components/MilestoneStub.svelte`:

```svelte
<script lang="ts">
  import { Construction } from "lucide-svelte";
  import { locale, translate } from "$lib/i18n";

  interface Props {
    title: string;
    milestone: string;
  }

  let { title, milestone }: Props = $props();
</script>

<div class="mx-auto flex max-w-3xl flex-col items-center gap-4 px-4 py-24 text-center">
  <Construction class="size-10 text-talon-gold" />
  <h1 class="font-heading text-2xl text-cream">{title}</h1>
  <p class="text-crown-ash">{translate("nav.stubComingSoon", $locale)} ({milestone})</p>
</div>
```

- [ ] **Step 2: Write the failing test (route smoke)**

Skip — SvelteKit pages are hard to unit-test in isolation without Playwright. Coverage for stub pages comes from Task 8 (end-to-end verification).

- [ ] **Step 3: Create `/inbox` stub**

Create `frontend/src/routes/inbox/+page.svelte`:

```svelte
<script lang="ts">
  import MilestoneStub from "$lib/components/MilestoneStub.svelte";
  import { locale, translate } from "$lib/i18n";
</script>

<svelte:head>
  <title>{translate("nav.needsYou", $locale)} · Harpia</title>
</svelte:head>

<MilestoneStub title={translate("nav.needsYou", $locale)} milestone="M2" />
```

- [ ] **Step 4: Create `/new` stub**

Create `frontend/src/routes/new/+page.svelte`:

```svelte
<script lang="ts">
  import MilestoneStub from "$lib/components/MilestoneStub.svelte";
  import { locale, translate } from "$lib/i18n";
</script>

<svelte:head>
  <title>{translate("nav.newPlan", $locale)} · Harpia</title>
</svelte:head>

<MilestoneStub title={translate("nav.newPlan", $locale)} milestone="M5" />
```

- [ ] **Step 5: Create `/discover` stub**

Create `frontend/src/routes/discover/+page.svelte`:

```svelte
<script lang="ts">
  import MilestoneStub from "$lib/components/MilestoneStub.svelte";
  import { locale, translate } from "$lib/i18n";
</script>

<svelte:head>
  <title>{translate("nav.discover", $locale)} · Harpia</title>
</svelte:head>

<MilestoneStub title={translate("nav.discover", $locale)} milestone="M5" />
```

- [ ] **Step 6: Create `/plans/[templateId]/canvas` stub**

Create `frontend/src/routes/plans/[templateId]/canvas/+page.svelte`:

```svelte
<script lang="ts">
  import MilestoneStub from "$lib/components/MilestoneStub.svelte";
</script>

<svelte:head>
  <title>Canvas · Harpia</title>
</svelte:head>

<MilestoneStub title="Plan canvas" milestone="M4" />
```

- [ ] **Step 7: Verify routes type-check**

```bash
cd frontend && npm run check
```

Expected: zero new errors. If `svelte-kit sync` complains about types, run it first: `cd frontend && npx svelte-kit sync`.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/lib/components/MilestoneStub.svelte frontend/src/routes/inbox frontend/src/routes/new frontend/src/routes/discover frontend/src/routes/plans/[templateId]/canvas
git commit -m "feat(ux-m1): scaffold operator-side route stubs"
```

---

## Task 6: Admin route stubs (redirect-only)

**Files:**
- Create: `frontend/src/routes/admin/integrations/+page.ts`
- Create: `frontend/src/routes/admin/agents/+page.ts`
- Create: `frontend/src/routes/admin/audit/+page.ts`
- Create: `frontend/src/routes/admin/settings/+page.ts`

**Interfaces:**
- Each `+page.ts` exports a SvelteKit `load` function that 302-redirects to the corresponding legacy route. This keeps the existing admin functionality intact during M1; M6 will swap the redirects for real pages and delete the legacy routes.
- Produces: four navigable routes under `/admin/*` that resolve to the legacy admin pages.

- [ ] **Step 1: Create `/admin/integrations` redirect**

Create `frontend/src/routes/admin/integrations/+page.ts`:

```ts
import { redirect } from "@sveltejs/kit";
import type { PageLoad } from "./$types";

export const load: PageLoad = () => {
  throw redirect(302, "/integrations");
};
```

- [ ] **Step 2: Create `/admin/agents` redirect**

Create `frontend/src/routes/admin/agents/+page.ts`:

```ts
import { redirect } from "@sveltejs/kit";
import type { PageLoad } from "./$types";

export const load: PageLoad = () => {
  throw redirect(302, "/agents");
};
```

- [ ] **Step 3: Create `/admin/audit` redirect**

Create `frontend/src/routes/admin/audit/+page.ts`:

```ts
import { redirect } from "@sveltejs/kit";
import type { PageLoad } from "./$types";

export const load: PageLoad = () => {
  throw redirect(302, "/audit");
};
```

- [ ] **Step 4: Create `/admin/settings` redirect**

Create `frontend/src/routes/admin/settings/+page.ts`:

```ts
import { redirect } from "@sveltejs/kit";
import type { PageLoad } from "./$types";

export const load: PageLoad = () => {
  throw redirect(302, "/settings");
};
```

- [ ] **Step 5: Verify routes type-check**

```bash
cd frontend && npx svelte-kit sync && npm run check
```

Expected: zero new errors.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/routes/admin/
git commit -m "feat(ux-m1): scaffold admin-side route redirects"
```

---

## Task 7: Sidebar layout integration

**Files:**
- Modify: `frontend/src/routes/+layout.svelte`

**Interfaces:**
- Consumes:
  - `hasFeature` from `$lib/flags`
  - `readPersonaMode`, `writePersonaMode`, `canSwitchToAdminPersona`, `type PersonaMode` from `$lib/personas/storage`
  - `resolveNavSectionsM1` from `$lib/nav/sections-m1`
- Behavior contract:
  - When `hasFeature('uxRealignment.m1')` is false → sidebar identical to today, no persona toggle.
  - When the flag is true and the user is operator → sidebar shows only operator items, persona toggle visible iff `canSwitchToAdminPersona(role)` is true.
  - When the flag is true and the user is admin → sidebar shows admin items filtered by permission, persona toggle visible (always — switching back to operator is always allowed).
  - Persona toggle button label uses `nav.personaToggle`; the displayed mode label uses `nav.personaOperator` / `nav.personaAdmin`.
  - Switching mode persists to localStorage and updates the sidebar immediately.

- [ ] **Step 1: Read the current layout to identify insertion points**

```bash
cd frontend && cat src/routes/+layout.svelte | head -80
```

Locate the `$derived(resolveNavSections(...))` line (around line 50-52 per the existing structure) and the user-menu / header region where language and theme toggles live (sticky header section).

- [ ] **Step 2: Add the flag-aware nav resolution**

In the `<script lang="ts">` block at the top of `frontend/src/routes/+layout.svelte`, ADD these imports:

```ts
import { hasFeature } from "$lib/flags";
import {
  canSwitchToAdminPersona,
  readPersonaMode,
  writePersonaMode,
  type PersonaMode,
} from "$lib/personas/storage";
import { resolveNavSectionsM1 } from "$lib/nav/sections-m1";
```

REPLACE the existing `const sections = $derived(...)` block with:

```ts
const m1Enabled = $derived(hasFeature("uxRealignment.m1"));

let personaMode = $state<PersonaMode>("operator");

$effect(() => {
  if (m1Enabled && typeof localStorage !== "undefined") {
    personaMode = readPersonaMode(localStorage);
  }
});

const canSwitchPersona = $derived(
  m1Enabled && canSwitchToAdminPersona(data?.user?.role),
);

const sections = $derived(
  m1Enabled
    ? resolveNavSectionsM1(
        personaMode,
        data?.user?.role,
        (key) => translate(key, $locale),
      )
    : resolveNavSections(
        data?.user?.role,
        (key) => translate(key, $locale),
      ),
);

function togglePersona() {
  const next: PersonaMode = personaMode === "operator" ? "admin" : "operator";
  personaMode = next;
  if (typeof localStorage !== "undefined") {
    writePersonaMode(localStorage, next);
  }
}
```

- [ ] **Step 3: Add the persona toggle to the header**

Find the sticky header region (where language / theme toggles render). Add a button in that region, BEFORE the language toggle:

```svelte
{#if canSwitchPersona}
  <button
    type="button"
    onclick={togglePersona}
    class="flex items-center gap-2 rounded-md border border-plumage bg-obsidian-light px-3 py-1.5 text-xs text-crown-ash hover:border-talon-gold hover:text-talon-gold"
    aria-label={translate("nav.personaToggle", $locale)}
    title={translate("nav.personaToggle", $locale)}
  >
    <span class="font-medium uppercase tracking-wider">
      {personaMode === "operator"
        ? translate("nav.personaOperator", $locale)
        : translate("nav.personaAdmin", $locale)}
    </span>
    <span class="text-talon-gold">⇄</span>
  </button>
{/if}
```

(The exact placement is "before language toggle in the header right-side cluster" — preserve existing layout classes.)

- [ ] **Step 4: Remove the hardcoded FeedbackBadge from M1 nav items**

The existing iteration block contains `{#if section.href === "/oversee"}<FeedbackBadge />{/if}`. The M1 nav doesn't have `/oversee` (it has `/inbox`). The conditional is href-specific so it harmlessly evaluates to false for M1 items — leave it untouched. It will be replaced in M2 when the inbox badge ships.

- [ ] **Step 5: Run all tests**

```bash
cd frontend && npm run test
```

Expected: all tests pass (including the new flags, personas, and sections-m1 suites).

- [ ] **Step 6: Run type check**

```bash
cd frontend && npm run check
```

Expected: zero errors.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/routes/+layout.svelte
git commit -m "feat(ux-m1): integrate persona-aware sidebar behind flag"
```

---

## Task 8: End-to-end verification in the browser

**Files:** None modified — this is a manual verification task. The deliverable is a verification log appended to the plan as part of the commit message.

**Interfaces:**
- Verifies: flag-off renders today's sidebar; flag-on operator renders 3 items; flag-on admin renders permission-filtered admin items; persona toggle persists across reloads; all new routes resolve.

- [ ] **Step 1: Start the dev server with the flag OFF**

```bash
cd frontend && npm run dev
```

In a browser, navigate to `http://localhost:5173/`. Log in with the dev `Leader` persona via `devLogin('Leader')`.

Expected: sidebar shows today's 10 items (Tasks / Ongoing / Oversee / Elicitations / Approvals / Plans / Integrations / Audit / Agents / Settings). No persona toggle in the header. **Stop the server (Ctrl-C).**

- [ ] **Step 2: Start the dev server with the flag ON**

```bash
cd frontend && PUBLIC_FEATURE_UX_REALIGNMENT_M1=true npm run dev
```

Refresh the browser. Log in as Leader (clear cookies first if needed).

Expected:
- Sidebar shows **3 items**: Needs you, Discover, New plan.
- Header shows the persona toggle reading "OPERATOR ⇄".
- Clicking each sidebar item navigates to the corresponding stub page showing "Coming in M{N}".

- [ ] **Step 3: Toggle to admin persona**

Click the persona toggle. It should flip to "PLATFORM ENGINEER ⇄" and the sidebar should now show 4 items: Integrations, Agents, Audit log, Tenant settings.

Click each admin item — each should redirect to the corresponding legacy route (`/integrations`, `/agents`, `/audit`, `/settings`) and the legacy page renders.

- [ ] **Step 4: Verify persistence**

Reload the page. The sidebar should remain on admin mode (localStorage persisted).

- [ ] **Step 5: Verify Overseer permission filtering**

Log out, log in as `Overseer`. Toggle to admin mode. The sidebar should show only **Audit log** (Overseer has `viewAudit` but no other admin permissions).

- [ ] **Step 6: Verify hide-toggle for unprivileged users**

If a `member` (or unknown) dev persona is available, log in as that. The persona toggle should NOT render (`canSwitchToAdminPersona` returns false). If no such persona exists, this assertion is covered by Task 2's unit test.

- [ ] **Step 7: Stop the server, write verification notes**

In a new file `docs/superpowers/plans/2026-06-19-harpia-ux-realignment-m1-foundation.verification.md`, log:

```markdown
# M1 verification — <date>

- Flag OFF: legacy sidebar unchanged ✓
- Flag ON, Leader, operator: 3 sidebar items, persona toggle visible ✓
- Flag ON, Leader, admin: 4 sidebar items, all admin routes redirect to legacy ✓
- Persona persistence across reload ✓
- Flag ON, Overseer, admin: only Audit log visible ✓
- Flag ON, unprivileged role: toggle hidden ✓
- All four operator stubs (/inbox, /new, /discover, /plans/[id]/canvas) render the milestone placeholder ✓
```

Replace `<date>` with the actual date.

- [ ] **Step 8: Final commit**

```bash
git add docs/superpowers/plans/2026-06-19-harpia-ux-realignment-m1-foundation.verification.md
git commit -m "chore(ux-m1): record M1 verification notes"
```

- [ ] **Step 9: Confirm clean state**

```bash
cd frontend && npm run lint && npm run test && npm run check
```

Expected: all three pass.

---

## Done criteria

M1 is complete when all of the following hold:

1. `PUBLIC_FEATURE_UX_REALIGNMENT_M1=false` (or unset) → frontend is byte-identical to pre-M1.
2. `PUBLIC_FEATURE_UX_REALIGNMENT_M1=true` → sidebar is persona-aware, with operator/admin modes that filter by permission.
3. All eight new routes (`/inbox`, `/new`, `/discover`, `/plans/[id]/canvas`, `/admin/{integrations,agents,audit,settings}`) resolve without 404.
4. Admin routes redirect 302 to their legacy counterparts.
5. Persona mode persists via localStorage and survives reloads.
6. All tests pass (`npm run test`), type-check passes (`npm run check`), lint passes (`npm run lint`).
7. Zero legacy routes deleted; zero permission changes; zero backend changes.

After M1 ships, M2 (unified inbox) can begin against the same flag, expanding it from "scaffold present" to "scaffold has real functionality."
