<script lang="ts">
  import { resolve } from "$app/paths";
  import { getTenant } from "$lib/auth";
  import { locale, translate } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";
  import { planClient } from "$lib/rpc";
  import { PlanConfigurationStatus, type PlanConfiguration } from "$lib/gen/harpia/plans/v1/plans_pb";

  const tenantId = $derived(getTenant()?.id ?? "");
  let plans = $state<PlanConfiguration[]>([]);
  let loadError = $state(false);

  const POLL_MS = 5000;

  $effect(() => {
    if (!tenantId) return;
    const controller = new AbortController();
    let cancelled = false;

    async function loadOnce() {
      try {
        const out: PlanConfiguration[] = [];
        for await (const page of planClient.listPlanConfigurations(
          { tenantId, pageSize: 50, pageToken: "" },
          { signal: controller.signal },
        )) {
          out.push(...page.planConfigurations);
        }
        if (cancelled) return;
        plans = out
          .filter(
            (p) =>
              p.status !== PlanConfigurationStatus.ARCHIVED &&
              p.status !== PlanConfigurationStatus.DISABLED,
          )
          .sort(
            (a, b) =>
              Date.parse(b.updatedAt || b.createdAt) -
              Date.parse(a.updatedAt || a.createdAt),
          )
          .slice(0, 10);
      } catch {
        if (!cancelled) loadError = true;
      }
    }

    void loadOnce();
    const intervalId = setInterval(loadOnce, POLL_MS);
    return () => {
      cancelled = true;
      controller.abort();
      clearInterval(intervalId);
    };
  });

  function statusLabelKey(status: PlanConfigurationStatus): string {
    switch (status) {
      case PlanConfigurationStatus.RUNNABLE: return "sidebar.yourPlans.status.runnable";
      case PlanConfigurationStatus.SCHEDULED: return "sidebar.yourPlans.status.scheduled";
      default: return "sidebar.yourPlans.status.draft";
    }
  }
</script>

<div class="mt-4">
  <p class="px-3 font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase">
    {translate("sidebar.yourPlans.heading", $locale)}
  </p>
  {#if loadError}
    <p class="px-3 mt-1 text-[11px] text-crown-ash-dark">{translate("sidebar.yourPlans.loadError", $locale)}</p>
  {:else if plans.length === 0}
    <p class="px-3 mt-1 text-[11px] text-crown-ash-dark">{translate("sidebar.yourPlans.empty", $locale)}</p>
  {:else}
    <div class="mt-1 space-y-0.5">
      {#each plans as plan (plan.id)}
        <a
          href={resolve(`/plans/configurations/${plan.id}`)}
          class="flex items-center gap-2 rounded-md px-3 py-1.5 text-[12px] text-crown-ash hover:bg-obsidian-light hover:text-cream"
        >
          <span class="flex-1 truncate">{plan.id.slice(0, 8)}</span>
          <span class="text-[9px] uppercase text-crown-ash-dark">
            {translate(statusLabelKey(plan.status), $locale)}
          </span>
          <span class="text-[9px] text-crown-ash-dark">
            {formatRelativeTime(plan.updatedAt || plan.createdAt, $locale)}
          </span>
        </a>
      {/each}
    </div>
    <a
      href={resolve(`/plans/configurations`)}
      class="mt-1 block px-3 py-1 text-[10px] text-crown-ash-dark hover:text-talon-gold"
    >
      {translate("sidebar.yourPlans.seeAll", $locale)}
    </a>
  {/if}
</div>
