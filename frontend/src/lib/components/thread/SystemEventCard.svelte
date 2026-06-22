<script lang="ts">
  import { Activity, Check, AlertTriangle, Play, Save } from "lucide-svelte";
  import type { ChatMessage } from "$lib/chat/types";
  import { locale, translate } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";

  interface Props {
    message: ChatMessage;
  }
  let { message }: Props = $props();

  const iconFor = $derived(
    message.kind === "RUN_STARTED"
      ? Play
      : message.kind === "RUN_COMPLETED"
        ? Check
        : message.kind === "RUN_FAILED"
          ? AlertTriangle
          : message.kind === "STEP_BOUND"
            ? Activity
            : Save,
  );

  const labelKey = $derived(`thread.event.${message.kind.toLowerCase()}`);
</script>

<div
  id={`m-${message.id}`}
  class="flex items-center gap-3 rounded-md border border-plumage/60 bg-obsidian-light px-3 py-2 text-[12px] text-crown-ash"
>
  <svelte:component this={iconFor} class="size-4 text-talon-gold" />
  <span class="flex-1 text-cream">{message.text}</span>
  <span class="text-[10px] text-crown-ash-dark">
    {formatRelativeTime(message.createdAt, $locale)}
  </span>
</div>
