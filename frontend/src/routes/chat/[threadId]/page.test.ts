import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

describe("conversation execution focus", () => {
  it("scopes primary execution to the selected configuration and keeps other runs compact", () => {
    const source = readFileSync(
      new URL("./+page.svelte", import.meta.url),
      "utf8",
    );
    expect(source).toContain("planConfigurationId === activeConfigurationId");
    expect(source).toContain("targetedExecutionId");
    expect(source).toContain("offFocusExecutionViewModels");
    expect(source).toContain("planTemplateSnapshot");
    expect(source).toContain("retryPlanExecution");
  });
});
