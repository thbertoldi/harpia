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

const MOCK_SKUS: ExecutorSKU[] = [
  mockSku(
    SKU_IDS.rssNewsFeed,
    "rss-news-feed",
    "RSS News Feed",
    "Fetches curated news articles from configured RSS feeds.",
    ExecutorCatalogKind.INTEGRATION,
  ),
  mockSku(
    SKU_IDS.newsletterWriter,
    "newsletter-writer-senior",
    "Newsletter Writer (Senior)",
    "Senior agent that synthesizes a platform-neutral newsletter draft from curated news.",
    ExecutorCatalogKind.AGENT,
  ),
  mockSku(
    SKU_IDS.linkedinVoice,
    "linkedin-voice-senior",
    "LinkedIn Voice (Senior)",
    "Senior agent that adapts a neutral text draft into a LinkedIn-ready post.",
    ExecutorCatalogKind.AGENT,
  ),
  mockSku(
    SKU_IDS.linkedinPublish,
    "linkedin-publish",
    "LinkedIn Publish",
    "Publishes a LinkedIn post draft through the tenant OAuth connection.",
    ExecutorCatalogKind.INTEGRATION,
  ),
];

export const WEEKLY_NEWSLETTER_TEMPLATE: PlanTemplate = create(
  PlanTemplateSchema,
  {
    id: "a1000000-0000-4000-8000-000000000001",
    key: "weekly-newsletter-linkedin",
    name: "Weekly Newsletter (LinkedIn)",
    description: "Fetch news, write a draft, adapt for LinkedIn, and publish.",
    vertical: "creator-economy",
    version: 1,
    steps: [
      create(PlanStepSchema, {
        id: "step-fetch-news",
        key: "fetch-news",
        title: "Fetch News",
        description: "Collect curated articles for the configured date range.",
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
        title: "Write Draft",
        description: "Synthesize a platform-neutral newsletter draft.",
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
        title: "Adapt for LinkedIn",
        description: "Transform the draft into a LinkedIn-specific post.",
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
        title: "Publish LinkedIn",
        description: "Publish the adapted post to LinkedIn.",
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
  },
);

export function mockPlanTemplates(): PlanTemplate[] {
  return [WEEKLY_NEWSLETTER_TEMPLATE];
}

/** Mock tenant executor state with mixed lock reasons for local development. */
export function mockExecutorContext(tenantId = "dev"): MockExecutorContext {
  return {
    skus: MOCK_SKUS,
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
            configJson: '{"feed_urls":["https://example.com/feed.xml"]}',
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
    ],
  };
}
