import { create } from "@bufbuild/protobuf";
import { toUserMessage } from "$lib/connect-errors";
import type { Locale } from "$lib/i18n";
import { resolveLocalizedContent } from "$lib/i18n/content";
import type { PlanTemplate } from "$lib/gen/harpia/plans/v1/plans_pb";
import {
  ExecutorKind,
  PlanStepDependencySchema,
  PlanStepSchema,
  PlanTemplateSchema,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import { allowsMockFallback } from "$lib/dev-mocks";
import { planClient } from "$lib/rpc";

export type PlanTemplateSource = "api" | "mock";

export interface PlanTemplateResult {
  template: PlanTemplate;
  source: PlanTemplateSource;
  error?: string;
}

export const NEWS_TO_SOCIAL_POST_TEMPLATE_ID =
  "a1000000-0000-4000-8000-000000000001";

export const NEWS_TO_SOCIAL_POST_TEMPLATE_KEY = "news-to-social-post";

const UUID_REGEX =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export function isPlanTemplateUuid(value: string): boolean {
  return UUID_REGEX.test(value);
}

export function mockNewsToSocialPostTemplate(
  locale: Locale = "en",
): PlanTemplate {
  return create(PlanTemplateSchema, {
    id: NEWS_TO_SOCIAL_POST_TEMPLATE_ID,
    key: NEWS_TO_SOCIAL_POST_TEMPLATE_KEY,
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
        id: "b1000000-0000-4000-8000-000000000001",
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
        executorRequirement: {
          executorKind: ExecutorKind.INTEGRATION,
          requiredCapabilities: [],
          connectionType: "rss_feed",
        },
      }),
      create(PlanStepSchema, {
        id: "b1000000-0000-4000-8000-000000000002",
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
        executorRequirement: {
          executorKind: ExecutorKind.AGENT,
          requiredCapabilities: [],
          connectionType: "",
        },
      }),
      create(PlanStepSchema, {
        id: "b1000000-0000-4000-8000-000000000003",
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
        executorRequirement: {
          executorKind: ExecutorKind.AGENT,
          requiredCapabilities: [],
          connectionType: "",
        },
      }),
      create(PlanStepSchema, {
        id: "b1000000-0000-4000-8000-000000000004",
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
        executorRequirement: {
          executorKind: ExecutorKind.INTEGRATION,
          requiredCapabilities: [],
          connectionType: "oauth_linkedin",
        },
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
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  });
}

export function localizePlanTemplate(
  template: PlanTemplate,
  locale: Locale,
): PlanTemplate {
  if (template.key !== NEWS_TO_SOCIAL_POST_TEMPLATE_KEY) {
    return template;
  }

  const contentPrefix = `catalog.plan.${template.key}`;

  return create(PlanTemplateSchema, {
    ...template,
    name: resolveLocalizedContent(`${contentPrefix}.name`, locale),
    description: resolveLocalizedContent(
      `${contentPrefix}.description`,
      locale,
    ),
    steps: template.steps.map((step) =>
      create(PlanStepSchema, {
        ...step,
        title: resolveLocalizedContent(
          `${contentPrefix}.step.${step.key}.title`,
          locale,
        ),
        description: resolveLocalizedContent(
          `${contentPrefix}.step.${step.key}.description`,
          locale,
        ),
      }),
    ),
  });
}

function matchesMockTemplate(templateIdOrKey: string): boolean {
  return (
    templateIdOrKey === NEWS_TO_SOCIAL_POST_TEMPLATE_ID ||
    templateIdOrKey === NEWS_TO_SOCIAL_POST_TEMPLATE_KEY
  );
}

async function fetchPlanTemplateFromApi(
  templateIdOrKey: string,
): Promise<PlanTemplate | undefined> {
  if (isPlanTemplateUuid(templateIdOrKey)) {
    const response = await planClient.getPlanTemplate({
      planTemplateId: templateIdOrKey,
    });
    return response.planTemplate;
  }

  const response = await planClient.getPlanTemplateByKey({
    key: templateIdOrKey,
  });
  return response.planTemplate;
}

/** Loads a plan template by UUID or key; mock fallback only when explicitly enabled for dev/test. */
export async function loadPlanTemplate(
  templateIdOrKey: string,
  locale: Locale = "en",
): Promise<PlanTemplateResult> {
  try {
    const template = await fetchPlanTemplateFromApi(templateIdOrKey);
    if (!template) {
      if (allowsMockFallback() && matchesMockTemplate(templateIdOrKey)) {
        return {
          template: mockNewsToSocialPostTemplate(locale),
          source: "mock",
          error: "Empty response from plan service",
        };
      }
      throw new Error("Plan template not found");
    }

    return {
      template: localizePlanTemplate(template, locale),
      source: "api",
    };
  } catch (error) {
    if (allowsMockFallback() && matchesMockTemplate(templateIdOrKey)) {
      return {
        template: mockNewsToSocialPostTemplate(locale),
        source: "mock",
        error: toUserMessage(error),
      };
    }

    throw error;
  }
}
