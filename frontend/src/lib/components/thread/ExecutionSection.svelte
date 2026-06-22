<script lang="ts">
  import { ChevronDown, ChevronRight } from "lucide-svelte";
  import type { ExecutionGroup } from "$lib/plans/thread";
  import { locale, translate } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";
  import ThreadMessage from "./ThreadMessage.svelte";

  interface Props {
    group: ExecutionGroup;
    defaultExpanded?: boolean;
  }
  let { group, defaultExpanded = false }: Props = $props();

  let expanded = $state(defaultExpanded);

  const statusKey = $derived(`thread.execution.status.${group.status}`);
  const startedAt = $derived(group.messages[0]?.createdAt ?? "");
</script>

<section class="rounded-md border border-plumage bg-obsidian-light">
  <button
    type="button"
    onclick={() => (expanded = !expanded)}
    class="flex w-full items-center gap-3 rounded-md px-3 py-2 text-left text-[12px] hover:bg-obsidian-light/70"
  >
    {#if expanded}
      <ChevronDown class="size-4 text-talon-gold" />
    {:else}
      <ChevronRight class="size-4 text-crown-ash-dark" />
    {/if}
    <span class="flex-1 font-heading font-semibold text-cream">
      {translate("thread.execution.runLabel", $locale).replace(
        "{n}",
        String(group.runNumber),
      )}
      <span class="text-crown-ash">· {translate(statusKey, $locale)}</span>
    </span>
    <span class="text-[10px] text-crown-ash-dark">
      {formatRelativeTime(startedAt, $locale)}
    </span>
  </button>
  {#if expanded}
    <div class="flex flex-col gap-2 border-t border-plumage/50 px-3 py-3">
      {#each group.messages as message (message.id)}
        <ThreadMessage {message} />
      {/each}
    </div>
  {/if}
</section>
