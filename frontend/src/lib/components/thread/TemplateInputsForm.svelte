<script lang="ts">
  import type { TemplateInputParameter } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { TemplateInputParameterType } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { selectOptions, type DateRangeValue } from "$lib/plans/template-inputs";
  import { executorClient } from "$lib/rpc";
  import type { ExecutorInstallation } from "$lib/gen/harpia/executors/v1/executors_pb";
  import { locale, translate } from "$lib/i18n";

  interface Props {
    params: TemplateInputParameter[];
    values: Record<string, unknown>;
    tenantId: string;
  }

  // `values` is bindable so the parent reads edits back.
  let { params, values = $bindable(), tenantId }: Props = $props();

  const T = TemplateInputParameterType;

  let installations = $state<ExecutorInstallation[]>([]);
  let installationsLoading = $state(false);

  // Load executor installations once when the component mounts
  $effect(() => {
    if (!tenantId) return;
    installationsLoading = true;
    (async () => {
      try {
        const loaded: ExecutorInstallation[] = [];
        for await (const response of executorClient.listExecutorInstallations({
          tenantId,
          pageSize: 100,
          pageToken: "",
        })) {
          loaded.push(...response.installations);
        }
        installations = loaded;
      } catch {
        installations = [];
      } finally {
        installationsLoading = false;
      }
    })();
  });

  // Helper to get or initialize a DateRangeValue
  function getDateRange(key: string): DateRangeValue {
    const val = values[key];
    if (val && typeof val === "object" && "startDate" in val && "endDate" in val) {
      return val as DateRangeValue;
    }
    return { startDate: "", endDate: "" };
  }

  // Helper to update a DateRangeValue
  function setDateRange(key: string, range: DateRangeValue): void {
    values[key] = range;
  }

  // Helper to translate a label with fallback to the original label
  function translatedLabel(p: TemplateInputParameter): string {
    const key = `plans.inputs.${p.key}.label`;
    const translated = translate(key, $locale);
    return translated === key ? (p.label || p.key) : translated;
  }
</script>

<div class="flex flex-col gap-3">
  {#each params as p (p.key)}
    <label class="flex flex-col gap-1 text-[12px] text-crown-ash">
      <span class="font-medium text-cream">
        {translatedLabel(p)}{#if p.required}<span class="text-talon-gold"> *</span>{/if}
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
          {#each selectOptions(p.optionsJson) as opt (opt.value)}
            <option value={opt.value}>{opt.label}</option>
          {/each}
        </select>
      {:else if p.type === T.DATE_RANGE}
        {@const range = getDateRange(p.key)}
        <div class="flex gap-2">
          <input
            type="date"
            value={range.startDate}
            oninput={(e) => setDateRange(p.key, { ...getDateRange(p.key), startDate: e.currentTarget.value })}
            class="flex-1 rounded border border-plumage bg-obsidian-light px-2 py-1 text-[12px] text-cream"
          />
          <input
            type="date"
            value={range.endDate}
            oninput={(e) => setDateRange(p.key, { ...getDateRange(p.key), endDate: e.currentTarget.value })}
            class="flex-1 rounded border border-plumage bg-obsidian-light px-2 py-1 text-[12px] text-cream"
          />
        </div>
      {:else if p.type === T.INTEGRATION_SELECTOR}
        <select
          bind:value={values[p.key]}
          class="rounded border border-plumage bg-obsidian-light px-2 py-1 text-[12px] text-cream"
        >
          {#if installations.length === 0}
            <option value="" disabled>No integrations available</option>
          {:else}
            <option value="">—</option>
            {#each installations as inst (inst.id)}
              <option value={inst.id}>{inst.displayName || inst.id}</option>
            {/each}
          {/if}
        </select>
      {:else}
        <!-- TEXT, UNSPECIFIED fall back to text input -->
        <input
          type="text"
          bind:value={values[p.key]}
          class="rounded border border-plumage bg-obsidian-light px-2 py-1 text-[12px] text-cream"
        />
      {/if}
    </label>
  {/each}
</div>
