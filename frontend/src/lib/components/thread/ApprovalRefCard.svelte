<script lang="ts">
  import { ShieldCheck, ShieldX, ShieldQuestion } from "lucide-svelte";
  import type { ChatMessage } from "$lib/chat/types";
  import { locale, translate } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";

  interface Props {
    message: ChatMessage;
  }
  let { message }: Props = $props();

  const decided = $derived(message.kind === "APPROVAL_DECIDED");

  const payloadApproved = $derived.by(() => {
    if (!decided) return null;
    try {
      const parsed = JSON.parse(message.payloadJson);
      return typeof parsed.approved === "boolean" ? parsed.approved : null;
    } catch {
      return null;
    }
  });

  const Icon = $derived(
    !decided
      ? ShieldQuestion
      : payloadApproved === true
        ? ShieldCheck
        : ShieldX,
  );

  // Generic ref text — the backend payload lacks a step_key today, so we
  // surface a fixed localized label driven by the decision outcome. A pending
  // request or an unrecognized decision falls back to the generic raised key.
  const labelText = $derived.by(() => {
    if (!decided) return translate("thread.approval.raised", $locale);
    if (payloadApproved === true)
      return translate("thread.approval.granted", $locale);
    if (payloadApproved === false)
      return translate("thread.approval.rejected", $locale);
    return translate("thread.approval.raised", $locale);
  });
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
