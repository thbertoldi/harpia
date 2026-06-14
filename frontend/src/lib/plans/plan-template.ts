import { create } from "@bufbuild/protobuf";
import { toUserMessage } from "$lib/connect-errors";
import type { PlanTemplate } from "$lib/gen/harpia/plans/v1/plans_pb";
import {
  ExecutorKind,
  PlanStepDependencySchema,
  PlanStepSchema,
  PlanTemplateSchema,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import { planClient } from "$lib/rpc";

export type PlanTemplateSource = "api" | "mock";

export interface PlanTemplateResult {
  template: PlanTemplate;
  source: PlanTemplateSource;
  error?: string;
}

export const WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_ID =
  "a1000000-0000-4000-8000-000000000001";

export const WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_KEY =
  "weekly-newsletter-linkedin";

const UUID_REGEX =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export function isPlanTemplateUuid(value: string): boolean {
  return UUID_REGEX.test(value);
}

export function mockWeeklyNewsletterLinkedInTemplate(): PlanTemplate {
  return create(PlanTemplateSchema, {
    id: WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_ID,
    key: WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_KEY,
    name: "Weekly Newsletter (LinkedIn)",
    description: "Fetch news, write a draft, adapt for LinkedIn, and publish.",
    vertical: "creator-economy",
    version: 1,
    steps: [
      create(PlanStepSchema, {
        id: "b1000000-0000-4000-8000-000000000001",
        key: "fetch-news",
        title: "Fetch News",
        description: "Collect curated articles for the configured date range.",
        inputArtifactTypeId: "harpia.artifacts.v1.DateRange",
        outputArtifactTypeId: "harpia.artifacts.v1.NewsList",
        defaultExecutorSkuKey: "news-fetcher",
        executorRequirement: {
          executorKind: ExecutorKind.AGENT,
          requiredCapabilities: ["news-fetch"],
          connectionType: "",
        },
      }),
      create(PlanStepSchema, {
        id: "b1000000-0000-4000-8000-000000000002",
        key: "write-draft",
        title: "Write Draft",
        description: "Synthesize a platform-neutral newsletter draft.",
        inputArtifactTypeId: "harpia.artifacts.v1.NewsList",
        outputArtifactTypeId: "harpia.artifacts.v1.TextDraft",
        defaultExecutorSkuKey: "newsletter-writer",
        executorRequirement: {
          executorKind: ExecutorKind.AGENT,
          requiredCapabilities: ["writing"],
          connectionType: "",
        },
      }),
      create(PlanStepSchema, {
        id: "b1000000-0000-4000-8000-000000000003",
        key: "adapt-for-linkedin",
        title: "Adapt for LinkedIn",
        description: "Transform the draft into a LinkedIn-specific post.",
        inputArtifactTypeId: "harpia.artifacts.v1.TextDraft",
        outputArtifactTypeId: "harpia.artifacts.v1.LinkedInPostDraft",
        defaultExecutorSkuKey: "linkedin-adapter",
        executorRequirement: {
          executorKind: ExecutorKind.AGENT,
          requiredCapabilities: ["social-adaptation"],
          connectionType: "",
        },
      }),
      create(PlanStepSchema, {
        id: "b1000000-0000-4000-8000-000000000004",
        key: "publish-linkedin",
        title: "Publish LinkedIn",
        description: "Publish the adapted post to LinkedIn.",
        inputArtifactTypeId: "harpia.artifacts.v1.LinkedInPostDraft",
        outputArtifactTypeId: "harpia.artifacts.v1.PublishConfirmation",
        defaultExecutorSkuKey: "linkedin-publisher",
        executorRequirement: {
          executorKind: ExecutorKind.INTEGRATION,
          requiredCapabilities: ["linkedin-publish"],
          connectionType: "linkedin",
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

function matchesMockTemplate(templateIdOrKey: string): boolean {
  return (
    templateIdOrKey === WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_ID ||
    templateIdOrKey === WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_KEY
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

/** Loads a plan template by UUID or key; falls back to mock data when API is unavailable. */
export async function loadPlanTemplate(
  templateIdOrKey: string,
): Promise<PlanTemplateResult> {
  try {
    const template = await fetchPlanTemplateFromApi(templateIdOrKey);
    if (!template) {
      if (matchesMockTemplate(templateIdOrKey)) {
        return {
          template: mockWeeklyNewsletterLinkedInTemplate(),
          source: "mock",
          error: "Empty response from plan service",
        };
      }
      throw new Error("Plan template not found");
    }

    return { template, source: "api" };
  } catch (error) {
    if (matchesMockTemplate(templateIdOrKey)) {
      return {
        template: mockWeeklyNewsletterLinkedInTemplate(),
        source: "mock",
        error: toUserMessage(error),
      };
    }

    throw error;
  }
}
