import { describe, expect, it } from "vitest";
import { PlanConfigurationStatus } from "$lib/gen/harpia/plans/v1/plans_pb";
import {
  confirmSummaryLabel,
  createdActionIds,
  createdActionI18nKey,
} from "./plan-proposal-logic";

describe("confirmSummaryLabel", () => {
  it("prefers the LLM summary when present", () => {
    expect(
      confirmSummaryLabel("a LinkedIn post about retail", "LinkedIn Post", "linkedin"),
    ).toBe("a LinkedIn post about retail");
  });

  it("falls back to template display name then key", () => {
    expect(confirmSummaryLabel("", "LinkedIn Post", "linkedin")).toBe(
      "LinkedIn Post",
    );
    expect(confirmSummaryLabel("  ", "", "linkedin")).toBe("linkedin");
  });
});

describe("createdActionIds", () => {
  it("offers run now for runnable configs", () => {
    expect(createdActionIds(PlanConfigurationStatus.RUNNABLE)).toEqual([
      "runNow",
      "schedule",
      "anythingElse",
    ]);
  });

  it("offers finish setup for draft configs", () => {
    expect(createdActionIds(PlanConfigurationStatus.DRAFT)).toEqual([
      "finishSetup",
      "schedule",
      "anythingElse",
    ]);
  });
});

describe("createdActionI18nKey", () => {
  it("maps action ids to i18n keys", () => {
    expect(createdActionI18nKey("runNow")).toBe("thread.created.runNow");
    expect(createdActionI18nKey("finishSetup")).toBe(
      "thread.created.finishSetup",
    );
  });
});
