<script lang="ts">
  import type { TemplateInputParameter } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { TemplateInputParameterType } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { resolve } from "$app/paths";
  import {
    isDateRangePreset,
    resolveDateRangePreset,
    selectOptions,
    type DateRangeValue,
    type SelectOption,
  } from "$lib/plans/template-inputs";
  import { sourceGroupInstallations } from "$lib/plans/source-groups";
  import {
    localizedInputDescription,
    localizedInputLabel,
  } from "$lib/plans/catalog-i18n";
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
    if (
      val &&
      typeof val === "object" &&
      "startDate" in val &&
      "endDate" in val
    ) {
      return val as DateRangeValue;
    }
    return { startDate: "", endDate: "" };
  }

  // Helper to update a DateRangeValue
  function setDateRange(key: string, range: DateRangeValue): void {
    values[key] = range;
  }

  // The active rolling preset for a date-range field, or null when explicit
  // dates are in use. A preset means the window is recomputed on every run
  // (e.g. a weekly newsletter always covering the previous week), so we render
  // it as a read-only indicator rather than a fixed picker.
  function dateRangePreset(key: string): string | null {
    const val = values[key];
    return isDateRangePreset(val) ? val.preset : null;
  }

  // Switch a rolling preset into editable explicit dates, seeded from the
  // preset's current resolution so the picker starts on sensible values.
  function useSpecificDates(key: string, preset: string): void {
    values[key] = resolveDateRangePreset(preset, new Date()) ?? {
      startDate: "",
      endDate: "",
    };
  }

  // Return to the automatic rolling window.
  function useRollingWindow(key: string, preset: string): void {
    values[key] = { preset };
  }

  // Translate a select option's label via catalog i18n
  // (plans.inputs.<key>.option.<value>), falling back to the catalog label.
  function translatedOption(
    p: TemplateInputParameter,
    opt: SelectOption,
  ): string {
    const key = `plans.inputs.${p.key}.option.${opt.value}`;
    const translated = translate(key, $locale);
    return translated === key ? opt.label : translated;
  }

  // Human-readable description of a rolling preset window.
  function rollingPresetLabel(preset: string): string {
    const key = `plans.inputs.date_range.rolling.${preset}`;
    const translated = translate(key, $locale);
    return translated === key
      ? translate("thread.propose.dateRangeRollingGeneric", $locale)
      : translated;
  }

  function getStringList(key: string): string[] {
    const plural = values[`${key}s`];
    if (Array.isArray(plural)) {
      return plural.filter(
        (entry): entry is string => typeof entry === "string",
      );
    }
    const singular = values[key];
    return typeof singular === "string" && singular ? [singular] : [];
  }

  function toggleStringListValue(key: string, value: string): void {
    const selected = getStringList(key);
    const next = selected.includes(value)
      ? selected.filter((entry) => entry !== value)
      : [...selected, value];
    values[`${key}s`] = next;
    values[key] = next[0] ?? "";
  }

  function chipValues(key: string): string[] {
    const value = values[key];
    if (typeof value !== "string") return [];
    return value
      .split(",")
      .map((entry) => entry.trim())
      .filter(Boolean);
  }

  function removeChipValue(key: string, value: string): void {
    values[key] = chipValues(key)
      .filter((entry) => entry !== value)
      .join(", ");
  }

  // Installations eligible as plan sources (RSS feeds). Action integrations such
  // as a LinkedIn publisher are destinations, not sources, and are excluded from
  // the "source group" picker.
  const sourceInstallations = $derived(sourceGroupInstallations(installations));
</script>

