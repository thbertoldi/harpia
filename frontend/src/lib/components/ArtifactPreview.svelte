<script lang="ts">
  import {
    fetchArtifactPreview,
    formatArtifactPreview,
    type FormattedArtifactPreview,
  } from "$lib/artifacts/preview";
  import { renderMarkdown } from "$lib/artifacts/markdown";
  import { locale, translate } from "$lib/i18n";

  let {
    tenantId,
    artifactId,
    artifactVersionId = "",
    constrained = true,
    preview = $bindable<FormattedArtifactPreview | null>(null),
    loading = $bindable(false),
    error = $bindable<string | null>(null),
  }: {
    tenantId: string;
    artifactId: string;
    artifactVersionId?: string;
    /** When true, caps preview height and scrolls overflow inline. */
    constrained?: boolean;
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

    fetchArtifactPreview(tenantId, artifactId, artifactVersionId)
      .then((response) => {
        if (cancelled) return;
        preview = formatArtifactPreview(response);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        preview = null;
        error =
          err instanceof Error
            ? err.message
            : translate("artifactPreview.loadError", $locale);
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

  const imageSrc = $derived.by(() => {
    if (preview?.kind !== "image" || !preview.image) return "";
    if (preview.image.inlineData && preview.image.inlineData.length > 0) {
      // Inline bytes -> data URL (assume PNG; backend decides the bytes).
      let bin = "";
      const bytes = preview.image.inlineData;
      const chunk = 0x8000;
      for (let i = 0; i < bytes.length; i += chunk) {
        bin += String.fromCharCode(...bytes.subarray(i, i + chunk));
      }
      return `data:image/png;base64,${btoa(bin)}`;
    }
    return preview.image.url ?? "";
  });
</script>

<div
  class="artifact-preview-body"
  class:artifact-preview-body--constrained={constrained}
>
  {#if loading}
    <p class="artifact-preview artifact-preview--loading">
      {translate("artifactPreview.loading", $locale)}
    </p>
  {:else if error}
    <p class="artifact-preview artifact-preview--error">{error}</p>
  {:else if preview}
    {#if preview.kind === "html" && preview.html}
      <iframe
        title="artifact-preview"
        class="artifact-preview artifact-preview--html"
        srcdoc={preview.html}
        sandbox="allow-same-origin"
      ></iframe>
    {:else if preview.kind === "markdown" && preview.markdown !== undefined}
      <div class="artifact-preview artifact-preview--markdown">
        <!-- eslint-disable-next-line svelte/no-at-html-tags -- renderMarkdown HTML-escapes its input before transforming, so injection is safe -->
        {@html renderMarkdown(preview.markdown)}
      </div>
    {:else if preview.kind === "image" && preview.image}
      <div class="artifact-preview artifact-preview--image">
        {#if imageSrc}
          <img src={imageSrc} alt={preview.image.altText ?? ""} />
        {:else}
          <p class="artifact-preview--error">
            {translate("artifactPreview.empty", $locale)}
          </p>
        {/if}
      </div>
    {:else if preview.kind === "linkedin_post" && preview.linkedInPost}
      <article class="artifact-preview artifact-preview--linkedin-post">
        <p>{preview.linkedInPost.text}</p>
        {#if preview.linkedInPost.slides.length > 0}
          <section class="mt-5 border-t border-plumage pt-4">
            {#if preview.linkedInPost.carouselTitle}<h3>
                {preview.linkedInPost.carouselTitle}
              </h3>{/if}
            <ol class="mt-3 grid gap-3">
              {#each preview.linkedInPost.slides as slide, index (index)}
                <li
                  class="rounded-md border border-plumage/70 bg-obsidian-light px-3 py-2"
                >
                  <strong>{slide.heading}</strong>{#if slide.body}<p
                      class="mt-1"
                    >
                      {slide.body}
                    </p>{/if}
                </li>
              {/each}
            </ol>
          </section>
        {/if}
      </article>
    {:else if preview.kind === "list" && preview.listSummary}
      <div class="artifact-preview artifact-preview--list">
        <p>
          {translate("artifactPreview.articles", $locale, {
            count: preview.listSummary.articleCount,
          })}
        </p>
        <ul>
          {#each preview.listSummary.titles as title, i (i)}
            <li>{title}</li>
          {/each}
        </ul>
      </div>
    {:else if preview.kind === "json"}
      <pre class="artifact-preview artifact-preview--json">{preview.text}</pre>
    {:else if preview.kind !== "empty"}
      <pre class="artifact-preview artifact-preview--text">{preview.text}</pre>
    {:else}
      <p class="artifact-preview artifact-preview--loading">
        {translate("artifactPreview.empty", $locale)}
      </p>
    {/if}
  {/if}
</div>

<style>
  /*
   * Base readable text color for every preview kind. The ArtifactPreviewSheet
   * is a fixed-position portal rendered outside the page flow, so its <aside>
   * sets bg-obsidian but no text color — without this rule the markdown/json/
   * text/list bodies inherit the browser default (#000) and have no contrast in
   * dark mode. Headings/blockquote/links/code already override below.
   */
  .artifact-preview-body {
    color: var(--color-text);
  }

  .artifact-preview-body:not(.artifact-preview-body--constrained) {
    min-height: 100%;
    background:
      radial-gradient(
        circle at top left,
        color-mix(in srgb, var(--color-energy) 8%, transparent),
        transparent 18rem
      ),
      var(--color-surface-deep);
  }

  .artifact-preview-body--constrained {
    max-height: 24rem;
    overflow-y: auto;
  }

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
    color: var(--color-danger);
  }

  .artifact-preview--html {
    width: 100%;
    min-height: 16rem;
    border: 0;
    background: #fff;
    white-space: normal;
  }

  .artifact-preview--markdown {
    font-size: 0.95rem;
    line-height: 1.65;
    white-space: normal;
  }

  .artifact-preview-body:not(.artifact-preview-body--constrained)
    .artifact-preview--markdown,
  .artifact-preview-body:not(.artifact-preview-body--constrained)
    .artifact-preview--text {
    box-sizing: border-box;
    width: min(100%, 58rem);
    margin: 0 auto;
    padding: clamp(1.25rem, 3vw, 2.5rem);
  }

  .artifact-preview-body:not(.artifact-preview-body--constrained)
    .artifact-preview--text {
    font-family: var(--font-body, ui-sans-serif, system-ui, sans-serif);
    font-size: clamp(0.95rem, 0.4vw + 0.85rem, 1.08rem);
    line-height: 1.7;
    white-space: pre-wrap;
  }
  .artifact-preview--markdown :global(h1),
  .artifact-preview--markdown :global(h2),
  .artifact-preview--markdown :global(h3) {
    margin: 0.6em 0 0.3em;
    font-family: var(--font-heading);
    color: var(--color-text);
  }
  .artifact-preview--markdown :global(p) {
    margin: 0.4em 0;
  }
  .artifact-preview--markdown :global(ul),
  .artifact-preview--markdown :global(ol) {
    margin: 0.4em 0;
    padding-left: 1.25rem;
  }
  .artifact-preview--markdown :global(blockquote) {
    margin: 0.5em 0;
    padding-left: 0.75rem;
    border-left: 2px solid var(--color-border);
    color: var(--color-text-muted);
  }
  .artifact-preview--markdown :global(code) {
    font-family: var(--font-mono, ui-monospace, monospace);
    font-size: 0.85em;
    background: var(--color-surface-hover);
    padding: 0.1em 0.3em;
    border-radius: 0.25rem;
  }
  .artifact-preview--markdown :global(pre) {
    padding: 0.75rem;
    background: var(--color-surface-deep);
    border-radius: 0.375rem;
    overflow-x: auto;
  }
  .artifact-preview--markdown :global(pre code) {
    background: transparent;
    padding: 0;
  }
  .artifact-preview--markdown :global(a) {
    color: var(--color-energy);
    text-decoration: underline;
  }
  .artifact-preview--markdown :global(hr) {
    border: 0;
    border-top: 1px solid var(--color-border);
    margin: 0.75rem 0;
  }

  .artifact-preview--image {
    text-align: center;
    white-space: normal;
  }
  .artifact-preview--image img {
    max-width: 100%;
    height: auto;
    border-radius: 0.375rem;
  }

  .artifact-preview--json,
  .artifact-preview-body--constrained .artifact-preview--text {
    font-family: var(--font-mono, ui-monospace, monospace);
    font-size: 0.875rem;
  }

  .artifact-preview--list ul {
    margin: 0.5rem 0 0;
    padding-left: 1.25rem;
  }
</style>
