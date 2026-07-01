<script lang="ts">
  import type { TemplateInputParameter } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { TemplateInputParameterType } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { selectOptions } from "$lib/plans/template-inputs";

  interface Props {
    params: TemplateInputParameter[];
    values: Record<string, string>;
  }

  // `values` is bindable so the parent reads edits back.
  let { params, values = $bindable() }: Props = $props();

  const T = TemplateInputParameterType;
</script>

<div class="flex flex-col gap-3">
  {#each params as p (p.key)}
    <label class="flex flex-col gap-1 text-[12px] text-crown-ash">
      <span class="font-medium text-cream">
        {p.label || p.key}{#if p.required}<span class="text-talon-gold"> *</span>{/if}
      </span>
      {#if p.description}
        <span class="text-[11px] text-crown-ash-dark">{p.description}</span>
      {/if}

      {#if p.type === T.TEXTAREA}
        <textarea
          bind:value={values[p.key]}
          rows="3"
          class="rounded border border-plumage bg-obsidian-light px-2 py-1 text-[12px] text-cream"
        ></textarea>
      {:else if p.type === T.SELECT || p.type === T.LANGUAGE}
        <select
          bind:value={values[p.key]}
          class="rounded border border-plumage bg-obsidian-light px-2 py-1 text-[12px] text-cream"
        >
          <option value="">—</option>
          {#each selectOptions(p.optionsJson) as opt (opt)}
            <option value={opt}>{opt}</option>
          {/each}
        </select>
      {:else}
        <!-- TEXT, DATE_RANGE, INTEGRATION_SELECTOR, UNSPECIFIED fall back to text input in D -->
        <input
          type="text"
          bind:value={values[p.key]}
          class="rounded border border-plumage bg-obsidian-light px-2 py-1 text-[12px] text-cream"
        />
      {/if}
    </label>
  {/each}
</div>
