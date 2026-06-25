<script lang="ts">
  import { tick } from "svelte";
  import { locale, translate } from "$lib/i18n";
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
  import NodeHoverCard from "./NodeHoverCard.svelte";
  import Skeleton from "$lib/components/Skeleton.svelte";

  interface Props {
    steps: PlanStep[];
    edges: PlanStepDependency[];
    canvasState: Record<string, CanvasStepState>;
    tenantId: string;
    approvalInputArtifactByStep?: Record<string, string>;
    /** When true, render content-shaped skeletons instead of the graph. */
    loading?: boolean;
  }
  let {
    steps,
    edges,
    canvasState,
    tenantId,
    approvalInputArtifactByStep,
    loading = false,
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

  // --- edge geometry: measure node boxes relative to the content layer -----
  let contentEl = $state<HTMLDivElement | null>(null);
  let nodeEls = $state<Record<string, HTMLElement>>({});

  interface EdgeGeom {
    key: string;
    label: string;
    anchor: {
      sourceRight: number;
      sourceMidY: number;
      targetLeft: number;
      targetMidY: number;
    };
  }
  let edgeGeoms = $state<EdgeGeom[]>([]);
  let contentSize = $state({ width: 0, height: 0 });

  function recomputeGeometry() {
    if (!contentEl) return;
    const base = contentEl.getBoundingClientRect();
    contentSize = { width: base.width, height: base.height };
    const geoms: EdgeGeom[] = [];
    for (let i = 0; i < orderedSteps.length - 1; i++) {
      const from = orderedSteps[i];
      const to = orderedSteps[i + 1];
      const fromEl = nodeEls[from.key];
      const toEl = nodeEls[to.key];
      if (!fromEl || !toEl) continue;
      const a = fromEl.getBoundingClientRect();
      const b = toEl.getBoundingClientRect();
      geoms.push({
        key: `${from.key}->${to.key}`,
        label: edgeLabelBetween(from.key, to.key),
        anchor: {
          sourceRight: a.right - base.left,
          sourceMidY: a.top - base.top + a.height / 2,
          targetLeft: b.left - base.left,
          targetMidY: b.top - base.top + b.height / 2,
        },
      });
    }
    edgeGeoms = geoms;
  }

  // Recompute whenever the ordered steps change or after layout settles.
  $effect(() => {
    // touch reactive deps so the effect re-runs on data/locale changes
    void orderedSteps;
    void $locale;
    void tick().then(recomputeGeometry);
  });

  $effect(() => {
    if (!contentEl) return;
    const ro = new ResizeObserver(() => recomputeGeometry());
    ro.observe(contentEl);
    return () => ro.disconnect();
  });

  // --- hover card ---------------------------------------------------------
  let hoverKey = $state<string | null>(null);
  let hoverRect = $state<DOMRect | null>(null);

  const hoverStep = $derived(
    hoverKey ? (orderedSteps.find((s) => s.key === hoverKey) ?? null) : null,
  );
  const hoverState = $derived(
    hoverKey ? (canvasState[hoverKey] ?? { status: "pending" }) : null,
  );
</script>

<div class="flex h-full">
  <div
    class="harpia-canvas-vignette relative flex flex-1 items-center justify-start overflow-auto px-6 py-6"
  >
    {#if loading}
      <div class="flex items-center gap-6">
        {#each [0, 1, 2] as i (i)}
          <Skeleton shape="rect" width="11rem" height="4.5rem" />
        {/each}
      </div>
    {:else if orderedSteps.length === 0}
      <div class="flex w-full items-center justify-center">
        <div
          class="harpia-empty-node flex w-48 flex-col items-center gap-1 rounded-lg px-4 py-6 text-center"
        >
          <span class="text-[11px] text-text-muted">
            {translate("canvas.empty", $locale)}
          </span>
        </div>
      </div>
    {:else}
      <div bind:this={contentEl} class="relative">
        <!-- edge overlay sits behind the nodes -->
        <svg
          class="pointer-events-none absolute inset-0"
          width={contentSize.width}
          height={contentSize.height}
          aria-hidden="true"
        >
          {#each edgeGeoms as edge (edge.key)}
            <PlanCanvasEdge label={edge.label} anchor={edge.anchor} />
          {/each}
        </svg>

        <div class="relative flex items-center gap-16">
          {#each orderedSteps as step (step.key)}
            <div bind:this={nodeEls[step.key]}>
              <PlanCanvasNode
                {step}
                nodeState={canvasState[step.key] ?? { status: "pending" }}
                selected={selectedKey === step.key}
                onSelect={() => (selectedKey = step.key)}
                onHoverShow={(rect) => {
                  hoverKey = step.key;
                  hoverRect = rect;
                }}
                onHoverHide={() => {
                  if (hoverKey === step.key) {
                    hoverKey = null;
                    hoverRect = null;
                  }
                }}
              />
            </div>
          {/each}
        </div>
      </div>
    {/if}
  </div>

  <PlanCanvasDetailPane
    step={selectedStep}
    stepState={selectedState}
    {tenantId}
    approvalInputArtifactId={selectedApprovalArtifact}
  />
</div>

{#if hoverStep && hoverState && hoverRect}
  <NodeHoverCard
    node={hoverStep}
    state={hoverState}
    anchorRect={hoverRect}
    overseerLabel={null}
  />
{/if}

<style>
  .harpia-canvas-vignette {
    /* Subtle vignette per spec §2.3: centre lighter than the corners.
       Both stops are themed tokens so it works in light and dark. */
    background-image: radial-gradient(
      circle at center,
      var(--token-surface) 0%,
      var(--token-surface-deep) 100%
    );
  }
  .harpia-empty-node {
    background-color: var(--token-surface-elevated);
    border: 1px dashed var(--token-text-muted-dark);
  }
</style>
