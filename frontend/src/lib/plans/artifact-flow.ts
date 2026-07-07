import type {
  PlanStep,
  PlanStepDependency,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import { translate } from "$lib/i18n";
import type { Locale } from "$lib/i18n";

const ARTIFACT_TYPE_KEYS: Record<string, string> = {
  DateRange: "artifact.type.dateRange",
  NewsList: "artifact.type.newsList",
  TextDraft: "artifact.type.textDraft",
  LinkedInPostDraft: "artifact.type.linkedInPostDraft",
  PublishConfirmation: "artifact.type.publishConfirmation",
};

/** Extracts the short artifact type key from a fully-qualified proto id. */
export function extractArtifactTypeKey(artifactTypeId: string): string {
  if (!artifactTypeId) {
    return "";
  }

  const segments = artifactTypeId.split(".");
  return segments[segments.length - 1] ?? artifactTypeId;
}

/** Formats an artifact type id into a human-readable label. */
export function formatArtifactTypeLabel(
  artifactTypeId: string,
  locale: Locale,
): string {
  const key = extractArtifactTypeKey(artifactTypeId);
  if (!key) {
    return translate("common.emDash", locale);
  }

  if (ARTIFACT_TYPE_KEYS[key]) {
    return translate(ARTIFACT_TYPE_KEYS[key], locale);
  }

  return key.replace(/([A-Z])/g, " $1").trim();
}

export interface PlanDagEdge {
  from: PlanStep;
  to: PlanStep;
  artifactTypeId: string;
  artifactLabel: string;
}

function stepByKey(steps: PlanStep[]): Map<string, PlanStep> {
  return new Map(steps.map((step) => [step.key, step]));
}

/** Orders plan steps into a linear chain using dependency edges. */
export function orderPlanStepsLinear(
  steps: PlanStep[],
  edges: PlanStepDependency[],
): PlanStep[] {
  if (steps.length === 0) {
    return [];
  }

  const lookup = stepByKey(steps);
  const incoming = new Map<string, number>();
  const outgoing = new Map<string, string>();

  for (const step of steps) {
    incoming.set(step.key, 0);
  }

  for (const edge of edges) {
    incoming.set(edge.toStepKey, (incoming.get(edge.toStepKey) ?? 0) + 1);
    outgoing.set(edge.fromStepKey, edge.toStepKey);
  }

  const roots = steps.filter((step) => (incoming.get(step.key) ?? 0) === 0);
  const ordered: PlanStep[] = [];
  let current: PlanStep | undefined = roots[0];

  while (current) {
    ordered.push(current);
    const nextKey = outgoing.get(current.key);
    current = nextKey ? lookup.get(nextKey) : undefined;
  }

  if (ordered.length !== steps.length) {
    return steps;
  }

  return ordered;
}

/** Builds labeled edges for a linear plan DAG diagram. */
export function buildLinearDagEdges(
  steps: PlanStep[],
  edges: PlanStepDependency[],
  locale: Locale,
): PlanDagEdge[] {
  const lookup = stepByKey(steps);

  return edges
    .map((edge) => {
      const from = lookup.get(edge.fromStepKey);
      const to = lookup.get(edge.toStepKey);
      if (!from || !to) {
        return null;
      }

      const artifactTypeId =
        from.outputArtifactTypeId || to.inputArtifactTypeId;
      return {
        from,
        to,
        artifactTypeId,
        artifactLabel: formatArtifactTypeLabel(artifactTypeId, locale),
      };
    })
    .filter((edge): edge is PlanDagEdge => edge !== null);
}

/**
 * Returns the set of step keys that are terminal in the plan DAG — i.e. their
 * output is not consumed by any downstream step. Terminal steps produce the
 * plan's "final" artifacts; everything else is an intermediate.
 */
export function terminalStepKeys(
  steps: PlanStep[],
  edges: PlanStepDependency[],
): Set<string> {
  const downstream = new Set(edges.map((edge) => edge.fromStepKey));
  return new Set(
    steps.filter((step) => !downstream.has(step.key)).map((step) => step.key),
  );
}

/**
 * The fully-qualified artifact type keys produced by terminal steps. An
 * `Artifact` whose `artifactTypeKey` is in this set is a final output; all
 * others are intermediate. Returns the empty set when the DAG has no steps.
 */
export function finalArtifactTypeKeys(
  steps: PlanStep[],
  edges: PlanStepDependency[],
): Set<string> {
  const lookup = stepByKey(steps);
  const keys = new Set<string>();
  for (const key of terminalStepKeys(steps, edges)) {
    const step = lookup.get(key);
    if (step?.outputArtifactTypeId) {
      keys.add(step.outputArtifactTypeId);
    }
  }
  return keys;
}
