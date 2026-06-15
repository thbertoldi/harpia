<script lang="ts">
  import {
    AlertTriangle,
    ArrowLeft,
    CheckCircle2,
    Info,
    Loader2,
    Save,
    Sparkles,
  } from "lucide-svelte";
  import { page } from "$app/state";
  import { resolve } from "$app/paths";
  import { formatArtifactTypeLabel } from "$lib/plans/artifact-flow";
  import {
    getCompatibleInstallationsForStep,
    loadSlotBindingPageData,
    savePlanConfiguration,
    selectionsToSlotBindings,
    slotBindingsToSelections,
    validateSlotBindings,
    type CompatibleInstallationOption,
  } from "$lib/plans/slot-binding";
  import type { SkuLockReason } from "$lib/plans/plan-catalog";
  import { PlanConfigurationStatus } from "$lib/gen/harpia/plans/v1/plans_pb";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import { locale, translate } from "$lib/i18n";

  let template = $state<
    Awaited<ReturnType<typeof loadSlotBindingPageData>>["template"] | null
  >(null);
  let context = $state<
    Awaited<ReturnType<typeof loadSlotBindingPageData>>["context"] | null
  >(null);
  let configurationStatus = $state<PlanConfigurationStatus>(
    PlanConfigurationStatus.DRAFT,
  );
  let selections = $state<Record<string, string>>({});
  let loading = $state(true);
  let saving = $state(false);
  let promoting = $state(false);
  let loadError = $state<string | null>(null);
  let saveError = $state<string | null>(null);
  let saveNotice = $state<string | null>(null);
  let dataSource = $state<"api" | "mock">("mock");

  const validation = $derived.by(() => {
    if (!template || !context) {
      return null;
    }
    return validateSlotBindings(
      template,
      selectionsToSlotBindings(template, selections, context),
      context,
    );
  });

  $effect(() => {
    void fetchPageData(page.params.templateId);
  });

  async function fetchPageData(templateId: string) {
    loading = true;
    loadError = null;
    saveNotice = null;

    try {
      const result = await loadSlotBindingPageData(templateId, $locale);
      template = result.template;
      context = result.context;
      dataSource = result.source;
      configurationStatus =
        result.configuration?.status ?? PlanConfigurationStatus.DRAFT;
      selections = result.configuration
        ? slotBindingsToSelections(result.configuration.slotBindings)
        : {};
      if (result.error) {
        loadError = result.error;
      }
    } catch (error) {
      template = null;
      context = null;
      loadError =
        error instanceof Error
          ? error.message
          : translate("plans.configure.loadError", $locale);
    } finally {
      loading = false;
    }
  }

  function skuStatusLabel(reason: SkuLockReason): string {
    return translate(`plans.lock.${reason}`, $locale);
  }

  function skuStatusClass(reason: SkuLockReason): string {
    if (reason === "available") {
      return "border-green-500/40 bg-green-500/10 text-green-400";
    }
    if (reason === "missing_entitlement") {
      return "border-red-400/40 bg-red-400/10 text-red-400";
    }
    return "border-talon-gold/40 bg-talon-gold/10 text-talon-gold";
  }

  function stepOptions(stepKey: string): CompatibleInstallationOption[] {
    if (!template || !context) return [];
    const step = template.steps.find((entry) => entry.key === stepKey);
    if (!step) return [];
    return getCompatibleInstallationsForStep(step, context);
  }

  async function persistConfiguration(
    status: PlanConfigurationStatus,
  ): Promise<boolean> {
    if (!template || !context) return false;

    saveError = null;
    saveNotice = null;

    if (status === PlanConfigurationStatus.RUNNABLE) {
      promoting = true;
    } else {
      saving = true;
    }

    try {
      const result = await savePlanConfiguration(
        template,
        selections,
        context,
        status,
      );
      configurationStatus = result.configuration.status;
      selections = slotBindingsToSelections(result.configuration.slotBindings);

      if (status === PlanConfigurationStatus.RUNNABLE) {
        saveNotice = translate("plans.configure.promoted", $locale);
      } else {
        saveNotice = translate("plans.configure.savedDraft", $locale);
      }

      if (result.error) {
        loadError = result.error;
      }

      return true;
    } catch (error) {
      saveError =
        error instanceof Error
          ? error.message
          : translate("plans.configure.saveError", $locale);
      return false;
    } finally {
      saving = false;
      promoting = false;
    }
  }

  async function saveDraft() {
    await persistConfiguration(PlanConfigurationStatus.DRAFT);
  }

  async function promoteToRunnable() {
    await persistConfiguration(PlanConfigurationStatus.RUNNABLE);
  }
</script>

