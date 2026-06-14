import type {
  PlanStep,
  PlanStepDependency,
} from "$lib/gen/harpia/plans/v1/plans_pb";

const ARTIFACT_TYPE_LABELS: Record<string, string> = {
  DateRange: "Date Range",
  NewsList: "News List",
  TextDraft: "Text Draft",
  LinkedInPostDraft: "LinkedIn Post Draft",
  PublishConfirmation: "Publish Confirmation",
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
export function formatArtifactTypeLabel(artifactTypeId: string): string {
  const key = extractArtifactTypeKey(artifactTypeId);
  if (!key) {
    return "—";
  }

  if (ARTIFACT_TYPE_LABELS[key]) {
    return ARTIFACT_TYPE_LABELS[key];
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
  let current = roots[0];

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
        artifactLabel: formatArtifactTypeLabel(artifactTypeId),
      };
    })
    .filter((edge): edge is PlanDagEdge => edge !== null);
}
