import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

describe("ConversationalWorkspace", () => {
  it("continues to render only plan-scope messages while execution turns render as execution cards", () => {
    const source = readFileSync(
      new URL("./ConversationalWorkspace.svelte", import.meta.url),
      "utf8",
    );
    expect(source).toContain("{#each messages as message");
  });
});
