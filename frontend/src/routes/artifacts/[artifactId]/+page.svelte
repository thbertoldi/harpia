<script lang="ts">
  import { invalidateAll } from "$app/navigation";
  import { resolve } from "$app/paths";
  import { page } from "$app/state";
  import { ArrowLeft, CheckCircle2, History } from "lucide-svelte";
  import { artifactStatusLabel, artifactTitle } from "$lib/artifacts/text";
  import ArtifactPreview from "$lib/components/ArtifactPreview.svelte";
  import TextArtifactEditor from "$lib/components/artifacts/TextArtifactEditor.svelte";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import { locale, translate } from "$lib/i18n";
  import { formatLocaleDateTime } from "$lib/i18n/format";

  let { data } = $props();

  const title = $derived(artifactTitle(data.artifact));
  const selectedVersionId = $derived(page.url.searchParams.get("version") ?? "");
  const editRequested = $derived(page.url.searchParams.get("edit") === "1");

  function formatTimestamp(value: string): string {
    if (!value) return translate("common.emDash", $locale);
    return formatLocaleDateTime(value, $locale);
  }

  function versionHref(versionId: string): string {
    return resolve(`/artifacts/${data.artifact.id}?version=${versionId}`);
  }
</script>

<svelte:head>
  <title>{title} · Harpia</title>
</svelte:head>

<div class="px-4 py-6 lg:px-6">
  <div class="mb-5">
    <a
      href={resolve("/artifacts")}
      class="inline-flex items-center gap-2 rounded-md border border-plumage px-3 py-2 text-[12px] text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
    >
      <ArrowLeft class="size-4" />
      {translate("artifacts.detail.back", $locale)}
    </a>
  </div>

  <header class="mb-6 border-b border-plumage pb-5">
    <div class="flex flex-wrap items-start justify-between gap-4">
      <div class="min-w-0">
        <HarpyHeading tag="h1" class="text-2xl text-cream">
          {title}
        </HarpyHeading>
        <p class="mt-1 font-mono text-[11px] break-all text-crown-ash-dark">
          {data.artifact.id}
        </p>
      </div>
      <span
        class="inline-flex items-center rounded-full border border-plumage px-2.5 py-1 font-mono text-[10px] tracking-wider text-crown-ash uppercase"
      >
        {artifactStatusLabel(data.artifact.status)}
      </span>
    </div>

    <dl class="mt-5 grid gap-3 md:grid-cols-4">
      <div>
        <dt class="font-mono text-[10px] text-crown-ash-dark uppercase">
          {translate("artifacts.detail.type", $locale)}
        </dt>
        <dd class="mt-1 text-[12px] break-all text-crown-ash">
          {data.artifact.artifactTypeKey}
        </dd>
      </div>
      <div>
        <dt class="font-mono text-[10px] text-crown-ash-dark uppercase">
          {translate("artifacts.detail.updatedAt", $locale)}
        </dt>
        <dd class="mt-1 text-[12px] text-crown-ash">
          {formatTimestamp(data.artifact.updatedAt || data.artifact.createdAt)}
        </dd>
      </div>
      <div>
        <dt class="font-mono text-[10px] text-crown-ash-dark uppercase">
          {translate("artifacts.detail.planExecution", $locale)}
        </dt>
        <dd class="mt-1 text-[12px] break-all text-crown-ash">
          {data.artifact.planExecutionId || translate("common.emDash", $locale)}
        </dd>
      </div>
      <div>
        <dt class="font-mono text-[10px] text-crown-ash-dark uppercase">
          {translate("artifacts.detail.hash", $locale)}
        </dt>
        <dd class="mt-1 truncate font-mono text-[12px] text-crown-ash">
          {data.contentHash}
        </dd>
      </div>
    </dl>
  </header>

  <div class="grid gap-5 xl:grid-cols-[minmax(0,1fr)_380px]">
    <main class="min-w-0 space-y-5">
      <section class="rounded-lg border border-plumage bg-obsidian-light/20 p-4">
        <div class="mb-3 flex items-center justify-between gap-3">
          <h2 class="font-heading text-base font-semibold text-cream">
            {translate("artifacts.detail.preview", $locale)}
          </h2>
          {#if selectedVersionId}
            <span class="font-mono text-[10px] text-talon-gold">
              {translate("artifacts.detail.versionPreview", $locale)}
            </span>
          {/if}
        </div>
        <ArtifactPreview
          tenantId={data.tenantId}
          artifactId={data.artifact.id}
          artifactVersionId={selectedVersionId}
        />
      </section>

      {#if data.editableText.editable && (editRequested || !selectedVersionId)}
        <TextArtifactEditor
          tenantId={data.tenantId}
          artifactId={data.artifact.id}
          contentHash={data.contentHash}
          title={data.editableText.title}
          text={data.editableText.text}
          onSaved={() => invalidateAll()}
        />
      {/if}
    </main>

    <aside class="space-y-3">
      <section class="rounded-lg border border-plumage bg-obsidian-light/20 p-4">
        <div class="mb-3 flex items-center gap-2">
          <History class="size-4 text-talon-gold" />
          <h2 class="font-heading text-base font-semibold text-cream">
            {translate("artifacts.versions.heading", $locale)}
          </h2>
        </div>
        {#if data.versions.length === 0}
          <p class="text-[12px] text-crown-ash">
            {translate("artifacts.versions.empty", $locale)}
          </p>
        {:else}
          <div class="flex flex-col gap-2">
            {#each data.versions as version (version.id)}
              <a
                href={versionHref(version.id)}
                class="rounded-md border border-plumage px-3 py-2 transition-colors hover:border-talon-gold/60"
              >
                <div class="flex items-center justify-between gap-3">
                  <span class="font-heading text-sm text-cream">
                    {translate("artifacts.versions.version", $locale, {
                      n: version.versionNumber,
                    })}
                  </span>
                  {#if version.id === data.artifact.currentVersionId}
                    <CheckCircle2 class="size-4 text-green-300" />
                  {/if}
                </div>
                <p class="mt-1 text-[11px] text-crown-ash">
                  {formatTimestamp(version.createdAt)}
                </p>
                {#if version.editSummary}
                  <p class="mt-1 text-[12px] text-crown-ash-dark">
                    {version.editSummary}
                  </p>
                {/if}
              </a>
            {/each}
          </div>
        {/if}
      </section>
    </aside>
  </div>
</div>
