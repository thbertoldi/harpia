<script lang="ts">
  import {
    Activity,
    Check,
    AlertTriangle,
    Play,
    Save,
    Repeat,
    Calendar,
    Sparkles,
  } from "lucide-svelte";
  import type { ChatMessage } from "$lib/chat/types";
  import type { Artifact } from "$lib/gen/harpia/artifacts/v1/artifacts_pb";
  import { locale } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";
  import { parseOutputArtifactId } from "$lib/plans/execution-view";
  import ArtifactCard from "$lib/components/artifacts/ArtifactCard.svelte";

  interface Props {
    message: ChatMessage;
    tenantId?: string;
    artifacts?: Artifact[];
    onOpenArtifact?: (artifactId: string) => void;
  }
  let {
    message,
    tenantId = "",
    artifacts = [],
    onOpenArtifact,
  }: Props = $props();

  const resolvedArtifact = $derived.by((): Artifact | null => {
    if (message.kind !== "STEP_BOUND") return null;
    const artifactId = parseOutputArtifactId(message.payloadJson);
    if (!artifactId) return null;
    return artifacts.find((a) => a.id === artifactId) ?? null;
  });

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

{#if resolvedArtifact}
  <div id={`m-${message.id}`}>
    <ArtifactCard
      {tenantId}
      artifact={resolvedArtifact}
      compact
      onOpen={onOpenArtifact}
    />
  </div>
{:else}
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
{/if}
