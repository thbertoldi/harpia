<script lang="ts">
  import type { Snippet } from "svelte";
  import { locale, translate } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";
  import type { InboxItem } from "$lib/inbox/types";
  import { hoverCardLift } from "$lib/motion/transitions";
  import { shortApprovalContextId } from "$lib/plans/approval-card";

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
      ? "border border-border bg-surface-hover text-text-muted"
      : item.kind === "approval"
        ? "border border-border bg-surface-hover text-text"
        : "border border-border bg-surface text-text-muted",
  );

  const sourceLabel = $derived(
    translate("inbox.row.source", $locale, {
      plan: item.planName,
      task: item.taskName,
    }),
  );

  const displaySummary = $derived(
    item.kind === "approval"
      ? translate("inbox.summary.approval", $locale, { task: item.taskName })
      : item.summary,
  );

  const approvalContextItems = $derived.by<{ key: string; label: string }[]>(
    () => {
      if (item.kind !== "approval") return [];
      const items: { key: string; label: string }[] = [];
      if (item.inputArtifactId) {
        items.push({
          key: "artifact",
          label: translate("inbox.context.artifact", $locale, {
            value: shortApprovalContextId(item.inputArtifactId),
          }),
        });
      }
      if (item.planExecutionId) {
        items.push({
          key: "execution",
          label: translate("inbox.context.execution", $locale, {
            value: shortApprovalContextId(item.planExecutionId),
          }),
        });
      }
      if (item.approvalRequestId) {
        items.push({
          key: "approval",
          label: translate("inbox.context.approval", $locale, {
            value: shortApprovalContextId(item.approvalRequestId),
          }),
        });
      }
      return items;
    },
  );
</script>

<div
  use:hoverCardLift
  class="rounded-lg border border-border bg-surface-elevated px-4 py-3 transition-colors hover:bg-surface-hover"
  class:border-l-2={urgent}
  class:border-l-primary={urgent}
>
  <div class="grid grid-cols-[auto_1fr_auto] items-center gap-4">
    <span
      class={`min-w-[64px] rounded px-2 py-1 text-center text-[9px] font-bold tracking-wider uppercase ${subtypePillClass}`}
    >
      {translate(subtypeKey, $locale)}
    </span>
    <div class="min-w-0">
      <div class="mb-1 font-mono text-[11px] text-text-muted">
        {sourceLabel}
      </div>
      <div class="mb-1 text-[13px] leading-snug text-text">
        {displaySummary}
      </div>
      <div class="flex flex-wrap gap-x-3 gap-y-1 text-[11px] text-text-muted">
        <span>{formatRelativeTime(item.createdAt, $locale)}</span>
        {#each approvalContextItems as contextItem (contextItem.key)}
          <span>{contextItem.label}</span>
        {/each}
      </div>
    </div>
    <div class="flex gap-1.5">
      {@render actions()}
    </div>
  </div>
  {#if footer}
    <div class="mt-3 border-t border-border pt-3">
      {@render footer()}
    </div>
  {/if}
</div>
