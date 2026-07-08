import { describe, expect, it } from "vitest";
import {
  ElicitationTimeoutBehavior,
  PublishApprovalMode,
  type PlanConfiguration,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import {
  buildBindingStepView,
  buildOverseerStepView,
  buildPoliciesStepView,
  hydratePoliciesPayloadFromValues,
  parseAssistantPromptState,
  parseGenericAssistantPromptPayload,
  policiesComplete,
  promptChipsShown,
  reflectBindingPayload,
  reflectOverseerPayload,
  reflectPolicyPayload,
} from "./configuration-flow";
import type {
  BindingStepPayload,
  MatrixRow,
  OverseerStepPayload,
  PoliciesStepPayload,
} from "./matrix";

const rows: MatrixRow[] = [
  {
    step_key: "fetch-news",
    step_title: "Fetch news",
    contracts: { input: "date_range", output: "NewsList" },
    options: [{ id: "rss", label: "RSS", value: "rss" }],
    current_executor_id: "rss",
    current_overseer_id: "",
    current_overseer_label: "",
  },
  {
    step_key: "write-draft",
    step_title: "Write draft",
    contracts: { input: "NewsList", output: "TextDraft" },
    options: [{ id: "writer", label: "Writer", value: "writer" }],
    current_executor_id: "",
    current_overseer_id: "",
    current_overseer_label: "",
  },
];

describe("parseAssistantPromptState", () => {
  it("returns prompt states without exposing raw JSON parsing to components", () => {
    expect(parseAssistantPromptState('{"state":"BINDING_STEP"}')).toBe(
      "BINDING_STEP",
    );
    expect(parseAssistantPromptState("{}")).toBeNull();
    expect(parseAssistantPromptState("not json")).toBeNull();
  });
});

describe("parseGenericAssistantPromptPayload", () => {
  it("normalizes generic assistant options and falls back safely", () => {
    expect(
      parseGenericAssistantPromptPayload(
        JSON.stringify({
          state: "AWAITING_TEMPLATE",
          options: [
            {
              id: "weekly",
              label: "Weekly newsletter",
              sublabel: "LinkedIn",
              value: "news-to-social-post",
              price_brl: 2,
            },
          ],
        }),
      ),
    ).toEqual({
      state: "AWAITING_TEMPLATE",
      step_key: "",
      options: [
        {
          id: "weekly",
          label: "Weekly newsletter",
          sublabel: "LinkedIn",
          value: "news-to-social-post",
          price_brl: 2,
        },
      ],
    });
    expect(parseGenericAssistantPromptPayload("bad")).toEqual({
      state: "",
      step_key: "",
      options: [],
    });
  });
});

describe("promptChipsShown", () => {
  it("keeps chips visible only for live or reopened prompts", () => {
    expect(
      promptChipsShown({
        isLive: true,
        editingAnswered: false,
        submitted: false,
      }),
    ).toBe(true);
    expect(
      promptChipsShown({
        isLive: false,
        editingAnswered: true,
        submitted: false,
      }),
    ).toBe(true);
    expect(
      promptChipsShown({
        isLive: true,
        editingAnswered: false,
        submitted: true,
      }),
    ).toBe(false);
  });
});

describe("buildBindingStepView", () => {
  it("summarizes focused binding progress for the golden setup path", () => {
    const payload: BindingStepPayload = {
      state: "BINDING_STEP",
      step_key: "write-draft",
      options: [{ id: "writer", label: "Writer", value: "writer" }],
      policies_set: false,
      rows,
    };

    const view = buildBindingStepView(payload, {
      isLive: true,
      editingAnswered: false,
      submitted: false,
    });

    expect(view.focusedRow?.step_key).toBe("write-draft");
    expect(view.boundCount).toBe(1);
    expect(view.totalCount).toBe(2);
    expect(view.showChips).toBe(true);
    expect(
      reflectBindingPayload(payload, "write-draft", "writer").rows[1],
    ).toMatchObject({ current_executor_id: "writer" });
  });
});

describe("buildOverseerStepView", () => {
  it("summarizes required overseer progress after executors are bound", () => {
    const payload: OverseerStepPayload = {
      state: "OVERSEER_STEP",
      step_key: "write-draft",
      options: [{ id: "me", label: "You", value: "user-1" }],
      required_step_keys: ["write-draft"],
      rows: rows.map((row) => ({ ...row, current_executor_id: "bound" })),
    };

    const view = buildOverseerStepView(payload, {
      isLive: true,
      editingAnswered: false,
      submitted: false,
    });

    expect(view.focusedRow?.step_key).toBe("write-draft");
    expect(view.requiredRows).toHaveLength(1);
    expect(view.overseerCount).toBe(0);
    expect(view.totalRequired).toBe(1);
    expect(view.showChips).toBe(true);
    expect(
      reflectOverseerPayload(payload, "write-draft", "user-1", "You").rows[1],
    ).toMatchObject({
      current_overseer_id: "user-1",
      current_overseer_label: "You",
    });
  });
});

describe("buildPoliciesStepView", () => {
  const payload: PoliciesStepPayload = {
    state: "POLICIES_STEP",
    policies_set: false,
    fields: [
      {
        key: "publish_approval_mode",
        parameter_key: "approval_mode",
        current_value: "",
        options: [
          {
            id: "require_approval",
            label: "Require approval",
            value: "require_approval",
          },
        ],
      },
      {
        key: "elicitation_timeout_behavior",
        parameter_key: "timeout_behavior",
        current_value: "skip",
        options: [{ id: "skip", label: "Skip", value: "skip" }],
      },
    ],
  };

  it("summarizes policy selection progress and reflects picks", () => {
    const view = buildPoliciesStepView(payload, {
      isLive: true,
      editingAnswered: false,
      submitted: false,
    });

    expect(view.selectedCount).toBe(1);
    expect(view.totalCount).toBe(2);
    expect(view.complete).toBe(false);
    expect(view.showChips).toBe(true);
    expect(
      reflectPolicyPayload(payload, "publish_approval_mode", "require_approval")
        .fields[0],
    ).toMatchObject({ current_value: "require_approval" });
  });

  it("hydrates policy values from parameter values", () => {
    const hydrated = hydratePoliciesPayloadFromValues(
      payload,
      { approval_mode: "auto_publish" },
      true,
    );

    expect(hydrated.policies_set).toBe(true);
    expect(hydrated.fields[0].current_value).toBe("auto_publish");
    expect(hydrated.fields[1].current_value).toBe("skip");
  });
});

describe("policiesComplete", () => {
  it("requires both behavior policy values for the review gate", () => {
    expect(
      policiesComplete({
        behaviorPolicies: {
          publishApprovalMode: PublishApprovalMode.REQUIRE_APPROVAL,
          elicitationTimeoutBehavior: ElicitationTimeoutBehavior.FAIL_PLAN,
        },
      } as PlanConfiguration),
    ).toBe(true);
    expect(
      policiesComplete({
        behaviorPolicies: {
          publishApprovalMode: PublishApprovalMode.REQUIRE_APPROVAL,
          elicitationTimeoutBehavior: ElicitationTimeoutBehavior.UNSPECIFIED,
        },
      } as PlanConfiguration),
    ).toBe(false);
  });
});
