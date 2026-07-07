<script lang="ts">
  import { Eye, Loader2 } from "lucide-svelte";
  import ArtifactPreview from "$lib/components/ArtifactPreview.svelte";
  import { locale, translate } from "$lib/i18n";
  import {
    artifactCardOpenActionKey,
    artifactCardStatusLabelKey,
    artifactTitle,
  } from "$lib/artifacts/text";
  import type { Artifact } from "$lib/gen/harpia/artifacts/v1/artifacts_pb";

  let {
    tenantId,
    artifact,
    compact = false,
    active = false,
    generating = false,
    iterationNumber = null,
    onOpen,
  }: {
    tenantId: string;
    artifact: Artifact;
    compact?: boolean;
    /** True while this artifact is the one currently shown in the preview panel. */
    active?: boolean;
    /** True while this artifact is currently being produced by a running step. */
    generating?: boolean;
    iterationNumber?: number | null;
    onOpen?: (artifactId: string) => void;
  } = $props();

  const title = $derived(artifactTitle(artifact));
</script>

<article
  class="w-full rounded-lg border p-3 transition-colors {active
    ? 'border-talon-gold bg-talon-gold/5'
    : 'border-plumage bg-obsidian-light/30'}"
>
  <div class="mb-2 flex items-start justify-between gap-2">
    <div class="min-w-0">
      <div class="flex items-center gap-1.5">
        <h3 class="truncate font-heading text-sm font-semibold text-cream">
          {title}
        </h3>
        {#if iterationNumber}
          <span
            class="shrink-0 rounded-full bg-plumage/60 px-1.5 py-px text-[10px] font-medium text-crown-ash tabular-nums"
          >
            {translate("artifacts.iteration.badge", $locale, {
              n: String(iterationNumber),
            })}
          </span>
        {/if}
      </div>
      <p class="mt-0.5 truncate font-mono text-[10px] text-crown-ash-dark">
        {translate(
          artifactCardStatusLabelKey(artifact.status, generating),
          $locale,
        )}
      </p>
    </div>
    <div class="flex gap-1">
      {#if generating}
        <span
          class="flex size-8 shrink-0 items-center justify-center text-energy"
          aria-label={translate("artifacts.status.generating", $locale)}
        >
          <Loader2 class="size-4 animate-spin" />
        </span>
      {:else if onOpen}
        <button
          type="button"
          class="inline-flex size-8 shrink-0 items-center justify-center rounded-md border transition-colors
            {active
            ? 'border-talon-gold text-talon-gold'
            : 'border-plumage text-crown-ash hover:border-talon-gold hover:text-talon-gold'}"
          aria-label={translate(artifactCardOpenActionKey(active), $locale)}
          title={translate(artifactCardOpenActionKey(active), $locale)}
          onclick={() => onOpen(artifact.id)}
        >
          <Eye class="size-4" />
        </button>
      {/if}
    </div>
  </div>
  {#if !compact}
    <ArtifactPreview {tenantId} artifactId={artifact.id} />
  {/if}
</article>
