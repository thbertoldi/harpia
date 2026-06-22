<script lang="ts">
  import type { ChatMessage } from "$lib/chat/types";
  import { locale } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";
  import SystemEventCard from "./SystemEventCard.svelte";
  import ElicitationRefCard from "./ElicitationRefCard.svelte";
  import ApprovalRefCard from "./ApprovalRefCard.svelte";

  interface Props {
    message: ChatMessage;
  }
  let { message }: Props = $props();
</script>

{#if message.kind === "USER_TEXT"}
  <div
    id={`m-${message.id}`}
    class="max-w-[85%] self-end rounded-lg border border-plumage bg-obsidian-light px-3 py-2"
  >
    <p class="text-[13px] whitespace-pre-wrap text-cream">{message.text}</p>
    <p class="mt-1 text-right text-[10px] text-crown-ash-dark">
      {formatRelativeTime(message.createdAt, $locale)}
    </p>
  </div>
{:else if message.kind === "ELICITATION_RAISED" || message.kind === "ELICITATION_ANSWERED"}
  <ElicitationRefCard {message} />
{:else if message.kind === "APPROVAL_RAISED" || message.kind === "APPROVAL_DECIDED"}
  <ApprovalRefCard {message} />
{:else}
  <SystemEventCard {message} />
{/if}
