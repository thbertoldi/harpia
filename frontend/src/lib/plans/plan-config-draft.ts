import { create } from "@bufbuild/protobuf";
import {
  PlanBehaviorPoliciesSchema,
  type OverseerBinding,
  type PlanBehaviorPolicies,
  type PlanConfiguration,
  type SlotBinding,
} from "$lib/gen/harpia/plans/v1/plans_pb";

export interface PlanConfigDraft {
  templateId: string;
  slotBindings: SlotBinding[];
  overseerBindings: OverseerBinding[];
  behaviorPolicies: PlanBehaviorPolicies;
}

export function planConfigDraftFromConfiguration(
  configuration: PlanConfiguration,
): PlanConfigDraft {
  return {
    templateId: configuration.planTemplateId,
    slotBindings: configuration.slotBindings,
    overseerBindings: configuration.overseerBindings,
    behaviorPolicies:
      configuration.behaviorPolicies ?? create(PlanBehaviorPoliciesSchema),
  };
}
