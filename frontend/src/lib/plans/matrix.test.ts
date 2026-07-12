import { create } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";
import {
  ExecutorRequirementSchema,
  PlanStepSchema,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import {
  computeRunCostBRL,
  hydrateMatrixPayload,
  isMatrixComplete,
  isPolicyOptionSelected,
  parseBindingStepPayload,
  parseMatrixPayload,
  parseOverseerStepPayload,
  parsePoliciesStepPayload,
  policyFieldChipsShown,
  policyChipsShown,
  matrixRowsThatWillRun,
  type BindingStepPayload,
  type MatrixPayload,
  type MatrixRow,
  type OverseerStepPayload,
  type PolicyField,
} from "./matrix";

function row(overrides: Partial<MatrixRow> = {}): MatrixRow {
  return {
    step_key: "step_1",
    step_title: "Fetch newsletter",
    contracts: { input: "", output: "NewsList" },
    options: [
      { id: "rss", label: "RSS Reader", value: "rss", price_brl: 0 },
      { id: "writer", label: "Writer", value: "writer", price_brl: 0.42 },
    ],
    current_executor_id: "",
    current_overseer_id: "ana",
    current_overseer_label: "Ana",
    ...overrides,
  };
}

function payload(overrides: Partial<MatrixPayload> = {}): MatrixPayload {
  return {
    state: "BINDING_MATRIX",
    policies_set: true,
    rows: [row()],
    ...overrides,
  };
}

describe("parseMatrixPayload", () => {
  it("parses a well-formed BINDING_MATRIX payload", () => {
    const json = JSON.stringify({
      state: "BINDING_MATRIX",
      policies_set: true,
      rows: [
        {
          step_key: "step_1",
          step_title: "Fetch newsletter",
          contracts: { input: "", output: "NewsList" },
          options: [
            {
              id: "rss",
              label: "RSS Reader",
              sublabel: "Bloomberg",
              value: "rss",
              price_brl: 0,
            },
          ],
          current_executor_id: "rss",
          current_overseer_id: "ana",
          current_overseer_label: "Ana",
        },
      ],
    });
    const parsed = parseMatrixPayload(json);
    expect(parsed.state).toBe("BINDING_MATRIX");
    expect(parsed.policies_set).toBe(true);
    expect(parsed.rows).toHaveLength(1);
    expect(parsed.rows[0].step_title).toBe("Fetch newsletter");
    expect(parsed.rows[0].contracts.output).toBe("NewsList");
    expect(parsed.rows[0].options[0].sublabel).toBe("Bloomberg");
    expect(parsed.rows[0].options[0].price_brl).toBe(0);
    expect(parsed.rows[0].current_executor_id).toBe("rss");
  });

  it("defaults policies_set to false and tolerates missing optionals", () => {
    const json = JSON.stringify({
      state: "BINDING_MATRIX",
      rows: [{ step_key: "s", step_title: "T", options: [] }],
    });
    const parsed = parseMatrixPayload(json);
    expect(parsed.policies_set).toBe(false);
    expect(parsed.rows[0].contracts).toEqual({ input: "", output: "" });
    expect(parsed.rows[0].current_executor_id).toBe("");
    expect(parsed.rows[0].options).toEqual([]);
  });

  it("throws on malformed JSON", () => {
    expect(() => parseMatrixPayload("{not json")).toThrow(/valid JSON/);
  });

  it("throws on a non-object payload", () => {
    expect(() => parseMatrixPayload("42")).toThrow(/not an object/);
    expect(() => parseMatrixPayload("null")).toThrow(/not an object/);
  });

  it("throws on the wrong state", () => {
    expect(() =>
      parseMatrixPayload(JSON.stringify({ state: "landing", rows: [] })),
    ).toThrow(/BINDING_MATRIX/);
  });

  it("throws when rows is not an array", () => {
    expect(() =>
      parseMatrixPayload(JSON.stringify({ state: "BINDING_MATRIX", rows: {} })),
    ).toThrow(/rows is not an array/);
  });
});

describe("parseBindingStepPayload", () => {
  it("parses focused options and shared matrix rows", () => {
    const json = JSON.stringify({
      state: "BINDING_STEP",
      step_key: "fetch-news",
      options: [
        { id: "rss-tech", label: "Tech RSS", value: "rss-tech" },
        { id: "rss-br", label: "Brazil RSS", value: "rss-br" },
      ],
      policies_set: false,
      rows: [
        row({
          step_key: "fetch-news",
          current_executor_id: "",
          options: [{ id: "rss-tech", label: "Tech RSS", value: "rss-tech" }],
        }),
        row({
          step_key: "write-draft",
          current_executor_id: "writer",
          options: [{ id: "writer", label: "Writer", value: "writer" }],
        }),
      ],
    });

    const parsed = parseBindingStepPayload(json);

    expect(parsed.state).toBe("BINDING_STEP");
    expect(parsed.step_key).toBe("fetch-news");
    expect(parsed.options.map((option) => option.id)).toEqual([
      "rss-tech",
      "rss-br",
    ]);
    expect(parsed.rows).toHaveLength(2);
    expect(parsed.rows[1].current_executor_id).toBe("writer");
  });

  it("hydrates rows from saved SlotBindings", () => {
    const payload: BindingStepPayload = {
      state: "BINDING_STEP",
      step_key: "fetch-news",
      options: [{ id: "rss-tech", label: "Tech RSS", value: "rss-tech" }],
      policies_set: false,
      rows: [
        row({ step_key: "fetch-news", current_executor_id: "" }),
        row({ step_key: "write-draft", current_executor_id: "" }),
      ],
    };

    const hydrated = hydrateMatrixPayload(payload, {
      policiesSet: true,
      slotBindings: [
        { stepKey: "fetch-news", executorInstallationId: "rss-tech" },
        { stepKey: "write-draft", executorInstallationId: "writer" },
      ],
    });

    expect(hydrated.policies_set).toBe(true);
    expect(hydrated.rows.map((r) => r.current_executor_id)).toEqual([
      "rss-tech",
      "writer",
    ]);
  });

  it("throws on the wrong state", () => {
    expect(() =>
      parseBindingStepPayload(
        JSON.stringify({ state: "BINDING_MATRIX", rows: [] }),
      ),
    ).toThrow(/BINDING_STEP/);
  });
});

describe("parseOverseerStepPayload", () => {
  it("parses overseer options, required step keys, and shared rows", () => {
    const json = JSON.stringify({
      state: "OVERSEER_STEP",
      step_key: "write-draft",
      options: [{ id: "user-ana", label: "Ana", value: "user-ana" }],
      required_step_keys: ["write-draft", "adapt-for-linkedin"],
      rows: [
        row({
          step_key: "write-draft",
          current_executor_id: "writer",
          current_overseer_id: "",
        }),
        row({
          step_key: "adapt-for-linkedin",
          current_executor_id: "voice",
          current_overseer_id: "user-paula",
          current_overseer_label: "Paula",
        }),
      ],
    });

    const parsed = parseOverseerStepPayload(json);

    expect(parsed.state).toBe("OVERSEER_STEP");
    expect(parsed.step_key).toBe("write-draft");
    expect(parsed.options).toEqual([
      { id: "user-ana", label: "Ana", value: "user-ana" },
    ]);
    expect(parsed.required_step_keys).toEqual([
      "write-draft",
      "adapt-for-linkedin",
    ]);
    expect(parsed.rows[1].current_overseer_id).toBe("user-paula");
  });

  it("hydrates rows from saved OverseerBindings", () => {
    const payload: OverseerStepPayload = {
      state: "OVERSEER_STEP",
      step_key: "write-draft",
      options: [{ id: "user-ana", label: "Ana", value: "user-ana" }],
      required_step_keys: ["write-draft"],
      rows: [
        row({
          step_key: "write-draft",
          current_executor_id: "writer",
          current_overseer_id: "",
          current_overseer_label: "",
        }),
      ],
    };

    const hydrated = hydrateMatrixPayload(payload, {
      policiesSet: true,
      slotBindings: [],
      overseerBindings: [
        { stepKey: "write-draft", overseerUserId: "user-ana" },
      ],
    });

    expect(hydrated.rows[0].current_overseer_id).toBe("user-ana");
    expect(hydrated.rows[0].current_overseer_label).toBe("user-ana");
  });

  it("throws on the wrong state", () => {
    expect(() =>
      parseOverseerStepPayload(
        JSON.stringify({ state: "BINDING_STEP", rows: [] }),
      ),
    ).toThrow(/OVERSEER_STEP/);
  });
});

describe("parsePoliciesStepPayload", () => {
  it("parses policy fields and options", () => {
    const json = JSON.stringify({
      state: "POLICIES_STEP",
      policies_set: false,
      fields: [
        {
          key: "publish_approval_mode",
          parameter_key: "approval_mode",
          current_value: "require_approval",
          options: [
            {
              id: "require_approval",
              label: "Require approval",
              value: "require_approval",
            },
          ],
        },
      ],
    });

    const parsed = parsePoliciesStepPayload(json);

    expect(parsed.state).toBe("POLICIES_STEP");
    expect(parsed.policies_set).toBe(false);
    expect(parsed.fields).toHaveLength(1);
    expect(parsed.fields[0].key).toBe("publish_approval_mode");
    expect(parsed.fields[0].parameter_key).toBe("approval_mode");
    expect(parsed.fields[0].current_value).toBe("require_approval");
    expect(parsed.fields[0].options[0].value).toBe("require_approval");
  });

  it("defaults missing field options and current values", () => {
    const parsed = parsePoliciesStepPayload(
      JSON.stringify({
        state: "POLICIES_STEP",
        fields: [{ key: "elicitation_timeout_behavior" }],
      }),
    );

    expect(parsed.policies_set).toBe(false);
    expect(parsed.fields[0]).toMatchObject({
      key: "elicitation_timeout_behavior",
      parameter_key: "",
      current_value: "",
      options: [],
    });
  });

  it("throws on malformed policies payloads", () => {
    expect(() => parsePoliciesStepPayload("{bad")).toThrow(/valid JSON/);
    expect(() =>
      parsePoliciesStepPayload(JSON.stringify({ state: "BINDING_STEP" })),
    ).toThrow(/POLICIES_STEP/);
    expect(() =>
      parsePoliciesStepPayload(
        JSON.stringify({ state: "POLICIES_STEP", fields: {} }),
      ),
    ).toThrow(/fields is not an array/);
  });
});

function field(overrides: Partial<PolicyField> = {}): PolicyField {
  return {
    key: "publish_approval_mode",
    parameter_key: "approval_mode",
    current_value: "",
    options: [
      {
        id: "require_approval",
        label: "Require approval",
        value: "require_approval",
      },
      { id: "auto_publish", label: "Auto publish", value: "auto_publish" },
    ],
    ...overrides,
  };
}

describe("policyChipsShown", () => {
  it("shows chips while live and unsubmitted", () => {
    expect(
      policyChipsShown({
        isLive: true,
        editingAnswered: false,
        submitted: false,
      }),
    ).toBe(true);
  });

  it("shows chips while editing an answered card", () => {
    expect(
      policyChipsShown({
        isLive: false,
        editingAnswered: true,
        submitted: false,
      }),
    ).toBe(true);
  });

  it("hides chips once submitted", () => {
    expect(
      policyChipsShown({
        isLive: true,
        editingAnswered: false,
        submitted: true,
      }),
    ).toBe(false);
    expect(
      policyChipsShown({
        isLive: false,
        editingAnswered: true,
        submitted: true,
      }),
    ).toBe(false);
  });

  it("hides chips on a settled answered card", () => {
    expect(
      policyChipsShown({
        isLive: false,
        editingAnswered: false,
        submitted: false,
      }),
    ).toBe(false);
  });
});

describe("policyFieldChipsShown", () => {
  it("hides chips for an answered field on the live flow", () => {
    const answered = field({ current_value: "require_approval" });
    expect(
      policyFieldChipsShown(answered, {
        isLive: true,
        editingAnswered: false,
        submitted: false,
      }),
    ).toBe(false);
  });

  it("regression: reveals chips for an answered field when editing", () => {
    const answered = field({ current_value: "require_approval" });
    expect(
      policyFieldChipsShown(answered, {
        isLive: false,
        editingAnswered: true,
        submitted: false,
      }),
    ).toBe(true);
  });

  it("shows chips for an unset field on the live flow", () => {
    expect(
      policyFieldChipsShown(field(), {
        isLive: true,
        editingAnswered: false,
        submitted: false,
      }),
    ).toBe(true);
  });
});

describe("isPolicyOptionSelected", () => {
  it("matches by value or id", () => {
    const answered = field({ current_value: "require_approval" });
    expect(isPolicyOptionSelected(answered, answered.options[0])).toBe(true);
    expect(isPolicyOptionSelected(answered, answered.options[1])).toBe(false);
  });
});

describe("computeRunCostBRL", () => {
  it("sums the bound option price across rows", () => {
    const rows = [
      row({ current_executor_id: "rss" }), // price 0
      row({ current_executor_id: "writer" }), // price 0.42
    ];
    expect(computeRunCostBRL(rows)).toEqual({
      totalBrl: 0.42,
      unboundCount: 0,
    });
  });

  it("counts unbound rows and adds 0 for them", () => {
    const rows = [
      row({ current_executor_id: "writer" }), // 0.42
      row({ current_executor_id: "" }), // unbound
      row({ current_executor_id: "" }), // unbound
    ];
    expect(computeRunCostBRL(rows)).toEqual({
      totalBrl: 0.42,
      unboundCount: 2,
    });
  });

  it("treats a binding to an unknown option as unbound", () => {
    const rows = [row({ current_executor_id: "ghost" })];
    expect(computeRunCostBRL(rows)).toEqual({ totalBrl: 0, unboundCount: 1 });
  });

  it("treats a missing price_brl as 0", () => {
    const rows = [
      row({
        current_executor_id: "free",
        options: [{ id: "free", label: "Free", value: "free" }],
      }),
    ];
    expect(computeRunCostBRL(rows)).toEqual({ totalBrl: 0, unboundCount: 0 });
  });

  it("returns zeros for an empty matrix", () => {
    expect(computeRunCostBRL([])).toEqual({ totalBrl: 0, unboundCount: 0 });
  });
});

describe("isMatrixComplete", () => {
  it("is false for an empty matrix", () => {
    expect(isMatrixComplete(payload({ rows: [] }))).toBe(false);
  });

  it("is false when policies are unset even if all rows are bound", () => {
    expect(
      isMatrixComplete(
        payload({
          policies_set: false,
          rows: [row({ current_executor_id: "rss" })],
        }),
      ),
    ).toBe(false);
  });

  it("is false when any row is unbound", () => {
    expect(
      isMatrixComplete(
        payload({
          rows: [
            row({ current_executor_id: "rss" }),
            row({ current_executor_id: "" }),
          ],
        }),
      ),
    ).toBe(false);
  });

  it("is true when all rows are bound and policies are set", () => {
    expect(
      isMatrixComplete(
        payload({
          policies_set: true,
          rows: [
            row({ current_executor_id: "rss" }),
            row({ current_executor_id: "writer" }),
          ],
        }),
      ),
    ).toBe(true);
  });

  it("ignores unbound opted-out rows", () => {
    const imageStep = create(PlanStepSchema, {
      key: "generate-image",
      executorRequirement: create(ExecutorRequirementSchema, {
        optionalCapabilities: ["image-generation"],
      }),
    });
    const rows = [
      row({ step_key: "draft", current_executor_id: "writer" }),
      row({ step_key: "generate-image", current_executor_id: "" }),
    ];
    const participation = {
      steps: [create(PlanStepSchema, { key: "draft" }), imageStep],
      includedOptionalCapabilities: [],
    };

    expect(isMatrixComplete(payload({ rows }), participation)).toBe(true);
    expect(matrixRowsThatWillRun(rows, participation)).toHaveLength(1);
  });

  it("requires opted-in rows to be bound", () => {
    const imageStep = create(PlanStepSchema, {
      key: "generate-image",
      executorRequirement: create(ExecutorRequirementSchema, {
        optionalCapabilities: ["image-generation"],
      }),
    });
    const participation = {
      steps: [imageStep],
      includedOptionalCapabilities: ["image-generation"],
    };

    expect(
      isMatrixComplete(
        payload({ rows: [row({ step_key: "generate-image" })] }),
        participation,
      ),
    ).toBe(false);
  });
});

describe("hydrateMatrixPayload", () => {
  it("uses saved slot bindings and behavior policies to refresh stale prompt payloads", () => {
    const hydrated = hydrateMatrixPayload(
      payload({
        policies_set: false,
        rows: [
          row({ step_key: "fetch-news", current_executor_id: "" }),
          row({ step_key: "write-draft", current_executor_id: "" }),
        ],
      }),
      {
        slotBindings: [
          {
            stepKey: "fetch-news",
            executorInstallationId: "rss",
          },
          {
            stepKey: "write-draft",
            executorInstallationId: "writer",
          },
        ],
        overseerBindings: [
          { stepKey: "write-draft", overseerUserId: "user-ana" },
        ],
        policiesSet: true,
      },
    );

    expect(hydrated.policies_set).toBe(true);
    expect(hydrated.rows.map((r) => r.current_executor_id)).toEqual([
      "rss",
      "writer",
    ]);
    expect(hydrated.rows[1].current_overseer_id).toBe("user-ana");
  });

  it("drops opted-out rows from a stale payload using the fresh configuration", () => {
    const imageStep = create(PlanStepSchema, {
      key: "generate-image",
      executorRequirement: create(ExecutorRequirementSchema, {
        optionalCapabilities: ["image-generation"],
      }),
    });
    const hydrated = hydrateMatrixPayload(
      payload({
        rows: [
          row({ step_key: "draft", current_executor_id: "writer" }),
          row({ step_key: "generate-image", current_executor_id: "" }),
        ],
      }),
      {
        slotBindings: [],
        policiesSet: true,
        steps: [create(PlanStepSchema, { key: "draft" }), imageStep],
        includedOptionalCapabilities: [],
      },
    );

    expect(hydrated.rows.map((row) => row.step_key)).toEqual(["draft"]);
  });
});
