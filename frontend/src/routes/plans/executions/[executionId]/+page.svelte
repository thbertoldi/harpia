<script lang="ts">
  import {
    AlertTriangle,
    ChevronDown,
    Loader2,
    RefreshCw,
  } from "lucide-svelte";
  import { page } from "$app/state";
  import { resolve } from "$app/paths";
  import { requireTenantId } from "$lib/auth";
  import { toUserMessage } from "$lib/connect-errors";
  import ArtifactPreview from "$lib/components/ArtifactPreview.svelte";
  import {
    loadPlanExecutionDetail,
    type StepExecutionDisplayRow,
  } from "$lib/plans/plan-execution-detail";
  import {
    PlanExecutionStatus,
    StepExecutionStatus,
    type PlanExecution,
  } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { locale, translate } from "$lib/i18n";

  let execution = $state<PlanExecution | null>(null);
  let rows = $state<StepExecutionDisplayRow[]>([]);
  let loading = $state(true);
  let loadError = $state<string | null>(null);
  let source = $state<"api" | "mock">("api");
  let expandedStepId = $state<string | null>(null);
  let selectedArtifactId = $state<string>("");

  $effect(() => {
    void fetchExecutionDetail();
  });

  async function fetchExecutionDetail() {
    const executionId = page.params.executionId;
    if (!executionId) {
      execution = null;
      rows = [];
      loadError = translate("plans.execution.invalidId", $locale);
      loading = false;
      return;
    }

    loading = true;
    loadError = null;
    expandedStepId = null;
    selectedArtifactId = "";

    try {
      const tenantId = requireTenantId();
      const detail = await loadPlanExecutionDetail(tenantId, executionId);
      execution = detail.execution;
      rows = detail.rows;
      source = detail.source;
      if (detail.error) {
        loadError = detail.error;
      }
      if (rows.length > 0) {
        expandedStepId = rows[0].id;
        selectedArtifactId = defaultArtifact(rows[0]);
      }
    } catch (error) {
      execution = null;
      rows = [];
      loadError = toUserMessage(error);
    } finally {
      loading = false;
    }
  }

  function defaultArtifact(row: StepExecutionDisplayRow): string {
    return row.outputArtifactId || row.inputArtifactId || "";
  }

  function toggleRow(row: StepExecutionDisplayRow): void {
    if (expandedStepId === row.id) {
      expandedStepId = null;
      selectedArtifactId = "";
      return;
    }
    expandedStepId = row.id;
    selectedArtifactId = defaultArtifact(row);
  }

  function selectArtifact(
    row: StepExecutionDisplayRow,
    artifactId: string,
    event: MouseEvent,
  ): void {
    event.preventDefault();
    expandedStepId = row.id;
    selectedArtifactId = artifactId;
  }

  function statusClass(status: StepExecutionStatus): string {
    switch (status) {
      case StepExecutionStatus.COMPLETED:
        return "border-green-500/40 bg-green-500/10 text-green-400";
      case StepExecutionStatus.FAILED:
        return "border-red-500/40 bg-red-500/10 text-red-400";
      case StepExecutionStatus.RUNNING:
        return "border-blue-500/40 bg-blue-500/10 text-blue-300";
      default:
        return "border-plumage bg-obsidian-light/40 text-crown-ash";
    }
  }

  function stepStatusLabel(status: StepExecutionStatus): string {
    switch (status) {
      case StepExecutionStatus.PENDING:
        return translate("plans.execution.stepStatus.pending", $locale);
      case StepExecutionStatus.RUNNING:
        return translate("plans.execution.stepStatus.running", $locale);
      case StepExecutionStatus.AWAITING_ELICITATION:
        return translate(
          "plans.execution.stepStatus.awaitingElicitation",
          $locale,
        );
      case StepExecutionStatus.AWAITING_APPROVAL:
        return translate(
          "plans.execution.stepStatus.awaitingApproval",
          $locale,
        );
      case StepExecutionStatus.COMPLETED:
        return translate("plans.execution.stepStatus.completed", $locale);
      case StepExecutionStatus.FAILED:
        return translate("plans.execution.stepStatus.failed", $locale);
      default:
        return translate("plans.execution.stepStatus.unknown", $locale);
    }
  }

  function executionStatusLabel(status: PlanExecutionStatus): string {
    switch (status) {
      case PlanExecutionStatus.PENDING:
        return translate("plans.execution.status.pending", $locale);
      case PlanExecutionStatus.RUNNING:
        return translate("plans.execution.status.running", $locale);
      case PlanExecutionStatus.COMPLETED:
        return translate("plans.execution.status.completed", $locale);
      case PlanExecutionStatus.FAILED:
        return translate("plans.execution.status.failed", $locale);
      case PlanExecutionStatus.CANCELLED:
        return translate("plans.execution.status.cancelled", $locale);
      default:
        return translate("plans.execution.status.unknown", $locale);
    }
  }

  function formatDateTime(value: string): string {
    if (!value) return translate("common.emDash", $locale);
    const parsed = Date.parse(value);
    if (Number.isNaN(parsed)) return value;
    const localeTag = $locale === "pt-BR" ? "pt-BR" : "en-US";
    return new Intl.DateTimeFormat(localeTag, {
      dateStyle: "medium",
      timeStyle: "short",
    }).format(new Date(parsed));
  }

  function onRetryClick(event: MouseEvent): void {
    event.preventDefault();
    // TODO(FR-8): wire RetryStepExecution RPC when available in plans proto.
  }
