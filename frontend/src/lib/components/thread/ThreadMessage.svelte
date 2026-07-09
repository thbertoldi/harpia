<script lang="ts">
  import type { ChatMessage } from "$lib/chat/types";
  import type { Artifact } from "$lib/gen/harpia/artifacts/v1/artifacts_pb";
  import type { PlanConfiguration } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { locale, translate } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";
  import type { StepTitleResolver } from "$lib/chat/event-text";
  import { parseAssistantPromptState } from "$lib/plans/configuration-flow";
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
    /**
     * Matching configuration for a PLAN_PROPOSED message, if one already
     * exists. Switches PlanProposalCard into read-only summary mode.
     */
    existingConfiguration?: PlanConfiguration;
    /**
     * Resolves a step_key to its human template title for STEP_STARTED /
     * STEP_BOUND system events. Optional; defaults to the raw key.
     */
    stepTitleFor?: StepTitleResolver;
    /**
     * Full thread message stream. Forwarded to ApprovalRefCard so an
     * APPROVAL_RAISED card can detect a matching APPROVAL_DECIDED that
     * arrived later and flip to its terminal state.
     */
    messages?: ChatMessage[];
    /**
     * Invoked after an in-thread approval decision succeeds. The chat page
     * uses this to invalidate load data so downstream surfaces refresh.
     */
    onApprovalDecided?: () => void;
    /** Id of the artifact currently shown in the preview panel, if open. */
    activeArtifactId?: string | null;
    /** Id of the artifact currently being produced by a running step. */
    generatingArtifactId?: string | null;
    iterationNumberFor?: (artifact: Artifact) => number | null;
  }
  let {
    message,
    configurationId = "",
    tenantId = "",
    artifacts = [],
    onOpenArtifact,
    isLive = false,
    isAnswered = false,
    existingConfiguration,
    stepTitleFor,
    messages = [],
    onApprovalDecided,
    activeArtifactId = null,
    generatingArtifactId = null,
    iterationNumberFor,
  }: Props = $props();

  const promptState = $derived(
    message.kind === "ASSISTANT_PROMPT"
      ? parseAssistantPromptState(message.payloadJson)
      : null,
  );

  const policySelectionKeys: Record<string, string> = {
    require_approval:
      "assistant.policiesStep.option.publish_approval_mode.require_approval",
    auto_publish:
      "assistant.policiesStep.option.publish_approval_mode.auto_publish",
    pause_until_answered:
      "assistant.policiesStep.option.elicitation_timeout_behavior.pause_until_answered",
    fail_step:
      "assistant.policiesStep.option.elicitation_timeout_behavior.fail_step",
    fail_plan:
      "assistant.policiesStep.option.elicitation_timeout_behavior.fail_plan",
  };

  function userSelectionLabel(): string {
    try {
      const payload = JSON.parse(message.payloadJson || "{}") as {
        value?: string;
        option_id?: string;
      };
      if (
        payload.value === "save_runnable" ||
        payload.option_id === "save-runnable"
      ) {
        return translate("assistant.bindingMatrix.savePrimary", $locale);
      }
      if (
        payload.value === "save_draft" ||
        payload.option_id === "save-draft"
      ) {
        return translate("assistant.bindingMatrix.saveSecondary", $locale);
      }
      const policyKey =
        (payload.value ? policySelectionKeys[payload.value] : undefined) ??
        (payload.option_id
          ? policySelectionKeys[payload.option_id]
          : undefined);
      if (policyKey) {
        return translate(policyKey, $locale);
      }
      return message.text || payload.value || "—";
    } catch {
      return message.text || "—";
    }
  }
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
    {userSelectionLabel()}
  </div>
{:else if message.kind === "PLAN_PROPOSED"}
  <PlanProposalCard
    {message}
    {tenantId}
    threadId={message.threadId}
    {existingConfiguration}
  />
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
  <ApprovalRefCard
    {message}
    {tenantId}
    {messages}
    {onOpenArtifact}
    {stepTitleFor}
    onDecided={onApprovalDecided}
  />
{:else}
  <SystemEventCard
    {message}
    {tenantId}
    {artifacts}
    {onOpenArtifact}
    {stepTitleFor}
    {activeArtifactId}
    {generatingArtifactId}
    {iterationNumberFor}
  />
{/if}
