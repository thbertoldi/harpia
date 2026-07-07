<script lang="ts">
  import ThreadMessage from "$lib/components/thread/ThreadMessage.svelte";
  import type { ChatMessage } from "$lib/chat/types";
  import type { StepTitleResolver } from "$lib/chat/event-text";
  import type { Artifact } from "$lib/gen/harpia/artifacts/v1/artifacts_pb";
  import type { PlanConfiguration } from "$lib/gen/harpia/plans/v1/plans_pb";

  let {
    tenantId,
    configurationId,
    messages,
    artifacts,
    onOpenArtifact,
    /**
     * Per-message lookup: returns the existing PlanConfiguration for a
     * PLAN_PROPOSED message (driving read-only mode), or undefined. Computed
     * in +page.svelte against data.configurations so the page is the single
     * source of truth for the match.
     */
    existingConfigurationFor,
    /**
     * Resolves a step_key to its human template title for STEP_STARTED /
     * STEP_BOUND system events. Optional; defaults to the raw key inside
     * SystemEventCard.
     */
    stepTitleFor,
    /**
     * Forwarded to ThreadMessage so an in-thread approval decision can
     * trigger a data refresh on the chat page.
     */
    onApprovalDecided,
  }: {
    tenantId: string;
    configurationId: string;
    messages: ChatMessage[];
    artifacts: Artifact[];
    onOpenArtifact: (artifactId: string) => void;
    existingConfigurationFor?: (
      message: ChatMessage,
    ) => PlanConfiguration | undefined;
    stepTitleFor?: StepTitleResolver;
    onApprovalDecided?: () => void;
  } = $props();

  function isSelectionAnswerFor(
    message: ChatMessage,
    promptId: string,
  ): boolean {
    if (message.kind !== "USER_SELECTION") return false;
    try {
      return (
        JSON.parse(message.payloadJson)?.in_response_to_message_id === promptId
      );
    } catch {
      return false;
    }
  }

  function isLastAssistantPrompt(message: ChatMessage): boolean {
    if (message.kind !== "ASSISTANT_PROMPT") return false;
    const index = messages.findIndex(
      (candidate) => candidate.id === message.id,
    );
    if (index < 0) return false;
    return !messages
      .slice(index + 1)
      .some((candidate) => isSelectionAnswerFor(candidate, message.id));
  }
</script>

<div class="w-full">
  <main class="min-w-0 space-y-4">
    <div class="flex flex-col gap-2">
      {#each messages as message (message.id)}
        {@const livePrompt = isLastAssistantPrompt(message)}
        <ThreadMessage
          {message}
          {tenantId}
          {configurationId}
          {artifacts}
          {onOpenArtifact}
          {messages}
          isLive={livePrompt}
          isAnswered={message.kind === "ASSISTANT_PROMPT" && !livePrompt}
          existingConfiguration={existingConfigurationFor?.(message)}
          {stepTitleFor}
          {onApprovalDecided}
        />
      {/each}
    </div>
  </main>
</div>
