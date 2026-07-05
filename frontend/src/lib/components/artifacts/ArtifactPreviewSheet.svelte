<script lang="ts">
  import {
    X,
    Copy,
    Check,
    RefreshCw,
    FileText,
    Code,
    Image as ImageIcon,
    List,
  } from "lucide-svelte";
  import ArtifactPreview from "$lib/components/ArtifactPreview.svelte";
  import { locale, translate } from "$lib/i18n";
  import { artifactTitle } from "$lib/artifacts/text";
  import type { FormattedArtifactPreview } from "$lib/artifacts/preview";
  import type { Artifact } from "$lib/gen/harpia/artifacts/v1/artifacts_pb";
  import { fade } from "svelte/transition";

  let {
    open,
    onClose,
    tenantId,
    artifact,
  }: {
    open: boolean;
    onClose: () => void;
    tenantId: string;
    artifact: Artifact | null;
  } = $props();

  let tab = $state<"preview" | "code">("preview");
  let copied = $state(false);
  let reloadKey = $state(0);
  let preview = $state<FormattedArtifactPreview | null>(null);
  let loading = $state(false);

  // Reset transient UI state when the artifact changes.
  $effect(() => {
    void artifact?.id;
    tab = "preview";
    copied = false;
  });

  const source = $derived(
    preview?.html ?? preview?.markdown ?? preview?.text ?? "",
  );
  const mime = $derived.by(() => {
    switch (preview?.kind) {
      case "html":
        return "text/html";
      case "markdown":
        return "text/markdown";
      case "json":
        return "application/json";
      case "image":
        return "image/*";
      case "list":
        return "list";
      default:
        return "text/plain";
    }
  });
  const sizeKb = $derived(((preview?.text?.length ?? 0) / 1024).toFixed(1));
  const Icon = $derived(
    preview?.kind === "html" || preview?.kind === "json"
      ? Code
      : preview?.kind === "image"
        ? ImageIcon
        : preview?.kind === "list"
          ? List
          : FileText,
  );

  async function copySource() {
    if (!source) return;
    try {
      await navigator.clipboard.writeText(source);
      copied = true;
      setTimeout(() => (copied = false), 1500);
    } catch {
      /* clipboard unavailable */
    }
  }
</script>

{#if open && artifact}
  <!-- Mobile-only scrim: below lg the panel is a full overlay; on lg+ it
       shares space with the chat (split view), so no backdrop is needed. -->
  <div
    role="presentation"
    onclick={onClose}
    transition:fade={{ duration: 150 }}
    class="fixed inset-0 z-40 bg-black/40 lg:hidden"
  ></div>
  <aside
    transition:fade={{ duration: 150 }}
    class="fixed inset-y-0 right-0 z-50 flex w-full flex-col border-l border-plumage bg-obsidian shadow-lg
      lg:sticky lg:top-6 lg:right-auto lg:bottom-auto lg:z-auto lg:h-[calc(100vh-3rem)] lg:w-[52%] lg:max-w-[860px] lg:min-w-[440px] lg:rounded-md"
  >
    <!-- Header -->
    <header
      class="flex h-14 shrink-0 items-center gap-2.5 border-b border-plumage px-4"
    >
      <span
        class="flex size-8 shrink-0 items-center justify-center rounded-md bg-energy/15 text-energy"
      >
        <Icon class="size-4" />
      </span>
      <div class="min-w-0 flex-1">
        <h2 class="truncate font-heading text-[13px] font-semibold text-cream">
          {artifactTitle(artifact)}
        </h2>
        <p class="truncate font-mono text-[10px] text-crown-ash-dark">
          {artifact.artifactTypeKey}
        </p>
      </div>

      <div class="flex rounded-md border border-plumage p-0.5">
        {#each ["preview", "code"] as const as t (t)}
          <button
            type="button"
            onclick={() => (tab = t)}
            class="rounded px-2.5 py-1 text-[11px] font-medium capitalize transition-colors
              {tab === t
              ? 'bg-plumage/60 text-cream'
              : 'text-crown-ash hover:text-cream'}"
          >
            {translate(`artifactPreview.tab.${t}`, $locale)}
          </button>
        {/each}
      </div>

      <div class="flex items-center gap-1">
        <button
          type="button"
          onclick={() => (reloadKey += 1)}
          aria-label={translate("artifactPreview.reload", $locale)}
          title={translate("artifactPreview.reload", $locale)}
          class="flex size-8 items-center justify-center rounded-md text-crown-ash hover:bg-plumage/40 hover:text-cream"
        >
          <RefreshCw class="size-4" />
        </button>
        <button
          type="button"
          onclick={copySource}
          aria-label={translate("artifactPreview.copy", $locale)}
          title={translate("artifactPreview.copy", $locale)}
          class="flex size-8 items-center justify-center rounded-md text-crown-ash hover:bg-plumage/40 hover:text-cream"
        >
          {#if copied}
            <Check class="size-4 text-status-done" />
          {:else}
            <Copy class="size-4" />
          {/if}
        </button>
        <button
          type="button"
          onclick={onClose}
          aria-label={translate("canvas.drawer.close", $locale)}
          class="flex size-8 items-center justify-center rounded-md text-crown-ash hover:bg-plumage/40 hover:text-cream"
        >
          <X class="size-4" />
        </button>
      </div>
    </header>

    <!-- Body -->
    <div class="min-h-0 flex-1 overflow-y-auto">
      {#if tab === "preview"}
        {#key reloadKey}
          <ArtifactPreview
            {tenantId}
            artifactId={artifact.id}
            constrained={false}
            bind:preview
            bind:loading
          />
        {/key}
      {:else}
        <div class="h-full overflow-auto bg-surface-deep px-4 py-3">
          {#if source}
            <pre
              class="font-mono text-[12px] leading-relaxed text-crown-ash">{source}</pre>
          {:else}
            <p class="text-[12px] text-crown-ash-dark">
              {translate("artifactPreview.noSource", $locale)}
            </p>
          {/if}
        </div>
      {/if}
    </div>

    <!-- Footer -->
    <footer
      class="flex h-7 shrink-0 items-center justify-between border-t border-plumage px-4 text-[10px] text-crown-ash-dark"
    >
      <span class="flex items-center gap-1.5">
        <span
          class="size-1.5 rounded-full {loading
            ? 'animate-pulse bg-energy'
            : 'bg-status-done/70'}"
        ></span>
        {loading
          ? translate("artifactPreview.generating", $locale)
          : translate("artifactPreview.ready", $locale)}
      </span>
      <span class="font-mono">{mime} · {sizeKb} KB</span>
    </footer>
  </aside>
{/if}
