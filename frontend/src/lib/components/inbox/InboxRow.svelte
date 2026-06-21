<script lang="ts">
  import type { Snippet } from "svelte";
  import { locale, translate } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";
  import type { InboxItem } from "$lib/inbox/types";

  interface Props {
    item: InboxItem;
    urgent?: boolean;
    actions: Snippet;
    footer?: Snippet;
  }

  let { item, urgent = false, actions, footer }: Props = $props();

  const subtypeKey = $derived(`inbox.subtype.${item.kind}`);
  const subtypePillClass = $derived(
    item.kind === "elicitation"
      ? "border border-talon-gold/40 bg-talon-gold/10 text-talon-gold"
      : item.kind === "approval"
        ? "border border-talon-gold bg-talon-gold/15 text-cream"
        : "border border-plumage bg-plumage text-crown-ash",
  );

  const sourceLabel = $derived(
    translate("inbox.row.source", $locale)
      .replace("{plan}", item.planName)
      .replace("{task}", item.taskName),
  );

  const displaySummary = $derived(
    item.kind === "approval"
      ? translate("inbox.summary.approval", $locale)
      : item.summary,
  );
</script>

<div
  class="rounded-lg border border-plumage bg-obsidian-light px-4 py-3 transition-colors hover:border-talon-gold"
  class:border-l-2={urgent}
  class:border-l-talon-gold={urgent}
>
  <div class="grid grid-cols-[auto_1fr_auto] items-center gap-4">
    <span
      class={`min-w-[64px] rounded px-2 py-1 text-center text-[9px] font-bold tracking-wider uppercase ${subtypePillClass}`}
    >
      {translate(subtypeKey, $locale)}
    </span>
    <div class="min-w-0">
      <div class="mb-1 font-mono text-[11px] text-crown-ash">
        {sourceLabel}
      </div>
      <div class="mb-1 text-[13px] leading-snug text-cream">
        {displaySummary}
      </div>
      <div class="flex gap-3 text-[11px] text-crown-ash">
        <span>{formatRelativeTime(item.createdAt, $locale)}</span>
      </div>
    </div>
    <div class="flex gap-1.5">
      {@render actions()}
    </div>
  </div>
  {#if footer}
    <div class="mt-3 border-t border-plumage pt-3">
      {@render footer()}
    </div>
  {/if}
</div>