<div class="mx-auto max-w-5xl px-4 py-6 lg:px-6">
  <div class="mb-6 flex flex-wrap items-center justify-between gap-3">
    <a
      href={resolve("/plans")}
      class="inline-flex items-center gap-2 font-body text-sm text-crown-ash transition-colors hover:text-talon-gold"
    >
      <ArrowLeft class="size-4" />
      {translate("plans.configure.backToCatalog", $locale)}
    </a>
    <span
      class="rounded-full border border-plumage px-2.5 py-1 font-mono text-[10px] tracking-wider text-crown-ash uppercase"
    >
      {dataSource === "api"
        ? translate("plans.source.live", $locale)
        : translate("plans.source.mock", $locale)}
    </span>
  </div>

  {#if loading}
    <div class="flex min-h-[40vh] items-center justify-center">
      <div class="flex items-center gap-2 text-crown-ash">
        <Loader2 class="size-5 animate-spin" />
        <span class="font-body text-sm"
          >{translate("plans.configure.loading", $locale)}</span
        >
      </div>
    </div>
  {:else if !template || !context}
    <div class="flex min-h-[40vh] flex-col items-center justify-center">
      <AlertTriangle class="mb-3 size-10 text-red-400" />
      <p class="font-body text-sm text-red-400">
        {translate("plans.configure.loadError", $locale)}
      </p>
      {#if loadError}
        <p class="mt-1 font-mono text-xs text-crown-ash">{loadError}</p>
      {/if}
      <button
        onclick={() => fetchPageData(page.params.templateId)}
        class="mt-4 cursor-pointer rounded-md border border-plumage px-4 py-2 font-body text-sm text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
      >
        {translate("plans.retry", $locale)}
      </button>
    </div>
  {:else}
    <div class="mb-6 flex flex-wrap items-start justify-between gap-4">
      <div>
        <HarpyHeading tag="h1" class="text-2xl text-cream">
          {translate("plans.configure.heading", $locale)}
        </HarpyHeading>
        <p class="mt-1 font-body text-sm text-crown-ash">
          {template.name}
        </p>
        <p class="mt-2 max-w-2xl font-body text-sm text-crown-ash">
          {translate("plans.configure.subheading", $locale)}
        </p>
      </div>
      <span
        class="inline-flex items-center gap-1 rounded-full border px-2.5 py-1 font-mono text-[10px] tracking-wide uppercase {configurationStatus ===
        PlanConfigurationStatus.RUNNABLE
          ? 'border-green-500/40 bg-green-500/10 text-green-400'
          : 'border-talon-gold/40 bg-talon-gold/10 text-talon-gold'}"
      >
        {#if configurationStatus === PlanConfigurationStatus.RUNNABLE}
          <CheckCircle2 class="size-3" />
          {translate("plans.configure.status.runnable", $locale)}
        {:else}
          {translate("plans.configure.status.draft", $locale)}
        {/if}
      </span>
    </div>

    {#if loadError}
      <div
        class="mb-4 rounded-md border border-talon-gold/30 bg-talon-gold/5 px-3 py-2"
      >
        <p class="font-mono text-xs text-talon-gold">
          {translate("plans.apiFallback", $locale)}
          {loadError}
        </p>
      </div>
    {/if}

    {#if saveNotice}
      <div
        class="mb-4 rounded-md border border-green-500/30 bg-green-500/5 px-3 py-2"
      >
        <p class="font-body text-sm text-green-400">{saveNotice}</p>
      </div>
    {/if}

    {#if saveError}
      <div
        class="mb-4 rounded-md border border-red-400/30 bg-red-400/5 px-3 py-2"
      >
        <p class="font-body text-sm text-red-400">{saveError}</p>
      </div>
    {/if}

    {#if validation && validation.draftWarnings.length > 0}
      <div
        class="mb-4 rounded-md border border-talon-gold/30 bg-talon-gold/5 px-3 py-2"
      >
        <p class="mb-2 font-heading text-sm font-semibold text-talon-gold">
          {translate("plans.configure.warningsHeading", $locale)}
        </p>
        <ul class="space-y-1">
          {#each validation.draftWarnings as warning, index (index)}
            <li class="flex items-start gap-2 font-body text-xs text-crown-ash">
              <Info class="mt-0.5 size-3 shrink-0 text-talon-gold" />
              <span
                >{warning.startsWith("plans.")
                  ? translate(warning, $locale)
                  : warning}</span
              >
            </li>
          {/each}
        </ul>
      </div>
    {/if}

    <div class="space-y-4">
      {#each template.steps as step (step.key)}
        {@const options = stepOptions(step.key)}
        {@const selectedOption = options.find(
          (option) => option.installation.id === selections[step.key],
        )}
        <section
          class="rounded-lg border border-plumage bg-obsidian-light/40 p-4"
        >
          <div class="mb-3 flex flex-wrap items-start justify-between gap-3">
            <div>
              <HarpyHeading tag="h2" class="text-lg text-cream">
                {step.title}
              </HarpyHeading>
              <p class="font-mono text-[10px] text-crown-ash-dark">
                {step.key}
              </p>
              <p class="mt-1 font-body text-sm text-crown-ash">
                {step.description}
              </p>
            </div>
            <div class="text-right font-mono text-[10px] text-crown-ash">
              <p>
                {translate("plans.detail.stepInput", $locale)}:
                {formatArtifactTypeLabel(step.inputArtifactTypeId, $locale)}
              </p>
              <p class="text-talon-gold">
                {translate("plans.detail.stepOutput", $locale)}:
                {formatArtifactTypeLabel(step.outputArtifactTypeId, $locale)}
              </p>
            </div>
          </div>

          <div class="mb-3 rounded-md border border-plumage/60 px-3 py-2">
            <p
              class="mb-1 font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase"
            >
              {translate("plans.configure.requiredSku", $locale)}
            </p>
            <p class="font-heading text-sm font-semibold text-cream">
              {step.defaultExecutorSkuKey}
            </p>
          </div>

          <label
            class="mb-2 block font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase"
          >
            {translate("plans.configure.installationLabel", $locale)}
          </label>
          <select
            class="mb-3 w-full rounded-md border border-plumage bg-obsidian px-3 py-2 font-body text-sm text-cream transition-colors outline-none focus:border-talon-gold"
            value={selections[step.key] ?? ""}
            onchange={(event) => {
              const value = event.currentTarget.value;
              if (value) {
                selections = { ...selections, [step.key]: value };
              } else {
                const next = { ...selections };
                delete next[step.key];
                selections = next;
              }
            }}
          >
            <option value="">
              {translate("plans.configure.selectPlaceholder", $locale)}
            </option>
            {#each options as option (option.installation.id)}
              <option value={option.installation.id}>
                {option.installation.displayName} · {option.sku.key}
                {#if !option.ready}
                  ({skuStatusLabel(option.lockReason)})
                {/if}
              </option>
            {/each}
          </select>

          {#if options.length === 0}
            <p
              class="mb-3 flex items-start gap-2 font-body text-xs text-talon-gold"
            >
              <AlertTriangle class="mt-0.5 size-3 shrink-0" />
              <span>{translate("plans.configure.noCompatible", $locale)}</span>
            </p>
          {/if}

          {#if selectedOption}
            <div
              class="rounded-md border px-3 py-2 {skuStatusClass(
                selectedOption.lockReason,
              )}"
            >
              <div class="flex flex-wrap items-start justify-between gap-2">
                <div>
                  <p class="font-heading text-sm font-semibold">
                    {selectedOption.sku.displayName}
                  </p>
                  <p class="font-mono text-[10px] opacity-80">
                    {selectedOption.sku.key}
                  </p>
                  <p class="mt-1 font-body text-xs opacity-90">
                    {selectedOption.installation.displayName}
                  </p>
                </div>
                <span class="font-mono text-[10px] tracking-wide uppercase">
                  {skuStatusLabel(selectedOption.lockReason)}
                </span>
              </div>
              {#if selectedOption.lockMessageKey}
                <p class="mt-2 flex items-start gap-1 font-body text-xs">
                  <Info class="mt-0.5 size-3 shrink-0" />
                  <span
                    >{translate(
                      selectedOption.lockMessageKey,
                      $locale,
                      selectedOption.lockMessageParams,
                    )}</span
                  >
                </p>
              {/if}
            </div>
          {/if}
        </section>
      {/each}
    </div>

    <div
      class="mt-6 flex flex-wrap items-center justify-between gap-3 border-t border-plumage/40 pt-4"
    >
      <p class="max-w-xl font-body text-xs text-crown-ash">
        {translate("plans.configure.draftHint", $locale)}
      </p>
      <div class="flex flex-wrap items-center gap-2">
        <button
          type="button"
          onclick={() => saveDraft()}
          disabled={saving || promoting}
          class="inline-flex cursor-pointer items-center gap-2 rounded-md border border-plumage px-4 py-2 font-body text-sm text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold disabled:cursor-not-allowed disabled:opacity-60"
        >
          {#if saving}
            <Loader2 class="size-4 animate-spin" />
          {:else}
            <Save class="size-4" />
          {/if}
          {translate("plans.configure.saveDraft", $locale)}
        </button>
        <button
          type="button"
          onclick={() => promoteToRunnable()}
          disabled={saving || promoting || !validation?.canPromoteToRunnable}
          title={validation?.canPromoteToRunnable
            ? translate("plans.configure.promoteReady", $locale)
            : translate("plans.configure.promoteBlocked", $locale)}
          class="inline-flex cursor-pointer items-center gap-2 rounded-md border border-talon-gold/60 bg-talon-gold/10 px-4 py-2 font-body text-sm text-talon-gold transition-colors hover:border-talon-gold hover:bg-talon-gold/20 disabled:cursor-not-allowed disabled:border-plumage/60 disabled:bg-transparent disabled:text-crown-ash-dark"
        >
          {#if promoting}
            <Loader2 class="size-4 animate-spin" />
          {:else}
            <Sparkles class="size-4" />
          {/if}
          {translate("plans.configure.promote", $locale)}
        </button>
      </div>
    </div>
  {/if}
</div>
