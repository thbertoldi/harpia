import { create } from "@bufbuild/protobuf";
import type { Locale } from "$lib/i18n";
import {
  ExecutorKind as CatalogExecutorKind,
  type ExecutorEntitlement,
  type ExecutorInstallation,
  type ExecutorSKU,
} from "$lib/gen/harpia/executors/v1/executors_pb";
import {
  ExecutorKind as PlanExecutorKind,
  PlanConfigurationStatus,
  SlotBindingSchema,
  type PlanConfiguration,
  type PlanStep,
  type PlanTemplate,
  type SlotBinding,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import { requireTenantId } from "$lib/auth";
import { executorClient } from "$lib/rpc";
import {
  type ExecutorContext,
  isInstallationReady,
  resolveSkuLockState,
  type SkuLockReason,
} from "$lib/plans/plan-catalog";
import {
  loadPlanConfigurationForTemplate,
  savePlanConfigurationRecord,
} from "$lib/plans/plan-configuration";
import { loadPlanTemplate } from "$lib/plans/plan-template";
import { stepWillRun } from "$lib/plans/step-participation";

export type SlotBindingSource = "api" | "mock";

export interface CompatibleInstallationOption {
  installation: ExecutorInstallation;
  sku: ExecutorSKU;
  lockReason: SkuLockReason;
  lockMessageKey: string;
  lockMessageParams?: Record<string, string>;
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
  const requirement = step.executorRequirement;
  if (!requirement) {
    return false;
  }

  if (!installation.enabled) {
    return false;
  }

  const sku = skus.find((entry) => entry.id === installation.executorSkuId);
  if (!sku || sku.key !== step.defaultExecutorSkuKey.trim()) {
    return false;
  }

  if (!executorKindsMatch(requirement.executorKind, installation.kind)) {
    return false;
  }

  const entitled = entitlements.some(
    (entitlement) => entitlement.executorSkuId === sku.id,
  );
  if (!entitled) {
    return false;
  }

  const connectionType = requirement.connectionType.trim();
  if (
    connectionType &&
    requirement.executorKind === PlanExecutorKind.INTEGRATION &&
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
): Pick<
  CompatibleInstallationOption,
  "lockReason" | "lockMessageKey" | "lockMessageParams" | "ready"
> {
  const entitled = entitlements.some(
    (entitlement) => entitlement.executorSkuId === sku.id,
  );

  if (!entitled) {
    return {
      lockReason: "missing_entitlement",
      lockMessageKey: "plans.lock.message.missing_entitlement",
      lockMessageParams: { name: sku.displayName, key: sku.key },
      ready: false,
    };
  }

  if (!installation.enabled) {
    return {
      lockReason: "not_configured",
      lockMessageKey: "plans.lock.message.installation_disabled",
      lockMessageParams: { name: installation.displayName },
      ready: false,
    };
  }

  if (isInstallationReady(installation)) {
    return {
      lockReason: "available",
      lockMessageKey: "plans.lock.message.available",
      ready: true,
    };
  }

  const skuLock = resolveSkuLockState(sku, entitlements, [installation]);
  return {
    lockReason: skuLock.reason,
    lockMessageKey: skuLock.messageKey,
    lockMessageParams: skuLock.messageParams,
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
    executorKind:
      step.executorRequirement?.executorKind ?? PlanExecutorKind.UNSPECIFIED,
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
  includedOptionalCapabilities: readonly string[] = [],
): ConfigurationValidation {
  const bindingByStep = new Map(
    bindings.map((binding) => [binding.stepKey, binding]),
  );
  const steps: StepBindingValidation[] = [];
  const draftWarnings: string[] = [];

  for (const step of template.steps) {
    if (!stepWillRun(step, includedOptionalCapabilities)) continue;
    const binding = bindingByStep.get(step.key);
    const warnings: string[] = [];
    const errors: string[] = [];

    if (!binding?.executorInstallationId) {
      warnings.push("plans.validation.stepUnbound");
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
      errors.push("plans.validation.missingInstallation");
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
      errors.push("plans.validation.incompatibleInstallation");
    }

    const sku = context.skus.find(
      (entry) => entry.id === installation.executorSkuId,
    );
    const lock = sku
      ? resolveInstallationLockState(installation, sku, context.entitlements)
      : {
          lockReason: "not_configured" as SkuLockReason,
          lockMessageKey: "plans.lock.message.sku_metadata_unavailable",
          ready: false,
        };

    if (!lock.ready) {
      warnings.push(lock.lockMessageKey);
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

  const canPromoteToRunnable = steps.every(
    (step) => step.bound && step.compatible && step.ready,
  );

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

/** Loads template, executor context, and any saved configuration for the configure UI. */
export async function loadSlotBindingPageData(
  templateIdOrKey: string,
  locale: Locale = "en",
): Promise<SlotBindingPageData> {
  const templateResult = await loadPlanTemplate(templateIdOrKey, locale);
  const tenantId = requireTenantId();
  const source: SlotBindingSource = templateResult.source;
  const error = templateResult.error;

  const [context, configuration] = await Promise.all([
    fetchExecutorContextFromApi(tenantId),
    loadPlanConfigurationForTemplate(templateResult.template.id, tenantId),
  ]);

  return {
    template: templateResult.template,
    context,
    configuration,
    source,
    error,
  };
}

/** Persists slot bindings as DRAFT or RUNNABLE through the backend PlanConfiguration API. */
export async function savePlanConfiguration(
  template: PlanTemplate,
  selections: Record<string, string>,
  context: ExecutorContext,
  status: PlanConfigurationStatus,
  existingConfiguration?: PlanConfiguration | null,
): Promise<SaveConfigurationResult> {
  const slotBindings = selectionsToSlotBindings(template, selections, context);
  const validation = validateSlotBindings(
    template,
    slotBindings,
    context,
    existingConfiguration?.includedOptionalCapabilities,
  );

  if (
    status === PlanConfigurationStatus.RUNNABLE &&
    !validation.canPromoteToRunnable
  ) {
    throw new Error("All plan steps must be bound to ready installations.");
  }

  const configuration = await savePlanConfigurationRecord({
    template,
    existingConfiguration,
    status,
    slotBindings,
  });

  return {
    configuration,
    source: "api",
    validation,
  };
}
