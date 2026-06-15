<script lang="ts">
  import { ArrowDown, ArrowRight } from "lucide-svelte";
  import type {
    PlanStep,
    PlanStepDependency,
  } from "$lib/gen/harpia/plans/v1/plans_pb";
  import {
    buildLinearDagEdges,
    formatArtifactTypeLabel,
    orderPlanStepsLinear,
  } from "$lib/plans/artifact-flow";
  import { locale, translate } from "$lib/i18n";

  let {
    steps,
    edges,
  }: {
    steps: PlanStep[];
    edges: PlanStepDependency[];
  } = $props();

  const orderedSteps = $derived(orderPlanStepsLinear(steps, edges));
  const dagEdges = $derived(buildLinearDagEdges(steps, edges, $locale));

  function edgeLabelBetween(fromKey: string, toKey: string): string {
    const edge = dagEdges.find(
      (candidate) =>
        candidate.from.key === fromKey && candidate.to.key === toKey,
    );
    return edge?.artifactLabel ?? translate("common.emDash", $locale);
  }
</script>

<div class="rounded-lg border border-plumage bg-obsidian-light/40 p-4 lg:p-6">
  <div class="flex flex-col gap-4 lg:hidden">
    {#each orderedSteps as step, index (step.key)}
      <div class="rounded-md border border-plumage bg-obsidian px-4 py-3">
        <p class="font-heading text-sm font-semibold text-cream">
          {step.title}
        </p>
        <p class="mt-1 font-mono text-[10px] text-crown-ash-dark">{step.key}</p>
        <div class="mt-3 flex flex-wrap gap-2 font-mono text-[10px]">
          <span
            class="rounded border border-plumage/60 px-2 py-0.5 text-crown-ash"
          >
            {formatArtifactTypeLabel(step.inputArtifactTypeId, $locale)}
          </span>
          <span class="text-crown-ash-dark">→</span>
          <span
            class="rounded border border-talon-gold/40 bg-talon-gold/5 px-2 py-0.5 text-talon-gold"
          >
            {formatArtifactTypeLabel(step.outputArtifactTypeId, $locale)}
          </span>
        </div>
      </div>

      {#if index < orderedSteps.length - 1}
        <div class="flex flex-col items-center gap-1 px-2">
          <ArrowDown class="size-4 text-talon-gold" aria-hidden="true" />
          <span
            class="rounded-full border border-plumage px-2 py-0.5 font-mono text-[10px] text-crown-ash"
          >
            {edgeLabelBetween(step.key, orderedSteps[index + 1].key)}
          </span>
        </div>
      {/if}
    {/each}
  </div>

  <div class="hidden items-stretch gap-2 lg:flex">
    {#each orderedSteps as step, index (step.key)}
      <div class="flex min-w-0 flex-1 items-center gap-2">
        <div
          class="flex min-w-[10rem] flex-1 flex-col rounded-md border border-plumage bg-obsidian px-4 py-3"
        >
          <p class="font-heading text-sm font-semibold text-cream">
            {step.title}
          </p>
          <p class="mt-1 font-mono text-[10px] text-crown-ash-dark">
            {step.key}
          </p>
          <div class="mt-3 flex flex-wrap gap-2 font-mono text-[10px]">
            <span
              class="rounded border border-plumage/60 px-2 py-0.5 text-crown-ash"
            >
              {formatArtifactTypeLabel(step.inputArtifactTypeId, $locale)}
            </span>
            <span class="text-crown-ash-dark">→</span>
            <span
              class="rounded border border-talon-gold/40 bg-talon-gold/5 px-2 py-0.5 text-talon-gold"
            >
              {formatArtifactTypeLabel(step.outputArtifactTypeId, $locale)}
            </span>
          </div>
        </div>

        {#if index < orderedSteps.length - 1}
          <div
            class="flex w-28 shrink-0 flex-col items-center justify-center gap-1"
          >
            <ArrowRight class="size-4 text-talon-gold" aria-hidden="true" />
            <span
              class="rounded-full border border-plumage px-2 py-0.5 text-center font-mono text-[10px] leading-tight text-crown-ash"
            >
              {edgeLabelBetween(step.key, orderedSteps[index + 1].key)}
            </span>
          </div>
        {/if}
      </div>
    {/each}
  </div>
</div>
