<script lang="ts">
  import { Bot, Clock, DollarSign } from "lucide-svelte";
  import type { Task } from "$lib/rpc";
  import {
    formatElapsed,
    formatEstimatedCost,
    getAgentLabel,
    getCurrentSubtask,
  } from "$lib/tasks/ongoing-tasks";
  import { locale, translate } from "$lib/i18n";

  let {
    task,
    onclick,
  }: {
    task: Task;
    onclick: () => void;
  } = $props();

  const currentSubtask = $derived(getCurrentSubtask(task));
  const agentLabel = $derived(getAgentLabel(task, currentSubtask, $locale));
</script>

<button
  type="button"
  {onclick}
  class="w-full rounded-lg border border-plumage bg-obsidian-light/60 p-4 text-left transition-all duration-200 hover:border-talon-gold/50 hover:bg-obsidian-light"
>
  <p class="truncate font-heading text-base font-semibold text-cream">
    {task.title}
  </p>

  {#if currentSubtask}
    <p class="mt-1 line-clamp-2 font-body text-sm text-crown-ash">
      {currentSubtask.description}
    </p>
  {:else}
    <p class="mt-1 font-body text-sm text-crown-ash-dark italic">
      {translate("ongoing.noActiveSubtask", $locale)}
    </p>
  {/if}

  <div
    class="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 font-mono text-[10px] tracking-wider text-crown-ash uppercase"
  >
    {#if agentLabel}
      <span class="inline-flex items-center gap-1">
        <Bot class="size-3" />
        {agentLabel.slice(0, 16)}{agentLabel.length > 16 ? "…" : ""}
      </span>
    {/if}
    <span class="inline-flex items-center gap-1">
      <Clock class="size-3" />
      {formatElapsed(task.createdAt, $locale)}
    </span>
    <span class="inline-flex items-center gap-1">
      <DollarSign class="size-3" />
      {formatEstimatedCost($locale)}
    </span>
  </div>
</button>
