<script lang="ts">
  import { Coins } from "lucide-svelte";
  import { locale, translate } from "$lib/i18n";
  import type { RunCost } from "$lib/plans/cost";

  interface Props { cost: RunCost }
  let { cost }: Props = $props();

  const label = $derived(
    cost.unboundStepCount > 0
      ? translate("cost.pillPartial", $locale, {
          amount: cost.totalPerRunBrl.toFixed(2),
          pending: cost.unboundStepCount,
        })
      : translate("cost.pillFull", $locale, { amount: cost.totalPerRunBrl.toFixed(2) }),
  );
</script>

<span
  class="inline-flex items-center gap-1 rounded-full border border-plumage bg-obsidian-light px-2 py-1 text-[11px] text-cream"
  title={cost.breakdown.map((b) => `${b.stepTitle}: ${b.pricePerRunBrl != null ? `R$ ${b.pricePerRunBrl.toFixed(2)}` : "—"}`).join("\n")}
>
  <Coins class="size-3.5 text-talon-gold" />
  {label}
</span>
