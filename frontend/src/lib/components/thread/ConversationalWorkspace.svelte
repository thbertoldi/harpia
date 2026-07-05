<script lang="ts">
  import ArtifactRail from "$lib/components/artifacts/ArtifactRail.svelte";
  import PlanActivityTimeline from "$lib/components/thread/PlanActivityTimeline.svelte";
  import ThreadMessage from "$lib/components/thread/ThreadMessage.svelte";
  import type { ChatMessage } from "$lib/chat/types";
  import type { StepTitleResolver } from "$lib/chat/event-text";
  import type { Artifact } from "$lib/gen/harpia/artifacts/v1/artifacts_pb";
  import type { PlanConfiguration } from "$lib/gen/harpia/plans/v1/plans_pb";
  import type { PlanActivityItem } from "$lib/plans/activity";

  let {
    tenantId,
    configurationId,
    messages,
    activityItems,
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
  }: {
    tenantId: string;
    configurationId: string;
    messages: ChatMessage[];
    activityItems: PlanActivityItem[];
    artifacts: Artifact[];
    onOpenArtifact: (artifactId: string) => void;
    existingConfigurationFor?: (
      message: ChatMessage,
    ) => PlanConfiguration | undefined;
    stepTitleFor?: StepTitleResolver;
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

<div class="grid gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
  <main class="min-w-0 space-y-4">
    {#if activityItems.length > 0}
      <PlanActivityTimeline items={activityItems} {onOpenArtifact} />
    {/if}
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
        />
      {/each}
    </div>
  </main>
  <ArtifactRail {tenantId} {artifacts} {onOpenArtifact} />
</div>
