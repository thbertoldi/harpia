<script lang="ts">
  import { page } from "$app/state";
  import { onDestroy } from "svelte";
  import { AlertTriangle, Loader2 } from "lucide-svelte";
  import PlanExecutionTimeline from "$lib/components/PlanExecutionTimeline.svelte";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import { locale, translate } from "$lib/i18n";
  import { formatLocaleDateTime } from "$lib/i18n/format";
  import {
    isPlanExecutionTerminal,
    loadPlanExecution,
    mergeStepExecutions,
    startPlanExecutionStream,
    statusKeyForPlanExecution,
  } from "$lib/plans/plan-execution";
  import {
    PlanExecutionStatus,
    type PlanExecution,
    type StepExecution,
  } from "$lib/rpc";

  let execution = $state<PlanExecution | null>(null);
  let steps = $state<StepExecution[]>([]);
  let loading = $state(true);
  let loadError = $state<string | null>(null);
  let streamWarning = $state<string | null>(null);
  let source = $state<"api" | "mock">("mock");
  let streamHandle = $state<{ stop: () => void } | null>(null);

  const executionId = $derived(page.params.executionId ?? "");

  $effect(() => {
    if (!executionId) {
      return;
    }
    void initialize(executionId);
  });

  async function initialize(id: string) {
    streamHandle?.stop();
    loading = true;
    loadError = null;
    streamWarning = null;

    try {
      const result = await loadPlanExecution(id);
      execution = result.execution;
      steps = result.execution.stepExecutions;
      source = result.source;
      loadError = result.error ?? null;

      streamHandle = startPlanExecutionStream(id, {
        onExecution: (next) => {
          execution = next;
        },
        onSteps: (updates) => {
          steps = mergeStepExecutions(steps, updates);
        },
        onWarning: (warning) => {
          streamWarning = warning;
        },
      });
    } catch (error) {
      loadError =
        error instanceof Error
          ? error.message
          : translate("executions.detail.loadError", $locale);
    } finally {
      loading = false;
    }
  }

  function statusClass(): string {
    if (!execution) {
      return "border-crown-ash/40 bg-crown-ash/10 text-crown-ash";
    }
    if (execution.status === PlanExecutionStatus.RUNNING) {
      return "border-blue-500/40 bg-blue-500/10 text-blue-300";
    }
    if (execution.status === PlanExecutionStatus.COMPLETED) {
      return "border-green-500/40 bg-green-500/10 text-green-300";
    }
    if (execution.status === PlanExecutionStatus.FAILED) {
      return "border-red-500/40 bg-red-500/10 text-red-300";
    }
    return "border-talon-gold/40 bg-talon-gold/10 text-talon-gold";
  }

  onDestroy(() => {
    streamHandle?.stop();
  });
</script>

<div class="px-4 py-6 lg:px-6">
  {#if loading}
    <div class="flex min-h-[40vh] items-center justify-center">
      <div class="flex items-center gap-2 text-crown-ash">
        <Loader2 class="size-5 animate-spin" />
        <span class="font-body text-sm"
          >{translate("executions.detail.loading", $locale)}</span
        >
      </div>
    </div>
  {:else if !execution}
    <div class="flex min-h-[40vh] items-center justify-center text-center">
      <div>
        <AlertTriangle class="mx-auto mb-3 size-10 text-red-400" />
        <p class="font-body text-sm text-red-400">
          {translate("executions.detail.loadError", $locale)}
        </p>
        {#if loadError}
          <p class="mt-1 font-mono text-xs text-crown-ash">{loadError}</p>
        {/if}
      </div>
    </div>
  {:else}
    <div class="mb-4 flex flex-wrap items-start justify-between gap-3">
      <div>
        <HarpyHeading tag="h1" class="text-2xl text-cream">
          {translate("executions.detail.heading", $locale)}
        </HarpyHeading>
        <p class="mt-1 font-mono text-xs text-crown-ash-dark">{execution.id}</p>
      </div>
      <div class="flex items-center gap-2">
        <span
          class="inline-flex items-center rounded-full border px-2.5 py-0.5 font-mono text-[10px] tracking-wider uppercase {statusClass()}"
        >
          {translate(statusKeyForPlanExecution(execution.status), $locale)}
        </span>
        <span
          class="rounded-full border border-plumage px-2.5 py-0.5 font-mono text-[10px] tracking-wider text-crown-ash uppercase"
        >
          {source === "api"
            ? translate("plans.source.live", $locale)
            : translate("plans.source.mock", $locale)}
        </span>
      </div>
    </div>

    <div
      class="mb-4 grid gap-2 rounded-lg border border-plumage bg-obsidian-light/20 p-4 lg:grid-cols-2"
    >
      <div>
        <p
          class="font-mono text-[10px] tracking-wider text-crown-ash-dark uppercase"
        >
          {translate("executions.detail.createdAt", $locale)}
        </p>
        <p class="font-body text-sm text-crown-ash">
          {formatLocaleDateTime(execution.createdAt, $locale)}
        </p>
      </div>
      <div>
        <p
          class="font-mono text-[10px] tracking-wider text-crown-ash-dark uppercase"
        >
          {translate("executions.detail.updatedAt", $locale)}
        </p>
        <p class="font-body text-sm text-crown-ash">
          {formatLocaleDateTime(execution.updatedAt, $locale)}
        </p>
      </div>
    </div>

    {#if loadError && source === "mock"}
      <div
        class="mb-3 rounded-md border border-talon-gold/30 bg-talon-gold/5 px-3 py-2"
      >
        <p class="font-mono text-xs text-talon-gold">
          {translate("executions.detail.apiFallback", $locale)}
          {loadError}
        </p>
      </div>
    {/if}

    {#if streamWarning}
      <div
        class="mb-3 rounded-md border border-talon-gold/30 bg-talon-gold/5 px-3 py-2"
      >
        <p class="font-mono text-xs text-talon-gold">{streamWarning}</p>
      </div>
    {/if}

    <PlanExecutionTimeline {steps} executionStatus={execution.status} />

    {#if isPlanExecutionTerminal(execution.status)}
      <p class="mt-3 font-body text-xs text-crown-ash">
        {translate("executions.detail.terminalHint", $locale)}
      </p>
    {/if}
  {/if}
</div>