<div class="flex flex-col gap-3">
  {#each params as p (p.key)}
    <label class="flex flex-col gap-1 text-[12px] text-crown-ash">
      <span class="font-medium text-cream">
        {localizedInputLabel(p, $locale)}{#if p.required}<span
            class="text-talon-gold"
          >
            *</span
          >{/if}
      </span>
      {#if localizedInputDescription(p, $locale)}
        <span class="text-[11px] text-crown-ash-dark"
          >{localizedInputDescription(p, $locale)}</span
        >
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
            <option value={opt.value}>{translatedOption(p, opt)}</option>
          {/each}
        </select>
      {:else if p.type === T.DATE_RANGE}
        {@const preset = dateRangePreset(p.key)}
        {#if preset}
          <div class="flex flex-wrap items-center gap-2">
            <span
              class="inline-flex items-center gap-1.5 rounded-full border border-talon-gold/40 bg-talon-gold/10 px-2.5 py-1 text-[11px] text-cream"
            >
              <span aria-hidden="true">↻</span>
              {rollingPresetLabel(preset)}
            </span>
            <button
              type="button"
              class="text-[11px] text-crown-ash underline hover:text-cream"
              onclick={() => useSpecificDates(p.key, preset)}
            >
              {translate("thread.propose.dateRangeCustomize", $locale)}
            </button>
          </div>
        {:else}
          {@const range = getDateRange(p.key)}
          <div class="flex gap-2">
            <input
              type="date"
              value={range.startDate}
              oninput={(e) =>
                setDateRange(p.key, {
                  ...getDateRange(p.key),
                  startDate: e.currentTarget.value,
                })}
              class="flex-1 rounded border border-plumage bg-obsidian-light px-2 py-1 text-[12px] text-cream"
            />
            <input
              type="date"
              value={range.endDate}
              oninput={(e) =>
                setDateRange(p.key, {
                  ...getDateRange(p.key),
                  endDate: e.currentTarget.value,
                })}
              class="flex-1 rounded border border-plumage bg-obsidian-light px-2 py-1 text-[12px] text-cream"
            />
          </div>
          {#if p.key === "date_range"}
            <button
              type="button"
              class="mt-1 self-start text-[11px] text-crown-ash underline hover:text-cream"
              onclick={() => useRollingWindow(p.key, "last_7_days")}
            >
              {translate("thread.propose.dateRangeUseRolling", $locale)}
            </button>
          {/if}
        {/if}
      {:else if p.type === T.INTEGRATION_SELECTOR}
        {#if p.key === "source_group"}
          {@const selected = getStringList(p.key)}
          <div class="flex flex-wrap gap-1.5">
            {#if sourceInstallations.length === 0}
              <span class="text-[11px] text-crown-ash-dark">
                {installationsLoading
                  ? translate("thread.propose.loadingIntegrations", $locale)
                  : translate("thread.propose.noIntegrations", $locale)}
              </span>
              <a
                href={resolve("/admin/integrations")}
                class="text-[11px] text-energy underline-offset-2 hover:underline"
              >
                {translate("thread.propose.createSourceGroup", $locale)}
              </a>
            {:else}
              {#each sourceInstallations as inst (inst.id)}
                {@const checked = selected.includes(inst.id)}
                <button
                  type="button"
                  aria-pressed={checked}
                  class="rounded-full border px-2.5 py-1 text-[11px] {checked
                    ? 'border-talon-gold text-cream'
                    : 'border-plumage text-crown-ash hover:border-talon-gold hover:text-cream'}"
                  onclick={() => toggleStringListValue(p.key, inst.id)}
                >
                  {inst.displayName || inst.id}
                </button>
              {/each}
            {/if}
          </div>
        {:else}
          <select
            bind:value={values[p.key]}
            class="rounded border border-plumage bg-obsidian-light px-2 py-1 text-[12px] text-cream"
          >
            {#if installations.length === 0}
              <option value="" disabled
                >{translate("thread.propose.noIntegrations", $locale)}</option
              >
            {:else}
              <option value="">—</option>
              {#each installations as inst (inst.id)}
                <option value={inst.id}>{inst.displayName || inst.id}</option>
              {/each}
            {/if}
          </select>
        {/if}
      {:else}
        <!-- TEXT, UNSPECIFIED fall back to text input -->
        <input
          type="text"
          bind:value={values[p.key]}
          class="rounded border border-plumage bg-obsidian-light px-2 py-1 text-[12px] text-cream"
        />
        {#if p.key === "theme" || p.key === "topics_to_avoid"}
          <div class="mt-1 flex flex-wrap gap-1.5">
            {#each chipValues(p.key) as chip (chip)}
              <button
                type="button"
                aria-label={translate("thread.propose.removeChip", $locale, {
                  value: chip,
                })}
                class="rounded-full border border-plumage px-2 py-0.5 text-[11px] text-crown-ash hover:border-talon-gold hover:text-cream"
                onclick={() => removeChipValue(p.key, chip)}
              >
                {chip}
                <span aria-hidden="true"> ×</span>
              </button>
            {/each}
          </div>
        {/if}
      {/if}
    </label>
  {/each}
</div>
