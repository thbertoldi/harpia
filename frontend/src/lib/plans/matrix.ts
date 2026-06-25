/**
 * Pure helpers for the binding-matrix card (M6 §2.1).
 *
 * The matrix card is driven by a single `ASSISTANT_PROMPT` whose
 * `payload_json` carries the whole step list. These helpers parse that
 * payload, total the per-run cost from the rows' own option pricing, and
 * decide whether the configuration is complete enough to be made runnable.
 *
 * The backend protocol is locked; the shapes below mirror it exactly. See
 * the spec at docs/superpowers/specs/2026-06-22-harpia-m6-lapidacao-design.md.
 */

export interface MatrixOption {
  id: string;
  label: string;
  sublabel?: string;
  value: string;
  /** Per-run price in BRL. May be absent (e.g. free integrations). */
  price_brl?: number;
}

export interface MatrixRow {
  step_key: string;
  step_title: string;
  contracts: {
    input: string;
    output: string;
  };
  options: MatrixOption[];
  /** Executor option id currently bound to this step, or "" when unbound. */
  current_executor_id: string;
  current_overseer_id: string;
  current_overseer_label: string;
}

export interface MatrixPayload {
  state: "BINDING_MATRIX";
  /** True once behavior policies have been set on the configuration. */
  policies_set: boolean;
  rows: MatrixRow[];
}

/**
 * Parse the matrix `ASSISTANT_PROMPT` payload.
 *
 * @throws if the JSON is malformed, not an object, or carries a state other
 *   than `BINDING_MATRIX`.
 */
export function parseMatrixPayload(json: string): MatrixPayload {
  let raw: unknown;
  try {
    raw = JSON.parse(json);
  } catch {
    throw new Error("parseMatrixPayload: payload is not valid JSON");
  }
  if (typeof raw !== "object" || raw === null) {
    throw new Error("parseMatrixPayload: payload is not an object");
  }
  const obj = raw as Record<string, unknown>;
  if (obj.state !== "BINDING_MATRIX") {
    throw new Error(
      `parseMatrixPayload: expected state "BINDING_MATRIX", got ${JSON.stringify(obj.state)}`,
    );
  }
  if (!Array.isArray(obj.rows)) {
    throw new Error("parseMatrixPayload: rows is not an array");
  }
  const rows: MatrixRow[] = obj.rows.map((r, i) => normalizeRow(r, i));
  return {
    state: "BINDING_MATRIX",
    policies_set: obj.policies_set === true,
    rows,
  };
}

function normalizeRow(raw: unknown, index: number): MatrixRow {
  if (typeof raw !== "object" || raw === null) {
    throw new Error(`parseMatrixPayload: row ${index} is not an object`);
  }
  const r = raw as Record<string, unknown>;
  const contracts =
    typeof r.contracts === "object" && r.contracts !== null
      ? (r.contracts as Record<string, unknown>)
      : {};
  const options = Array.isArray(r.options)
    ? r.options.map((o, j) => normalizeOption(o, index, j))
    : [];
  return {
    step_key: String(r.step_key ?? ""),
    step_title: String(r.step_title ?? ""),
    contracts: {
      input: String(contracts.input ?? ""),
      output: String(contracts.output ?? ""),
    },
    options,
    current_executor_id: String(r.current_executor_id ?? ""),
    current_overseer_id: String(r.current_overseer_id ?? ""),
    current_overseer_label: String(r.current_overseer_label ?? ""),
  };
}

function normalizeOption(
  raw: unknown,
  rowIndex: number,
  optIndex: number,
): MatrixOption {
  if (typeof raw !== "object" || raw === null) {
    throw new Error(
      `parseMatrixPayload: row ${rowIndex} option ${optIndex} is not an object`,
    );
  }
  const o = raw as Record<string, unknown>;
  const option: MatrixOption = {
    id: String(o.id ?? ""),
    label: String(o.label ?? ""),
    value: String(o.value ?? ""),
  };
  if (typeof o.sublabel === "string") option.sublabel = o.sublabel;
  if (typeof o.price_brl === "number" && Number.isFinite(o.price_brl)) {
    option.price_brl = o.price_brl;
  }
  return option;
}

export interface RunCostSummary {
  totalBrl: number;
  unboundCount: number;
}

/**
 * Sum the per-run cost across rows.
 *
 * For each row, the bound executor is the option whose id matches
 * `current_executor_id`; its `price_brl` (defaulting to 0) is added to the
 * total. Rows with no binding — or a binding that doesn't resolve to a known
 * option — add 0 and bump `unboundCount`.
 *
 * Note: `cost.ts` (M5) computes the equivalent total from the proto
 * configuration + executor catalog. The matrix payload already carries
 * per-option prices inline, so this avoids the extra catalog lookup at the
 * card layer.
 */
export function computeRunCostBRL(rows: MatrixRow[]): RunCostSummary {
  let totalBrl = 0;
  let unboundCount = 0;
  for (const row of rows) {
    if (!row.current_executor_id) {
      unboundCount += 1;
      continue;
    }
    const bound = row.options.find((o) => o.id === row.current_executor_id);
    if (!bound) {
      unboundCount += 1;
      continue;
    }
    totalBrl += bound.price_brl ?? 0;
  }
  return { totalBrl, unboundCount };
}

/**
 * The configuration is complete (and the primary "Save & make runnable"
 * action may enable) when every row has an executor bound and the behavior
 * policies have been set.
 */
export function isMatrixComplete(payload: MatrixPayload): boolean {
  if (!payload.policies_set) return false;
  if (payload.rows.length === 0) return false;
  return payload.rows.every((row) => row.current_executor_id !== "");
}
