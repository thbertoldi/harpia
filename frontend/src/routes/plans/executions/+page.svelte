<script lang="ts">
  import { resolve } from "$app/paths";
  import { AlertTriangle, Loader2, Radar } from "lucide-svelte";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import { locale, translate } from "$lib/i18n";
  import {
    loadPlanExecutions,
    statusKeyForPlanExecution,
  } from "$lib/plans/plan-execution";
  import { PlanExecutionStatus, type PlanExecution } from "$lib/rpc";

  let executions = $state<PlanExecution[]>([]);
  let loading = $state(true);
  let loadError = $state<string | null>(null);
  let source = $state<"api" | "mock">("mock");

  $effect(() => {
    void fetchExecutions();
  });

  async function fetchExecutions() {
    loading = true;
    loadError = null;

    try {
      const result = await loadPlanExecutions();
      executions = result.executions;
      source = result.source;
      loadError = result.error ?? null;
    } catch (error) {
      loadError =
        error instanceof Error
          ? error.message
          : translate("executions.list.loadError", $locale);
    } finally {
      loading = false;
    }
  }

  function statusClass(status: PlanExecutionStatus): string {
    if (status === PlanExecutionStatus.RUNNING) {
      return "border-blue-500/40 bg-blue-500/10 text-blue-300";
    }
    if (status === PlanExecutionStatus.COMPLETED) {
      return "border-green-500/40 bg-green-500/10 text-green-300";
    }
    if (status === PlanExecutionStatus.FAILED) {
      return "border-red-500/40 bg-red-500/10 text-red-300";
    }
    if (status === PlanExecutionStatus.CANCELLED) {
      return "border-crown-ash/40 bg-crown-ash/10 text-crown-ash";
    }
    return "border-talon-gold/40 bg-talon-gold/10 text-talon-gold";
  }
</script>

<div class="px-4 py-6 lg:px-6">
  <div class="mb-6 flex items-center justify-between gap-4">
    <div>
      <HarpyHeading tag="h1" class="text-2xl text-cream">
        {translate("executions.list.heading", $locale)}
      </HarpyHeading>
      <p class="mt-1 font-body text-sm text-crown-ash">
        {translate("executions.list.subheading", $locale)}
      </p>
    </div>
    <span
      class="rounded-full border border-plumage px-2.5 py-1 font-mono text-[10px] tracking-wider text-crown-ash uppercase"
    >
      {source === "api"
        ? translate("plans.source.live", $locale)
        : translate("plans.source.mock", $locale)}
    </span>
  </div>

  {#if loading}
    <div class="flex min-h-[40vh] items-center justify-center">
      <div class="flex items-center gap-2 text-crown-ash">
        <Loader2 class="size-5 animate-spin" />
        <span class="font-body text-sm"
          >{translate("executions.list.loading", $locale)}</span
        >
      </div>
    </div>
  {:else if executions.length === 0}
    <div
      class="flex min-h-[40vh] flex-col items-center justify-center rounded-lg border border-dashed border-plumage bg-obsidian-light/30 px-6 py-12 text-center"
    >
      <Radar class="mb-4 size-12 text-talon-gold" />
      <HarpyHeading tag="h2" class="mb-2 text-xl text-cream">
        {translate("executions.list.empty.title", $locale)}
      </HarpyHeading>
      <p class="max-w-md font-body text-sm text-crown-ash">
        {translate("executions.list.empty.description", $locale)}
      </p>
    </div>
  {:else}
    {#if loadError}
      <div
        class="mb-3 rounded-md border border-talon-gold/30 bg-talon-gold/5 px-3 py-2"
      >
        <p class="font-mono text-xs text-talon-gold">
          {translate("executions.list.apiFallback", $locale)}
          {loadError}
        </p>
      </div>
    {/if}

    <div class="space-y-3">
      {#each executions as execution (execution.id)}
        <a
          href={resolve(`/plans/executions/${execution.id}`)}
          class="flex items-center justify-between rounded-lg border border-plumage bg-obsidian-light/20 px-4 py-3 transition-colors hover:border-talon-gold/60"
        >
          <div>
            <p class="font-heading text-base text-cream">{execution.id}</p>
            <p class="mt-1 font-mono text-[10px] text-crown-ash-dark">
              {translate("executions.list.updatedAt", $locale)}: {execution.updatedAt}
            </p>
          </div>
          <span
            class="inline-flex items-center rounded-full border px-2.5 py-0.5 font-mono text-[10px] tracking-wider uppercase {statusClass(
              execution.status,
            )}"
          >
            {translate(statusKeyForPlanExecution(execution.status), $locale)}
          </span>
        </a>
      {/each}
    </div>
  {/if}

  {#if !loading && loadError && executions.length === 0}
    <div class="mt-4 text-center">
      <AlertTriangle class="mx-auto mb-2 size-6 text-red-400" />
      <button
        type="button"
        onclick={() => fetchExecutions()}
        class="rounded-md border border-plumage px-4 py-2 font-body text-sm text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
      >
        {translate("common.retry", $locale)}
      </button>
    </div>
  {/if}
</div>
