<script lang="ts">
  import { resolve } from "$app/paths";
  import { invalidateAll } from "$app/navigation";
  import {
    MessageSquare,
    Pencil,
    CalendarClock,
    Activity,
  } from "lucide-svelte";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import ScheduleDialog from "$lib/components/canvas/ScheduleDialog.svelte";
  import { locale, translate } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";
  import { statusKeyForPlanExecution } from "$lib/plans/plan-execution";
  import {
    canInlineEdit,
    countActiveExecutions,
    type PlanRunGroup,
  } from "$lib/plans/runs-panel";
  import {
    PlanConfigurationStatus,
    type PlanConfiguration,
    type PlanTemplate,
  } from "$lib/gen/harpia/plans/v1/plans_pb";

  type RunsPageData = {
    groups: PlanRunGroup[];
    templateByConfigurationId: Map<string, PlanTemplate>;
  };

  let { data }: { data: RunsPageData } = $props();

  const groups = $derived<PlanRunGroup[]>(data.groups);
  const templateByConfigurationId = $derived(data.templateByConfigurationId);
  let loadError = $state(false);
  let loading = $state(false);

  // Inline schedule editing for recurring plans (runs-panel spec: "Inline Edits
  // for Recurring Plans"). Only the schedule is editable here; sources and tone
  // are structural enough that they redirect to the conversation.
  let scheduleTarget = $state<PlanConfiguration | undefined>();

  function statusKey(status: PlanConfigurationStatus): string {
    switch (status) {
      case PlanConfigurationStatus.DRAFT:
        return "runs.status.draft";
      case PlanConfigurationStatus.RUNNABLE:
        return "runs.status.runnable";
      case PlanConfigurationStatus.SCHEDULED:
        return "runs.status.scheduled";
      case PlanConfigurationStatus.DISABLED:
        return "runs.status.disabled";
      case PlanConfigurationStatus.ARCHIVED:
        return "runs.status.archived";
      default:
        return "runs.status.unspecified";
    }
  }

  // The origin thread a plan belongs to. origin_thread_id is the canonical link
  // (ADR-017); thread_id is a legacy fallback for older records.
  function originThreadId(group: PlanRunGroup): string | null {
    return (
      group.configuration?.originThreadId ||
      group.configuration?.threadId ||
      null
    );
  }

  function planTitle(group: PlanRunGroup): string {
    const configuration = group.configuration;
    if (!configuration) return translate("runs.plan.untitled", $locale);
    const template = templateByConfigurationId.get(configuration.id);
    return template?.name || configuration.id.slice(0, 8);
  }

  function executionStatusClass(status: number): string {
    switch (status) {
      case 1: // PENDING
      case 2: // RUNNING
        return "bg-blue-500/20 text-blue-300";
      case 3: // COMPLETED
        return "bg-green-500/20 text-green-300";
      case 4: // FAILED
        return "bg-red-500/20 text-red-200";
      case 5: // CANCELLED
        return "bg-crown-ash/20 text-crown-ash";
      default:
        return "bg-crown-ash/20 text-crown-ash";
    }
  }

  function scheduleLabel(configuration: PlanConfiguration | undefined): string {
    const cron = configuration?.schedule?.cronExpression?.trim();
    return cron || translate("runs.plan.noSchedule", $locale);
  }

  async function reload() {
    loading = true;
    loadError = false;
    try {
      await invalidateAll();
    } catch {
      loadError = true;
    } finally {
      loading = false;
    }
  }

  function onScheduleSaved() {
    // Refresh the grouped data so the new cadence is reflected; the load
    // function re-runs and the derived groups update reactively.
    scheduleTarget = undefined;
    void reload();
  }
</script>

<svelte:head>
  <title>{translate("runs.heading", $locale)} · Harpia</title>
</svelte:head>

