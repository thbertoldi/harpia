import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

describe("ExecutionTurnCard", () => {
  it("renders only projected interaction identities and terminal retry actions", () => {
    const source = readFileSync(
      new URL("./ExecutionTurnCard.svelte", import.meta.url),
      "utf8",
    );
    expect(source).toContain("elicitationId={interaction.requestId}");
    expect(source).toContain("reviewRequestId={interaction.requestId}");
    expect(source).toContain("approvalRequestId={interaction.requestId}");
    expect(source).toContain("onRetry?.(turn.stepExecutionId)");
  });
});
