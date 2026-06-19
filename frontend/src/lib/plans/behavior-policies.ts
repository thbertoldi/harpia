import { create } from "@bufbuild/protobuf";
import type {
  PlanConfiguration,
  PlanTemplate,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import {
  ElicitationTimeoutBehavior,
  PlanBehaviorPoliciesSchema,
  PlanConfigurationStatus,
  PublishApprovalMode,
  type PlanBehaviorPolicies,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import {
  loadPlanConfigurationForTemplate,
  savePlanConfigurationRecord,
  type PlanConfigurationSource,
} from "$lib/plans/plan-configuration";

export type { PlanConfigurationSource };

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

export const ELICITATION_TIMEOUT_BEHAVIOR_OPTIONS: readonly ElicitationTimeoutBehavior[] =
  [
    ElicitationTimeoutBehavior.PAUSE_UNTIL_ANSWERED,
    ElicitationTimeoutBehavior.FAIL_STEP,
    ElicitationTimeoutBehavior.FAIL_PLAN,
  ];

export const PUBLISH_APPROVAL_MODE_OPTIONS: readonly PublishApprovalMode[] = [
  PublishApprovalMode.REQUIRE_APPROVAL,
  PublishApprovalMode.AUTO_PUBLISH,
];

export const DEFAULT_BEHAVIOR_POLICIES: BehaviorPoliciesFormValues = {
  elicitationTimeoutBehavior: ElicitationTimeoutBehavior.PAUSE_UNTIL_ANSWERED,
  elicitationTimeoutHours: 48,
  publishApprovalMode: PublishApprovalMode.REQUIRE_APPROVAL,
};

export function elicitationBehaviorLabelKey(
  behavior: ElicitationTimeoutBehavior,
): string {
  switch (behavior) {
    case ElicitationTimeoutBehavior.PAUSE_UNTIL_ANSWERED:
      return "plans.policies.elicitation.pauseUntilAnswered";
    case ElicitationTimeoutBehavior.FAIL_STEP:
      return "plans.policies.elicitation.failStep";
    case ElicitationTimeoutBehavior.FAIL_PLAN:
      return "plans.policies.elicitation.failPlan";
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

/** Loads behavior policies for a persisted plan configuration. */
export async function loadBehaviorPoliciesForTemplate(
  templateId: string,
): Promise<BehaviorPoliciesLoadResult> {
  const configuration = await loadPlanConfigurationForTemplate(templateId);

  return {
    configuration,
    policies: policiesFromProto(configuration?.behaviorPolicies),
    source: "api",
  };
}

/** Persists behavior policies via Create/UpdatePlanConfiguration. */
export async function saveBehaviorPoliciesForTemplate(
  template: PlanTemplate,
  values: BehaviorPoliciesFormValues,
  existingConfiguration?: PlanConfiguration | null,
): Promise<BehaviorPoliciesSaveResult> {
  const validationError = validateBehaviorPolicies(values);
  if (validationError) {
    throw new Error(validationError);
  }

  const configuration = await savePlanConfigurationRecord({
    template,
    existingConfiguration,
    status: existingConfiguration?.status ?? PlanConfigurationStatus.DRAFT,
    behaviorPolicies: policiesToProto(values),
  });

  return { configuration, source: "api" };
}
