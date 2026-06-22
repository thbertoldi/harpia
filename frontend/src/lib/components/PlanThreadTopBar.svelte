<script lang="ts">
  import { Calendar } from "lucide-svelte";
  import PlanCostPill from "./PlanCostPill.svelte";
  import { locale, translate } from "$lib/i18n";
  import type { RunCost } from "$lib/plans/cost";

  interface Props {
    planName: string;
    statusLabel: string;
    cost: RunCost;
    onOpenSchedule?: () => void;
  }
  let { planName, statusLabel, cost, onOpenSchedule }: Props = $props();
</script>

<header
  class="flex flex-wrap items-center gap-3 border-b border-plumage/60 bg-obsidian px-4 py-2"
>
  <span class="font-heading text-[13px] font-semibold text-cream"
    >{planName}</span
  >
  <span
    class="rounded-full border border-plumage px-2 py-0.5 font-mono text-[10px] text-crown-ash uppercase"
  >
    {statusLabel}
  </span>
  <div class="ml-auto flex items-center gap-2">
    <PlanCostPill {cost} />
    {#if onOpenSchedule}
      <button
        type="button"
        onclick={onOpenSchedule}
        class="flex items-center gap-1 rounded border border-plumage px-2 py-1 text-[11px] text-crown-ash hover:border-talon-gold hover:text-talon-gold"
      >
        <Calendar class="size-3.5" />
        {translate("schedule.openButton", $locale)}
      </button>
    {/if}
  </div>
</header>
