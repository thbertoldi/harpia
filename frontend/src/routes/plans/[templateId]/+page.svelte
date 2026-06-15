<script lang="ts">
  import {
    AlertTriangle,
    Loader2,
    Map,
    Receipt,
    Settings2,
    UserCheck,
  } from "lucide-svelte";
  import { page } from "$app/state";
  import { resolve } from "$app/paths";
  import {
    loadPlanTemplate,
    type PlanTemplateSource,
  } from "$lib/plans/plan-template";
  import type { PlanTemplate } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { formatArtifactTypeLabel } from "$lib/plans/artifact-flow";
  import { getAgentBackedSteps } from "$lib/plans/overseer-binding";
  import PlanDagDiagram from "$lib/components/PlanDagDiagram.svelte";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import { locale, translate } from "$lib/i18n";

  let template = $state<PlanTemplate | null>(null);
  let loading = $state(true);
  let loadError = $state<string | null>(null);
  let dataSource = $state<PlanTemplateSource>("mock");

  $effect(() => {
    void fetchTemplate(page.params.templateId);
  });

  async function fetchTemplate(templateId: string) {
    loading = true;
    loadError = null;

    try {
      const result = await loadPlanTemplate(templateId);
      template = result.template;
      dataSource = result.source;
      if (result.error && result.source === "mock") {
        loadError = result.error;
      }
    } catch (error) {
      template = null;
      loadError =
        error instanceof Error
          ? error.message
          : translate("plans.detail.loadError", $locale);
    } finally {
      loading = false;
    }
  }
</script>

<div class="mx-auto max-w-6xl px-4 py-6 lg:px-6">
  {#if loading}
    <div class="flex min-h-[40vh] items-center justify-center">
      <div class="flex items-center gap-2 text-crown-ash">
        <Loader2 class="size-5 animate-spin" />
        <span class="font-body text-sm"
          >{translate("plans.detail.loading", $locale)}</span
        >
      </div>
    </div>
  {:else if !template}
    <div class="flex min-h-[40vh] flex-col items-center justify-center">
      <AlertTriangle class="mb-3 size-10 text-red-400" />
      <p class="font-body text-sm text-red-400">
        {translate("plans.detail.loadError", $locale)}
      </p>
      {#if loadError}
        <p class="mt-1 font-mono text-xs text-crown-ash">{loadError}</p>
      {/if}
      <button
        onclick={() => fetchTemplate(page.params.templateId)}
        class="mt-4 cursor-pointer rounded-md border border-plumage px-4 py-2 font-body text-sm text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
      >
        {translate("plans.detail.retry", $locale)}
      </button>
    </div>
  {:else}
    <div class="mb-6 flex flex-wrap items-start justify-between gap-4">
      <div>
        <HarpyHeading tag="h1" class="text-2xl text-cream">
          {template.name}
        </HarpyHeading>
        <p class="mt-1 max-w-2xl font-body text-sm text-crown-ash">
          {template.description}
        </p>
        <p class="mt-2 font-mono text-[10px] text-crown-ash-dark">
          {template.key} · v{template.version} · {template.vertical}
        </p>
      </div>
      <div class="flex flex-col items-end gap-2">
        <span
          class="rounded-full border border-plumage px-2.5 py-1 font-mono text-[10px] tracking-wider text-crown-ash uppercase"
        >
          {dataSource === "api"
            ? translate("plans.detail.liveApi", $locale)
            : translate("plans.detail.mockData", $locale)}
        </span>
        <a
          href={resolve(`/plans/${page.params.templateId}/configure/policies`)}
          class="inline-flex items-center gap-2 rounded-md border border-talon-gold/60 bg-talon-gold/10 px-3 py-1.5 font-body text-xs text-talon-gold transition-colors hover:bg-talon-gold/20"
        >
          <Settings2 class="size-3.5" />
          {translate("plans.detail.configurePolicies", $locale)}
        </a>
        <a
          href={resolve(`/plans/${page.params.templateId}/configure/summary`)}
          class="inline-flex items-center gap-1.5 rounded-md border border-plumage px-3 py-1.5 font-body text-xs text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
        >
          <Receipt class="size-3.5" />
          {translate("plans.detail.viewSummary", $locale)}
        </a>
      </div>
    </div>

    {#if loadError}
      <div
        class="mb-4 rounded-md border border-talon-gold/30 bg-talon-gold/5 px-3 py-2"
      >
        <p class="font-mono text-xs text-talon-gold">
          {translate("plans.detail.mockFallback", $locale)}
          {loadError}
        </p>
      </div>
    {/if}

    {#if getAgentBackedSteps(template).length > 0}
      <section class="mb-8">
        <div
          class="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-plumage bg-obsidian-light/40 px-4 py-3"
        >
          <div>
            <p class="font-heading text-sm font-semibold text-cream">
              {translate("plans.overseer.configureLinkTitle", $locale)}
            </p>
            <p class="mt-1 font-body text-xs text-crown-ash">
              {translate("plans.overseer.configureLinkDescription", $locale)}
            </p>
          </div>
          <a
            href={resolve(
              `/plans/${page.params.templateId}/configure/overseer`,
            )}
            class="inline-flex items-center gap-2 rounded-md border border-talon-gold/40 bg-talon-gold/10 px-3 py-1.5 font-body text-xs text-talon-gold transition-colors hover:border-talon-gold"
          >
            <UserCheck class="size-3.5" />
            {translate("plans.overseer.configureLinkAction", $locale)}
          </a>
        </div>
      </section>
    {/if}

    <section class="mb-8">
      <div class="mb-3 flex items-center gap-2">
        <Map class="size-4 text-talon-gold" />
        <HarpyHeading tag="h2" class="text-lg text-cream">
          {translate("plans.detail.dagHeading", $locale)}
        </HarpyHeading>
      </div>
      <p class="mb-4 font-body text-sm text-crown-ash">
        {translate("plans.detail.dagDescription", $locale)}
      </p>
      <PlanDagDiagram steps={template.steps} edges={template.edges} />
    </section>

    <section>
      <HarpyHeading tag="h2" class="mb-4 text-lg text-cream">
        {translate("plans.detail.stepsHeading", $locale)}
      </HarpyHeading>
      <div class="overflow-x-auto rounded-lg border border-plumage">
        <table class="w-full min-w-[720px] border-collapse">
          <thead>
            <tr
              class="border-b border-plumage bg-obsidian-light/60 font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase"
            >
              <th class="px-3 py-2 text-left"
                >{translate("plans.detail.stepTitle", $locale)}</th
              >
              <th class="px-3 py-2 text-left"
                >{translate("plans.detail.stepInput", $locale)}</th
              >
              <th class="px-3 py-2 text-left"
                >{translate("plans.detail.stepOutput", $locale)}</th
              >
            </tr>
          </thead>
          <tbody>
            {#each template.steps as step (step.key)}
              <tr class="border-b border-plumage/40">
                <td class="px-3 py-3">
                  <p class="font-heading text-sm font-semibold text-cream">
                    {step.title}
                  </p>
                  <p class="font-mono text-[10px] text-crown-ash-dark">
                    {step.key}
                  </p>
                  <p class="mt-1 font-body text-xs text-crown-ash">
                    {step.description}
                  </p>
                </td>
                <td class="px-3 py-3 font-mono text-xs text-crown-ash">
                  {formatArtifactTypeLabel(step.inputArtifactTypeId, $locale)}
                </td>
                <td class="px-3 py-3 font-mono text-xs text-talon-gold">
                  {formatArtifactTypeLabel(step.outputArtifactTypeId, $locale)}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </section>
  {/if}
</div>
