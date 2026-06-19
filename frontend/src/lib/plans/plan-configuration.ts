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
}

export function workspaceIdForTenant(_tenantId: string): string {
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
}: PlanConfigurationSaveInput): Promise<PlanConfiguration> {
  const existing =
    existingConfiguration ??
    (await loadPlanConfigurationForTemplate(template.id, tenantId));

  const nextStatus =
    status ?? existing?.status ?? PlanConfigurationStatus.DRAFT;
  const nextSeedArtifacts = seedArtifacts ?? existing?.seedArtifacts ?? [];
  const nextSlotBindings = slotBindings ?? existing?.slotBindings ?? [];
  const nextOverseerBindings =
    overseerBindings ?? existing?.overseerBindings ?? [];
  const nextBehaviorPolicies =
    behaviorPolicies ?? existing?.behaviorPolicies ?? undefined;
  const nextSchedule = schedule ?? existing?.schedule ?? undefined;

  const response = existing?.id
    ? await planClient.updatePlanConfiguration({
        tenantId,
        planConfigurationId: existing.id,
        status: nextStatus,
        seedArtifacts: nextSeedArtifacts,
        slotBindings: nextSlotBindings,
        overseerBindings: nextOverseerBindings,
        behaviorPolicies: nextBehaviorPolicies,
        schedule: nextSchedule,
      })
    : await planClient.createPlanConfiguration({
        tenantId,
        workspaceId: workspaceIdForTenant(tenantId),
        planTemplateId: template.id,
        status: nextStatus,
        seedArtifacts: nextSeedArtifacts,
        slotBindings: nextSlotBindings,
        overseerBindings: nextOverseerBindings,
        behaviorPolicies: nextBehaviorPolicies,
        schedule: nextSchedule,
      });

  if (!response.planConfiguration) {
    throw new Error("PlanConfiguration save returned no configuration.");
  }

  return response.planConfiguration;
}
