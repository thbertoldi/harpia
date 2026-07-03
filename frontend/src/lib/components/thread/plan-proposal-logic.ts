import { PlanConfigurationStatus } from "$lib/gen/harpia/plans/v1/plans_pb";
import { translate, type Locale } from "$lib/i18n";

export type ProposalStage = "confirm" | "form" | "created";

export type CreatedActionId =
  | "runNow"
  | "finishSetup"
  | "schedule"
  | "reviewPlan"
  | "adjustConfiguration"
  | "anythingElse";

export interface PlanProposalCandidate {
  template_id: string;
  template_key: string;
  template_name: string;
  confidence: number;
  input_values_json: string;
}

export interface RefinementDateRange {
  startDate: string;
  endDate: string;
}

export type RefinementSelectionKind =
  | "candidate"
  | "audience"
  | "themes"
  | "topicsToAvoid"
  | "sourceGroups"
  | "dateRange"
  | "confirmation";

export interface RefinementSelection {
  kind: RefinementSelectionKind;
  label: string;
  value: string | string[] | RefinementDateRange;
}

export interface RefinementTurn extends RefinementSelection {
  summary: string;
}

export interface RefinementState {
  selectedCandidateId: string;
  audience: string;
  themes: string[];
  topicsToAvoid: string[];
  sourceGroups: string[];
  dateRange: RefinementDateRange | null;
  confirmed: boolean;
  turns: RefinementTurn[];
}

export interface FinalConfirmationInput {
  planName: string;
  audience: string;
  themes: string[];
  sourceGroups: string[];
  dateRange: RefinementDateRange;
}

function emptyRefinementState(): RefinementState {
  return {
    selectedCandidateId: "",
    audience: "",
    themes: [],
    topicsToAvoid: [],
    sourceGroups: [],
    dateRange: null,
    confirmed: false,
    turns: [],
  };
}

function selectionSummary(value: RefinementSelection["value"]): string {
  if (Array.isArray(value)) return value.join(", ");
  if (typeof value === "string") return value;
  return `${value.startDate} - ${value.endDate}`;
}

function stringList(value: RefinementSelection["value"]): string[] {
  if (Array.isArray(value)) return value.map(String).filter(Boolean);
  if (typeof value === "string") return value ? [value] : [];
  return [];
}

export function confirmSummaryLabel(
  summary: string,
  templateName: string,
  templateKey: string,
): string {
  const trimmed = normalizeUserFacingSummary(summary);
  if (trimmed) return trimmed;
  return templateName || templateKey;
}

function normalizeUserFacingSummary(summary: string): string {
  return summary
    .trim()
    .replace(/[.?!。]+$/u, "")
    .replace(/^o usuário quer\s+/i, "você quer ")
    .replace(/^a usuária quer\s+/i, "você quer ")
    .replace(/^the user wants to\s+/i, "you want to ");
}

// confirmPrompt builds the assistant's opening confirmation sentence. It names
// the selected plan directly so the sentence is always grammatical (regardless
// of the model's free-text summary) and so it visibly changes when the user
// picks a different candidate.
export function confirmPrompt(
  loc: Locale,
  templateName: string,
  templateKey: string,
): string {
  return translate("thread.propose.confirm", loc, {
    plan: templateName || templateKey,
  });
}

export function selectBestCandidate(
  candidates: PlanProposalCandidate[],
  bestCandidateId?: string,
): PlanProposalCandidate | null {
  if (candidates.length === 0) return null;
  const marked = candidates.find(
    (candidate) =>
      candidate.template_id === bestCandidateId ||
      candidate.template_key === bestCandidateId,
  );
  if (marked) return marked;
  return [...candidates].sort((a, b) => b.confidence - a.confidence)[0];
}

export function applyRefinementSelection(
  state: RefinementState | undefined,
  selection: RefinementSelection,
): RefinementState {
  const next: RefinementState = {
    ...(state ?? emptyRefinementState()),
    turns: [...(state?.turns ?? [])],
  };
  if (selection.kind === "candidate" && typeof selection.value === "string") {
    next.selectedCandidateId = selection.value;
  } else if (
    selection.kind === "audience" &&
    typeof selection.value === "string"
  ) {
    next.audience = selection.value;
  } else if (selection.kind === "themes") {
    next.themes = stringList(selection.value);
  } else if (selection.kind === "topicsToAvoid") {
    next.topicsToAvoid = stringList(selection.value);
  } else if (selection.kind === "sourceGroups") {
    next.sourceGroups = stringList(selection.value);
  } else if (
    selection.kind === "dateRange" &&
    typeof selection.value === "object" &&
    !Array.isArray(selection.value)
  ) {
    next.dateRange = selection.value;
  } else if (selection.kind === "confirmation") {
    next.confirmed = true;
  }
  next.turns.push({ ...selection, summary: selectionSummary(selection.value) });
  return next;
}

export function buildFinalConfirmation(
  loc: Locale,
  input: FinalConfirmationInput,
): string {
  return translate("thread.propose.finalConfirmation", loc, {
    plan: input.planName,
    audience: input.audience,
    themes: input.themes.join(", "),
    sourceGroups: input.sourceGroups.join(", "),
    startDate: input.dateRange.startDate,
    endDate: input.dateRange.endDate,
  });
}

export function createdActionIds(
  status: PlanConfigurationStatus,
): CreatedActionId[] {
  const primary =
    status === PlanConfigurationStatus.RUNNABLE ? "runNow" : "finishSetup";
  return [primary, "schedule", "reviewPlan", "adjustConfiguration"];
}

export function createdActionI18nKey(id: CreatedActionId): string {
  switch (id) {
    case "runNow":
      return "thread.created.runNow";
    case "finishSetup":
      return "thread.created.finishSetup";
    case "schedule":
      return "thread.created.schedule";
    case "reviewPlan":
      return "thread.created.reviewPlan";
    case "adjustConfiguration":
      return "thread.created.adjustConfiguration";
    case "anythingElse":
      return "thread.created.anythingElse";
  }
}
