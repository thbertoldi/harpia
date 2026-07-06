<script lang="ts">
  import { Eye } from "lucide-svelte";
  import ArtifactPreview from "$lib/components/ArtifactPreview.svelte";
  import { locale, translate } from "$lib/i18n";
  import { artifactStatusLabel, artifactTitle } from "$lib/artifacts/text";
  import type { Artifact } from "$lib/gen/harpia/artifacts/v1/artifacts_pb";

  let {
    tenantId,
    artifact,
    compact = false,
    onOpen,
  }: {
    tenantId: string;
    artifact: Artifact;
    compact?: boolean;
    onOpen?: (artifactId: string) => void;
  } = $props();

  const title = $derived(artifactTitle(artifact));
</script>

<article
  class="w-full rounded-lg border border-plumage bg-obsidian-light/30 p-3"
>
  <div class="mb-2 flex items-start justify-between gap-2">
    <div class="min-w-0">
      <h3 class="truncate font-heading text-sm font-semibold text-cream">
        {title}
      </h3>
      <p class="mt-0.5 truncate font-mono text-[10px] text-crown-ash-dark">
        {artifactStatusLabel(artifact.status)}
      </p>
    </div>
    <div class="flex gap-1">
      {#if onOpen}
        <button
          type="button"
          class="inline-flex size-8 items-center justify-center rounded-md border border-plumage text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
          aria-label={translate("artifacts.actions.open", $locale)}
          title={translate("artifacts.actions.open", $locale)}
          onclick={() => onOpen(artifact.id)}
        >
          <Eye class="size-4" />
        </button>
      {/if}
    </div>
  </div>
  <p class="mb-2 truncate font-mono text-[10px] text-crown-ash-dark">
    {artifact.id}
  </p>
  {#if !compact}
    <ArtifactPreview {tenantId} artifactId={artifact.id} />
  {/if}
</article>
