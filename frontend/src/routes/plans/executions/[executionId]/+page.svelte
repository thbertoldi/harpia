<script lang="ts">
  import { resolve } from "$app/paths";
  import { ArrowLeft, ExternalLink, FileText, ListChecks } from "lucide-svelte";
  import ArtifactPreview from "$lib/components/ArtifactPreview.svelte";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import StepExecutionStatusBadge from "$lib/components/StepExecutionStatusBadge.svelte";
  import ThreadMessage from "$lib/components/thread/ThreadMessage.svelte";
  import { locale, translate } from "$lib/i18n";
  import { formatLocaleDateTime } from "$lib/i18n/format";
  import {
    statusKeyForPlanExecution,
    statusKeyForStepExecution,
  } from "$lib/plans/plan-execution";
  import { PlanExecutionStatus, type StepExecutionStatus } from "$lib/rpc";

  let { data } = $props();

  const execution = $derived(data.detail.execution);
  const rows = $derived(data.detail.rows);
  const activity = $derived(data.activity);

  function formatTimestamp(value: string): string {
    if (!value) return translate("common.emDash", $locale);
    return formatLocaleDateTime(value, $locale);
  }

  function planStatusClass(status: PlanExecutionStatus): string {
    if (status === PlanExecutionStatus.RUNNING) {
      return "border-blue-500/40 bg-blue-500/10 text-blue-300";
    }
    if (status === PlanExecutionStatus.COMPLETED) {
      return "border-green-500/40 bg-green-500/10 text-green-300";
    }
    if (status === PlanExecutionStatus.FAILED) {
      return "border-red-500/40 bg-red-500/10 text-red-300";
    }
    return "border-plumage bg-obsidian-light text-crown-ash";
  }

  function statusText(status: StepExecutionStatus): string {
    return translate(statusKeyForStepExecution(status), $locale);
  }
</script>

<svelte:head>
  <title>{translate("executions.detail.heading", $locale)} · Harpia</title>
</svelte:head>

