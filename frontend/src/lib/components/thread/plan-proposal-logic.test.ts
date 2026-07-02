import { describe, expect, it } from "vitest";
import { PlanConfigurationStatus } from "$lib/gen/harpia/plans/v1/plans_pb";
import {
  applyRefinementSelection,
  buildFinalConfirmation,
  confirmPrompt,
  confirmSummaryLabel,
  createdActionIds,
  createdActionI18nKey,
  selectBestCandidate,
  type PlanProposalCandidate,
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

  it("normalizes third-person model summaries for direct user-facing confirmation", () => {
    expect(
      confirmSummaryLabel(
        "O usuário quer escrever uma newsletter.",
        "Newsletter",
        "newsletter",
      ),
    ).toBe("você quer escrever uma newsletter");
    expect(
      confirmSummaryLabel(
        "The user wants to write a newsletter.",
        "Newsletter",
        "newsletter",
      ),
    ).toBe("you want to write a newsletter");
  });
});

describe("confirmPrompt", () => {
  it("wraps a normalized second-person clause in a natural confirmation sentence", () => {
    expect(
      confirmPrompt(
        "pt-BR",
        "O usuário quer escrever uma newsletter.",
        "Newsletter",
        "newsletter",
      ),
    ).toBe("Entendi que você quer escrever uma newsletter. Posso criar esse plano?");
    expect(
      confirmPrompt(
        "en",
        "The user wants to write a newsletter.",
        "Newsletter",
        "newsletter",
      ),
    ).toBe("Got it — you want to write a newsletter. Shall I create this plan?");
  });

  it("falls back to the plan name when there is no usable summary", () => {
    expect(confirmPrompt("pt-BR", "", "Newsletter semanal", "weekly")).toBe(
      "Você quer criar Newsletter semanal. Posso seguir?",
    );
    expect(confirmPrompt("en", "  ", "", "weekly")).toBe(
      "You want to create weekly. Shall I proceed?",
    );
  });
});

describe("createdActionIds", () => {
  it("offers run now for runnable configs", () => {
    expect(createdActionIds(PlanConfigurationStatus.RUNNABLE)).toEqual([
      "runNow",
      "schedule",
      "reviewPlan",
      "adjustConfiguration",
      "anythingElse",
    ]);
  });

  it("offers finish setup for draft configs", () => {
    expect(createdActionIds(PlanConfigurationStatus.DRAFT)).toEqual([
      "finishSetup",
      "schedule",
      "reviewPlan",
      "adjustConfiguration",
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
    expect(createdActionI18nKey("reviewPlan")).toBe(
      "thread.created.reviewPlan",
    );
    expect(createdActionI18nKey("adjustConfiguration")).toBe(
      "thread.created.adjustConfiguration",
    );
  });
});

describe("selectBestCandidate", () => {
  const candidates: PlanProposalCandidate[] = [
    {
      template_id: "template-a",
      template_key: "weekly-a",
      template_name: "Weekly A",
      confidence: 0.82,
      input_values_json: "{}",
    },
    {
      template_id: "template-b",
      template_key: "weekly-b",
      template_name: "Weekly B",
      confidence: 0.74,
      input_values_json: "{}",
    },
  ];

  it("uses the backend best-candidate marker when present", () => {
    expect(selectBestCandidate(candidates, "template-b")?.template_id).toBe(
      "template-b",
    );
  });

  it("falls back to highest confidence when no marker matches", () => {
    expect(selectBestCandidate(candidates, "missing")?.template_id).toBe(
      "template-a",
    );
  });
});

describe("applyRefinementSelection", () => {
  it("accumulates refinement turns without dropping previous selections", () => {
    const afterCandidate = applyRefinementSelection(undefined, {
      kind: "candidate",
      label: "Plano",
      value: "weekly-newsletter-linkedin",
    });
    const afterAudience = applyRefinementSelection(afterCandidate, {
      kind: "audience",
      label: "Público",
      value: "founders",
    });
    const afterThemes = applyRefinementSelection(afterAudience, {
      kind: "themes",
      label: "Temas",
      value: ["AI", "B2B"],
    });
    const afterAvoid = applyRefinementSelection(afterThemes, {
      kind: "topicsToAvoid",
      label: "Evitar",
      value: ["rumores"],
    });
    const afterSources = applyRefinementSelection(afterAvoid, {
      kind: "sourceGroups",
      label: "Fontes",
      value: ["tech", "business"],
    });
    const afterRange = applyRefinementSelection(afterSources, {
      kind: "dateRange",
      label: "Período",
      value: { startDate: "2026-06-01", endDate: "2026-07-01" },
    });
    const final = applyRefinementSelection(afterRange, {
      kind: "confirmation",
      label: "Confirmação",
      value: "confirm",
    });

    expect(final.turns.map((turn) => turn.kind)).toEqual([
      "candidate",
      "audience",
      "themes",
      "topicsToAvoid",
      "sourceGroups",
      "dateRange",
      "confirmation",
    ]);
    expect(final.audience).toBe("founders");
    expect(final.themes).toEqual(["AI", "B2B"]);
    expect(final.topicsToAvoid).toEqual(["rumores"]);
    expect(final.sourceGroups).toEqual(["tech", "business"]);
    expect(final.dateRange).toEqual({
      startDate: "2026-06-01",
      endDate: "2026-07-01",
    });
  });
});

describe("buildFinalConfirmation", () => {
  it("builds a complete localized sentence in English", () => {
    expect(
      buildFinalConfirmation("en", {
        planName: "LinkedIn news digest",
        audience: "startup founders",
        themes: ["AI", "B2B"],
        sourceGroups: ["Tech", "Business"],
        dateRange: { startDate: "2026-06-01", endDate: "2026-07-01" },
      }),
    ).toBe(
      "I'll create LinkedIn news digest for startup founders, focused on AI, B2B, using Tech, Business as source groups, with articles published from 2026-06-01 to 2026-07-01.",
    );
  });

  it("builds a complete localized sentence in Portuguese", () => {
    expect(
      buildFinalConfirmation("pt-BR", {
        planName: "boletim para LinkedIn",
        audience: "fundadores",
        themes: ["IA", "B2B"],
        sourceGroups: ["Tecnologia", "Negócios"],
        dateRange: { startDate: "2026-06-01", endDate: "2026-07-01" },
      }),
    ).toBe(
      "Vou criar boletim para LinkedIn para fundadores, com foco em IA, B2B, usando Tecnologia, Negócios como grupos de fontes, com artigos publicados de 2026-06-01 a 2026-07-01.",
    );
  });
});
