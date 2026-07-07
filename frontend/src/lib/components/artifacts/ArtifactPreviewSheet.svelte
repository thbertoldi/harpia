<script lang="ts">
  import {
    X,
    RefreshCw,
    FileText,
    Code,
    Image as ImageIcon,
    List,
  } from "lucide-svelte";
  import ArtifactPreview from "$lib/components/ArtifactPreview.svelte";
  import {
    getArtifactPayload,
    saveTextArtifactVersion,
  } from "$lib/artifacts/artifacts";
  import { locale, translate } from "$lib/i18n";
  import {
    artifactTitle,
    artifactTypeLabelKey,
    editableTextFromPayload,
  } from "$lib/artifacts/text";
  import type { FormattedArtifactPreview } from "$lib/artifacts/preview";
  import type { Artifact } from "$lib/gen/harpia/artifacts/v1/artifacts_pb";
  import { fade } from "svelte/transition";

  type ArtifactPreviewTab = "preview" | "edit";

  const defaultTabs = ["preview"] satisfies readonly ArtifactPreviewTab[];
  const editableTabs = [
    "preview",
    "edit",
  ] satisfies readonly ArtifactPreviewTab[];

  let {
    open,
    onClose,
    tenantId,
    artifact,
    artifactLoading = false,
  }: {
    open: boolean;
    onClose: () => void;
    tenantId: string;
    artifact: Artifact | null;
    artifactLoading?: boolean;
  } = $props();

  let tab = $state<ArtifactPreviewTab>("preview");
  let reloadKey = $state(0);
  let preview = $state<FormattedArtifactPreview | null>(null);
  let loading = $state(false);
  let editTitle = $state("");
  let editText = $state("");
  let editContentHash = $state("");
  let editLoading = $state(false);
  let editSaving = $state(false);
  let editSaved = $state(false);
  let editError = $state(false);

  // Reset transient UI state when the artifact changes.
  $effect(() => {
    void artifact?.id;
    tab = "preview";
    editTitle = "";
    editText = "";
    editContentHash = "";
    editSaved = false;
    editError = false;
  });

  const editable = $derived(
    artifact?.artifactTypeKey === "harpia.artifacts.v1.TextDraft" ||
      artifact?.artifactTypeKey === "harpia.artifacts.v1.LinkedInPostDraft",
  );
  const tabs = $derived(editable ? editableTabs : defaultTabs);

  $effect(() => {
    if (!artifact || !editable || !tenantId) return;
    const artifactId = artifact.id;
    const artifactTypeKey = artifact.artifactTypeKey;
    const controller = new AbortController();
    editLoading = true;
    editError = false;
    void (async () => {
      try {
        const { payload, contentHash } = await getArtifactPayload(
          tenantId,
          artifactId,
        );
        if (controller.signal.aborted) return;
        const projection = editableTextFromPayload(artifactTypeKey, payload);
        editTitle = projection.title;
        editText = projection.text;
        editContentHash = contentHash;
      } catch {
        if (!controller.signal.aborted) editError = true;
      } finally {
        if (!controller.signal.aborted) editLoading = false;
      }
    })();
    return () => controller.abort();
  });

  const busy = $derived(artifactLoading || loading);
  const Icon = $derived(
    preview?.kind === "html" || preview?.kind === "json"
      ? Code
      : preview?.kind === "image"
        ? ImageIcon
        : preview?.kind === "list"
          ? List
          : FileText,
  );

  async function saveEditedText() {
    if (!artifact || !editContentHash || editSaving) return;
    editSaving = true;
    editSaved = false;
    editError = false;
    try {
      const response = await saveTextArtifactVersion({
        tenantId,
        artifactId: artifact.id,
        expectedContentHash: editContentHash,
        title: editTitle,
        text: editText,
        editSummary: translate("artifactPreview.editSummary", $locale),
      });
      editContentHash = response.artifact?.contentHash ?? editContentHash;
      reloadKey += 1;
      editSaved = true;
    } catch {
      editError = true;
    } finally {
      editSaving = false;
    }
  }
</script>

{#if open}
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
      lg:sticky lg:top-6 lg:right-auto lg:bottom-auto lg:z-auto lg:h-[calc(100vh-3rem)] lg:w-[46%] lg:max-w-[980px] lg:min-w-[420px] lg:rounded-md 2xl:w-[48%]"
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
          {artifact
            ? artifactTitle(artifact)
            : translate("artifactPreview.loading", $locale)}
        </h2>
        {#if artifact}
          <p class="truncate text-[10px] text-crown-ash-dark">
            {translate(artifactTypeLabelKey(artifact.artifactTypeKey), $locale)}
          </p>
        {/if}
      </div>

      <div class="flex rounded-md border border-plumage p-0.5">
        {#each tabs as t (t)}
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
      {#if !artifact}
        <div
          class="flex h-full items-center justify-center px-4 py-10 text-[12px] text-crown-ash"
        >
          {translate("artifactPreview.loading", $locale)}
        </div>
      {:else if tab === "preview"}
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
        <div class="flex h-full flex-col gap-3 bg-surface-deep px-4 py-3">
          {#if editLoading}
            <p class="text-[12px] text-crown-ash">
              {translate("artifactPreview.loading", $locale)}
            </p>
          {:else}
            <label class="grid gap-1 text-[11px] text-crown-ash">
              {translate("artifactPreview.editTitle", $locale)}
              <input
                class="rounded border border-plumage bg-obsidian px-3 py-2 text-[12px] text-cream outline-none focus:border-talon-gold"
                bind:value={editTitle}
              />
            </label>
            <label class="grid min-h-0 flex-1 gap-1 text-[11px] text-crown-ash">
              {translate("artifactPreview.editText", $locale)}
              <textarea
                class="min-h-[18rem] flex-1 resize-none rounded border border-plumage bg-obsidian px-3 py-2 font-mono text-[12px] leading-relaxed text-cream outline-none focus:border-talon-gold"
                bind:value={editText}
              ></textarea>
            </label>
            <div class="flex flex-wrap items-center gap-2">
              <button
                type="button"
                onclick={saveEditedText}
                disabled={editSaving || !editContentHash}
                class="rounded-md border border-talon-gold bg-talon-gold px-3 py-2 text-[12px] font-semibold text-on-primary hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {translate(
                  editSaving
                    ? "artifactPreview.saving"
                    : "artifactPreview.save",
                  $locale,
                )}
              </button>
              {#if editSaved}
                <span class="text-[11px] text-status-done">
                  {translate("artifactPreview.saved", $locale)}
                </span>
              {/if}
              {#if editError}
                <span class="text-[11px] text-danger">
                  {translate("artifactPreview.saveError", $locale)}
                </span>
              {/if}
            </div>
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
          class="size-1.5 rounded-full {busy
            ? 'animate-pulse bg-energy'
            : 'bg-status-done/70'}"
        ></span>
        {busy
          ? translate("artifactPreview.generating", $locale)
          : translate("artifactPreview.ready", $locale)}
      </span>
    </footer>
  </aside>
{/if}
