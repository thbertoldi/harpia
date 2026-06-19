<script lang="ts">
  import { AlertTriangle, ArrowLeft, Loader2 } from "lucide-svelte";
  import { page } from "$app/state";
  import { resolve } from "$app/paths";
  import { getSession } from "$lib/auth";
  import OverseerBindingPanel from "$lib/components/OverseerBindingPanel.svelte";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import type { OverseerBinding } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { PlanConfigurationStatus } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { locale, translate } from "$lib/i18n";
  import {
    loadOverseerBindingsDraft,
    saveOverseerBindingsDraft,
  } from "$lib/plans/overseer-binding";
  import {
    loadPlanTemplate,
    type PlanTemplateSource,
  } from "$lib/plans/plan-template";

  const session = getSession();

  let template = $state<
    Awaited<ReturnType<typeof loadPlanTemplate>>["template"] | null
  >(null);
  let bindings = $state<OverseerBinding[]>([]);
  let loading = $state(true);
  let loadError = $state<string | null>(null);
  let dataSource = $state<PlanTemplateSource>("api");

  $effect(() => {
    void fetchTemplate(page.params.templateId);
  });

  async function fetchTemplate(templateId: string) {
    loading = true;
    loadError = null;

    try {
      const result = await loadPlanTemplate(templateId, $locale);
      template = result.template;
      dataSource = result.source;
      bindings = loadOverseerBindingsDraft(templateId);
      if (result.error && result.source === "mock") {
        loadError = result.error;
      }
    } catch (error) {
      template = null;
      loadError =
        error instanceof Error
          ? error.message
          : translate("plans.overseer.loadError", $locale);
    } finally {
      loading = false;
    }
  }

  async function handleSaveDraft(nextBindings: OverseerBinding[]) {
    bindings = nextBindings;
    saveOverseerBindingsDraft(page.params.templateId, nextBindings);
  }

  async function handlePromote(
    nextBindings: OverseerBinding[],
    status: PlanConfigurationStatus,
  ) {
    bindings = nextBindings;
    saveOverseerBindingsDraft(page.params.templateId, nextBindings);
    void status;
  }
</script>

<div class="mx-auto max-w-4xl px-4 py-6 lg:px-6">
  <a
    href={resolve(`/plans/${page.params.templateId}`)}
    class="mb-4 inline-flex items-center gap-2 font-body text-sm text-crown-ash transition-colors hover:text-talon-gold"
  >
    <ArrowLeft class="size-4" />
    {translate("plans.overseer.backToTemplate", $locale)}
  </a>

  {#if loading}
    <div class="flex min-h-[40vh] items-center justify-center">
      <div class="flex items-center gap-2 text-crown-ash">
        <Loader2 class="size-5 animate-spin" />
        <span class="font-body text-sm"
          >{translate("plans.overseer.loading", $locale)}</span
        >
      </div>
    </div>
  {:else if !template || !session}
    <div class="flex min-h-[40vh] flex-col items-center justify-center">
      <AlertTriangle class="mb-3 size-10 text-red-400" />
      <p class="font-body text-sm text-red-400">
        {!session
          ? translate("plans.overseer.sessionRequired", $locale)
          : translate("plans.overseer.loadError", $locale)}
      </p>
      {#if loadError}
        <p class="mt-1 font-mono text-xs text-crown-ash">{loadError}</p>
      {/if}
    </div>
  {:else}
    <div class="mb-6 flex flex-wrap items-start justify-between gap-4">
      <div>
        <HarpyHeading tag="h1" class="text-2xl text-cream">
          {translate("plans.overseer.heading", $locale)}
        </HarpyHeading>
        <p class="mt-1 max-w-2xl font-body text-sm text-crown-ash">
          {translate("plans.overseer.subheading", $locale, {
            plan: template.name,
          })}
        </p>
      </div>
      {#if dataSource === "api"}
        <span
          class="rounded-full border border-plumage px-2.5 py-1 font-mono text-[10px] tracking-wider text-crown-ash uppercase"
        >
          {translate("plans.detail.liveApi", $locale)}
        </span>
      {/if}
    </div>

    {#if loadError && dataSource === "mock"}
      <div
        class="mb-4 rounded-md border border-talon-gold/30 bg-talon-gold/5 px-3 py-2"
      >
        <p class="font-mono text-xs text-talon-gold">
          {translate("plans.detail.mockFallback", $locale)}
          {loadError}
        </p>
      </div>
    {/if}

    <OverseerBindingPanel
      {template}
      bind:bindings
      sessionUser={session.user}
      onSaveDraft={handleSaveDraft}
      onPromote={handlePromote}
    />
  {/if}
</div>
