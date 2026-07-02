import { create } from "@bufbuild/protobuf";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  DEFAULT_BEHAVIOR_POLICIES,
  elicitationBehaviorLabelKey,
  loadBehaviorPoliciesForTemplate,
  policiesFromProto,
  policiesToProto,
  publishModeLabelKey,
  saveBehaviorPoliciesForTemplate,
  validateBehaviorPolicies,
} from "$lib/plans/behavior-policies";
import {
  ElicitationTimeoutBehavior,
  ExecutorKind,
  OverseerBindingSchema,
  PlanBehaviorPoliciesSchema,
  PlanConfigurationSchema,
  PlanConfigurationStatus,
  SlotBindingSchema,
  PublishApprovalMode,
  type PlanConfiguration,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import {
  mockWeeklyNewsletterLinkedInTemplate,
  WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_ID,
} from "$lib/plans/plan-template";

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

const CONFIGURATION_ID = "c1000000-0000-4000-8000-000000000001";

function listConfigurations(configurations: PlanConfiguration[]) {
  return (async function* () {
    yield { planConfigurations: configurations };
  })();
}

describe("behavior policies", () => {
  beforeEach(() => {
    listPlanConfigurations.mockReset();
    createPlanConfiguration.mockReset();
    updatePlanConfiguration.mockReset();
    listPlanConfigurations.mockReturnValue(listConfigurations([]));
  });

  it("provides sensible defaults", () => {
    expect(DEFAULT_BEHAVIOR_POLICIES).toEqual({
      elicitationTimeoutBehavior:
        ElicitationTimeoutBehavior.PAUSE_UNTIL_ANSWERED,
      elicitationTimeoutHours: 48,
      publishApprovalMode: PublishApprovalMode.REQUIRE_APPROVAL,
    });
  });

  it("maps proto policies to form values", () => {
    const policies = create(PlanBehaviorPoliciesSchema, {
      elicitationTimeoutBehavior: ElicitationTimeoutBehavior.FAIL_STEP,
      elicitationTimeoutHours: 12,
      publishApprovalMode: PublishApprovalMode.AUTO_PUBLISH,
    });

    expect(policiesFromProto(policies)).toEqual({
      elicitationTimeoutBehavior: ElicitationTimeoutBehavior.FAIL_STEP,
      elicitationTimeoutHours: 12,
      publishApprovalMode: PublishApprovalMode.AUTO_PUBLISH,
    });
  });

  it("round-trips form values through proto", () => {
    const values = {
      elicitationTimeoutBehavior: ElicitationTimeoutBehavior.FAIL_PLAN,
      elicitationTimeoutHours: 24,
      publishApprovalMode: PublishApprovalMode.REQUIRE_APPROVAL,
    };

    expect(policiesFromProto(policiesToProto(values))).toEqual(values);
  });

  it("validates timeout hour bounds", () => {
    expect(
      validateBehaviorPolicies({
        ...DEFAULT_BEHAVIOR_POLICIES,
        elicitationTimeoutHours: 0,
      }),
    ).toBe("plans.policies.validation.timeoutHours");

    expect(
      validateBehaviorPolicies({
        ...DEFAULT_BEHAVIOR_POLICIES,
        elicitationTimeoutHours: 721,
      }),
    ).toBe("plans.policies.validation.timeoutHours");
  });

  it("maps enum values to i18n label keys", () => {
    expect(
      elicitationBehaviorLabelKey(
        ElicitationTimeoutBehavior.PAUSE_UNTIL_ANSWERED,
      ),
    ).toBe("plans.policies.elicitation.pauseUntilAnswered");
    expect(publishModeLabelKey(PublishApprovalMode.AUTO_PUBLISH)).toBe(
      "plans.policies.publish.autoPublish",
    );
  });

  it("loads policies from the API when a configuration exists", async () => {
    const configuration = create(PlanConfigurationSchema, {
      id: CONFIGURATION_ID,
      tenantId: "dev",
      workspaceId: "",
      planTemplateId: WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_ID,
      planTemplateVersion: 1,
      status: PlanConfigurationStatus.DRAFT,
      behaviorPolicies: create(PlanBehaviorPoliciesSchema, {
        elicitationTimeoutBehavior: ElicitationTimeoutBehavior.FAIL_STEP,
        elicitationTimeoutHours: 6,
        publishApprovalMode: PublishApprovalMode.REQUIRE_APPROVAL,
      }),
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-01T00:00:00Z",
    });

    listPlanConfigurations.mockReturnValue(listConfigurations([configuration]));

    const result = await loadBehaviorPoliciesForTemplate(
      WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_ID,
    );

    expect(result.source).toBe("api");
    expect(result.configuration?.id).toBe(CONFIGURATION_ID);
    expect(result.policies.elicitationTimeoutHours).toBe(6);
  });

  it("rejects when listing configurations fails", async () => {
    listPlanConfigurations.mockImplementation(() => {
      throw new Error("network error");
    });

    await expect(
      loadBehaviorPoliciesForTemplate(WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_ID),
    ).rejects.toThrow("network error");
  });

  it("creates a configuration through the API", async () => {
    const template = mockWeeklyNewsletterLinkedInTemplate();
    const saved = create(PlanConfigurationSchema, {
      id: CONFIGURATION_ID,
      tenantId: "dev",
      workspaceId: "",
      planTemplateId: template.id,
      planTemplateVersion: template.version,
      status: PlanConfigurationStatus.DRAFT,
      behaviorPolicies: policiesToProto(DEFAULT_BEHAVIOR_POLICIES),
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-01T00:00:00Z",
    });

    createPlanConfiguration.mockResolvedValue({ planConfiguration: saved });

    const result = await saveBehaviorPoliciesForTemplate(
      template,
      DEFAULT_BEHAVIOR_POLICIES,
      undefined,
      "test-thread",
    );

    expect(createPlanConfiguration).toHaveBeenCalledWith(
      expect.objectContaining({
        tenantId: "dev",
        workspaceId: "",
        planTemplateId: template.id,
        status: PlanConfigurationStatus.DRAFT,
      }),
    );
    expect(result.source).toBe("api");
    expect(result.configuration.id).toBe(CONFIGURATION_ID);
  });

  it("updates an existing configuration through the API without dropping other fields", async () => {
    const template = mockWeeklyNewsletterLinkedInTemplate();
    const existing = create(PlanConfigurationSchema, {
      id: CONFIGURATION_ID,
      tenantId: "dev",
      workspaceId: "",
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
      behaviorPolicies: policiesToProto(DEFAULT_BEHAVIOR_POLICIES),
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-01T00:00:00Z",
    });

    const updatedValues = {
      ...DEFAULT_BEHAVIOR_POLICIES,
      publishApprovalMode: PublishApprovalMode.AUTO_PUBLISH,
    };

    updatePlanConfiguration.mockResolvedValue({
      planConfiguration: {
        ...existing,
        behaviorPolicies: policiesToProto(updatedValues),
      },
    });

    const result = await saveBehaviorPoliciesForTemplate(
      template,
      updatedValues,
      existing,
    );

    expect(updatePlanConfiguration).toHaveBeenCalledWith(
      expect.objectContaining({
        tenantId: "dev",
        planConfigurationId: CONFIGURATION_ID,
        status: PlanConfigurationStatus.RUNNABLE,
        slotBindings: existing.slotBindings,
        overseerBindings: existing.overseerBindings,
      }),
    );
    expect(result.source).toBe("api");
  });

  it("rejects when the API save fails", async () => {
    const template = mockWeeklyNewsletterLinkedInTemplate();
    createPlanConfiguration.mockRejectedValue(new Error("network error"));

    await expect(
      saveBehaviorPoliciesForTemplate(template, DEFAULT_BEHAVIOR_POLICIES, undefined, "test-thread"),
    ).rejects.toThrow("network error");
  });
});
