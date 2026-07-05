<script lang="ts">
  import { ChevronRight } from "lucide-svelte";
  import type {
    PlanStep,
    PlanStepDependency,
    PlanTemplate,
  } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { locale } from "$lib/i18n";
  import { localizedStepTitle } from "$lib/plans/catalog-i18n";
  import { orderPlanStepsLinear } from "$lib/plans/artifact-flow";

  let {
    steps,
    edges,
    template,
  }: {
    steps: PlanStep[];
    edges: PlanStepDependency[];
    /**
     * Optional template used to resolve step titles through the catalog
     * content keys (`catalog.plan.<key>.step.<stepKey>.title`). When absent,
     * each step's own English `title` is rendered as a fallback.
     */
    template?: PlanTemplate;
  } = $props();

  const orderedSteps = $derived(orderPlanStepsLinear(steps, edges));

  function stepLabel(step: PlanStep): string {
    return template
      ? localizedStepTitle(template, step.key, $locale)
      : step.title;
  }
</script>

<div
  class="flex flex-wrap items-center gap-1.5 rounded-md border border-plumage bg-obsidian-light px-3 py-2"
>
  {#each orderedSteps as step, index (step.key)}
    <span
      class="rounded border border-plumage/60 bg-obsidian px-2 py-1 font-heading text-[11px] font-semibold text-cream"
    >
      {stepLabel(step)}
    </span>
    {#if index < orderedSteps.length - 1}
      <ChevronRight class="size-3 text-crown-ash-dark" />
    {/if}
  {/each}
</div>
