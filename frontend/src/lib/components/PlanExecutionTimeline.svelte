<script lang="ts">
  import { Clock3 } from "lucide-svelte";
  import StepExecutionStatusBadge from "$lib/components/StepExecutionStatusBadge.svelte";
  import { locale, translate } from "$lib/i18n";
  import { formatLocaleDateTime } from "$lib/i18n/format";
  import {
    formatStepDuration,
    isPlanExecutionTerminal,
  } from "$lib/plans/plan-execution";
  import {
    PlanExecutionStatus,
    StepExecutionStatus,
    type StepExecution,
  } from "$lib/rpc";

  let {
    steps,
    executionStatus,
  }: {
    steps: StepExecution[];
    executionStatus: PlanExecutionStatus;
  } = $props();

  function formatDuration(step: StepExecution): string {
    const durationMs = formatStepDuration(step);
    if (durationMs === null) {
      return translate("common.emDash", $locale);
    }

    const totalSeconds = Math.floor(durationMs / 1000);
    const seconds = totalSeconds % 60;
    const totalMinutes = Math.floor(totalSeconds / 60);
    const minutes = totalMinutes % 60;
    const hours = Math.floor(totalMinutes / 60);

    if (hours > 0) {
      return `${hours}h ${minutes}m`;
    }
    if (minutes > 0) {
      return `${minutes}m ${seconds}s`;
    }
    return `${seconds}s`;
  }
</script>

{#if steps.length === 0}
  <div
    class="rounded-lg border border-dashed border-plumage bg-obsidian-light/20 px-5 py-8 text-center"
  >
    <p class="font-body text-sm text-crown-ash">
      {translate("executions.timeline.empty", $locale)}
    </p>
  </div>
{:else}
  <ol class="space-y-3">
    {#each steps as step (step.id || step.planStepKey)}
      <li
        class="rounded-lg border px-4 py-3 {step.status ===
        StepExecutionStatus.FAILED
          ? 'border-red-500/30 bg-red-500/10'
          : 'border-plumage bg-obsidian-light/20'}"
      >
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p class="font-heading text-sm font-semibold text-cream">
              {step.planStepKey}
            </p>
            <p class="mt-1 font-mono text-[10px] text-crown-ash-dark">
              {translate("executions.timeline.updatedAt", $locale)}:
              {formatLocaleDateTime(step.updatedAt, $locale)}
            </p>
          </div>
          <StepExecutionStatusBadge
            status={step.status}
            pulsing={!isPlanExecutionTerminal(executionStatus)}
          />
        </div>
        <div
          class="mt-2 inline-flex items-center gap-1.5 font-mono text-[10px] tracking-wider text-crown-ash uppercase"
        >
          <Clock3 class="size-3" />
          <span>{translate("executions.timeline.duration", $locale)}</span>
          <span>{formatDuration(step)}</span>
        </div>
      </li>
    {/each}
  </ol>
{/if}
