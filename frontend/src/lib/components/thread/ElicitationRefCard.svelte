<script lang="ts">
  import { MessageSquare, CheckCircle2 } from "lucide-svelte";
  import type { ChatMessage } from "$lib/chat/types";
  import { locale, translate } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";

  interface Props {
    message: ChatMessage;
  }
  let { message }: Props = $props();

  const isAnswered = $derived(message.kind === "ELICITATION_ANSWERED");

  const Icon = $derived(isAnswered ? CheckCircle2 : MessageSquare);

  // Generic ref text — the backend payload lacks a step_key today, so we
  // surface a fixed localized label instead of the English `text`.
  const labelText = $derived(
    isAnswered
      ? translate("thread.elicitation.answered", $locale)
      : translate("thread.elicitation.raised", $locale),
  );
</script>

<div
  id={`m-${message.id}`}
  class="flex items-center gap-3 rounded-md border border-talon-gold/40 bg-talon-gold/10 px-3 py-2 text-[12px]"
>
  <Icon class="size-4 text-talon-gold" />
  <span class="flex-1 text-cream">{labelText}</span>
  <span class="text-[10px] text-crown-ash-dark">
    {formatRelativeTime(message.createdAt, $locale)}
  </span>
</div>