<div class="px-4 py-6 lg:px-6">
  <div class="mb-5 flex flex-wrap items-center justify-between gap-3">
    <a
      href={resolve("/plans/executions")}
      class="inline-flex items-center gap-2 rounded-md border border-plumage px-3 py-2 text-[12px] text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
    >
      <ArrowLeft class="size-4" />
      {translate("executions.detail.backToList", $locale)}
    </a>
    <a
      href={resolve(
        `/plans/configurations/${execution.planConfigurationId}/canvas?run=${execution.id}`,
      )}
      class="inline-flex items-center gap-2 rounded-md border border-talon-gold/50 px-3 py-2 text-[12px] text-talon-gold transition-colors hover:bg-talon-gold/10"
    >
      <ExternalLink class="size-4" />
      {translate("executions.detail.openCanvas", $locale)}
    </a>
  </div>

  <header class="mb-6 border-b border-plumage pb-5">
    <div class="flex flex-wrap items-start justify-between gap-4">
      <div class="min-w-0">
        <HarpyHeading tag="h1" class="text-2xl text-cream">
          {translate("executions.detail.heading", $locale)}
        </HarpyHeading>
        <p class="mt-1 font-mono text-[11px] break-all text-crown-ash-dark">
          {execution.id}
        </p>
      </div>
      <span
        class="inline-flex items-center rounded-full border px-2.5 py-1 font-mono text-[10px] tracking-wider uppercase {planStatusClass(
          execution.status,
        )}"
      >
        {translate(statusKeyForPlanExecution(execution.status), $locale)}
      </span>
    </div>

    <dl class="mt-5 grid gap-3 md:grid-cols-4">
      <div>
        <dt class="font-mono text-[10px] text-crown-ash-dark uppercase">
          {translate("executions.detail.planConfiguration", $locale)}
        </dt>
        <dd class="mt-1 text-[12px] break-all text-crown-ash">
          {execution.planConfigurationId}
        </dd>
      </div>
      <div>
        <dt class="font-mono text-[10px] text-crown-ash-dark uppercase">
          {translate("executions.detail.triggeredAt", $locale)}
        </dt>
        <dd class="mt-1 text-[12px] text-crown-ash">
          {formatTimestamp(execution.triggeredAt)}
        </dd>
      </div>
      <div>
        <dt class="font-mono text-[10px] text-crown-ash-dark uppercase">
          {translate("executions.detail.updatedAt", $locale)}
        </dt>
        <dd class="mt-1 text-[12px] text-crown-ash">
          {formatTimestamp(execution.updatedAt)}
        </dd>
      </div>
      <div>
        <dt class="font-mono text-[10px] text-crown-ash-dark uppercase">
          {translate("executions.detail.completedAt", $locale)}
        </dt>
        <dd class="mt-1 text-[12px] text-crown-ash">
          {formatTimestamp(execution.completedAt)}
        </dd>
      </div>
    </dl>

    {#if data.detail.source === "mock"}
      <p
        class="mt-4 rounded-md border border-talon-gold/30 bg-talon-gold/5 px-3 py-2 font-mono text-xs text-talon-gold"
      >
        {translate("executions.detail.apiFallback", $locale)}
        {data.detail.error ?? ""}
      </p>
    {/if}
  </header>

  <div class="grid gap-5 xl:grid-cols-[minmax(0,1fr)_420px]">
    <section>
      <div class="mb-3 flex items-center gap-2">
        <ListChecks class="size-4 text-talon-gold" />
        <h2 class="font-heading text-base font-semibold text-cream">
          {translate("executions.detail.stepsHeading", $locale)}
        </h2>
      </div>

      {#if rows.length === 0}
        <div
          class="rounded-lg border border-dashed border-plumage bg-obsidian-light/20 px-5 py-8 text-center"
        >
          <p class="font-body text-sm text-crown-ash">
            {translate("executions.timeline.empty", $locale)}
          </p>
        </div>
      {:else}
        <div class="space-y-3">
          {#each rows as row (row.id)}
            <article
              class="rounded-lg border border-plumage bg-obsidian-light/20 p-4"
            >
              <div class="flex flex-wrap items-start justify-between gap-3">
                <div class="min-w-0">
                  <p class="font-heading text-sm font-semibold text-cream">
                    {row.index}. {row.stepLabel}
                  </p>
                  <p class="mt-1 font-mono text-[10px] text-crown-ash-dark">
                    {row.stepKey} · {translate(
                      "executions.detail.attempt",
                      $locale,
                    )}
                    {row.attempt}
                  </p>
                </div>
                <StepExecutionStatusBadge status={row.status} />
              </div>

              <dl class="mt-3 grid gap-2 sm:grid-cols-2">
                <div>
                  <dt
                    class="font-mono text-[10px] text-crown-ash-dark uppercase"
                  >
                    {translate("executions.detail.createdAt", $locale)}
                  </dt>
                  <dd class="mt-1 text-[12px] text-crown-ash">
                    {formatTimestamp(row.createdAt)}
                  </dd>
                </div>
                <div>
                  <dt
                    class="font-mono text-[10px] text-crown-ash-dark uppercase"
                  >
                    {translate("executions.detail.updatedAt", $locale)}
                  </dt>
                  <dd class="mt-1 text-[12px] text-crown-ash">
                    {formatTimestamp(row.updatedAt)}
                  </dd>
                </div>
              </dl>

              <div class="mt-4 grid gap-3 lg:grid-cols-2">
                <section class="min-w-0 border-t border-plumage/60 pt-3">
                  <div class="mb-2 flex items-center gap-2">
                    <FileText class="size-3.5 text-crown-ash-dark" />
                    <h3
                      class="font-mono text-[10px] text-crown-ash-dark uppercase"
                    >
                      {translate("executions.detail.previewInput", $locale)}
                    </h3>
                  </div>
                  {#if row.inputArtifactId}
                    <p
                      class="mb-2 font-mono text-[10px] break-all text-crown-ash-dark"
                    >
                      {row.inputArtifactId}
                    </p>
                    <ArtifactPreview
                      tenantId={data.tenantId}
                      artifactId={row.inputArtifactId}
                    />
                  {:else}
                    <p class="text-[12px] text-crown-ash-dark">
                      {translate("executions.detail.noArtifact", $locale)}
                    </p>
                  {/if}
                </section>

                <section class="min-w-0 border-t border-plumage/60 pt-3">
                  <div class="mb-2 flex items-center gap-2">
                    <FileText class="size-3.5 text-crown-ash-dark" />
                    <h3
                      class="font-mono text-[10px] text-crown-ash-dark uppercase"
                    >
                      {translate("executions.detail.previewOutput", $locale)}
                    </h3>
                  </div>
                  {#if row.outputArtifactId}
                    <p
                      class="mb-2 font-mono text-[10px] break-all text-crown-ash-dark"
                    >
                      {row.outputArtifactId}
                    </p>
                    <ArtifactPreview
                      tenantId={data.tenantId}
                      artifactId={row.outputArtifactId}
                    />
                  {:else}
                    <p class="text-[12px] text-crown-ash-dark">
                      {translate("executions.detail.noArtifact", $locale)}
                    </p>
                  {/if}
                </section>
              </div>

              {#if row.status}
                <p class="sr-only">{statusText(row.status)}</p>
              {/if}
            </article>
          {/each}
        </div>
      {/if}
    </section>

    <aside>
      <div class="mb-3 flex items-center gap-2">
        <ListChecks class="size-4 text-talon-gold" />
        <h2 class="font-heading text-base font-semibold text-cream">
          {translate("executions.detail.activityHeading", $locale)}
        </h2>
      </div>
      {#if data.activityError}
        <p
          class="mb-3 rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 font-mono text-xs text-red-300"
        >
          {translate("executions.detail.activityError", $locale, {
            error: data.activityError,
          })}
        </p>
      {/if}
      {#if activity.length === 0}
        <div
          class="rounded-lg border border-dashed border-plumage bg-obsidian-light/20 px-5 py-8 text-center"
        >
          <p class="font-body text-sm text-crown-ash">
            {translate("executions.detail.activityEmpty", $locale)}
          </p>
        </div>
      {:else}
        <div class="flex flex-col gap-2">
          {#each activity as message (message.id)}
            <ThreadMessage
              {message}
              tenantId={data.tenantId}
              configurationId={execution.planConfigurationId}
            />
          {/each}
        </div>
      {/if}
    </aside>
  </div>
</div>
