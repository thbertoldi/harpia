<script lang="ts">
  import { AlertTriangle, ArrowLeft, Loader2 } from "lucide-svelte";
  import { page } from "$app/state";
  import { resolve } from "$app/paths";
  import ConfigurationSummaryCard from "$lib/components/ConfigurationSummaryCard.svelte";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import {
    buildConfigCostSummary,
    loadExecutorSkuCatalog,
    type ConfigCostSummary,
  } from "$lib/plans/config-summary";
  import { getPlanConfigDraft } from "$lib/plans/plan-config-draft";
  import {
    loadPlanTemplate,
    type PlanTemplateSource,
  } from "$lib/plans/plan-template";
  import type { PlanTemplate } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { locale, translate } from "$lib/i18n";

  let template = $state<PlanTemplate | null>(null);
  let summary = $state<ConfigCostSummary | null>(null);
  let loading = $state(true);
  let loadError = $state<string | null>(null);
  let templateSource = $state<PlanTemplateSource>("mock");
  let priceSource = $state<"api" | "mock">("mock");
  let priceWarning = $state<string | null>(null);

  $effect(() => {
    void fetchSummary(page.params.templateId);
  });

  async function fetchSummary(templateId: string) {
    loading = true;
    loadError = null;
    priceWarning = null;

    try {
      const [templateResult, skuCatalog] = await Promise.all([
        loadPlanTemplate(templateId),
        loadExecutorSkuCatalog(),
      ]);

      template = templateResult.template;
      templateSource = templateResult.source;
      priceSource = skuCatalog.source;

      if (templateResult.error && templateResult.source === "mock") {
        priceWarning = templateResult.error;
      }
      if (skuCatalog.error && skuCatalog.source === "mock") {
        priceWarning = priceWarning
          ? `${priceWarning} ${skuCatalog.error}`
          : skuCatalog.error;
      }

      const draft = getPlanConfigDraft(templateId);
      summary = buildConfigCostSummary(
        templateResult.template,
        draft,
        skuCatalog.skus,
      );
    } catch (error) {
      template = null;
      summary = null;
      loadError =
        error instanceof Error
          ? error.message
          : translate("plans.summary.loadError", $locale);
    } finally {
      loading = false;
    }
  }
</script>

<div class="mx-auto max-w-4xl px-4 py-6 lg:px-6">
  <a
    href={resolve(`/plans/${page.params.templateId}`)}
    class="mb-4 inline-flex items-center gap-1 font-body text-sm text-crown-ash transition-colors hover:text-talon-gold"
  >
    <ArrowLeft class="size-4" />
    {translate("plans.summary.backToTemplate", $locale)}
  </a>

  {#if loading}
    <div class="flex min-h-[40vh] items-center justify-center">
      <div class="flex items-center gap-2 text-crown-ash">
        <Loader2 class="size-5 animate-spin" />
        <span class="font-body text-sm"
          >{translate("plans.summary.loading", $locale)}</span
        >
      </div>
    </div>
  {:else if !template || !summary}
    <div class="flex min-h-[40vh] flex-col items-center justify-center">
      <AlertTriangle class="mb-3 size-10 text-red-400" />
      <p class="font-body text-sm text-red-400">
        {translate("plans.summary.loadError", $locale)}
      </p>
      {#if loadError}
        <p class="mt-1 font-mono text-xs text-crown-ash">{loadError}</p>
      {/if}
      <button
        onclick={() => fetchSummary(page.params.templateId)}
        class="mt-4 cursor-pointer rounded-md border border-plumage px-4 py-2 font-body text-sm text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
      >
        {translate("plans.summary.retry", $locale)}
      </button>
    </div>
  {:else}
    <div class="mb-6">
      <HarpyHeading tag="h1" class="text-2xl text-cream">
        {translate("plans.summary.pageTitle", $locale)}
      </HarpyHeading>
      <p class="mt-1 font-body text-sm text-crown-ash">
        {template.name}
      </p>
      <p class="mt-2 font-mono text-[10px] text-crown-ash-dark">
        {template.key} · {templateSource === "api"
          ? translate("plans.detail.liveApi", $locale)
          : translate("plans.detail.mockData", $locale)}
      </p>
    </div>

    {#if priceWarning}
      <div
        class="mb-4 rounded-md border border-talon-gold/30 bg-talon-gold/5 px-3 py-2"
      >
        <p class="font-mono text-xs text-talon-gold">
          {translate("plans.summary.mockFallback", $locale)}
          {priceWarning}
        </p>
      </div>
    {/if}

    <ConfigurationSummaryCard {summary} {priceSource} />

    <p class="mt-4 font-body text-xs text-crown-ash">
      {translate("plans.summary.saveHint", $locale)}
    </p>
  {/if}
</div>
