import { create } from "@bufbuild/protobuf";
import {
  ConnectionStatus,
  ExecutorEntitlementSchema,
  ExecutorInstallationSchema,
  ExecutorKind as ExecutorCatalogKind,
  ExecutorSKUSchema,
  IntegrationInstallationSchema,
  AgentInstallationSchema,
  ListPriceSchema,
  type ExecutorEntitlement,
  type ExecutorInstallation,
  type ExecutorSKU,
  type ListPrice,
} from "$lib/gen/harpia/executors/v1/executors_pb";
import {
  ExecutorKind as PlanExecutorKind,
  ExecutorRequirementSchema,
  PlanStepDependencySchema,
  PlanStepSchema,
  PlanTemplateSchema,
  type PlanTemplate,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import type { Locale } from "$lib/i18n";
import { resolveLocalizedContent } from "$lib/i18n/content";

export interface MockExecutorContext {
  skus: ExecutorSKU[];
  entitlements: ExecutorEntitlement[];
  installations: ExecutorInstallation[];
}

export const SKU_IDS = {
  rssNewsFeed: "b1000000-0000-4000-8000-000000000001",
  newsletterWriter: "b1000000-0000-4000-8000-000000000002",
  linkedinVoice: "b1000000-0000-4000-8000-000000000003",
  linkedinPublish: "b1000000-0000-4000-8000-000000000004",
} as const;

/** List prices aligned with control-plane executor catalog seeds. */
export const MOCK_SKU_LIST_PRICES: Record<
  string,
  { priceCents: number; currency: string }
> = {
  "rss-news-feed": { priceCents: 500, currency: "USD" },
  "newsletter-writer-senior": { priceCents: 200, currency: "USD" },
  "linkedin-voice-senior": { priceCents: 150, currency: "USD" },
  "linkedin-publish": { priceCents: 1000, currency: "USD" },
};

export function getMockSkuListPrice(skuKey: string): ListPrice | undefined {
  const price = MOCK_SKU_LIST_PRICES[skuKey];
  if (!price) return undefined;

  return create(ListPriceSchema, {
    priceCents: BigInt(price.priceCents),
    currency: price.currency,
  });
}

function mockSku(
  id: string,
  key: string,
  displayName: string,
  description: string,
  kind: ExecutorCatalogKind,
): ExecutorSKU {
  return create(ExecutorSKUSchema, {
    id,
    key,
    displayName,
    description,
    kind,
    listPrice: getMockSkuListPrice(key),
    createdAt: "2026-06-01T10:00:00Z",
    updatedAt: "2026-06-01T10:00:00Z",
  });
}

function mockSkus(locale: Locale): ExecutorSKU[] {
  return [
    mockSku(
      SKU_IDS.rssNewsFeed,
      "rss-news-feed",
      resolveLocalizedContent("catalog.sku.rss-news-feed.name", locale),
      resolveLocalizedContent("catalog.sku.rss-news-feed.description", locale),
      ExecutorCatalogKind.INTEGRATION,
    ),
    mockSku(
      SKU_IDS.newsletterWriter,
      "newsletter-writer-senior",
      resolveLocalizedContent(
        "catalog.sku.newsletter-writer-senior.name",
        locale,
      ),
      resolveLocalizedContent(
        "catalog.sku.newsletter-writer-senior.description",
        locale,
      ),
      ExecutorCatalogKind.AGENT,
    ),
    mockSku(
      SKU_IDS.linkedinVoice,
      "linkedin-voice-senior",
      resolveLocalizedContent("catalog.sku.linkedin-voice-senior.name", locale),
      resolveLocalizedContent(
        "catalog.sku.linkedin-voice-senior.description",
        locale,
      ),
      ExecutorCatalogKind.AGENT,
    ),
    mockSku(
      SKU_IDS.linkedinPublish,
      "linkedin-publish",
      resolveLocalizedContent("catalog.sku.linkedin-publish.name", locale),
      resolveLocalizedContent(
        "catalog.sku.linkedin-publish.description",
        locale,
      ),
      ExecutorCatalogKind.INTEGRATION,
    ),
  ];
}

function mockNewsToSocialPostTemplate(locale: Locale): PlanTemplate {
  return create(PlanTemplateSchema, {
    id: "a1000000-0000-4000-8000-000000000001",
    key: "news-to-social-post",
    name: resolveLocalizedContent(
      "catalog.plan.news-to-social-post.name",
      locale,
    ),
    description: resolveLocalizedContent(
      "catalog.plan.news-to-social-post.description",
      locale,
    ),
    vertical: "creator-economy",
    version: 1,
    steps: [
      create(PlanStepSchema, {
        id: "step-fetch-news",
        key: "fetch-news",
        title: resolveLocalizedContent(
          "catalog.plan.news-to-social-post.step.fetch-news.title",
          locale,
        ),
        description: resolveLocalizedContent(
          "catalog.plan.news-to-social-post.step.fetch-news.description",
          locale,
        ),
        inputArtifactTypeId: "harpia.artifacts.v1.DateRange",
        outputArtifactTypeId: "harpia.artifacts.v1.NewsList",
        defaultExecutorSkuKey: "rss-news-feed",
        executorRequirement: create(ExecutorRequirementSchema, {
          executorKind: PlanExecutorKind.INTEGRATION,
          connectionType: "rss_feed",
        }),
      }),
      create(PlanStepSchema, {
        id: "step-write-draft",
        key: "write-draft",
        title: resolveLocalizedContent(
          "catalog.plan.news-to-social-post.step.write-draft.title",
          locale,
        ),
        description: resolveLocalizedContent(
          "catalog.plan.news-to-social-post.step.write-draft.description",
          locale,
        ),
        inputArtifactTypeId: "harpia.artifacts.v1.NewsList",
        outputArtifactTypeId: "harpia.artifacts.v1.TextDraft",
        defaultExecutorSkuKey: "newsletter-writer-senior",
        executorRequirement: create(ExecutorRequirementSchema, {
          executorKind: PlanExecutorKind.AGENT,
        }),
      }),
      create(PlanStepSchema, {
        id: "step-adapt-linkedin",
        key: "adapt-for-linkedin",
        title: resolveLocalizedContent(
          "catalog.plan.news-to-social-post.step.adapt-for-linkedin.title",
          locale,
        ),
        description: resolveLocalizedContent(
          "catalog.plan.news-to-social-post.step.adapt-for-linkedin.description",
          locale,
        ),
        inputArtifactTypeId: "harpia.artifacts.v1.TextDraft",
        outputArtifactTypeId: "harpia.artifacts.v1.LinkedInPostDraft",
        defaultExecutorSkuKey: "linkedin-voice-senior",
        executorRequirement: create(ExecutorRequirementSchema, {
          executorKind: PlanExecutorKind.AGENT,
        }),
      }),
      create(PlanStepSchema, {
        id: "step-publish-linkedin",
        key: "publish-linkedin",
        title: resolveLocalizedContent(
          "catalog.plan.news-to-social-post.step.publish-linkedin.title",
          locale,
        ),
        description: resolveLocalizedContent(
          "catalog.plan.news-to-social-post.step.publish-linkedin.description",
          locale,
        ),
        inputArtifactTypeId: "harpia.artifacts.v1.LinkedInPostDraft",
        outputArtifactTypeId: "harpia.artifacts.v1.PublishConfirmation",
        defaultExecutorSkuKey: "linkedin-publish",
        executorRequirement: create(ExecutorRequirementSchema, {
          executorKind: PlanExecutorKind.INTEGRATION,
          connectionType: "oauth_linkedin",
        }),
      }),
    ],
    edges: [
      create(PlanStepDependencySchema, {
        fromStepKey: "fetch-news",
        toStepKey: "write-draft",
      }),
      create(PlanStepDependencySchema, {
        fromStepKey: "write-draft",
        toStepKey: "adapt-for-linkedin",
      }),
      create(PlanStepDependencySchema, {
        fromStepKey: "adapt-for-linkedin",
        toStepKey: "publish-linkedin",
      }),
    ],
    createdAt: "2026-06-01T10:00:00Z",
    updatedAt: "2026-06-01T10:00:00Z",
  });
}

export const NEWS_TO_SOCIAL_POST_TEMPLATE: PlanTemplate =
  mockNewsToSocialPostTemplate("en");

export function mockPlanTemplates(locale: Locale = "en"): PlanTemplate[] {
  return [mockNewsToSocialPostTemplate(locale)];
}

/** Mock tenant executor state with mixed lock reasons for local development. */
export function mockExecutorContext(
  tenantId = "dev",
  locale: Locale = "en",
): MockExecutorContext {
  return {
    skus: mockSkus(locale),
    entitlements: [
      create(ExecutorEntitlementSchema, {
        id: "ent-rss",
        tenantId,
        executorSkuId: SKU_IDS.rssNewsFeed,
        grantedAt: "2026-06-01T10:00:00Z",
      }),
      create(ExecutorEntitlementSchema, {
        id: "ent-writer",
        tenantId,
        executorSkuId: SKU_IDS.newsletterWriter,
        grantedAt: "2026-06-01T10:00:00Z",
      }),
      create(ExecutorEntitlementSchema, {
        id: "ent-voice",
        tenantId,
        executorSkuId: SKU_IDS.linkedinVoice,
        grantedAt: "2026-06-01T10:00:00Z",
      }),
      create(ExecutorEntitlementSchema, {
        id: "ent-linkedin",
        tenantId,
        executorSkuId: SKU_IDS.linkedinPublish,
        grantedAt: "2026-06-01T10:00:00Z",
      }),
    ],
    installations: [
      create(ExecutorInstallationSchema, {
        id: "inst-rss",
        tenantId,
        executorSkuId: SKU_IDS.rssNewsFeed,
        kind: ExecutorCatalogKind.INTEGRATION,
        displayName: "Company RSS Feeds",
        enabled: true,
        createdAt: "2026-06-02T10:00:00Z",
        updatedAt: "2026-06-02T10:00:00Z",
        detail: {
          case: "integration",
          value: create(IntegrationInstallationSchema, {
            connectionStatus: ConnectionStatus.CONNECTED,
            configJson: '{"feeds":["https://example.com/feed.xml"]}',
          }),
        },
      }),
      create(ExecutorInstallationSchema, {
        id: "inst-rss-hn",
        tenantId,
        executorSkuId: SKU_IDS.rssNewsFeed,
        kind: ExecutorCatalogKind.INTEGRATION,
        displayName: "Hacker News Frontpage RSS",
        enabled: true,
        createdAt: "2026-06-02T10:00:00Z",
        updatedAt: "2026-06-02T10:00:00Z",
        detail: {
          case: "integration",
          value: create(IntegrationInstallationSchema, {
            connectionStatus: ConnectionStatus.CONNECTED,
            configJson: '{"feeds":["https://hnrss.org/frontpage"]}',
          }),
        },
      }),
      create(ExecutorInstallationSchema, {
        id: "inst-rss-sports",
        tenantId,
        executorSkuId: SKU_IDS.rssNewsFeed,
        kind: ExecutorCatalogKind.INTEGRATION,
        displayName: "Sports headlines RSS",
        enabled: true,
        createdAt: "2026-06-02T10:00:00Z",
        updatedAt: "2026-06-02T10:00:00Z",
        detail: {
          case: "integration",
          value: create(IntegrationInstallationSchema, {
            connectionStatus: ConnectionStatus.CONNECTED,
            configJson:
              '{"feeds":["https://feeds.folha.uol.com.br/esporte/rss091.xml"]}',
          }),
        },
      }),
      create(ExecutorInstallationSchema, {
        id: "inst-writer",
        tenantId,
        executorSkuId: SKU_IDS.newsletterWriter,
        kind: ExecutorCatalogKind.AGENT,
        displayName: "Newsletter Writer",
        enabled: true,
        createdAt: "2026-06-02T10:00:00Z",
        updatedAt: "2026-06-02T10:00:00Z",
        detail: {
          case: "agent",
          value: create(AgentInstallationSchema, {
            manifestId: "newsletter-writer-senior",
            manifestVersion: "1.0.0",
          }),
        },
      }),
      create(ExecutorInstallationSchema, {
        id: "inst-linkedin-voice",
        tenantId,
        executorSkuId: SKU_IDS.linkedinVoice,
        kind: ExecutorCatalogKind.AGENT,
        displayName: "LinkedIn Voice",
        enabled: true,
        createdAt: "2026-06-02T10:00:00Z",
        updatedAt: "2026-06-02T10:00:00Z",
        detail: {
          case: "agent",
          value: create(AgentInstallationSchema, {
            manifestId: "linkedin-voice-senior",
            manifestVersion: "1.0.0",
          }),
        },
      }),
      create(ExecutorInstallationSchema, {
        id: "inst-linkedin",
        tenantId,
        executorSkuId: SKU_IDS.linkedinPublish,
        kind: ExecutorCatalogKind.INTEGRATION,
        displayName: "LinkedIn OAuth",
        enabled: true,
        createdAt: "2026-06-02T10:00:00Z",
        updatedAt: "2026-06-02T10:00:00Z",
        detail: {
          case: "integration",
          value: create(IntegrationInstallationSchema, {
            connectionStatus: ConnectionStatus.DISCONNECTED,
            configJson: "{}",
          }),
        },
      }),
      create(ExecutorInstallationSchema, {
        id: "inst-linkedin-approval",
        tenantId,
        executorSkuId: SKU_IDS.linkedinPublish,
        kind: ExecutorCatalogKind.INTEGRATION,
        displayName: "LinkedIn Approval Only",
        enabled: true,
        createdAt: "2026-06-02T10:00:00Z",
        updatedAt: "2026-06-02T10:00:00Z",
        detail: {
          case: "integration",
          value: create(IntegrationInstallationSchema, {
            connectionStatus: ConnectionStatus.CONNECTED,
            configJson: '{"mode":"approval_only"}',
          }),
        },
      }),
    ],
  };
}
