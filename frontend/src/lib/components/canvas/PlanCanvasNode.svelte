<script lang="ts">
  import {
    Loader2,
    Check,
    AlertTriangle,
    MessageSquare,
    ShieldQuestion,
    Clock,
  } from "lucide-svelte";
  import type { PlanStep } from "$lib/gen/harpia/plans/v1/plans_pb";
  import type { CanvasStepState } from "$lib/plans/canvas-state";
  import { locale, translate } from "$lib/i18n";

  interface Props {
    step: PlanStep;
    state: CanvasStepState;
    selected: boolean;
    onSelect: () => void;
  }
  let { step, state, selected, onSelect }: Props = $props();

  const StatusIcon = $derived(
    state.status === "running"
      ? Loader2
      : state.status === "done"
        ? Check
        : state.status === "failed"
          ? AlertTriangle
          : state.status === "awaiting_elicitation"
            ? MessageSquare
            : state.status === "awaiting_approval"
              ? ShieldQuestion
              : Clock,
  );

  const statusLabelKey = $derived(`canvas.status.${state.status}`);
  const ringClass = $derived(
    selected ? "ring-2 ring-talon-gold ring-offset-2 ring-offset-obsidian" : "",
  );
  const borderClass = $derived(
    state.status === "done"
      ? "border-talon-gold/60"
      : state.status === "failed"
        ? "border-red-500/60"
        : state.status === "running"
          ? "border-talon-gold"
          : "border-plumage",
  );
</script>

<button
  type="button"
  onclick={onSelect}
  class={`flex w-44 flex-col gap-1.5 rounded-lg border bg-obsidian-light px-3 py-2 text-left transition-colors hover:border-talon-gold ${borderClass} ${ringClass}`}
>
  <div class="flex items-center gap-1.5">
    <StatusIcon
      class={`size-3.5 ${state.status === "running" ? "animate-spin text-talon-gold" : "text-crown-ash"}`}
    />
    <span class="font-heading text-[12px] font-semibold text-cream">
      {step.title}
    </span>
  </div>
  <span class="font-mono text-[9px] text-crown-ash-dark">{step.key}</span>
  <span class="text-[10px] text-crown-ash">
    {translate(statusLabelKey, $locale)}
  </span>
</button>
