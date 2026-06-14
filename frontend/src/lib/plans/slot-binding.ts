import { create } from "@bufbuild/protobuf";
import { toUserMessage } from "$lib/connect-errors";
import {
  ExecutorKind as CatalogExecutorKind,
  type ExecutorEntitlement,
  type ExecutorInstallation,
  type ExecutorSKU,
} from "$lib/gen/harpia/executors/v1/executors_pb";
import {
  ExecutorKind as PlanExecutorKind,
  PlanConfigurationSchema,
  PlanConfigurationStatus,
  SlotBindingSchema,
  type PlanConfiguration,
  type PlanStep,
  type PlanTemplate,
  type SlotBinding,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import { mockExecutorContext } from "$lib/mocks/plan-catalog";
import { getSession, getTenant } from "$lib/auth";
import { executorClient, planClient } from "$lib/rpc";
import {
  type ExecutorContext,
  isInstallationReady,
  resolveSkuLockState,
  type SkuLockReason,
} from "$lib/plans/plan-catalog";
import { loadPlanTemplate } from "$lib/plans/plan-template";

export type SlotBindingSource = "api" | "mock";

export interface CompatibleInstallationOption {
  installation: ExecutorInstallation;
  sku: ExecutorSKU;
  lockReason: SkuLockReason;
  lockMessage: string;
  ready: boolean;
}

export interface StepBindingValidation {
  stepKey: string;
  bound: boolean;
  installationId?: string;
  compatible: boolean;
  ready: boolean;
  warnings: string[];
  errors: string[];
}

export interface ConfigurationValidation {
  steps: StepBindingValidation[];
  canPromoteToRunnable: boolean;
  draftWarnings: string[];
}

export interface SlotBindingPageData {
  template: PlanTemplate;
  context: ExecutorContext;
  configuration: PlanConfiguration | null;
  source: SlotBindingSource;
  error?: string;
}

export interface SaveConfigurationResult {
  configuration: PlanConfiguration;
  source: SlotBindingSource;
  validation: ConfigurationValidation;
  error?: string;
}

const mockConfigurations = new Map<string, PlanConfiguration>();

function resolveTenantId(): string | undefined {
  return getTenant()?.id ?? getSession()?.tenant?.id;
}

function mockConfigurationKey(tenantId: string, templateId: string): string {
  return `${tenantId}:${templateId}`;
}

export function planExecutorKindToCatalogKind(
  kind: PlanExecutorKind,
): CatalogExecutorKind {
  switch (kind) {
    case PlanExecutorKind.AGENT:
      return CatalogExecutorKind.AGENT;
    case PlanExecutorKind.INTEGRATION:
      return CatalogExecutorKind.INTEGRATION;
    default:
      return CatalogExecutorKind.UNSPECIFIED;
  }
}

export function catalogExecutorKindToPlanKind(
  kind: CatalogExecutorKind,
): PlanExecutorKind {
  switch (kind) {
    case CatalogExecutorKind.AGENT:
      return PlanExecutorKind.AGENT;
    case CatalogExecutorKind.INTEGRATION:
      return PlanExecutorKind.INTEGRATION;
    default:
      return PlanExecutorKind.UNSPECIFIED;
  }
}

export function executorKindsMatch(
  stepKind: PlanExecutorKind,
  installationKind: CatalogExecutorKind,
): boolean {
  return (
    planExecutorKindToCatalogKind(stepKind) === installationKind &&
    stepKind !== PlanExecutorKind.UNSPECIFIED
  );
}

export function isInstallationCompatibleWithStep(
  step: PlanStep,
  installation: ExecutorInstallation,
  skus: ExecutorSKU[],
  entitlements: ExecutorEntitlement[],
): boolean {
  if (!installation.enabled) {
    return false;
  }

  const sku = skus.find((entry) => entry.id === installation.executorSkuId);
  if (!sku || sku.key !== step.defaultExecutorSkuKey.trim()) {
    return false;
  }

  if (
    !executorKindsMatch(
      step.executorRequirement.executorKind,
      installation.kind,
    )
  ) {
    return false;
  }

  const entitled = entitlements.some(
    (entitlement) => entitlement.executorSkuId === sku.id,
  );
  if (!entitled) {
    return false;
  }

  const connectionType = step.executorRequirement.connectionType.trim();
  if (
    connectionType &&
    step.executorRequirement.executorKind === PlanExecutorKind.INTEGRATION &&
    sku.kind !== CatalogExecutorKind.INTEGRATION
  ) {
    return false;
  }

  return true;
}

export function resolveInstallationLockState(
  installation: ExecutorInstallation,
  sku: ExecutorSKU,
  entitlements: ExecutorEntitlement[],
): Pick<CompatibleInstallationOption, "lockReason" | "lockMessage" | "ready"> {
  const entitled = entitlements.some(
    (entitlement) => entitlement.executorSkuId === sku.id,
  );

  if (!entitled) {
    return {
      lockReason: "missing_entitlement",
      lockMessage: `Missing SKU entitlement for ${sku.displayName} (${sku.key}).`,
      ready: false,
    };
  }

  if (!installation.enabled) {
    return {
      lockReason: "not_configured",
      lockMessage: `${installation.displayName} is disabled.`,
      ready: false,
    };
  }

  if (isInstallationReady(installation)) {
    return { lockReason: "available", lockMessage: "", ready: true };
  }

  const skuLock = resolveSkuLockState(sku, entitlements, [installation]);
  return {
    lockReason: skuLock.reason,
    lockMessage: skuLock.message,
    ready: false,
  };
}

export function getCompatibleInstallationsForStep(
  step: PlanStep,
  context: ExecutorContext,
): CompatibleInstallationOption[] {
  const options: CompatibleInstallationOption[] = [];

  for (const installation of context.installations) {
    if (
      !isInstallationCompatibleWithStep(
        step,
        installation,
        context.skus,
        context.entitlements,
      )
    ) {
      continue;
    }

    const sku = context.skus.find(
      (entry) => entry.id === installation.executorSkuId,
    );
    if (!sku) continue;

    const lock = resolveInstallationLockState(
      installation,
      sku,
      context.entitlements,
    );

    options.push({
      installation,
      sku,
      ...lock,
    });
  }

  options.sort((a, b) =>
    a.installation.displayName.localeCompare(b.installation.displayName),
  );

  return options;
}

export function buildSlotBinding(
  step: PlanStep,
  installation: ExecutorInstallation,
  sku: ExecutorSKU,
): SlotBinding {
  return create(SlotBindingSchema, {
    stepKey: step.key,
    executorKind: step.executorRequirement.executorKind,
    executorSkuId: sku.id,
    executorInstallationId: installation.id,
  });
}

export function selectionsToSlotBindings(
  template: PlanTemplate,
  selections: Record<string, string>,
  context: ExecutorContext,
): SlotBinding[] {
  const bindings: SlotBinding[] = [];

  for (const step of template.steps) {
    const installationId = selections[step.key]?.trim();
    if (!installationId) continue;

    const installation = context.installations.find(
      (entry) => entry.id === installationId,
    );
    if (!installation) continue;

    const sku = context.skus.find(
      (entry) => entry.id === installation.executorSkuId,
    );
    if (!sku) continue;

    if (
      !isInstallationCompatibleWithStep(
        step,
        installation,
        context.skus,
        context.entitlements,
      )
    ) {
      continue;
    }

    bindings.push(buildSlotBinding(step, installation, sku));
  }

  return bindings;
}

export function slotBindingsToSelections(
  bindings: SlotBinding[],
): Record<string, string> {
  const selections: Record<string, string> = {};

  for (const binding of bindings) {
    if (binding.stepKey && binding.executorInstallationId) {
      selections[binding.stepKey] = binding.executorInstallationId;
    }
  }

  return selections;
}

export function validateSlotBindings(
  template: PlanTemplate,
  bindings: SlotBinding[],
  context: ExecutorContext,
): ConfigurationValidation {
  const bindingByStep = new Map(
    bindings.map((binding) => [binding.stepKey, binding]),
  );
  const steps: StepBindingValidation[] = [];
  const draftWarnings: string[] = [];

  for (const step of template.steps) {
    const binding = bindingByStep.get(step.key);
    const warnings: string[] = [];
    const errors: string[] = [];

    if (!binding?.executorInstallationId) {
      warnings.push(`Step "${step.title}" has no executor installation bound.`);
      steps.push({
        stepKey: step.key,
        bound: false,
        compatible: false,
        ready: false,
        warnings,
        errors,
      });
      continue;
    }

    const installation = context.installations.find(
      (entry) => entry.id === binding.executorInstallationId,
    );
    if (!installation) {
      errors.push(`Step "${step.title}" references a missing installation.`);
      steps.push({
        stepKey: step.key,
        bound: true,
        installationId: binding.executorInstallationId,
        compatible: false,
        ready: false,
        warnings,
        errors,
      });
      continue;
    }

    const compatible = isInstallationCompatibleWithStep(
      step,
      installation,
      context.skus,
      context.entitlements,
    );
    if (!compatible) {
      errors.push(
        `Step "${step.title}" is bound to an incompatible installation.`,
      );
    }

    const sku = context.skus.find(
      (entry) => entry.id === installation.executorSkuId,
    );
    const lock = sku
      ? resolveInstallationLockState(installation, sku, context.entitlements)
      : {
          lockReason: "not_configured" as SkuLockReason,
          lockMessage: "Executor SKU metadata is unavailable.",
          ready: false,
        };

    if (!lock.ready) {
      warnings.push(
        lock.lockMessage ||
          `Installation for step "${step.title}" is not ready to run.`,
      );
    }

    steps.push({
      stepKey: step.key,
      bound: true,
      installationId: binding.executorInstallationId,
      compatible,
      ready: compatible && lock.ready,
      warnings,
      errors,
    });
  }

  for (const step of steps) {
    draftWarnings.push(...step.warnings);
  }

  const canPromoteToRunnable =
    steps.length === template.steps.length &&
    steps.every((step) => step.bound && step.compatible && step.ready);

  return {
    steps,
    canPromoteToRunnable,
    draftWarnings,
  };
}

async function fetchExecutorContextFromApi(
  tenantId: string,
): Promise<ExecutorContext> {
  const skus: ExecutorSKU[] = [];
  for await (const page of executorClient.listExecutorSKUs({
    pageSize: 100,
    pageToken: "",
  })) {
    skus.push(...page.executorSkus);
  }

  const entitlements: ExecutorEntitlement[] = [];
  for await (const page of executorClient.listExecutorEntitlements({
    tenantId,
    pageSize: 100,
    pageToken: "",
  })) {
    entitlements.push(...page.entitlements);
  }

  const installations: ExecutorInstallation[] = [];
  for await (const page of executorClient.listExecutorInstallations({
    tenantId,
    pageSize: 100,
    pageToken: "",
  })) {
    installations.push(...page.installations);
  }

  return { skus, entitlements, installations };
}

async function fetchConfigurationFromApi(
  tenantId: string,
  templateId: string,
): Promise<PlanConfiguration | null> {
  for await (const page of planClient.listPlanConfigurations({
    tenantId,
    pageSize: 100,
    pageToken: "",
  })) {
    const match = page.planConfigurations.find(
      (configuration) => configuration.planTemplateId === templateId,
    );
    if (match) {
      return match;
    }
  }

  return null;
}

function readMockConfiguration(
  tenantId: string,
  templateId: string,
): PlanConfiguration | null {
  return (
    mockConfigurations.get(mockConfigurationKey(tenantId, templateId)) ?? null
  );
}

function writeMockConfiguration(
  tenantId: string,
  configuration: PlanConfiguration,
): PlanConfiguration {
  mockConfigurations.set(
    mockConfigurationKey(tenantId, configuration.planTemplateId),
    configuration,
  );
  return configuration;
}

/** Loads template, executor context, and any saved configuration for the configure UI. */
export async function loadSlotBindingPageData(
  templateIdOrKey: string,
): Promise<SlotBindingPageData> {
  const templateResult = await loadPlanTemplate(templateIdOrKey);
  const tenantId = resolveTenantId() ?? "dev";
  let source: SlotBindingSource = templateResult.source;
  let error = templateResult.error;
  let context: ExecutorContext;

  try {
    context = await fetchExecutorContextFromApi(tenantId);
  } catch (err) {
    context = mockExecutorContext(tenantId);
    if (source === "api") {
      source = "mock";
    }
    error = error ?? toUserMessage(err);
  }

  let configuration: PlanConfiguration | null = null;

  try {
    configuration = await fetchConfigurationFromApi(
      tenantId,
      templateResult.template.id,
    );
  } catch (err) {
    configuration = readMockConfiguration(tenantId, templateResult.template.id);
    if (source === "api") {
      source = "mock";
    }
    error = error ?? toUserMessage(err);
  }

  if (!configuration) {
    configuration = readMockConfiguration(tenantId, templateResult.template.id);
  }

  return {
    template: templateResult.template,
    context,
    configuration,
    source,
    error,
  };
}

/** Persists slot bindings as DRAFT or RUNNABLE; mock fallback when API is unavailable. */
export async function savePlanConfiguration(
  template: PlanTemplate,
  selections: Record<string, string>,
  context: ExecutorContext,
  status: PlanConfigurationStatus,
): Promise<SaveConfigurationResult> {
  const slotBindings = selectionsToSlotBindings(template, selections, context);
  const validation = validateSlotBindings(template, slotBindings, context);

  if (
    status === PlanConfigurationStatus.RUNNABLE &&
    !validation.canPromoteToRunnable
  ) {
    throw new Error("All plan steps must be bound to ready installations.");
  }

  const tenantId = resolveTenantId() ?? "dev";
  const existing = readMockConfiguration(tenantId, template.id);
  const now = new Date().toISOString();

  const payload = {
    tenantId,
    workspaceId: tenantId,
    planTemplateId: template.id,
    planTemplateVersion: template.version,
    status,
    slotBindings,
  };

  try {
    const response = existing?.id
      ? await planClient.updatePlanConfiguration({
          planConfigurationId: existing.id,
          ...payload,
        })
      : await planClient.createPlanConfiguration(payload);

    const configuration = response.planConfiguration;
    if (configuration) {
      writeMockConfiguration(tenantId, configuration);
    }

    return {
      configuration: configuration!,
      source: "api",
      validation,
    };
  } catch (err) {
    const configuration = create(PlanConfigurationSchema, {
      id: existing?.id ?? `mock-config-${template.id}`,
      tenantId,
      workspaceId: tenantId,
      planTemplateId: template.id,
      planTemplateVersion: template.version,
      status,
      seedArtifacts: existing?.seedArtifacts ?? [],
      slotBindings,
      overseerBindings: existing?.overseerBindings ?? [],
      createdAt: existing?.createdAt ?? now,
      updatedAt: now,
    });

    writeMockConfiguration(tenantId, configuration);

    return {
      configuration,
      source: "mock",
      validation,
      error: toUserMessage(err),
    };
  }
}