</script>

<div class="mx-auto flex w-full max-w-5xl flex-col gap-4 px-4 py-4 lg:px-6">
  <div class="flex items-center justify-between">
    <a
      href={resolve("/plans")}
      class="font-body text-sm text-crown-ash transition-colors hover:text-talon-gold"
    >
      {translate("plans.execution.backToPlans", $locale)}
    </a>
    <span
      class="rounded-full border border-plumage px-2.5 py-1 font-mono text-[10px] tracking-wider text-crown-ash uppercase"
    >
      {source === "api"
        ? translate("plans.source.live", $locale)
        : translate("plans.source.mock", $locale)}
    </span>
  </div>

  {#if loading}
    <div class="flex min-h-[16rem] items-center justify-center">
      <div class="flex items-center gap-2 text-crown-ash">
        <Loader2 class="size-5 animate-spin" />
        <span class="font-body text-sm"
          >{translate("plans.execution.loading", $locale)}</span
        >
      </div>
    </div>
  {:else if !execution}
    <div class="rounded-lg border border-red-500/30 bg-red-500/10 p-5">
      <div class="flex items-start gap-3">
        <AlertTriangle class="mt-0.5 size-5 text-red-400" />
        <div>
          <p class="font-body text-sm text-red-300">
            {translate("plans.execution.loadError", $locale)}
          </p>
          {#if loadError}
            <p class="mt-1 font-mono text-xs text-red-200/80">{loadError}</p>
          {/if}
          <button
            onclick={() => fetchExecutionDetail()}
            class="mt-3 inline-flex cursor-pointer items-center gap-2 rounded-md border border-red-400/40 px-3 py-1.5 font-body text-xs text-red-200 transition-colors hover:border-red-300"
          >
            <RefreshCw class="size-3.5" />
            {translate("plans.execution.retryLoad", $locale)}
          </button>
        </div>
      </div>
    </div>
  {:else}
    <section class="rounded-lg border border-plumage bg-obsidian-light/50 p-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 class="font-heading text-xl font-semibold text-cream">
            {translate("plans.execution.heading", $locale)}
          </h1>
          <p class="font-mono text-xs text-crown-ash">{execution.id}</p>
        </div>
        <span
          class="rounded-full border px-2 py-0.5 font-mono text-[10px] tracking-wide uppercase {statusClass(
            rows.some((row) => row.status === StepExecutionStatus.FAILED)
              ? StepExecutionStatus.FAILED
              : rows.every(
                    (row) => row.status === StepExecutionStatus.COMPLETED,
                  )
                ? StepExecutionStatus.COMPLETED
                : StepExecutionStatus.RUNNING,
          )}"
        >
          {executionStatusLabel(execution.status)}
        </span>
      </div>
      <div class="mt-3 grid gap-2 text-xs text-crown-ash sm:grid-cols-2">
        <p>
          {translate("plans.execution.triggeredAt", $locale)}:
          <span class="font-mono">{formatDateTime(execution.triggeredAt)}</span>
        </p>
        <p>
          {translate("plans.execution.completedAt", $locale)}:
          <span class="font-mono">{formatDateTime(execution.completedAt)}</span>
        </p>
      </div>
      {#if loadError}
        <p class="mt-3 font-mono text-xs text-talon-gold">
          {translate("plans.execution.mockWarning", $locale)}
          {loadError}
        </p>
      {/if}
    </section>

    {#if rows.length === 0}
      <div
        class="rounded-lg border border-plumage bg-obsidian-light/40 p-5 text-center font-body text-sm text-crown-ash"
      >
        {translate("plans.execution.emptySteps", $locale)}
      </div>
    {:else}
      <section class="rounded-lg border border-plumage bg-obsidian-light/30">
        <div class="border-b border-plumage/40 px-4 py-3">
          <h2 class="font-heading text-lg font-semibold text-cream">
            {translate("plans.execution.timelineHeading", $locale)}
          </h2>
        </div>
        <ol class="divide-y divide-plumage/40">
          {#each rows as row (row.id)}
            <li class="px-4 py-3">
              <button
                class="flex w-full cursor-pointer items-start justify-between gap-3 text-left"
                onclick={() => toggleRow(row)}
              >
                <div class="min-w-0">
                  <p class="font-heading text-sm text-cream">
                    {row.index}. {row.stepLabel}
                  </p>
                  <p class="mt-1 font-mono text-[11px] text-crown-ash">
                    {row.stepKey} · {translate(
                      "plans.execution.attempt",
                      $locale,
                      {
                        count: row.attempt,
                      },
                    )}
                  </p>
                  <p class="mt-1 font-body text-xs text-crown-ash">
                    {formatDateTime(row.createdAt)} - {formatDateTime(
                      row.updatedAt,
                    )}
                  </p>
                </div>
                <div class="flex items-center gap-2">
                  <span
                    class="rounded-full border px-2 py-0.5 font-mono text-[10px] uppercase {statusClass(
                      row.status,
                    )}"
                  >
                    {stepStatusLabel(row.status)}
                  </span>
                  <ChevronDown
                    class="size-4 text-crown-ash transition-transform {expandedStepId ===
                    row.id
                      ? 'rotate-180'
                      : ''}"
                  />
                </div>
              </button>

              {#if expandedStepId === row.id}
                <div
                  class="mt-3 space-y-3 rounded-md border border-plumage/40 bg-obsidian/40 p-3"
                >
                  <div class="grid gap-2 text-xs sm:grid-cols-2">
                    <div>
                      <p
                        class="font-mono text-[10px] tracking-wide text-crown-ash-dark uppercase"
                      >
                        {translate("plans.execution.inputArtifact", $locale)}
                      </p>
                      {#if row.inputArtifactId}
                        <a
                          href={"#" + row.inputArtifactId}
                          class="font-mono text-xs text-talon-gold underline-offset-2 hover:underline"
                          onclick={(event) =>
                            selectArtifact(row, row.inputArtifactId, event)}
                        >
                          {row.inputArtifactId}
                        </a>
                      {:else}
                        <p class="font-mono text-xs text-crown-ash">
                          {translate("common.emDash", $locale)}
                        </p>
                      {/if}
                    </div>
                    <div>
                      <p
                        class="font-mono text-[10px] tracking-wide text-crown-ash-dark uppercase"
                      >
                        {translate("plans.execution.outputArtifact", $locale)}
                      </p>
                      {#if row.outputArtifactId}
                        <a
                          href={"#" + row.outputArtifactId}
                          class="font-mono text-xs text-talon-gold underline-offset-2 hover:underline"
                          onclick={(event) =>
                            selectArtifact(row, row.outputArtifactId, event)}
                        >
                          {row.outputArtifactId}
                        </a>
                      {:else}
                        <p class="font-mono text-xs text-crown-ash">
                          {translate("common.emDash", $locale)}
                        </p>
                      {/if}
                    </div>
                  </div>

                  {#if row.status === StepExecutionStatus.AWAITING_APPROVAL && row.approvalRequestId && execution}
                    <div class="border-t border-plumage/30 pt-2">
                      <a
                        href={resolve(
                          `/plans/executions/${execution.id}/approvals/${row.approvalRequestId}`,
                        )}
                        class="inline-flex items-center gap-2 rounded-md border border-talon-gold/50 bg-talon-gold/10 px-3 py-1.5 font-body text-xs text-talon-gold transition-colors hover:bg-talon-gold/20"
                      >
                        {translate("plans.execution.reviewApproval", $locale)}
                      </a>
                    </div>
                  {/if}

                  {#if row.canRetry}
                    <div class="border-t border-plumage/30 pt-2">
                      <button
                        disabled
                        title={translate(
                          "plans.execution.retryComingSoon",
                          $locale,
                        )}
                        onclick={onRetryClick}
                        class="inline-flex cursor-not-allowed items-center gap-2 rounded-md border border-plumage/60 px-3 py-1.5 font-body text-xs text-crown-ash-dark opacity-70"
                      >
                        {translate("plans.execution.retryFromStep", $locale)}
                      </button>
                    </div>
                  {/if}

                  {#if selectedArtifactId}
                    <div class="border-t border-plumage/30 pt-3">
                      <p
                        class="mb-2 font-mono text-[10px] tracking-wide text-crown-ash-dark uppercase"
                      >
                        {translate("plans.execution.previewHeading", $locale)}
                      </p>
                      <ArtifactPreview
                        tenantId={execution.tenantId}
                        artifactId={selectedArtifactId}
                      />
                    </div>
                  {/if}
                </div>
              {/if}
            </li>
          {/each}
        </ol>
      </section>
    {/if}
  {/if}
</div>
