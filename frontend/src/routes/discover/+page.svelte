<script lang="ts">
  import { goto } from "$app/navigation";
  import { resolve } from "$app/paths";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import TemplatePickerCard from "$lib/components/thread/TemplatePickerCard.svelte";
  import { locale, translate } from "$lib/i18n";

  let { data } = $props();

  const groups = $derived.by(() => {
    const byVertical: Record<string, typeof data.templates> = {};
    for (const t of data.templates) {
      const key = t.vertical || "general";
      (byVertical[key] ??= []).push(t);
    }
    return Object.entries(byVertical).sort(([a], [b]) => a.localeCompare(b));
  });

  function pick(templateId: string) {
    void goto(resolve(`/new?template=${encodeURIComponent(templateId)}`));
  }
</script>

<svelte:head>
  <title>{translate("nav.discover", $locale)} · Harpia</title>
</svelte:head>

<div class="mx-auto max-w-5xl px-4 py-6">
  <HarpyHeading tag="h1" class="text-2xl text-text">
    {translate("discover.heading", $locale)}
  </HarpyHeading>
  <p class="mt-1 font-body text-[13px] text-text-muted">
    {translate("discover.subheading", $locale)}
  </p>

  {#if data.templates.length === 0}
    <p
      class="mt-6 rounded border border-border bg-surface-elevated px-4 py-3 text-sm text-text-muted"
    >
      {translate("discover.empty", $locale)}
    </p>
  {:else}
    <div class="mt-6 space-y-6">
      {#each groups as [vertical, templates] (vertical)}
        <section>
          <p
            class="mb-2 font-mono text-[10px] tracking-widest text-text-muted-dark uppercase"
          >
            {vertical}
          </p>
          <TemplatePickerCard {templates} onPick={pick} />
        </section>
      {/each}
    </div>
  {/if}
</div>
