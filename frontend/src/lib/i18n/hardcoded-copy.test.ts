import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import en from "./en.json";
import pt from "./pt-BR.json";

const FRONTEND_ROOT = resolve(import.meta.dirname, "../../..");

const HIGH_RISK_FILES = [
  "src/routes/+page.svelte",
  "src/lib/components/TaskDetail.svelte",
  "src/lib/components/FeedbackPanel.svelte",
  "src/lib/components/OngoingTaskCard.svelte",
];

const BANNED_LITERALS = [
  "What do you want to get done?",
  "Try asking",
  "Integrations",
  "Oversee",
  "Ongoing Tasks",
  "Loading ongoing tasks...",
  "Feedback submitted",
  "Task Detail",
  "Audit Log",
  "No active subtask",
];

function markupOnly(content: string): string {
  return content.replace(/<script[\s\S]*?<\/script>/gi, "");
}

describe("hardcoded copy guard", () => {
  it("avoids known hard-coded literals in high risk files", () => {
    for (const relativePath of HIGH_RISK_FILES) {
      const content = markupOnly(
        readFileSync(resolve(FRONTEND_ROOT, relativePath), "utf8"),
      );
      for (const literal of BANNED_LITERALS) {
        expect(
          content,
          `${relativePath} still contains "${literal}"`,
        ).not.toContain(literal);
      }
    }
  });
});

describe("i18n parity", () => {
  it("en.json and pt-BR.json have identical key sets", () => {
    expect(Object.keys(pt).sort()).toEqual(Object.keys(en).sort());
  });
});
