<script lang="ts">
  import { AlertTriangle, CheckCircle2, Clock3, FileText } from "lucide-svelte";
  import { locale, translate } from "$lib/i18n";
  import type { PlanActivityItem } from "$lib/plans/activity";

  let {
    items,
    onOpenArtifact,
  }: {
    items: PlanActivityItem[];
    onOpenArtifact?: (artifactId: string) => void;
  } = $props();

  function iconFor(item: PlanActivityItem) {
    if (item.status === "failed") return AlertTriangle;
    if (item.status === "waiting" || item.status === "running") return Clock3;
    if (item.kind === "artifact_created") return FileText;
    return CheckCircle2;
  }
</script>

<div class="flex flex-col gap-2">
  {#each items as item (item.id)}
    {@const Icon = iconFor(item)}
    <article class="rounded-lg border border-plumage bg-obsidian-light/25 p-3">
      <div class="flex gap-3">
        <Icon class="mt-0.5 size-4 shrink-0 text-talon-gold" />
        <div class="min-w-0 flex-1">
          <h3 class="font-heading text-sm font-semibold text-cream">
            {item.title}
          </h3>
          <p class="mt-1 text-[12px] leading-relaxed text-crown-ash">
            {item.body}
          </p>
          {#if item.artifactId}
            <button
              type="button"
              onclick={() => onOpenArtifact?.(item.artifactId)}
              class="mt-2 rounded-md border border-plumage px-2 py-1 text-[11px] text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
            >
              {translate("artifacts.actions.open", $locale)}
            </button>
          {/if}
        </div>
      </div>
    </article>
  {/each}
</div>
