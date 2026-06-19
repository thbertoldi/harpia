<script lang="ts">
  import { AlertTriangle, ArrowLeft, Loader2 } from "lucide-svelte";
  import { page } from "$app/state";
  import { resolve } from "$app/paths";
  import PlanPoliciesForm from "$lib/components/PlanPoliciesForm.svelte";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import { toUserMessage } from "$lib/connect-errors";
  import { locale, translate } from "$lib/i18n";
  import {
    loadBehaviorPoliciesForTemplate,
    saveBehaviorPoliciesForTemplate,
    type BehaviorPoliciesFormValues,
  } from "$lib/plans/behavior-policies";
  import { loadPlanTemplate } from "$lib/plans/plan-template";
  import type {
    PlanConfiguration,
    PlanTemplate,
  } from "$lib/gen/harpia/plans/v1/plans_pb";

  let loading = $state(true);
  let loadError = $state<string | null>(null);
  let saveError = $state<string | null>(null);
  let saveNotice = $state<string | null>(null);
  let saving = $state(false);
  let template = $state<PlanTemplate | null>(null);
  let configuration = $state<PlanConfiguration | null>(null);
  let policies = $state<BehaviorPoliciesFormValues | null>(null);

  $effect(() => {
    const templateId = page.params.templateId;
    if (templateId) {
      void loadPage(templateId);
    }
  });

  async function loadPage(templateIdOrKey: string) {
    loading = true;
    loadError = null;
    saveError = null;
    saveNotice = null;

    try {
      const templateResult = await loadPlanTemplate(templateIdOrKey, $locale);
      const policiesResult = await loadBehaviorPoliciesForTemplate(
        templateResult.template.id,
      );

      template = templateResult.template;
      configuration = policiesResult.configuration;
      policies = policiesResult.policies;
    } catch (error) {
      template = null;
      configuration = null;
      policies = null;
      loadError =
        error instanceof Error
          ? error.message
          : translate("plans.policies.loadError", $locale);
    } finally {
      loading = false;
    }
  }

  async function handleSave() {
    if (!template || !policies) {
      return;
    }

    saving = true;
    saveError = null;
    saveNotice = null;

    try {
      const result = await saveBehaviorPoliciesForTemplate(
        template,
        policies,
        configuration,
      );

      configuration = result.configuration;
      saveNotice = translate("plans.policies.saved", $locale);
    } catch (error) {
      const message = error instanceof Error ? error.message : "";
      saveError = message.startsWith("plans.policies.")
        ? translate(message, $locale)
        : toUserMessage(error);
    } finally {
      saving = false;
    }
  }
</script>

<div class="mx-auto max-w-3xl px-4 py-6 lg:px-6">
  <a
    href={resolve(`/plans/${page.params.templateId}`)}
    class="mb-4 inline-flex items-center gap-2 font-body text-sm text-crown-ash transition-colors hover:text-talon-gold"
  >
    <ArrowLeft class="size-4" />
    {translate("plans.policies.backToTemplate", $locale)}
  </a>

  <div class="mb-6">
    <HarpyHeading tag="h1" class="text-2xl text-cream">
      {translate("plans.policies.heading", $locale)}
    </HarpyHeading>
    <p class="mt-1 font-body text-sm text-crown-ash">
      {translate("plans.policies.subheading", $locale, {
        template: template?.name || page.params.templateId || "",
      })}
    </p>
  </div>

  {#if loading}
    <div class="flex min-h-[30vh] items-center justify-center">
      <div class="flex items-center gap-2 text-crown-ash">
        <Loader2 class="size-5 animate-spin" />
        <span class="font-body text-sm"
          >{translate("plans.policies.loading", $locale)}</span
        >
      </div>
    </div>
  {:else if !policies}
    <div class="flex min-h-[30vh] flex-col items-center justify-center">
      <AlertTriangle class="mb-3 size-10 text-red-400" />
      <p class="font-body text-sm text-red-400">
        {translate("plans.policies.loadError", $locale)}
      </p>
      {#if loadError}
        <p class="mt-1 font-mono text-xs text-crown-ash">{loadError}</p>
      {/if}
      <button
        onclick={() => {
          const templateId = page.params.templateId;
          if (templateId) {
            void loadPage(templateId);
          }
        }}
        class="mt-4 cursor-pointer rounded-md border border-plumage px-4 py-2 font-body text-sm text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
      >
        {translate("plans.retry", $locale)}
      </button>
    </div>
  {:else}
    <PlanPoliciesForm
      bind:values={policies}
      {saving}
      {saveError}
      {saveNotice}
      onSave={handleSave}
    />
  {/if}
</div>
