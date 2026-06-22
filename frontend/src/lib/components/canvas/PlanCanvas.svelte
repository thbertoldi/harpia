<script lang="ts">
  import { locale } from "$lib/i18n";
  import type {
    PlanStep,
    PlanStepDependency,
  } from "$lib/gen/harpia/plans/v1/plans_pb";
  import {
    orderPlanStepsLinear,
    buildLinearDagEdges,
  } from "$lib/plans/artifact-flow";
  import type { CanvasStepState } from "$lib/plans/canvas-state";
  import PlanCanvasNode from "./PlanCanvasNode.svelte";
  import PlanCanvasEdge from "./PlanCanvasEdge.svelte";
  import PlanCanvasDetailPane from "./PlanCanvasDetailPane.svelte";

  interface Props {
    steps: PlanStep[];
    edges: PlanStepDependency[];
    canvasState: Record<string, CanvasStepState>;
    tenantId: string;
    approvalInputArtifactByStep?: Record<string, string>;
  }
  let {
    steps,
    edges,
    canvasState,
    tenantId,
    approvalInputArtifactByStep,
  }: Props = $props();

  const orderedSteps = $derived(orderPlanStepsLinear(steps, edges));
  const dagEdges = $derived(buildLinearDagEdges(steps, edges, $locale));

  let selectedKey = $state<string | null>(null);

  const selectedStep = $derived(
    selectedKey
      ? (orderedSteps.find((s) => s.key === selectedKey) ?? null)
      : null,
  );
  const selectedState = $derived(
    selectedKey ? (canvasState[selectedKey] ?? null) : null,
  );
  const selectedApprovalArtifact = $derived(
    selectedKey ? approvalInputArtifactByStep?.[selectedKey] : undefined,
  );

  function edgeLabelBetween(fromKey: string, toKey: string): string {
    const edge = dagEdges.find(
      (e) => e.from.key === fromKey && e.to.key === toKey,
    );
    return edge?.artifactLabel ?? "—";
  }
</script>

<div class="flex h-full">
  <div
    class="harpia-canvas-grid flex flex-1 items-center justify-start overflow-auto px-6 py-6"
  >
    <div class="flex items-center gap-2">
      {#each orderedSteps as step, index (step.key)}
        <PlanCanvasNode
          {step}
          state={canvasState[step.key] ?? { status: "pending" }}
          selected={selectedKey === step.key}
          onSelect={() => (selectedKey = step.key)}
        />
        {#if index < orderedSteps.length - 1}
          <PlanCanvasEdge
            label={edgeLabelBetween(step.key, orderedSteps[index + 1].key)}
          />
        {/if}
      {/each}
    </div>
  </div>

  <PlanCanvasDetailPane
    step={selectedStep}
    stepState={selectedState}
    {tenantId}
    approvalInputArtifactId={selectedApprovalArtifact}
  />
</div>

<style>
  .harpia-canvas-grid {
    background-color: var(--color-obsidian);
    background-image: radial-gradient(
      rgba(255, 255, 255, 0.04) 1px,
      transparent 1px
    );
    background-size: 24px 24px;
  }
</style>