<div class="mx-auto max-w-5xl px-6 py-6">
  <header class="mb-6 flex items-baseline justify-between">
    <div>
      <HarpyHeading tag="h1" class="text-2xl text-text">
        {translate("runs.heading", $locale)}
      </HarpyHeading>
      <p class="mt-1 font-body text-sm text-text-muted">
        {translate("runs.subheading", $locale)}
      </p>
    </div>
    {#if loadError}
      <button
        type="button"
        onclick={reload}
        disabled={loading}
        class="cursor-pointer rounded-md border border-border px-3 py-1.5 text-[12px] text-text-muted transition hover:bg-surface-hover hover:text-text disabled:opacity-50"
      >
        {translate("runs.retry", $locale)}
      </button>
    {/if}
  </header>

  {#if loadError}
    <p
      class="rounded-lg border border-border bg-surface-elevated px-4 py-3 text-sm text-text-muted"
    >
      {translate("runs.loadError", $locale)}
    </p>
  {:else if loading && groups.length === 0}
    <p
      class="rounded-lg border border-border bg-surface-elevated px-4 py-3 text-sm text-text-muted"
    >
      {translate("runs.loading", $locale)}
    </p>
  {:else if groups.length === 0}
    <div
      class="flex flex-col items-center justify-center rounded-lg border border-border bg-surface-elevated px-4 py-12 text-center"
    >
      <Activity class="mb-3 size-8 text-text-muted" />
      <HarpyHeading tag="h2" class="mb-1 text-base text-text">
        {translate("runs.empty.title", $locale)}
      </HarpyHeading>
      <p class="max-w-sm font-body text-sm text-text-muted">
        {translate("runs.empty.description", $locale)}
      </p>
    </div>
  {:else}
    <ul class="flex flex-col gap-3">
      {#each groups as group (group.configuration?.id ?? crypto.randomUUID())}
        {@const configuration = group.configuration}
        {@const inline = canInlineEdit(configuration)}
        {@const active = countActiveExecutions(group)}
        {@const threadId = originThreadId(group)}
        {@const planParam = configuration?.id
          ? `?plan=${encodeURIComponent(configuration.id)}`
          : ""}
        {@const template = configuration
          ? templateByConfigurationId.get(configuration.id)
          : undefined}
        <li
          class="rounded-lg border border-border bg-surface-elevated p-4 shadow-sm"
        >
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                {#if group.orphan}
                  <span
                    class="rounded-full border border-border bg-surface px-2 py-0.5 font-mono text-[10px] tracking-wider text-text-muted-dark uppercase"
                  >
                    {translate("runs.plan.orphan", $locale)}
                  </span>
                {:else if configuration}
                  <span
                    class="rounded-full border border-border bg-surface px-2 py-0.5 font-mono text-[10px] tracking-wider text-text-muted-dark uppercase"
                  >
                    {translate(statusKey(configuration.status), $locale)}
                  </span>
                {/if}
                {#if active > 0}
                  <span
                    class="rounded-full bg-blue-500/20 px-2 py-0.5 font-mono text-[10px] tracking-wider text-blue-300 uppercase"
                  >
                    {translate("runs.plan.running", $locale).replace(
                      "{count}",
                      String(active),
                    )}
                  </span>
                {/if}
              </div>

              <HarpyHeading tag="h2" class="mt-2 truncate text-base text-text">
                {planTitle(group)}
              </HarpyHeading>
              {#if template?.description}
                <p class="mt-0.5 truncate font-body text-xs text-text-muted">
                  {template.description}
                </p>
              {/if}

              {#if group.orphan}
                <p class="mt-2 font-body text-xs text-text-muted-dark">
                  {translate("runs.plan.orphanHint", $locale)}
                </p>
              {:else if configuration}
                <div
                  class="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 font-mono text-[11px] text-text-muted-dark"
                >
                  <span>
                    {translate("runs.plan.cron", $locale)}:
                    {scheduleLabel(configuration)}
                  </span>
                  {#if group.executions.length > 0}
                    <span>
                      {translate(
                        group.executions.length === 1
                          ? "runs.plan.runCount.one"
                          : "runs.plan.runCount",
                        $locale,
                      ).replace("{count}", String(group.executions.length))}
                    </span>
                  {/if}
                </div>
              {/if}
            </div>

            <div class="flex shrink-0 items-center gap-1.5">
              {#if inline && configuration}
                <button
                  type="button"
                  onclick={() => (scheduleTarget = configuration)}
                  class="flex cursor-pointer items-center gap-1 rounded-md border border-border px-2.5 py-1.5 text-[11px] text-text-muted transition hover:bg-surface-hover hover:text-text"
                >
                  <CalendarClock class="size-3.5" />
                  {translate("runs.plan.schedule", $locale)}
                </button>
              {/if}
              {#if threadId}
                <a
                  href={resolve(`/chat/${threadId}${planParam}`)}
                  class="flex cursor-pointer items-center gap-1 rounded-md border border-border px-2.5 py-1.5 text-[11px] text-text-muted transition hover:bg-surface-hover hover:text-text"
                >
                  <MessageSquare class="size-3.5" />
                  {translate("runs.plan.originThread", $locale)}
                </a>
                <a
                  href={resolve(`/chat/${threadId}${planParam}`)}
                  class="flex cursor-pointer items-center gap-1 rounded-md border border-primary px-2.5 py-1.5 text-[11px] font-semibold text-primary transition hover:bg-primary/10"
                >
                  <Pencil class="size-3.5" />
                  {translate("runs.plan.configure", $locale)}
                </a>
              {/if}
            </div>
          </div>

          {#if group.executions.length > 0}
            <ul class="mt-4 flex flex-col divide-y divide-border/50">
              {#each group.executions as execution (execution.id)}
                <li
                  class="flex flex-wrap items-center gap-x-4 gap-y-1.5 py-2.5 text-[12px]"
                >
                  <span
                    class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 font-mono text-[10px] tracking-wider uppercase {executionStatusClass(
                      execution.status,
                    )}"
                  >
                    {translate(
                      statusKeyForPlanExecution(execution.status),
                      $locale,
                    )}
                  </span>
                  <span class="font-mono text-[11px] text-text-muted-dark">
                    {execution.id.slice(0, 8)}
                  </span>
                  {#if execution.triggeredAt}
                    <span class="font-body text-[11px] text-text-muted">
                      {translate("runs.execution.triggered", $locale)}
                      {formatRelativeTime(execution.triggeredAt, $locale)}
                    </span>
                  {/if}
                  <span class="flex-1"></span>
                  {#if threadId}
                    <a
                      href={resolve(`/chat/${threadId}${planParam}`)}
                      class="font-body text-[11px] text-primary transition hover:opacity-80"
                    >
                      {translate("runs.execution.openThread", $locale)}
                    </a>
                  {/if}
                </li>
              {/each}
            </ul>
          {:else if !group.orphan}
            <p class="mt-3 font-body text-xs text-text-muted-dark">
              {translate("runs.plan.noExecutions", $locale)}
            </p>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</div>

{#if scheduleTarget}
  <ScheduleDialog
    open={true}
    configuration={scheduleTarget}
    onClose={() => (scheduleTarget = undefined)}
    onSaved={onScheduleSaved}
  />
{/if}
