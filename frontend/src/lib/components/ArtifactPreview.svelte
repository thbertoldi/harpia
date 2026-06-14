<script lang="ts">
  import {
    fetchArtifactPreview,
    formatArtifactPreview,
    type FormattedArtifactPreview,
  } from "$lib/artifacts/preview";

  let {
    tenantId,
    artifactId,
    preview = $bindable<FormattedArtifactPreview | null>(null),
    loading = $bindable(false),
    error = $bindable<string | null>(null),
  }: {
    tenantId: string;
    artifactId: string;
    preview?: FormattedArtifactPreview | null;
    loading?: boolean;
    error?: string | null;
  } = $props();

  $effect(() => {
    if (!tenantId || !artifactId) {
      preview = null;
      error = null;
      return;
    }

    let cancelled = false;
    loading = true;
    error = null;

    fetchArtifactPreview(tenantId, artifactId)
      .then((response) => {
        if (cancelled) return;
        preview = formatArtifactPreview(response);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        preview = null;
        error = err instanceof Error ? err.message : "Failed to load preview";
      })
      .finally(() => {
        if (!cancelled) {
          loading = false;
        }
      });

    return () => {
      cancelled = true;
    };
  });
</script>

{#if loading}
  <p class="artifact-preview artifact-preview--loading">Loading preview…</p>
{:else if error}
  <p class="artifact-preview artifact-preview--error">{error}</p>
{:else if preview}
  {#if preview.kind === "list" && preview.listSummary}
    <div class="artifact-preview artifact-preview--list">
      <p>{preview.listSummary.articleCount} articles</p>
      <ul>
        {#each preview.listSummary.titles as title, i (i)}
          <li>{title}</li>
        {/each}
      </ul>
    </div>
  {:else if preview.kind === "json"}
    <pre class="artifact-preview artifact-preview--json">{preview.text}</pre>
  {:else}
    <pre class="artifact-preview artifact-preview--text">{preview.text}</pre>
  {/if}
{/if}

<style>
  .artifact-preview {
    margin: 0;
    white-space: pre-wrap;
    word-break: break-word;
  }

  .artifact-preview--loading,
  .artifact-preview--error {
    font-size: 0.875rem;
  }

  .artifact-preview--error {
    color: var(--color-error, #b42318);
  }

  .artifact-preview--json,
  .artifact-preview--text {
    font-family: var(--font-mono, ui-monospace, monospace);
    font-size: 0.875rem;
  }

  .artifact-preview--list ul {
    margin: 0.5rem 0 0;
    padding-left: 1.25rem;
  }
</style>
