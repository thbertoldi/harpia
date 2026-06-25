import { describe, expect, it } from "vitest";
import {
  computeRunCostBRL,
  isMatrixComplete,
  parseMatrixPayload,
  type MatrixPayload,
  type MatrixRow,
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
      parseMatrixPayload(
        JSON.stringify({ state: "BINDING_MATRIX", rows: {} }),
      ),
    ).toThrow(/rows is not an array/);
  });
});

describe("computeRunCostBRL", () => {
  it("sums the bound option price across rows", () => {
    const rows = [
      row({ current_executor_id: "rss" }), // price 0
      row({ current_executor_id: "writer" }), // price 0.42
    ];
    expect(computeRunCostBRL(rows)).toEqual({ totalBrl: 0.42, unboundCount: 0 });
  });

  it("counts unbound rows and adds 0 for them", () => {
    const rows = [
      row({ current_executor_id: "writer" }), // 0.42
      row({ current_executor_id: "" }), // unbound
      row({ current_executor_id: "" }), // unbound
    ];
    expect(computeRunCostBRL(rows)).toEqual({ totalBrl: 0.42, unboundCount: 2 });
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
});
