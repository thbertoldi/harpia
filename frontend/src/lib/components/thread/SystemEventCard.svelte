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
  import { locale, translate } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";
  import { eventTextKey, type StepTitleResolver } from "$lib/chat/event-text";
  import { parseOutputArtifactId } from "$lib/plans/execution-view";
  import ArtifactCard from "$lib/components/artifacts/ArtifactCard.svelte";

  interface Props {
    message: ChatMessage;
    tenantId?: string;
    artifacts?: Artifact[];
    onOpenArtifact?: (artifactId: string) => void;
    /**
     * Resolves a step_key to its human template title. Defaults to the raw
     * key; the page wires in the real template lookup so STEP_STARTED /
     * STEP_BOUND events render the step's title instead of its id.
     */
    stepTitleFor?: StepTitleResolver;
  }
  let {
    message,
    tenantId = "",
    artifacts = [],
    onOpenArtifact,
    stepTitleFor = (key: string) => key,
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

  // Resolve the localized event text from the message kind + payload. Kinds
  // without a canonical key (STEP_REBOUND echoes a dynamic user-selection
  // label; unknown kinds) fall back to the backend's English `text`.
  const eventText = $derived.by(() => {
    const resolved = eventTextKey(message, stepTitleFor);
    if (!resolved) return message.text;
    return translate(resolved.key, $locale, resolved.params);
  });
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
    <span class="flex-1 text-cream">{eventText}</span>
    <span class="text-[10px] text-crown-ash-dark">
      {formatRelativeTime(message.createdAt, $locale)}
    </span>
  </div>
{/if}
