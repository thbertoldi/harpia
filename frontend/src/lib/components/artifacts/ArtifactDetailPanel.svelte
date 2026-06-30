<script lang="ts">
  import { resolve } from "$app/paths";
  import { ArrowLeft, ExternalLink } from "lucide-svelte";
  import ArtifactPreview from "$lib/components/ArtifactPreview.svelte";
  import { locale, translate } from "$lib/i18n";
  import { formatLocaleDateTime } from "$lib/i18n/format";
  import { artifactStatusLabel, artifactTitle } from "$lib/artifacts/text";
  import type { Artifact } from "$lib/gen/harpia/artifacts/v1/artifacts_pb";

  let {
    tenantId,
    artifact,
    onClose,
  }: {
    tenantId: string;
    artifact: Artifact;
    onClose: () => void;
  } = $props();

  const title = $derived(artifactTitle(artifact));

  function formatTimestamp(value: string): string {
    if (!value) return translate("common.emDash", $locale);
    return formatLocaleDateTime(value, $locale);
  }
</script>

<aside class="sticky top-4 flex max-h-[calc(100vh-2rem)] flex-col gap-3 overflow-y-auto">
  <button
    type="button"
    class="inline-flex items-center gap-2 self-start rounded-md border border-plumage px-3 py-2 text-[12px] text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
    onclick={onClose}
  >
    <ArrowLeft class="size-4" />
    {translate("artifacts.rail.backToRail", $locale)}
  </button>

  <header class="rounded-lg border border-plumage bg-obsidian-light/30 p-3">
    <div class="flex items-start justify-between gap-2">
      <div class="min-w-0">
        <h2 class="font-heading text-base font-semibold text-cream">{title}</h2>
        <p class="mt-0.5 truncate font-mono text-[10px] text-crown-ash-dark">
          {artifactStatusLabel(artifact.status)}
        </p>
      </div>
      <a
        href={resolve(`/artifacts/${artifact.id}`)}
        class="inline-flex size-8 shrink-0 items-center justify-center rounded-md border border-plumage text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
        aria-label={translate("artifacts.actions.open", $locale)}
        title={translate("artifacts.actions.open", $locale)}
      >
        <ExternalLink class="size-4" />
      </a>
    </div>
    <p class="mt-2 truncate font-mono text-[10px] text-crown-ash-dark">
      {artifact.id}
    </p>
    <dl class="mt-3 grid gap-2">
      <div>
        <dt class="font-mono text-[10px] text-crown-ash-dark uppercase">
          {translate("artifacts.detail.type", $locale)}
        </dt>
        <dd class="mt-0.5 text-[11px] break-all text-crown-ash">
          {artifact.artifactTypeKey}
        </dd>
      </div>
      <div>
        <dt class="font-mono text-[10px] text-crown-ash-dark uppercase">
          {translate("artifacts.detail.updatedAt", $locale)}
        </dt>
        <dd class="mt-0.5 text-[11px] text-crown-ash">
          {formatTimestamp(artifact.updatedAt || artifact.createdAt)}
        </dd>
      </div>
    </dl>
  </header>

  <section class="rounded-lg border border-plumage bg-obsidian-light/20 p-3">
    <h3 class="mb-2 font-heading text-sm font-semibold text-cream">
      {translate("artifacts.detail.preview", $locale)}
    </h3>
    <ArtifactPreview {tenantId} artifactId={artifact.id} constrained={false} />
  </section>
</aside>
