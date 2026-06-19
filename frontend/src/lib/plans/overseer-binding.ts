import { create } from "@bufbuild/protobuf";
import type { User } from "$lib/auth";
import {
  ExecutorKind,
  type OverseerBinding,
  OverseerBindingSchema,
  PlanConfigurationStatus,
  type PlanStep,
  type PlanTemplate,
  type SlotBinding,
} from "$lib/gen/harpia/plans/v1/plans_pb";

export interface AgentStepInfo {
  stepKey: string;
  title: string;
}

export interface OverseerValidationIssue {
  stepKey: string;
  stepTitle: string;
  code: "missing_overseer";
}

export interface OverseerValidationResult {
  ok: boolean;
  issues: OverseerValidationIssue[];
  missingStepKeys: string[];
}

const PROMOTION_STATUSES = new Set<PlanConfigurationStatus>([
  PlanConfigurationStatus.RUNNABLE,
  PlanConfigurationStatus.SCHEDULED,
]);

export function isAgentBackedStep(step: PlanStep): boolean {
  return step.executorRequirement?.executorKind === ExecutorKind.AGENT;
}

export function getAgentBackedSteps(template: PlanTemplate): AgentStepInfo[] {
  return template.steps
    .filter(isAgentBackedStep)
    .map((step) => ({ stepKey: step.key, title: step.title }));
}

export function getOverseerRequiredSteps(
  template: PlanTemplate,
  slotBindings: SlotBinding[] = [],
): AgentStepInfo[] {
  const agentSteps = getAgentBackedSteps(template);
  if (agentSteps.length === 0) {
    return [];
  }

  const agentSlotBindings = slotBindings.filter(
    (binding) =>
      binding.executorKind === ExecutorKind.AGENT &&
      binding.executorInstallationId.trim() !== "",
  );

  if (agentSlotBindings.length === 0) {
    return agentSteps;
  }

  const requiredKeys = new Set(
    agentSlotBindings.map((binding) => binding.stepKey),
  );
  return agentSteps.filter((step) => requiredKeys.has(step.stepKey));
}

export function requiresOverseerValidation(
  status: PlanConfigurationStatus,
): boolean {
  return PROMOTION_STATUSES.has(status);
}

export function createSelfOverseerBinding(
  stepKey: string,
  user: Pick<User, "sub">,
): OverseerBinding {
  return create(OverseerBindingSchema, {
    stepKey,
    overseerUserId: user.sub,
  });
}

export function assignSelfAsOverseerForSteps(
  steps: AgentStepInfo[],
  user: Pick<User, "sub">,
): OverseerBinding[] {
  return steps.map((step) => createSelfOverseerBinding(step.stepKey, user));
}

export function upsertOverseerBinding(
  bindings: OverseerBinding[],
  binding: OverseerBinding,
): OverseerBinding[] {
  const next = bindings.filter((item) => item.stepKey !== binding.stepKey);
  next.push(binding);
  return next.sort((left, right) => left.stepKey.localeCompare(right.stepKey));
}

export function removeOverseerBinding(
  bindings: OverseerBinding[],
  stepKey: string,
): OverseerBinding[] {
  return bindings.filter((binding) => binding.stepKey !== stepKey);
}

export function getOverseerBindingForStep(
  bindings: OverseerBinding[],
  stepKey: string,
): OverseerBinding | undefined {
  return bindings.find((binding) => binding.stepKey === stepKey);
}

export function validateOverseerBindings(
  template: PlanTemplate,
  overseerBindings: OverseerBinding[],
  targetStatus: PlanConfigurationStatus,
  slotBindings: SlotBinding[] = [],
): OverseerValidationResult {
  const requiredSteps = getOverseerRequiredSteps(template, slotBindings);

  if (!requiresOverseerValidation(targetStatus)) {
    return { ok: true, issues: [], missingStepKeys: [] };
  }

  const bindingsByStep = new Map(
    overseerBindings.map((binding) => [binding.stepKey, binding]),
  );

  const issues: OverseerValidationIssue[] = [];
  for (const step of requiredSteps) {
    const binding = bindingsByStep.get(step.stepKey);
    if (!binding?.overseerUserId?.trim()) {
      issues.push({
        stepKey: step.stepKey,
        stepTitle: step.title,
        code: "missing_overseer",
      });
    }
  }

  return {
    ok: issues.length === 0,
    issues,
    missingStepKeys: issues.map((issue) => issue.stepKey),
  };
}

export function getDraftOverseerWarning(
  template: PlanTemplate,
  overseerBindings: OverseerBinding[],
  slotBindings: SlotBinding[] = [],
): string | null {
  const validation = validateOverseerBindings(
    template,
    overseerBindings,
    PlanConfigurationStatus.RUNNABLE,
    slotBindings,
  );

  if (validation.ok) {
    return null;
  }

  return formatOverseerPromotionErrors(validation.issues).join(" ");
}

export function formatMissingOverseerError(
  stepKey: string,
  stepTitle: string,
): string {
  return `Agent step "${stepTitle}" (${stepKey}) is missing an overseer.`;
}

export function formatOverseerPromotionErrors(
  issues: OverseerValidationIssue[],
): string[] {
  return issues.map((issue) =>
    formatMissingOverseerError(issue.stepKey, issue.stepTitle),
  );
}
