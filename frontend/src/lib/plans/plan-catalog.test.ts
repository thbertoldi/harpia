import { create } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";
import {
  ConnectionStatus,
  ExecutorEntitlementSchema,
  ExecutorInstallationSchema,
  ExecutorKind,
  ExecutorSKUSchema,
  IntegrationInstallationSchema,
  AgentInstallationSchema,
} from "$lib/gen/harpia/executors/v1/executors_pb";
import {
  buildPlanCatalogEntry,
  collectRequiredSkuKeys,
  isInstallationReady,
  resolveSkuLockState,
} from "$lib/plans/plan-catalog";
import {
  mockExecutorContext,
  mockPlanTemplates,
  WEEKLY_NEWSLETTER_TEMPLATE,
} from "$lib/mocks/plan-catalog";

const RSS_SKU = create(ExecutorSKUSchema, {
  id: "sku-rss",
  key: "rss-news-feed",
  displayName: "RSS News Feed",
  description: "",
  kind: ExecutorKind.INTEGRATION,
  createdAt: "2026-06-01T10:00:00Z",
  updatedAt: "2026-06-01T10:00:00Z",
});

describe("plan catalog lock state", () => {
  it("collects unique executor SKU keys from plan steps", () => {
    const required = collectRequiredSkuKeys(WEEKLY_NEWSLETTER_TEMPLATE.steps);

    expect([...required.keys()].sort()).toEqual([
      "linkedin-publish",
      "linkedin-voice-senior",
      "newsletter-writer-senior",
      "rss-news-feed",
    ]);
    expect(required.get("fetch-news")).toBeUndefined();
    expect(required.get("rss-news-feed")).toEqual(["fetch-news"]);
  });

  it("marks SKU as missing entitlement when tenant lacks access", () => {
    const lock = resolveSkuLockState(RSS_SKU, [], []);

    expect(lock.reason).toBe("missing_entitlement");
    expect(lock.messageKey).toBe("plans.lock.message.missing_entitlement");
  });

  it("marks SKU as missing installation when entitled but not installed", () => {
    const lock = resolveSkuLockState(
      RSS_SKU,
      [
        create(ExecutorEntitlementSchema, {
          id: "ent-1",
          tenantId: "dev",
          executorSkuId: RSS_SKU.id,
          grantedAt: "2026-06-01T10:00:00Z",
        }),
      ],
      [],
    );

    expect(lock.reason).toBe("missing_installation");
  });

  it("marks integration SKU as not connected when OAuth is incomplete", () => {
    const lock = resolveSkuLockState(
      RSS_SKU,
      [
        create(ExecutorEntitlementSchema, {
          id: "ent-1",
          tenantId: "dev",
          executorSkuId: RSS_SKU.id,
          grantedAt: "2026-06-01T10:00:00Z",
        }),
      ],
      [
        create(ExecutorInstallationSchema, {
          id: "inst-1",
          tenantId: "dev",
          executorSkuId: RSS_SKU.id,
          kind: ExecutorKind.INTEGRATION,
          displayName: "RSS",
          enabled: true,
          createdAt: "2026-06-01T10:00:00Z",
          updatedAt: "2026-06-01T10:00:00Z",
          detail: {
            case: "integration",
            value: create(IntegrationInstallationSchema, {
              connectionStatus: ConnectionStatus.DISCONNECTED,
              configJson: "{}",
            }),
          },
        }),
      ],
    );

    expect(lock.reason).toBe("not_connected");
  });

  it("marks SKU as available when entitled and installation is ready", () => {
    const installation = create(ExecutorInstallationSchema, {
      id: "inst-1",
      tenantId: "dev",
      executorSkuId: RSS_SKU.id,
      kind: ExecutorKind.INTEGRATION,
      displayName: "RSS",
      enabled: true,
      createdAt: "2026-06-01T10:00:00Z",
      updatedAt: "2026-06-01T10:00:00Z",
      detail: {
        case: "integration",
        value: create(IntegrationInstallationSchema, {
          connectionStatus: ConnectionStatus.CONNECTED,
          configJson: '{"feed_urls":["https://example.com/feed.xml"]}',
        }),
      },
    });

    expect(isInstallationReady(installation)).toBe(true);

    const lock = resolveSkuLockState(
      RSS_SKU,
      [
        create(ExecutorEntitlementSchema, {
          id: "ent-1",
          tenantId: "dev",
          executorSkuId: RSS_SKU.id,
          grantedAt: "2026-06-01T10:00:00Z",
        }),
      ],
      [installation],
    );

    expect(lock.reason).toBe("available");
  });

  it("builds a locked catalog entry from mock executor context", () => {
    const entry = buildPlanCatalogEntry(
      WEEKLY_NEWSLETTER_TEMPLATE,
      mockExecutorContext(),
    );

    expect(entry.template.key).toBe("weekly-newsletter-linkedin");
    expect(entry.requiredSkus).toHaveLength(4);
    expect(entry.isLocked).toBe(true);
    expect(
      entry.requiredSkus.some((sku) => sku.reason === "missing_installation"),
    ).toBe(true);
    expect(
      entry.requiredSkus.some((sku) => sku.reason === "not_connected"),
    ).toBe(true);
    expect(entry.requiredSkus.some((sku) => sku.reason === "available")).toBe(
      true,
    );
  });

  it("localizes catalog SKUs and templates for pt-BR", () => {
    const [template] = mockPlanTemplates("pt-BR");
    const context = mockExecutorContext("dev", "pt-BR");

    expect(template.name).toBe("Newsletter Semanal (LinkedIn)");
    expect(template.steps[0]?.title).toBe("Buscar Noticias");
    expect(context.skus[0]?.displayName).toBe("Feed de Noticias RSS");
  });

  it("marks agent installation as not configured without manifest metadata", () => {
    const agentSku = create(ExecutorSKUSchema, {
      id: "sku-agent",
      key: "newsletter-writer-senior",
      displayName: "Newsletter Writer (Senior)",
      description: "",
      kind: ExecutorKind.AGENT,
      createdAt: "2026-06-01T10:00:00Z",
      updatedAt: "2026-06-01T10:00:00Z",
    });

    const lock = resolveSkuLockState(
      agentSku,
      [
        create(ExecutorEntitlementSchema, {
          id: "ent-agent",
          tenantId: "dev",
          executorSkuId: agentSku.id,
          grantedAt: "2026-06-01T10:00:00Z",
        }),
      ],
      [
        create(ExecutorInstallationSchema, {
          id: "inst-agent",
          tenantId: "dev",
          executorSkuId: agentSku.id,
          kind: ExecutorKind.AGENT,
          displayName: "Writer",
          enabled: true,
          createdAt: "2026-06-01T10:00:00Z",
          updatedAt: "2026-06-01T10:00:00Z",
          detail: {
            case: "agent",
            value: create(AgentInstallationSchema, {
              manifestId: "",
              manifestVersion: "",
            }),
          },
        }),
      ],
    );

    expect(lock.reason).toBe("not_configured");
  });
});
