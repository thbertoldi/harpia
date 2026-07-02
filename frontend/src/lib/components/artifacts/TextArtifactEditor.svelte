<script lang="ts">
  import { Code, ConnectError } from "@connectrpc/connect";
  import { Loader2, Save } from "lucide-svelte";
  import { saveTextArtifactVersion } from "$lib/artifacts/artifacts";
  import { locale, translate } from "$lib/i18n";

  let {
    tenantId,
    artifactId,
    contentHash,
    title,
    text,
    onSaved,
  }: {
    tenantId: string;
    artifactId: string;
    contentHash: string;
    title: string;
    text: string;
    onSaved?: (artifactId: string) => void;
  } = $props();

  let draftTitle = $state("");
  let draftText = $state("");
  let saving = $state(false);
  let error = $state<string | null>(null);

  const canSave = $derived(draftText.trim().length > 0 && !saving);

  $effect(() => {
    draftTitle = title;
    draftText = text;
    error = null;
  });

  async function save() {
    if (!canSave) return;
    saving = true;
    error = null;
    try {
      await saveTextArtifactVersion({
        tenantId,
        artifactId,
        expectedContentHash: contentHash,
        title: draftTitle,
        text: draftText,
        editSummary: translate("artifacts.editor.defaultSummary", $locale),
      });
      onSaved?.(artifactId);
    } catch (err) {
      const connectError = ConnectError.from(err);
      error =
        connectError.code === Code.FailedPrecondition
          ? translate("artifacts.editor.conflict", $locale)
          : translate("artifacts.editor.saveError", $locale);
    } finally {
      saving = false;
    }
  }
</script>

<section class="rounded-lg border border-plumage bg-obsidian-light/25 p-4">
  <div class="mb-3">
    <h2 class="font-heading text-base font-semibold text-cream">
      {translate("artifacts.editor.heading", $locale)}
    </h2>
    <p class="mt-1 text-[12px] text-crown-ash">
      {translate("artifacts.editor.subheading", $locale)}
    </p>
  </div>

  <label class="block">
    <span class="font-mono text-[10px] text-crown-ash-dark uppercase">
      {translate("artifacts.editor.title", $locale)}
    </span>
    <input
      bind:value={draftTitle}
      class="mt-1 w-full rounded-md border border-plumage bg-obsidian px-3 py-2 text-sm text-cream transition-colors outline-none focus:border-talon-gold"
    />
  </label>

  <label class="mt-3 block">
    <span class="font-mono text-[10px] text-crown-ash-dark uppercase">
      {translate("artifacts.editor.text", $locale)}
    </span>
    <textarea
      bind:value={draftText}
      rows="12"
      class="mt-1 w-full resize-y rounded-md border border-plumage bg-obsidian px-3 py-2 font-mono text-sm leading-relaxed text-cream transition-colors outline-none focus:border-talon-gold"
    ></textarea>
  </label>

  {#if error}
    <p
      class="mt-3 rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-[12px] text-red-300"
    >
      {error}
    </p>
  {/if}

  <div class="mt-4 flex justify-end">
    <button
      type="button"
      onclick={save}
      disabled={!canSave}
      class="inline-flex items-center gap-2 rounded-md border border-talon-gold/50 px-3 py-2 text-[12px] text-talon-gold transition-colors hover:bg-talon-gold/10 disabled:cursor-not-allowed disabled:border-plumage disabled:text-crown-ash-dark"
    >
      {#if saving}
        <Loader2 class="size-4 animate-spin" />
      {:else}
        <Save class="size-4" />
      {/if}
      {translate("artifacts.editor.save", $locale)}
    </button>
  </div>
</section>
