import { create } from "@bufbuild/protobuf";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  clearMockPlanConfigurations,
  DEFAULT_BEHAVIOR_POLICIES,
  elicitationBehaviorLabelKey,
  loadBehaviorPoliciesForTemplate,
  MOCK_PLAN_CONFIGURATION_ID,
  policiesFromProto,
  policiesToProto,
  publishModeLabelKey,
  saveBehaviorPoliciesForTemplate,
  validateBehaviorPolicies,
} from "$lib/plans/behavior-policies";
import { WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_ID } from "$lib/plans/plan-template";
import {
  ElicitationTimeoutBehavior,
  PlanBehaviorPoliciesSchema,
  PlanConfigurationSchema,
  PlanConfigurationStatus,
  PublishApprovalMode,
} from "$lib/gen/harpia/plans/v1/plans_pb";

const {
  listPlanConfigurations,
  createPlanConfiguration,
  updatePlanConfiguration,
  allowsMockFallback,
} = vi.hoisted(() => ({
  listPlanConfigurations: vi.fn(),
  createPlanConfiguration: vi.fn(),
  updatePlanConfiguration: vi.fn(),
  allowsMockFallback: vi.fn(() => false),
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

vi.mock("$lib/dev-mocks", () => ({
  allowsMockFallback,
}));

describe("behavior policies", () => {
  beforeEach(() => {
    listPlanConfigurations.mockReset();
    createPlanConfiguration.mockReset();
    updatePlanConfiguration.mockReset();
    allowsMockFallback.mockReturnValue(false);
    clearMockPlanConfigurations();
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
      id: MOCK_PLAN_CONFIGURATION_ID,
      tenantId: "dev",
      workspaceId: "dev",
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

    listPlanConfigurations.mockReturnValue(
      (async function* () {
        yield { planConfigurations: [configuration] };
      })(),
    );

    const result = await loadBehaviorPoliciesForTemplate(
      WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_ID,
    );

    expect(result.source).toBe("api");
    expect(result.configuration?.id).toBe(MOCK_PLAN_CONFIGURATION_ID);
    expect(result.policies.elicitationTimeoutHours).toBe(6);
  });

  it("rejects when listing configurations fails without mock fallback", async () => {
    listPlanConfigurations.mockImplementation(() => {
      throw new Error("network error");
    });

    await expect(
      loadBehaviorPoliciesForTemplate(WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_ID),
    ).rejects.toThrow("network error");
  });

  it("falls back to mock storage when listing configurations fails with mock fallback enabled", async () => {
    allowsMockFallback.mockReturnValue(true);
    listPlanConfigurations.mockImplementation(() => {
      throw new Error("network error");
    });

    const result = await loadBehaviorPoliciesForTemplate(
      WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_ID,
    );

    expect(result.source).toBe("mock");
    expect(result.configuration).toBeNull();
    expect(result.policies).toEqual(DEFAULT_BEHAVIOR_POLICIES);
    expect(result.error).toContain("network error");
  });

  it("creates a configuration through the API", async () => {
    const saved = create(PlanConfigurationSchema, {
      id: MOCK_PLAN_CONFIGURATION_ID,
      tenantId: "dev",
      workspaceId: "dev",
      planTemplateId: WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_ID,
      planTemplateVersion: 1,
      status: PlanConfigurationStatus.DRAFT,
      behaviorPolicies: policiesToProto(DEFAULT_BEHAVIOR_POLICIES),
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-01T00:00:00Z",
    });

    createPlanConfiguration.mockResolvedValue({ planConfiguration: saved });

    const result = await saveBehaviorPoliciesForTemplate(
      WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_ID,
      1,
      DEFAULT_BEHAVIOR_POLICIES,
    );

    expect(createPlanConfiguration).toHaveBeenCalled();
    expect(result.source).toBe("api");
    expect(result.configuration.id).toBe(MOCK_PLAN_CONFIGURATION_ID);
  });

  it("updates an existing configuration through the API", async () => {
    const existing = create(PlanConfigurationSchema, {
      id: MOCK_PLAN_CONFIGURATION_ID,
      tenantId: "dev",
      workspaceId: "dev",
      planTemplateId: WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_ID,
      planTemplateVersion: 1,
      status: PlanConfigurationStatus.DRAFT,
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
      WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_ID,
      1,
      updatedValues,
      existing,
    );

    expect(updatePlanConfiguration).toHaveBeenCalledWith(
      expect.objectContaining({
        planConfigurationId: MOCK_PLAN_CONFIGURATION_ID,
      }),
    );
    expect(result.source).toBe("api");
  });

  it("rejects when the API save fails without mock fallback", async () => {
    createPlanConfiguration.mockRejectedValue(new Error("network error"));

    await expect(
      saveBehaviorPoliciesForTemplate(
        WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_ID,
        1,
        DEFAULT_BEHAVIOR_POLICIES,
      ),
    ).rejects.toThrow("network error");
  });

  it("persists to mock storage when the API save fails with mock fallback enabled", async () => {
    allowsMockFallback.mockReturnValue(true);
    createPlanConfiguration.mockRejectedValue(new Error("network error"));

    const result = await saveBehaviorPoliciesForTemplate(
      WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_ID,
      1,
      DEFAULT_BEHAVIOR_POLICIES,
    );

    expect(result.source).toBe("mock");
    expect(result.configuration.planTemplateId).toBe(
      WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_ID,
    );
    expect(result.error).toContain("network error");

    listPlanConfigurations.mockImplementation(() => {
      throw new Error("still offline");
    });
    const loaded = await loadBehaviorPoliciesForTemplate(
      WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_ID,
    );

    expect(loaded.configuration?.id).toBe(MOCK_PLAN_CONFIGURATION_ID);
    expect(loaded.policies).toEqual(DEFAULT_BEHAVIOR_POLICIES);
  });
});
