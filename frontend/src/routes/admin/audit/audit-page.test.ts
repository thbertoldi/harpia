import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { canViewAudit } from "$lib/auth-roles";
import { AUDIT_EVENT_TYPES } from "$lib/audit/audit-store";
import {
  auditBoundedContextName,
  auditEventTypeName,
  auditSubjectTypeName,
} from "$lib/audit/audit-store";
import {
  AuditSubjectType,
  BoundedContext,
} from "$lib/gen/harpia/audit/v1/audit_pb";

// ---------------------------------------------------------------------------
// AuthZ: canViewAudit permission
// ---------------------------------------------------------------------------

describe("canViewAudit AuthZ", () => {
  it("grants access to Leader, Overseer, and Engineer roles", () => {
    expect(canViewAudit({ role: "Leader" })).toBe(true);
    expect(canViewAudit({ role: "Overseer" })).toBe(true);
    expect(canViewAudit({ role: "Engineer" })).toBe(true);
  });

  it("grants access to admin aliases (mapped to Leader)", () => {
    expect(canViewAudit({ role: "admin" })).toBe(true);
    expect(canViewAudit({ role: "owner" })).toBe(true);
  });

  it("denies access when user is null or role is missing/unknown", () => {
    expect(canViewAudit(null)).toBe(false);
    expect(canViewAudit({})).toBe(false);
    expect(canViewAudit({ role: "member" })).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// Page wiring: the route uses the real adapter and no mock audit source
// ---------------------------------------------------------------------------

const FRONTEND_ROOT = resolve(import.meta.dirname, "../../../..");
const AUDIT_PAGE = resolve(
  FRONTEND_ROOT,
  "src/routes/admin/audit/+page.svelte",
);
const PAGE_SOURCE = readFileSync(AUDIT_PAGE, "utf8");

describe("audit page wiring", () => {
  it("imports data access from the generated-AuditService adapter", () => {
    expect(PAGE_SOURCE).toContain('from "$lib/audit/audit-store"');
  });

  it("does not import the deleted mock audit dataset", () => {
    expect(PAGE_SOURCE).not.toContain("mocks/audit-events");
    expect(PAGE_SOURCE).not.toContain("appendMockAuditEvent");
    expect(PAGE_SOURCE).not.toContain("localStorage");
  });

  it("renders the typed related subject instead of legacy task labeling", () => {
    expect(PAGE_SOURCE).not.toContain("event.taskId");
    expect(PAGE_SOURCE).not.toContain("data-task-id");
    expect(PAGE_SOURCE).toContain("subjectLabel");
    expect(PAGE_SOURCE).toContain("data-subject-id");
  });

  it("exposes the API event-type filter", () => {
    expect(PAGE_SOURCE).toContain("AUDIT_EVENT_TYPES");
    expect(PAGE_SOURCE).toContain("audit-filter-event-type");
  });
});

// ---------------------------------------------------------------------------
// i18n catalog completeness: every audit.* key referenced by the page and
// adapter exists in both en and pt-BR, with non-empty values.
// ---------------------------------------------------------------------------

const EN_CATALOG = JSON.parse(
  readFileSync(resolve(FRONTEND_ROOT, "src/lib/i18n/en.json"), "utf8"),
) as Record<string, string>;

const PT_BR_CATALOG = JSON.parse(
  readFileSync(resolve(FRONTEND_ROOT, "src/lib/i18n/pt-BR.json"), "utf8"),
) as Record<string, string>;

const STATIC_AUDIT_KEYS = [
  "audit.heading",
  "audit.subheading",
  "audit.exportCsv",
  "audit.exportJson",
  "audit.filters",
  "audit.apply",
  "audit.clear",
  "audit.empty.title",
  "audit.empty.description",
  "audit.pageSummary",
  "audit.field.subjectId",
  "audit.field.user",
  "audit.field.agentType",
  "audit.field.eventType",
  "audit.field.decision",
  "audit.trace",
  "audit.event",
  "audit.actor",
  "audit.payloadDiff",
  "audit.subject",
  "audit.noSubject",
];

const DYNAMIC_AUDIT_KEYS: string[] = [
  ...AUDIT_EVENT_TYPES.map((t) => `audit.eventType.${auditEventTypeName(t)}`),
  ...Object.values(BoundedContext)
    .filter((v): v is BoundedContext => typeof v === "number" && v !== 0)
    .map((c) => `audit.context.${auditBoundedContextName(c)}`),
  ...Object.values(AuditSubjectType)
    .filter((v): v is AuditSubjectType => typeof v === "number" && v !== 0)
    .map((s) => `audit.subjectType.${auditSubjectTypeName(s)}`),
  "audit.actorKind.HUMAN",
  "audit.actorKind.AGENT",
  "audit.actorKind.WORKFLOW_ENGINE",
  "audit.decision.approve",
  "audit.decision.reject",
  "audit.decision.modify",
  "audit.decision.escalate",
];

const ALL_AUDIT_KEYS = [...STATIC_AUDIT_KEYS, ...DYNAMIC_AUDIT_KEYS];

describe("audit i18n catalog completeness (en)", () => {
  it("has every audit.* key with a non-empty value in en.json", () => {
    for (const key of ALL_AUDIT_KEYS) {
      expect(
        EN_CATALOG,
        `Missing i18n key: "${key}" in en.json`,
      ).toHaveProperty(key);
      expect(
        EN_CATALOG[key],
        `Empty value for key "${key}" in en.json`,
      ).toBeTruthy();
    }
  });
});

describe("audit i18n catalog completeness (pt-BR)", () => {
  it("has every audit.* key in pt-BR.json", () => {
    for (const key of ALL_AUDIT_KEYS) {
      expect(
        PT_BR_CATALOG,
        `Missing i18n key: "${key}" in pt-BR.json`,
      ).toHaveProperty(key);
      expect(
        PT_BR_CATALOG[key],
        `Empty value for key "${key}" in pt-BR.json`,
      ).toBeTruthy();
    }
  });

  it("ships the same set of audit.* keys in en and pt-BR", () => {
    const enAudit = new Set(
      Object.keys(EN_CATALOG).filter((k) => k.startsWith("audit.")),
    );
    const ptAudit = new Set(
      Object.keys(PT_BR_CATALOG).filter((k) => k.startsWith("audit.")),
    );
    expect([...enAudit].sort()).toEqual([...ptAudit].sort());
  });
});
