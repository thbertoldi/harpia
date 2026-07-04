<script lang="ts">
  import type { ChatMessage } from "$lib/chat/types";
  import type { Artifact } from "$lib/gen/harpia/artifacts/v1/artifacts_pb";
  import { locale } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";
  import SystemEventCard from "./SystemEventCard.svelte";
  import ElicitationRefCard from "./ElicitationRefCard.svelte";
  import ApprovalRefCard from "./ApprovalRefCard.svelte";
  import AssistantPromptCard from "./AssistantPromptCard.svelte";
  import BindingMatrixCard from "./BindingMatrixCard.svelte";
  import ConversationalBindingCard from "./ConversationalBindingCard.svelte";
  import ConversationalOverseerCard from "./ConversationalOverseerCard.svelte";
  import ConversationalPoliciesCard from "./ConversationalPoliciesCard.svelte";
  import LandingCard from "./LandingCard.svelte";
  import PlanProposalCard from "./PlanProposalCard.svelte";

  interface Props {
    message: ChatMessage;
    configurationId?: string;
    tenantId?: string;
    artifacts?: Artifact[];
    onOpenArtifact?: (artifactId: string) => void;
    isLive?: boolean;
    isAnswered?: boolean;
  }
  let {
    message,
    configurationId = "",
    tenantId = "",
    artifacts = [],
    onOpenArtifact,
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
    class="max-w-[85%] self-end rounded-2xl rounded-tr-sm border border-talon-gold/40 bg-talon-gold/10 px-3 py-2"
  >
    <p class="text-[13px] whitespace-pre-wrap text-cream">{message.text}</p>
    <p class="mt-1 text-right text-[10px] text-crown-ash-dark">
      {formatRelativeTime(message.createdAt, $locale)}
    </p>
  </div>
{:else if message.kind === "USER_SELECTION"}
  <div
    id={`m-${message.id}`}
    class="max-w-[85%] self-end rounded-2xl rounded-tr-sm border border-talon-gold/40 bg-talon-gold/10 px-3 py-1.5 text-[11px] font-medium text-cream"
  >
    {message.text || JSON.parse(message.payloadJson || "{}").value || "—"}
  </div>
{:else if message.kind === "PLAN_PROPOSED"}
  <PlanProposalCard {message} {tenantId} threadId={message.threadId} />
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
{:else if message.kind === "ASSISTANT_PROMPT" && promptState === "OVERSEER_STEP"}
  <ConversationalOverseerCard
    {message}
    {configurationId}
    {tenantId}
    {isAnswered}
    {isLive}
  />
{:else if message.kind === "ASSISTANT_PROMPT" && promptState === "POLICIES_STEP"}
  <ConversationalPoliciesCard
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
  <SystemEventCard {message} {tenantId} {artifacts} {onOpenArtifact} />
{/if}
