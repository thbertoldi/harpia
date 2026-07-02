import { create } from "@bufbuild/protobuf";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  ExecutorKind,
  OverseerBindingSchema,
  PlanBehaviorPoliciesSchema,
  PlanConfigurationSchema,
  PlanConfigurationStatus,
  SlotBindingSchema,
  type PlanConfiguration,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import { mockWeeklyNewsletterLinkedInTemplate } from "$lib/plans/plan-template";
import {
  loadPlanConfigurationForTemplate,
  pickPreferredPlanConfiguration,
  savePlanConfigurationRecord,
  workspaceIdForTenant,
} from "$lib/plans/plan-configuration";

const {
  listPlanConfigurations,
  createPlanConfiguration,
  updatePlanConfiguration,
} = vi.hoisted(() => ({
  listPlanConfigurations: vi.fn(),
  createPlanConfiguration: vi.fn(),
  updatePlanConfiguration: vi.fn(),
}));

vi.mock("$lib/rpc", () => ({
  planClient: {
    listPlanConfigurations,
    createPlanConfiguration,
    updatePlanConfiguration,
  },
}));

vi.mock("$lib/auth", () => ({
  requireTenantId: () => "dev",
}));

function listConfigurations(configurations: PlanConfiguration[]) {
  return (async function* () {
    yield { planConfigurations: configurations };
  })();
}

describe("plan configuration persistence", () => {
  const template = mockWeeklyNewsletterLinkedInTemplate();

  beforeEach(() => {
    listPlanConfigurations.mockReset();
    createPlanConfiguration.mockReset();
    updatePlanConfiguration.mockReset();
    listPlanConfigurations.mockReturnValue(listConfigurations([]));
  });

  it("leaves workspace id empty until workspace resolution exists", () => {
    expect(workspaceIdForTenant("dev")).toBe("");
    expect(workspaceIdForTenant("123e4567-e89b-42d3-a456-426614174000")).toBe(
      "",
    );
  });

  it("prefers the newest active configuration over archived records", () => {
    const active = create(PlanConfigurationSchema, {
      id: "active",
      planTemplateId: template.id,
      status: PlanConfigurationStatus.DRAFT,
      updatedAt: "2026-01-01T00:00:00Z",
    });
    const archived = create(PlanConfigurationSchema, {
      id: "archived",
      planTemplateId: template.id,
      status: PlanConfigurationStatus.ARCHIVED,
      updatedAt: "2026-02-01T00:00:00Z",
    });

    expect(pickPreferredPlanConfiguration([archived, active])?.id).toBe(
      "active",
    );
  });

  it("loads the preferred configuration for a template", async () => {
    const ignored = create(PlanConfigurationSchema, {
      id: "other-template",
      planTemplateId: "other",
      updatedAt: "2026-02-01T00:00:00Z",
    });
    const selected = create(PlanConfigurationSchema, {
      id: "selected",
      planTemplateId: template.id,
      updatedAt: "2026-01-01T00:00:00Z",
    });
    listPlanConfigurations.mockReturnValue(
      listConfigurations([ignored, selected]),
    );

    await expect(loadPlanConfigurationForTemplate(template.id)).resolves.toBe(
      selected,
    );
  });

  it("creates a backend configuration when no existing record is found", async () => {
    const saved = create(PlanConfigurationSchema, {
      id: "created",
      tenantId: "dev",
      workspaceId: "",
      planTemplateId: template.id,
      planTemplateVersion: template.version,
      status: PlanConfigurationStatus.DRAFT,
    });
    createPlanConfiguration.mockResolvedValue({ planConfiguration: saved });

    const result = await savePlanConfigurationRecord({
      template,
      status: PlanConfigurationStatus.DRAFT,
      threadId: "test-thread",
      parameterValuesJson: '{"theme":"sports"}',
    });

    expect(createPlanConfiguration).toHaveBeenCalledWith(
      expect.objectContaining({
        tenantId: "dev",
        workspaceId: "",
        planTemplateId: template.id,
        status: PlanConfigurationStatus.DRAFT,
        parameterValuesJson: '{"theme":"sports"}',
      }),
    );
    expect(
      createPlanConfiguration.mock.calls[0][0].seedArtifacts,
    ).toBeUndefined();
    expect(
      createPlanConfiguration.mock.calls[0][0].slotBindings,
    ).toBeUndefined();
    expect(
      createPlanConfiguration.mock.calls[0][0].behaviorPolicies,
    ).toBeUndefined();
    expect(result.id).toBe("created");
  });

  it("updates a backend configuration without dropping unchanged fields", async () => {
    const existing = create(PlanConfigurationSchema, {
      id: "existing",
      tenantId: "dev",
      planTemplateId: template.id,
      planTemplateVersion: template.version,
      status: PlanConfigurationStatus.RUNNABLE,
      slotBindings: [
        create(SlotBindingSchema, {
          stepKey: "write-draft",
          executorKind: ExecutorKind.AGENT,
          executorSkuId: "sku-writer",
          executorInstallationId: "install-writer",
        }),
      ],
      overseerBindings: [
        create(OverseerBindingSchema, {
          stepKey: "write-draft",
          overseerUserId: "dev-overseer",
        }),
      ],
      behaviorPolicies: create(PlanBehaviorPoliciesSchema, {
        elicitationTimeoutHours: 48,
      }),
    });
    updatePlanConfiguration.mockResolvedValue({
      planConfiguration: {
        ...existing,
        status: PlanConfigurationStatus.DRAFT,
      },
    });

    await savePlanConfigurationRecord({
      template,
      existingConfiguration: existing,
      status: PlanConfigurationStatus.DRAFT,
    });

    expect(updatePlanConfiguration).toHaveBeenCalledWith(
      expect.objectContaining({
        tenantId: "dev",
        planConfigurationId: "existing",
        status: PlanConfigurationStatus.DRAFT,
        overseerBindings: existing.overseerBindings,
      }),
    );
    expect(
      updatePlanConfiguration.mock.calls[0][0].seedArtifacts,
    ).toBeUndefined();
    expect(
      updatePlanConfiguration.mock.calls[0][0].slotBindings,
    ).toBeUndefined();
    expect(
      updatePlanConfiguration.mock.calls[0][0].behaviorPolicies,
    ).toBeUndefined();
  });
});
