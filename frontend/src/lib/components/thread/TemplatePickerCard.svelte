<script lang="ts">
  import type { PlanTemplate } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { locale, translate } from "$lib/i18n";

  interface Props {
    templates: PlanTemplate[];
    onPick: (templateId: string) => void;
    disabled?: boolean;
  }
  let { templates, onPick, disabled = false }: Props = $props();
</script>

<div class="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
  {#each templates as template (template.id)}
    <button
      type="button"
      {disabled}
      onclick={() => onPick(template.id)}
      class="cursor-pointer rounded-lg border border-plumage bg-obsidian-light px-3 py-3 text-left hover:border-talon-gold disabled:cursor-not-allowed disabled:opacity-50"
    >
      <p class="font-heading text-[13px] font-semibold text-cream">
        {template.name}
      </p>
      {#if template.description}
        <p class="mt-1 font-body text-[11px] text-crown-ash">
          {template.description}
        </p>
      {/if}
      <p class="mt-2 font-mono text-[10px] text-crown-ash-dark">
        {translate("new.stepsCount", $locale, {
          count: template.steps?.length ?? 0,
        })}
      </p>
    </button>
  {/each}
</div>
