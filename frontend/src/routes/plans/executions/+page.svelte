<script lang="ts">
  import { resolve } from "$app/paths";
  import { AlertTriangle, Loader2, Radar, RefreshCw } from "lucide-svelte";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import { locale, translate } from "$lib/i18n";
  import { formatLocaleDateTime } from "$lib/i18n/format";
  import {
    filterPlanExecutionsByView,
    loadPlanExecutions,
    statusKeyForPlanExecution,
    type PlanExecutionViewFilter,
  } from "$lib/plans/plan-execution";
  import { PlanExecutionStatus, type PlanExecution } from "$lib/rpc";

  let executions = $state<PlanExecution[]>([]);
  let loading = $state(true);
  let loadError = $state<string | null>(null);
  let source = $state<"api" | "mock">("api");
  let selectedView = $state<PlanExecutionViewFilter>("all");

  const filterViews: Array<{
    key: PlanExecutionViewFilter;
    labelKey: string;
  }> = [
    { key: "all", labelKey: "executions.list.filter.all" },
    { key: "running", labelKey: "executions.list.filter.running" },
    { key: "failed", labelKey: "executions.list.filter.failed" },
    { key: "history", labelKey: "executions.list.filter.history" },
  ];

  const visibleExecutions = $derived(
    filterPlanExecutionsByView(executions, selectedView),
  );

  const filterCounts = $derived({
    all: executions.length,
    running: filterPlanExecutionsByView(executions, "running").length,
    failed: filterPlanExecutionsByView(executions, "failed").length,
    history: filterPlanExecutionsByView(executions, "history").length,
  });

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

  function filterButtonClass(view: PlanExecutionViewFilter): string {
    return selectedView === view
      ? "border-talon-gold bg-talon-gold/10 text-talon-gold"
      : "border-plumage bg-obsidian-light/20 text-crown-ash hover:border-talon-gold/60 hover:text-cream";
  }

  function formatTimestamp(value: string): string {
    if (!value) return translate("common.emDash", $locale);
    return formatLocaleDateTime(value, $locale);
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
    {#if source === "api"}
      <div class="flex items-center gap-2">
        <button
          type="button"
          onclick={() => fetchExecutions()}
          class="inline-flex size-9 items-center justify-center rounded-md border border-plumage text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
          aria-label={translate("common.retry", $locale)}
          title={translate("common.retry", $locale)}
        >
          <RefreshCw class="size-4" />
        </button>
        <span
          class="rounded-full border border-plumage px-2.5 py-1 font-mono text-[10px] tracking-wider text-crown-ash uppercase"
        >
          {translate("plans.source.live", $locale)}
        </span>
      </div>
    {/if}
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
  {:else if loadError && executions.length === 0}
    <div
      class="flex min-h-[40vh] flex-col items-center justify-center rounded-lg border border-dashed border-plumage bg-obsidian-light/30 px-6 py-12 text-center"
    >
      <AlertTriangle class="mb-4 size-12 text-red-400" />
      <HarpyHeading tag="h2" class="mb-2 text-xl text-cream">
        {translate("executions.list.loadError", $locale)}
      </HarpyHeading>
      <p class="max-w-md font-mono text-xs text-crown-ash">{loadError}</p>
      <button
        type="button"
        onclick={() => fetchExecutions()}
        class="mt-4 rounded-md border border-plumage px-4 py-2 font-body text-sm text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
      >
        {translate("common.retry", $locale)}
      </button>
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
    {#if loadError && source === "mock"}
      <div
        class="mb-3 rounded-md border border-talon-gold/30 bg-talon-gold/5 px-3 py-2"
      >
        <p class="font-mono text-xs text-talon-gold">
          {translate("executions.list.apiFallback", $locale)}
          {loadError}
        </p>
      </div>
    {/if}

    <div class="mb-4 flex flex-wrap gap-2">
      {#each filterViews as view (view.key)}
        <button
          type="button"
          onclick={() => (selectedView = view.key)}
          class="rounded-md border px-3 py-1.5 font-mono text-[10px] tracking-wider uppercase transition-colors {filterButtonClass(
            view.key,
          )}"
        >
          {translate(view.labelKey, $locale)}
          <span class="ml-1 text-crown-ash-dark">
            {filterCounts[view.key]}
          </span>
        </button>
      {/each}
    </div>

    <div class="space-y-3">
      {#each visibleExecutions as execution (execution.id)}
        <a
          href={resolve(`/plans/executions/${execution.id}`)}
          class="grid gap-3 rounded-lg border border-plumage bg-obsidian-light/20 px-4 py-3 transition-colors hover:border-talon-gold/60 md:grid-cols-[minmax(0,1fr)_auto]"
        >
          <div class="min-w-0">
            <p class="truncate font-heading text-base text-cream">
              {execution.id}
            </p>
            <p class="mt-1 truncate font-mono text-[10px] text-crown-ash-dark">
              {translate("executions.list.configuration", $locale)}:
              {execution.planConfigurationId}
            </p>
            <dl class="mt-3 grid gap-2 sm:grid-cols-3">
              <div>
                <dt class="font-mono text-[10px] text-crown-ash-dark uppercase">
                  {translate("executions.list.triggeredAt", $locale)}
                </dt>
                <dd class="mt-1 text-[12px] text-crown-ash">
                  {formatTimestamp(execution.triggeredAt)}
                </dd>
              </div>
              <div>
                <dt class="font-mono text-[10px] text-crown-ash-dark uppercase">
                  {translate("executions.list.updatedAt", $locale)}
                </dt>
                <dd class="mt-1 text-[12px] text-crown-ash">
                  {formatTimestamp(execution.updatedAt)}
                </dd>
              </div>
              <div>
                <dt class="font-mono text-[10px] text-crown-ash-dark uppercase">
                  {translate("executions.list.completedAt", $locale)}
                </dt>
                <dd class="mt-1 text-[12px] text-crown-ash">
                  {formatTimestamp(execution.completedAt)}
                </dd>
              </div>
            </dl>
          </div>
          <span
            class="inline-flex h-6 items-center justify-self-start rounded-full border px-2.5 py-0.5 font-mono text-[10px] tracking-wider uppercase md:justify-self-end {statusClass(
              execution.status,
            )}"
          >
            {translate(statusKeyForPlanExecution(execution.status), $locale)}
          </span>
        </a>
      {/each}
    </div>
  {/if}
</div>
