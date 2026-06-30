<script lang="ts">
  import ArtifactCard from "$lib/components/artifacts/ArtifactCard.svelte";
  import { locale, translate } from "$lib/i18n";
  import type { Artifact } from "$lib/gen/harpia/artifacts/v1/artifacts_pb";

  let {
    tenantId,
    artifacts,
  }: {
    tenantId: string;
    artifacts: Artifact[];
  } = $props();
</script>

<aside class="sticky top-4 flex max-h-[calc(100vh-2rem)] flex-col gap-3 overflow-y-auto">
  <div>
    <h2 class="font-heading text-base font-semibold text-cream">
      {translate("artifacts.rail.heading", $locale)}
    </h2>
    <p class="mt-1 text-[12px] text-crown-ash">
      {translate("artifacts.rail.subheading", $locale)}
    </p>
  </div>

  {#if artifacts.length === 0}
    <p
      class="rounded-lg border border-dashed border-plumage bg-obsidian-light/20 px-3 py-4 text-[12px] text-crown-ash"
    >
      {translate("artifacts.empty", $locale)}
    </p>
  {:else}
    <div class="flex flex-col gap-2">
      {#each artifacts as artifact (artifact.id)}
        <ArtifactCard {tenantId} {artifact} compact />
      {/each}
    </div>
  {/if}
</aside>
