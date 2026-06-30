<script lang="ts">
  import { AlertTriangle, Files, Loader2, RefreshCw } from "lucide-svelte";
  import { getTenant } from "$lib/auth";
  import { listArtifacts } from "$lib/artifacts/artifacts";
  import ArtifactCard from "$lib/components/artifacts/ArtifactCard.svelte";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import { locale, translate } from "$lib/i18n";
  import type { Artifact } from "$lib/gen/harpia/artifacts/v1/artifacts_pb";

  let tenantId = $state("");
  let artifacts = $state<Artifact[]>([]);
  let loading = $state(true);
  let loadError = $state<string | null>(null);

  $effect(() => {
    tenantId = getTenant()?.id ?? "";
    void loadArtifacts();
  });

  async function loadArtifacts() {
    if (!tenantId) {
      loading = false;
      loadError = translate("artifacts.loadError", $locale);
      return;
    }
    loading = true;
    loadError = null;
    try {
      artifacts = await listArtifacts({ tenantId });
    } catch (err) {
      loadError =
        err instanceof Error
          ? err.message
          : translate("artifacts.loadError", $locale);
    } finally {
      loading = false;
    }
  }
</script>

<svelte:head>
  <title>{translate("artifacts.library.heading", $locale)} · Harpia</title>
</svelte:head>

<div class="px-4 py-6 lg:px-6">
  <div class="mb-6 flex items-start justify-between gap-4">
    <div>
      <HarpyHeading tag="h1" class="text-2xl text-cream">
        {translate("artifacts.library.heading", $locale)}
      </HarpyHeading>
      <p class="mt-1 max-w-2xl text-sm text-crown-ash">
        {translate("artifacts.library.subheading", $locale)}
      </p>
    </div>
    <button
      type="button"
      onclick={() => loadArtifacts()}
      class="inline-flex size-9 items-center justify-center rounded-md border border-plumage text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
      aria-label={translate("common.retry", $locale)}
      title={translate("common.retry", $locale)}
    >
      <RefreshCw class="size-4" />
    </button>
  </div>

  {#if loading}
    <div class="flex min-h-[40vh] items-center justify-center">
      <div class="flex items-center gap-2 text-crown-ash">
        <Loader2 class="size-5 animate-spin" />
        <span class="text-sm">{translate("artifacts.loading", $locale)}</span>
      </div>
    </div>
  {:else if loadError && artifacts.length === 0}
    <div
      class="flex min-h-[40vh] flex-col items-center justify-center rounded-lg border border-dashed border-plumage bg-obsidian-light/30 px-6 py-12 text-center"
    >
      <AlertTriangle class="mb-4 size-12 text-red-400" />
      <HarpyHeading tag="h2" class="mb-2 text-xl text-cream">
        {translate("artifacts.loadError", $locale)}
      </HarpyHeading>
      <p class="max-w-md font-mono text-xs text-crown-ash">{loadError}</p>
    </div>
  {:else if artifacts.length === 0}
    <div
      class="flex min-h-[40vh] flex-col items-center justify-center rounded-lg border border-dashed border-plumage bg-obsidian-light/30 px-6 py-12 text-center"
    >
      <Files class="mb-4 size-12 text-talon-gold" />
      <HarpyHeading tag="h2" class="mb-2 text-xl text-cream">
        {translate("artifacts.emptyTitle", $locale)}
      </HarpyHeading>
      <p class="max-w-md text-sm text-crown-ash">
        {translate("artifacts.emptyDescription", $locale)}
      </p>
    </div>
  {:else}
    <div class="grid gap-3 lg:grid-cols-2 2xl:grid-cols-3">
      {#each artifacts as artifact (artifact.id)}
        <ArtifactCard {tenantId} {artifact} />
      {/each}
    </div>
  {/if}
</div>
