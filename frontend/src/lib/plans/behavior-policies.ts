import { create } from "@bufbuild/protobuf";
import { toUserMessage } from "$lib/connect-errors";
import type { PlanConfiguration } from "$lib/gen/harpia/plans/v1/plans_pb";
import {
  ElicitationTimeoutBehavior,
  PlanBehaviorPoliciesSchema,
  PlanConfigurationSchema,
  PlanConfigurationStatus,
  PublishApprovalMode,
  type PlanBehaviorPolicies,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import { requireTenantId } from "$lib/auth";
import { planClient } from "$lib/rpc";
import { WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_ID } from "$lib/plans/plan-template";

export type PlanConfigurationSource = "api" | "mock";

export interface BehaviorPoliciesFormValues {
  elicitationTimeoutBehavior: ElicitationTimeoutBehavior;
  elicitationTimeoutHours: number;
  publishApprovalMode: PublishApprovalMode;
}

export interface BehaviorPoliciesLoadResult {
  configuration: PlanConfiguration | null;
  policies: BehaviorPoliciesFormValues;
  source: PlanConfigurationSource;
  error?: string;
}

export interface BehaviorPoliciesSaveResult {
  configuration: PlanConfiguration;
  source: PlanConfigurationSource;
  error?: string;
}

export const ELICITATION_TIMEOUT_BEHAVIOR_OPTIONS = [
  ElicitationTimeoutBehavior.PAUSE_UNTIL_ANSWERED,
  ElicitationTimeoutBehavior.FAIL_STEP,
  ElicitationTimeoutBehavior.SKIP_WITH_DEFAULT,
] as const;

export const PUBLISH_APPROVAL_MODE_OPTIONS = [
  PublishApprovalMode.REQUIRE_APPROVAL,
  PublishApprovalMode.AUTO_PUBLISH,
] as const;

export const DEFAULT_BEHAVIOR_POLICIES: BehaviorPoliciesFormValues = {
  elicitationTimeoutBehavior: ElicitationTimeoutBehavior.PAUSE_UNTIL_ANSWERED,
  elicitationTimeoutHours: 48,
  publishApprovalMode: PublishApprovalMode.REQUIRE_APPROVAL,
};

export const MOCK_PLAN_CONFIGURATION_ID =
  "c1000000-0000-4000-8000-000000000001";

const mockConfigurations = new Map<string, PlanConfiguration>();

function mockStorageKey(tenantId: string, templateId: string): string {
  return `${tenantId}:${templateId}`;
}

function supportsMockFallback(templateId: string): boolean {
  return templateId === WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_ID;
}

export function clearMockPlanConfigurations(): void {
  mockConfigurations.clear();
}

export function elicitationBehaviorLabelKey(
  behavior: ElicitationTimeoutBehavior,
): string {
  switch (behavior) {
    case ElicitationTimeoutBehavior.PAUSE_UNTIL_ANSWERED:
      return "plans.policies.elicitation.pauseUntilAnswered";
    case ElicitationTimeoutBehavior.FAIL_STEP:
      return "plans.policies.elicitation.failStep";
    case ElicitationTimeoutBehavior.SKIP_WITH_DEFAULT:
      return "plans.policies.elicitation.skipWithDefault";
    default:
      return "plans.policies.elicitation.unspecified";
  }
}

export function elicitationBehaviorDescriptionKey(
  behavior: ElicitationTimeoutBehavior,
): string {
  return `${elicitationBehaviorLabelKey(behavior)}.desc`;
}

export function publishModeLabelKey(mode: PublishApprovalMode): string {
  switch (mode) {
    case PublishApprovalMode.REQUIRE_APPROVAL:
      return "plans.policies.publish.requireApproval";
    case PublishApprovalMode.AUTO_PUBLISH:
      return "plans.policies.publish.autoPublish";
    default:
      return "plans.policies.publish.unspecified";
  }
}

export function publishModeDescriptionKey(mode: PublishApprovalMode): string {
  return `${publishModeLabelKey(mode)}.desc`;
}

export function policiesFromProto(
  policies?: PlanBehaviorPolicies,
): BehaviorPoliciesFormValues {
  if (!policies) {
    return { ...DEFAULT_BEHAVIOR_POLICIES };
  }

  return {
    elicitationTimeoutBehavior:
      policies.elicitationTimeoutBehavior ||
      DEFAULT_BEHAVIOR_POLICIES.elicitationTimeoutBehavior,
    elicitationTimeoutHours:
      policies.elicitationTimeoutHours > 0
        ? policies.elicitationTimeoutHours
        : DEFAULT_BEHAVIOR_POLICIES.elicitationTimeoutHours,
    publishApprovalMode:
      policies.publishApprovalMode ||
      DEFAULT_BEHAVIOR_POLICIES.publishApprovalMode,
  };
}

export function policiesToProto(
  values: BehaviorPoliciesFormValues,
): PlanBehaviorPolicies {
  return create(PlanBehaviorPoliciesSchema, {
    elicitationTimeoutBehavior: values.elicitationTimeoutBehavior,
    elicitationTimeoutHours: values.elicitationTimeoutHours,
    publishApprovalMode: values.publishApprovalMode,
  });
}

export function validateBehaviorPolicies(
  values: BehaviorPoliciesFormValues,
): string | null {
  if (
    !ELICITATION_TIMEOUT_BEHAVIOR_OPTIONS.includes(
      values.elicitationTimeoutBehavior,
    )
  ) {
    return "plans.policies.validation.elicitationBehavior";
  }

  if (
    !Number.isInteger(values.elicitationTimeoutHours) ||
    values.elicitationTimeoutHours < 1 ||
    values.elicitationTimeoutHours > 720
  ) {
    return "plans.policies.validation.timeoutHours";
  }

  if (!PUBLISH_APPROVAL_MODE_OPTIONS.includes(values.publishApprovalMode)) {
    return "plans.policies.validation.publishMode";
  }

  return null;
}

async function listConfigurationsForTemplate(
  tenantId: string,
  templateId: string,
): Promise<PlanConfiguration[]> {
  const configurations: PlanConfiguration[] = [];

  for await (const page of planClient.listPlanConfigurations({
    tenantId,
    pageSize: 100,
    pageToken: "",
  })) {
    configurations.push(
      ...page.planConfigurations.filter(
        (config) => config.planTemplateId === templateId,
      ),
    );
  }

  return configurations;
}

function pickPreferredConfiguration(
  configurations: PlanConfiguration[],
): PlanConfiguration | null {
  if (configurations.length === 0) {
    return null;
  }

  const draft = configurations.find(
    (config) => config.status === PlanConfigurationStatus.DRAFT,
  );
  return draft ?? configurations[0] ?? null;
}

function loadMockConfiguration(
  tenantId: string,
  templateId: string,
): PlanConfiguration | null {
  return mockConfigurations.get(mockStorageKey(tenantId, templateId)) ?? null;
}

function saveMockConfiguration(
  tenantId: string,
  templateId: string,
  templateVersion: number,
  values: BehaviorPoliciesFormValues,
  existing?: PlanConfiguration | null,
): PlanConfiguration {
  const now = new Date().toISOString();
  const configuration = create(PlanConfigurationSchema, {
    id: existing?.id ?? MOCK_PLAN_CONFIGURATION_ID,
    tenantId,
    workspaceId: tenantId,
    planTemplateId: templateId,
    planTemplateVersion: templateVersion,
    status: existing?.status ?? PlanConfigurationStatus.DRAFT,
    seedArtifacts: existing?.seedArtifacts ?? [],
    slotBindings: existing?.slotBindings ?? [],
    overseerBindings: existing?.overseerBindings ?? [],
    behaviorPolicies: policiesToProto(values),
    schedule: existing?.schedule,
    createdAt: existing?.createdAt ?? now,
    updatedAt: now,
  });

  mockConfigurations.set(mockStorageKey(tenantId, templateId), configuration);
  return configuration;
}

/** Loads behavior policies for a plan template, falling back to mock storage when needed. */
export async function loadBehaviorPoliciesForTemplate(
  templateId: string,
): Promise<BehaviorPoliciesLoadResult> {
  const tenantId = requireTenantId();

  try {
    const configurations = await listConfigurationsForTemplate(
      tenantId,
      templateId,
    );
    const configuration = pickPreferredConfiguration(configurations);

    return {
      configuration,
      policies: policiesFromProto(configuration?.behaviorPolicies),
      source: "api",
    };
  } catch (error) {
    if (!supportsMockFallback(templateId)) {
      throw error;
    }

    const configuration = loadMockConfiguration(tenantId, templateId);
    return {
      configuration,
      policies: policiesFromProto(configuration?.behaviorPolicies),
      source: "mock",
      error: toUserMessage(error),
    };
  }
}

/** Persists behavior policies via Create/UpdatePlanConfiguration with mock fallback. */
export async function saveBehaviorPoliciesForTemplate(
  templateId: string,
  templateVersion: number,
  values: BehaviorPoliciesFormValues,
  existingConfiguration?: PlanConfiguration | null,
): Promise<BehaviorPoliciesSaveResult> {
  const validationError = validateBehaviorPolicies(values);
  if (validationError) {
    throw new Error(validationError);
  }

  const tenantId = requireTenantId();
  const behaviorPolicies = policiesToProto(values);

  try {
    if (existingConfiguration?.id) {
      const response = await planClient.updatePlanConfiguration({
        tenantId,
        planConfigurationId: existingConfiguration.id,
        status: existingConfiguration.status,
        seedArtifacts: existingConfiguration.seedArtifacts,
        slotBindings: existingConfiguration.slotBindings,
        overseerBindings: existingConfiguration.overseerBindings,
        behaviorPolicies,
        schedule: existingConfiguration.schedule,
      });

      if (!response.planConfiguration) {
        throw new Error("UpdatePlanConfiguration returned no configuration");
      }

      return { configuration: response.planConfiguration, source: "api" };
    }

    const response = await planClient.createPlanConfiguration({
      tenantId,
      workspaceId: tenantId,
      planTemplateId: templateId,
      status: PlanConfigurationStatus.DRAFT,
      seedArtifacts: [],
      slotBindings: [],
      overseerBindings: [],
      behaviorPolicies,
    });

    if (!response.planConfiguration) {
      throw new Error("CreatePlanConfiguration returned no configuration");
    }

    return { configuration: response.planConfiguration, source: "api" };
  } catch (error) {
    if (!supportsMockFallback(templateId)) {
      throw error;
    }

    const configuration = saveMockConfiguration(
      tenantId,
      templateId,
      templateVersion,
      values,
      existingConfiguration,
    );

    return {
      configuration,
      source: "mock",
      error: toUserMessage(error),
    };
  }
}
