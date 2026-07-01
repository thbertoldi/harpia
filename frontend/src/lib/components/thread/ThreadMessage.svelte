<script lang="ts">
  import type { ChatMessage } from "$lib/chat/types";
  import { locale } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";
  import SystemEventCard from "./SystemEventCard.svelte";
  import ElicitationRefCard from "./ElicitationRefCard.svelte";
  import ApprovalRefCard from "./ApprovalRefCard.svelte";
  import AssistantPromptCard from "./AssistantPromptCard.svelte";
  import BindingMatrixCard from "./BindingMatrixCard.svelte";
  import ConversationalBindingCard from "./ConversationalBindingCard.svelte";
  import LandingCard from "./LandingCard.svelte";

  interface Props {
    message: ChatMessage;
    configurationId?: string;
    tenantId?: string;
    isLive?: boolean;
    isAnswered?: boolean;
  }
  let {
    message,
    configurationId = "",
    tenantId = "",
    isLive = false,
    isAnswered = false,
  }: Props = $props();

  const promptState = $derived.by<string | null>(() => {
    if (message.kind !== "ASSISTANT_PROMPT") return null;
    try {
      const s = JSON.parse(message.payloadJson)?.state;
      return typeof s === "string" ? s : null;
    } catch {
      return null;
    }
  });
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
{:else if message.kind === "USER_SELECTION"}
  <div
    id={`m-${message.id}`}
    class="max-w-[85%] self-end rounded-md border border-plumage/60 bg-obsidian px-3 py-1 text-[11px] text-crown-ash"
  >
    → {message.text || JSON.parse(message.payloadJson || "{}").value || "—"}
  </div>
{:else if message.kind === "ASSISTANT_PROMPT" && promptState === "BINDING_MATRIX"}
  <BindingMatrixCard {message} {configurationId} {tenantId} />
{:else if message.kind === "ASSISTANT_PROMPT" && promptState === "BINDING_STEP"}
  <ConversationalBindingCard
    {message}
    {configurationId}
    {tenantId}
    {isAnswered}
    {isLive}
  />
{:else if message.kind === "ASSISTANT_PROMPT" && promptState === "landing"}
  <LandingCard {message} {configurationId} {tenantId} />
{:else if message.kind === "ASSISTANT_PROMPT"}
  <AssistantPromptCard
    {message}
    {configurationId}
    {tenantId}
    {isAnswered}
    {isLive}
  />
{:else if message.kind === "ELICITATION_RAISED" || message.kind === "ELICITATION_ANSWERED"}
  <ElicitationRefCard {message} />
{:else if message.kind === "APPROVAL_RAISED" || message.kind === "APPROVAL_DECIDED"}
  <ApprovalRefCard {message} />
{:else}
  <SystemEventCard {message} />
{/if}
