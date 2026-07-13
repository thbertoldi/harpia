import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

describe("PlanExecutionCard class contract", () => {
  it("renders as a full-width structured card, not a constrained chat bubble", () => {
    const source = readFileSync(
      new URL("./PlanExecutionCard.svelte", import.meta.url),
      "utf8",
    );

    expect(source).toContain('class="w-full rounded-md');
    expect(source).not.toContain("max-w-[85%] rounded-md");
    expect(source).toContain("ExecutionTurnCard");
    expect(source).not.toContain("ApprovalRefCard");
    expect(source).not.toContain("ReviewRefCard");
  });
});
