<script lang="ts">
  import ArtifactDetailPanel from "$lib/components/artifacts/ArtifactDetailPanel.svelte";
  import ArtifactRail from "$lib/components/artifacts/ArtifactRail.svelte";
  import PlanActivityTimeline from "$lib/components/thread/PlanActivityTimeline.svelte";
  import ThreadMessage from "$lib/components/thread/ThreadMessage.svelte";
  import type { ChatMessage } from "$lib/chat/types";
  import type { Artifact } from "$lib/gen/harpia/artifacts/v1/artifacts_pb";
  import type { PlanActivityItem } from "$lib/plans/activity";

  let {
    tenantId,
    configurationId,
    messages,
    activityItems,
    artifacts,
  }: {
    tenantId: string;
    configurationId: string;
    messages: ChatMessage[];
    activityItems: PlanActivityItem[];
    artifacts: Artifact[];
  } = $props();

  let selectedPreviewArtifactId = $state<string | null>(null);

  const selectedArtifact = $derived(
    selectedPreviewArtifactId
      ? (artifacts.find((artifact) => artifact.id === selectedPreviewArtifactId) ??
        null)
      : null,
  );

  $effect(() => {
    if (selectedPreviewArtifactId && !selectedArtifact) {
      selectedPreviewArtifactId = null;
    }
  });

  function openArtifact(artifactId: string) {
    selectedPreviewArtifactId = artifactId;
  }

  function closeArtifactPreview() {
    selectedPreviewArtifactId = null;
  }

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
      <PlanActivityTimeline
        items={activityItems}
        onOpenArtifact={openArtifact}
      />
    {/if}
    <div class="flex flex-col gap-2">
      {#each messages as message (message.id)}
        {@const livePrompt = isLastAssistantPrompt(message)}
        <ThreadMessage
          {message}
          {tenantId}
          {configurationId}
          isLive={livePrompt}
          isAnswered={message.kind === "ASSISTANT_PROMPT" && !livePrompt}
        />
      {/each}
    </div>
  </main>
  {#if selectedArtifact}
    <ArtifactDetailPanel
      {tenantId}
      artifact={selectedArtifact}
      onClose={closeArtifactPreview}
    />
  {:else}
    <ArtifactRail {tenantId} {artifacts} onOpenArtifact={openArtifact} />
  {/if}
</div>
