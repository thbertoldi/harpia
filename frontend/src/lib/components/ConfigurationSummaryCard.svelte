<script lang="ts">
  import { Receipt } from "lucide-svelte";
  import Card from "$lib/components/Card.svelte";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import {
    formatMoney,
    type ConfigCostSummary,
    type ConfigLineItemKind,
  } from "$lib/plans/config-summary";
  import { locale, translate } from "$lib/i18n";

  let {
    summary,
    priceSource = "mock",
  }: {
    summary: ConfigCostSummary;
    priceSource?: "api" | "mock";
  } = $props();

  const kindOrder: ConfigLineItemKind[] = [
    "step",
    "executor",
    "overseer",
    "policy",
  ];

  function kindLabel(kind: ConfigLineItemKind): string {
    return translate(`plans.summary.kind.${kind}`, $locale);
  }

  function kindClass(kind: ConfigLineItemKind): string {
    switch (kind) {
      case "step":
        return "text-text-muted";
      case "executor":
        return "text-primary";
      case "overseer":
        return "text-text";
      case "policy":
        return "text-text-muted";
      default:
        return "text-text-muted";
    }
  }
</script>

<Card>
  <div class="mb-4 flex flex-wrap items-start justify-between gap-3">
    <div class="flex items-center gap-2">
      <Receipt class="size-4 text-text-muted" />
      <HarpyHeading tag="h2" class="text-lg text-text">
        {translate("plans.summary.heading", $locale)}
      </HarpyHeading>
    </div>
    <span
      class="rounded-full border border-border px-2.5 py-1 font-mono text-[10px] tracking-wider text-text-muted uppercase"
    >
      {priceSource === "api"
        ? translate("plans.summary.priceSource.live", $locale)
        : translate("plans.summary.priceSource.mock", $locale)}
    </span>
  </div>

  <p class="mb-4 font-body text-sm text-text-muted">
    {translate("plans.summary.description", $locale)}
  </p>

  <div class="overflow-x-auto rounded-lg border border-border">
    <table class="w-full min-w-[640px] border-collapse">
      <thead>
        <tr
          class="border-b border-border bg-surface-hover font-mono text-[10px] tracking-widest text-text-muted-dark uppercase"
        >
          <th class="px-3 py-2 text-left"
            >{translate("plans.summary.column.kind", $locale)}</th
          >
          <th class="px-3 py-2 text-left"
            >{translate("plans.summary.column.item", $locale)}</th
          >
          <th class="px-3 py-2 text-right"
            >{translate("plans.summary.column.cost", $locale)}</th
          >
        </tr>
      </thead>
      <tbody>
        {#each kindOrder as kind (kind)}
          {#each summary.lineItems.filter((item) => item.kind === kind) as item (item.key)}
            <tr class="border-b border-border/40">
              <td
                class="px-3 py-2 font-mono text-[10px] uppercase {kindClass(
                  kind,
                )}"
              >
                {kindLabel(kind)}
              </td>
              <td class="px-3 py-3">
                <p class="font-heading text-sm font-semibold text-text">
                  {item.label}
                </p>
                {#if item.detail}
                  <p class="font-mono text-[10px] text-text-muted-dark">
                    {item.detail}
                  </p>
                {/if}
              </td>
              <td class="px-3 py-3 text-right font-mono text-sm text-text">
                {formatMoney(item.priceCents, item.currency, $locale)}
              </td>
            </tr>
          {/each}
        {/each}
      </tbody>
      <tfoot>
        <tr class="border-t border-border bg-surface-hover">
          <td
            colspan="2"
            class="px-3 py-3 text-right font-heading text-sm font-semibold text-text"
          >
            {translate("plans.summary.total", $locale)}
          </td>
          <td class="px-3 py-3 text-right font-mono text-base text-primary">
            {formatMoney(summary.totalCents, summary.currency, $locale)}
          </td>
        </tr>
      </tfoot>
    </table>
  </div>

  <p class="mt-3 font-body text-xs text-text-muted-dark">
    {translate("plans.summary.perRunNote", $locale)}
  </p>
</Card>
