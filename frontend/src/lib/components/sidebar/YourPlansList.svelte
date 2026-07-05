<script lang="ts">
  import { resolve } from "$app/paths";
  import { getTenant } from "$lib/auth";
  import { locale, translate, type Locale } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";
  import { planClient } from "$lib/rpc";
  import { SvelteMap } from "svelte/reactivity";
  import {
    PlanConfigurationStatus,
    type PlanConfiguration,
    type PlanTemplate,
  } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { localizedPlanName } from "$lib/plans/catalog-i18n";

  const tenantId = $derived(getTenant()?.id ?? "");
  let plans = $state<PlanConfiguration[]>([]);
  let loadError = $state(false);
  // id -> template, fetched once per tenant so the sidebar list can render a
  // recognizable plan name instead of a truncated id. PlanConfiguration only
  // carries planTemplateId; the display name lives on the template. SvelteMap
  // is reactive on its own, so no $state wrap.
  let templatesById = new SvelteMap<string, PlanTemplate>();

  const POLL_MS = 5000;

  // Templates change rarely (catalog authoring), so fetch them once when the
  // tenant becomes available — not on every 5s plan poll. Falls back to the
  // localized "Plan" label + short id if this fetch fails or hasn't resolved.
  $effect(() => {
    if (!tenantId) return;
    const controller = new AbortController();
    let cancelled = false;
    void (async () => {
      try {
        const map = new SvelteMap<string, PlanTemplate>();
        for await (const page of planClient.listPlanTemplates(
          { pageSize: 100, pageToken: "" },
          { signal: controller.signal },
        )) {
          for (const template of page.planTemplates) {
            map.set(template.id, template);
          }
        }
        if (!cancelled) templatesById = map;
      } catch {
        // best-effort; plan labels fall back to localized "Plan" + short id
      }
    })();
    return () => {
      cancelled = true;
      controller.abort();
    };
  });

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
      case PlanConfigurationStatus.RUNNABLE:
        return "sidebar.yourPlans.status.runnable";
      case PlanConfigurationStatus.SCHEDULED:
        return "sidebar.yourPlans.status.scheduled";
      default:
        return "sidebar.yourPlans.status.draft";
    }
  }

  // Recognizable label for the plan: the localized template name when the
  // template is loaded, else the localized "Plan" label plus the short id so
  // multiple fallback entries stay distinguishable. `locale` is threaded in
  // (rather than read via `$locale` inside the helper) to match the file's
  // existing pure-helper style; `templatesById` reads are tracked by runes.
  function planLabel(plan: PlanConfiguration, locale: Locale): string {
    const template = templatesById.get(plan.planTemplateId);
    if (template) return localizedPlanName(template, locale);
    return `${translate("sidebar.yourPlans.planFallback", locale)} ${plan.id.slice(0, 8)}`;
  }
</script>

<div class="mt-4">
  <p
    class="px-3 font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase"
  >
    {translate("sidebar.yourPlans.heading", $locale)}
  </p>
  {#if loadError}
    <p class="mt-1 px-3 text-[11px] text-crown-ash-dark">
      {translate("sidebar.yourPlans.loadError", $locale)}
    </p>
  {:else if plans.length === 0}
    <p class="mt-1 px-3 text-[11px] text-crown-ash-dark">
      {translate("sidebar.yourPlans.empty", $locale)}
    </p>
  {:else}
    <div class="mt-1 space-y-0.5">
      {#each plans as plan (plan.id)}
        <a
          href={plan.originThreadId
            ? resolve(`/chat/${plan.originThreadId}`)
            : resolve("/runs")}
          class="flex items-center gap-2 rounded-md px-3 py-1.5 text-[12px] text-crown-ash hover:bg-obsidian-light hover:text-cream"
        >
          <span class="flex-1 truncate">{planLabel(plan, $locale)}</span>
          <span class="text-[9px] text-crown-ash-dark uppercase">
            {translate(statusLabelKey(plan.status), $locale)}
          </span>
          <span class="text-[9px] text-crown-ash-dark">
            {formatRelativeTime(plan.updatedAt || plan.createdAt, $locale)}
          </span>
        </a>
      {/each}
    </div>
    <a
      href={resolve("/runs")}
      class="mt-1 block px-3 py-1 text-[10px] text-crown-ash-dark hover:text-talon-gold"
    >
      {translate("sidebar.yourPlans.seeAll", $locale)}
    </a>
  {/if}
</div>
