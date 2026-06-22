<script lang="ts">
  import { ChevronRight } from "lucide-svelte";
  import type {
    PlanStep,
    PlanStepDependency,
  } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { orderPlanStepsLinear } from "$lib/plans/artifact-flow";

  let {
    steps,
    edges,
  }: {
    steps: PlanStep[];
    edges: PlanStepDependency[];
  } = $props();

  const orderedSteps = $derived(orderPlanStepsLinear(steps, edges));
</script>

<div
  class="flex flex-wrap items-center gap-1.5 rounded-md border border-plumage bg-obsidian-light px-3 py-2"
>
  {#each orderedSteps as step, index (step.key)}
    <span
      class="rounded border border-plumage/60 bg-obsidian px-2 py-1 font-heading text-[11px] font-semibold text-cream"
    >
      {step.title}
    </span>
    {#if index < orderedSteps.length - 1}
      <ChevronRight class="size-3 text-crown-ash-dark" />
    {/if}
  {/each}
</div>
