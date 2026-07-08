import { requireTenantId } from "$lib/auth";
import type {
  OverseerBinding,
  PlanBehaviorPolicies,
  PlanConfiguration,
  PlanSchedule,
  PlanTemplate,
  SeedArtifactBinding,
  SlotBinding,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import { PlanConfigurationStatus } from "$lib/gen/harpia/plans/v1/plans_pb";
import { planClient } from "$lib/rpc";

export type PlanConfigurationSource = "api";

export interface PlanConfigurationSaveInput {
  tenantId?: string;
  template: PlanTemplate;
  existingConfiguration?: PlanConfiguration | null;
  status?: PlanConfigurationStatus;
  seedArtifacts?: SeedArtifactBinding[];
  slotBindings?: SlotBinding[];
  overseerBindings?: OverseerBinding[];
  behaviorPolicies?: PlanBehaviorPolicies;
  schedule?: PlanSchedule;
  threadId?: string;
  parameterValuesJson?: string;
}

export function workspaceIdForTenant(tenantId: string): string {
  void tenantId;
  return "";
}

export async function listPlanConfigurationsForTemplate(
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
        (configuration) => configuration.planTemplateId === templateId,
      ),
    );
  }

  return configurations;
}

function timestampOf(configuration: PlanConfiguration): number {
  const updated = Date.parse(configuration.updatedAt);
  if (!Number.isNaN(updated)) {
    return updated;
  }

  const created = Date.parse(configuration.createdAt);
  return Number.isNaN(created) ? 0 : created;
}

function isActiveConfiguration(configuration: PlanConfiguration): boolean {
  return (
    configuration.status !== PlanConfigurationStatus.ARCHIVED &&
    configuration.status !== PlanConfigurationStatus.DISABLED
  );
}

export function pickPreferredPlanConfiguration(
  configurations: PlanConfiguration[],
): PlanConfiguration | null {
  if (configurations.length === 0) {
    return null;
  }

  const active = configurations.filter(isActiveConfiguration);
  const candidates = active.length > 0 ? active : configurations;

  return [...candidates].sort(
    (left, right) => timestampOf(right) - timestampOf(left),
  )[0]!;
}

export async function loadPlanConfigurationForTemplate(
  templateId: string,
  tenantId: string = requireTenantId(),
): Promise<PlanConfiguration | null> {
  return pickPreferredPlanConfiguration(
    await listPlanConfigurationsForTemplate(tenantId, templateId),
  );
}

export async function savePlanConfigurationRecord({
  tenantId = requireTenantId(),
  template,
  existingConfiguration,
  status,
  seedArtifacts,
  slotBindings,
  overseerBindings,
  behaviorPolicies,
  schedule,
  threadId,
  parameterValuesJson,
}: PlanConfigurationSaveInput): Promise<PlanConfiguration> {
  const existing =
    existingConfiguration ??
    (threadId
      ? null
      : await loadPlanConfigurationForTemplate(template.id, tenantId));

  if (!existing?.id && !threadId) {
    throw new Error("threadId is required when creating a plan configuration.");
  }

  const nextStatus =
    status ?? existing?.status ?? PlanConfigurationStatus.DRAFT;
  void seedArtifacts;
  void slotBindings;
  const nextOverseerBindings =
    overseerBindings ?? existing?.overseerBindings ?? [];
  void behaviorPolicies;
  const nextSchedule = schedule ?? existing?.schedule ?? undefined;
  const nextParameterValuesJson =
    parameterValuesJson ?? existing?.parameterValuesJson ?? "";

  const response = existing?.id
    ? await planClient.updatePlanConfiguration({
        tenantId,
        planConfigurationId: existing.id,
        status: nextStatus,
        overseerBindings: nextOverseerBindings,
        schedule: nextSchedule,
        parameterValuesJson: nextParameterValuesJson,
      })
    : await planClient.createPlanConfiguration({
        tenantId,
        workspaceId: workspaceIdForTenant(tenantId),
        planTemplateId: template.id,
        status: nextStatus,
        overseerBindings: nextOverseerBindings,
        schedule: nextSchedule,
        threadId: threadId ?? existing?.originThreadId ?? "",
        parameterValuesJson: nextParameterValuesJson,
      });

  if (!response.planConfiguration) {
    throw new Error("PlanConfiguration save returned no configuration.");
  }

  return response.planConfiguration;
}
