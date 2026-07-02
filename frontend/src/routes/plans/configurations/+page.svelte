<script lang="ts">
  import { resolve } from "$app/paths";
  import { Trash2, Loader2 } from "lucide-svelte";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import { getTenant } from "$lib/auth";
  import { locale, translate } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";
  import { planClient } from "$lib/rpc";
  import {
    PlanConfigurationStatus,
    type PlanConfiguration,
  } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { buildPlanSummary } from "$lib/plans/config-summary";

  let { data } = $props();

  type Filter = "all" | "draft" | "runnable" | "scheduled" | "archived";
  let filter = $state<Filter>("all");
  let plans = $state<PlanConfiguration[]>(data.configurations);
  let archiving = $state<string[]>([]);
  let archiveError = $state<string | null>(null);

  const filters: { key: Filter; labelKey: string }[] = [
    { key: "all", labelKey: "plans.list.filter.all" },
    { key: "draft", labelKey: "plans.list.filter.draft" },
    { key: "runnable", labelKey: "plans.list.filter.runnable" },
    { key: "scheduled", labelKey: "plans.list.filter.scheduled" },
    { key: "archived", labelKey: "plans.list.filter.archived" },
  ];

  function statusOf(p: PlanConfiguration): Filter {
    switch (p.status) {
      case PlanConfigurationStatus.DRAFT:
        return "draft";
      case PlanConfigurationStatus.RUNNABLE:
        return "runnable";
      case PlanConfigurationStatus.SCHEDULED:
        return "scheduled";
      case PlanConfigurationStatus.ARCHIVED:
      case PlanConfigurationStatus.DISABLED:
        return "archived";
      default:
        return "all";
    }
  }

  function statusLabel(p: PlanConfiguration): string {
    return translate(`plans.list.status.${statusOf(p)}`, $locale);
  }

  const visible = $derived(
    filter === "all"
      ? plans.filter((p) => statusOf(p) !== "archived")
      : plans.filter((p) => statusOf(p) === filter),
  );

  const sorted = $derived(
    [...visible].sort(
      (a, b) =>
        Date.parse(b.updatedAt || b.createdAt) -
        Date.parse(a.updatedAt || a.createdAt),
    ),
  );

  function templateName(p: PlanConfiguration): string {
    return (
      data.templatesById.get(p.planTemplateId)?.name ||
      p.planTemplateId.slice(0, 8)
    );
  }

  function summaryFor(p: PlanConfiguration) {
    const template = data.templatesById.get(p.planTemplateId);
    return template ? buildPlanSummary(template, p) : null;
  }

  async function archive(p: PlanConfiguration) {
    if (archiving.includes(p.id)) return;
    const tenant = getTenant();
    if (!tenant?.id) return;
    archiving = [...archiving, p.id];
    archiveError = null;
    try {
      const res = await planClient.updatePlanConfiguration({
        tenantId: tenant.id,
        planConfigurationId: p.id,
        status: PlanConfigurationStatus.ARCHIVED,
        seedArtifacts: p.seedArtifacts,
        slotBindings: p.slotBindings,
        overseerBindings: p.overseerBindings,
        behaviorPolicies: p.behaviorPolicies,
        schedule: p.schedule,
      });
      if (res.planConfiguration) {
        plans = plans.map((x) => (x.id === p.id ? res.planConfiguration! : x));
      }
    } catch (e) {
      archiveError = e instanceof Error ? e.message : "Failed to archive plan";
    } finally {
      archiving = archiving.filter((id) => id !== p.id);
    }
  }
</script>

<svelte:head>
  <title>{translate("plans.list.title", $locale)} · Harpia</title>
</svelte:head>

<div class="mx-auto max-w-4xl px-4 py-6">
  <HarpyHeading tag="h1" class="text-2xl text-cream">
    {translate("plans.list.title", $locale)}
  </HarpyHeading>
  <p class="mt-1 font-body text-[13px] text-crown-ash">
    {translate("plans.list.subtitle", $locale)}
  </p>

  <div class="mt-4 flex flex-wrap gap-2">
    {#each filters as f (f.key)}
      <button
        type="button"
        onclick={() => (filter = f.key)}
        class="cursor-pointer rounded-md border px-2.5 py-1 text-[11px] {filter ===
        f.key
          ? 'border-talon-gold bg-talon-gold/10 text-talon-gold'
          : 'border-plumage text-crown-ash hover:border-talon-gold'}"
      >
        {translate(f.labelKey, $locale)}
      </button>
    {/each}
  </div>

  {#if archiveError}
    <p
      class="mt-3 rounded border border-red-400/40 bg-red-400/10 px-3 py-2 text-[12px] text-red-300"
    >
      {archiveError}
    </p>
  {/if}

  {#if sorted.length === 0}
    <p
      class="mt-6 rounded border border-plumage bg-obsidian-light px-4 py-3 text-sm text-crown-ash"
    >
      {translate("plans.list.empty", $locale)}
    </p>
  {:else}
    <div
      class="mt-4 divide-y divide-plumage/40 rounded-lg border border-plumage bg-obsidian-light"
    >
      {#each sorted as p (p.id)}
        {@const summary = summaryFor(p)}
        <div class="flex items-center gap-3 px-4 py-3">
          <a
            href={resolve(`/plans/configurations/${p.id}`)}
            class="min-w-0 flex-1"
          >
            <p
              class="truncate font-heading text-[13px] font-semibold text-cream hover:text-talon-gold"
            >
              {summary?.intent || templateName(p)}
            </p>
            <p
              class="mt-0.5 truncate font-mono text-[10px] text-crown-ash-dark"
            >
              {summary?.templateName || templateName(p)} · {p.id}
            </p>
          </a>
          <span
            class="rounded-full border border-plumage px-2 py-0.5 font-mono text-[10px] text-crown-ash uppercase"
          >
            {statusLabel(p)}
          </span>
          <span class="text-[10px] whitespace-nowrap text-crown-ash-dark">
            {formatRelativeTime(p.updatedAt || p.createdAt, $locale)}
          </span>
          {#if statusOf(p) !== "archived"}
            <button
              type="button"
              onclick={() => archive(p)}
              disabled={archiving.includes(p.id)}
              aria-label={translate("plans.list.archive", $locale)}
              title={translate("plans.list.archive", $locale)}
              class="cursor-pointer rounded p-1 text-crown-ash hover:text-red-400 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {#if archiving.includes(p.id)}
                <Loader2 class="size-3.5 animate-spin" />
              {:else}
                <Trash2 class="size-3.5" />
              {/if}
            </button>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>
