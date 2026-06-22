<script lang="ts">
  import { Activity, Check, AlertTriangle, Play, Save, Repeat, Calendar, Sparkles } from "lucide-svelte";
  import type { ChatMessage } from "$lib/chat/types";
  import { locale } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";

  interface Props {
    message: ChatMessage;
  }
  let { message }: Props = $props();

  const Icon = $derived(
    message.kind === "RUN_STARTED"
      ? Play
      : message.kind === "RUN_COMPLETED"
        ? Check
        : message.kind === "RUN_FAILED"
          ? AlertTriangle
          : message.kind === "STEP_BOUND" || message.kind === "STEP_STARTED"
            ? Activity
            : message.kind === "STEP_REBOUND"
              ? Repeat
              : message.kind === "SCHEDULE_SET"
                ? Calendar
                : message.kind === "CONFIGURATION_STARTED"
                  ? Sparkles
                  : Save,
  );
</script>

<div
  id={`m-${message.id}`}
  class="flex items-center gap-3 rounded-md border border-plumage/60 bg-obsidian-light px-3 py-2 text-[12px] text-crown-ash"
>
  <Icon class="size-4 text-talon-gold" />
  <span class="flex-1 text-cream">{message.text}</span>
  <span class="text-[10px] text-crown-ash-dark">
    {formatRelativeTime(message.createdAt, $locale)}
  </span>
</div>
