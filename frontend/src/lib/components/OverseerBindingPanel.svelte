<script lang="ts">
  import { AlertTriangle, CheckCircle2, UserCheck } from "lucide-svelte";
  import type { User } from "$lib/auth";
  import type {
    OverseerBinding,
    PlanTemplate,
  } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { PlanConfigurationStatus } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { locale, translate } from "$lib/i18n";
  import {
    assignSelfAsOverseerForSteps,
    createSelfOverseerBinding,
    formatOverseerPromotionErrors,
    getDraftOverseerWarning,
    getOverseerBindingForStep,
    getOverseerRequiredSteps,
    removeOverseerBinding,
    upsertOverseerBinding,
    validateOverseerBindings,
  } from "$lib/plans/overseer-binding";

  let {
    template,
    bindings = $bindable([]),
    sessionUser,
    onSaveDraft,
    onPromote,
  }: {
    template: PlanTemplate;
    bindings?: OverseerBinding[];
    sessionUser: User;
    onSaveDraft?: (bindings: OverseerBinding[]) => void | Promise<void>;
    onPromote?: (
      bindings: OverseerBinding[],
      status: PlanConfigurationStatus,
    ) => void | Promise<void>;
  } = $props();

  let saveMessage = $state<string | null>(null);
  let saveError = $state<string | null>(null);
  let promoteError = $state<string | null>(null);

  const requiredSteps = $derived(getOverseerRequiredSteps(template));
  const draftWarning = $derived(getDraftOverseerWarning(template, bindings));
  const runnableValidation = $derived(
    validateOverseerBindings(
      template,
      bindings,
      PlanConfigurationStatus.RUNNABLE,
    ),
  );

  function isSelfAssigned(stepKey: string): boolean {
    const binding = getOverseerBindingForStep(bindings, stepKey);
    return binding?.overseerUserId === sessionUser.sub;
  }

  function toggleSelfAssignment(stepKey: string): void {
    saveMessage = null;
    saveError = null;
    promoteError = null;

    if (isSelfAssigned(stepKey)) {
      bindings = removeOverseerBinding(bindings, stepKey);
      return;
    }

    bindings = upsertOverseerBinding(
      bindings,
      createSelfOverseerBinding(stepKey, sessionUser),
    );
  }

  function assignSelfToAll(): void {
    bindings = assignSelfAsOverseerForSteps(requiredSteps, sessionUser);
    saveMessage = null;
    saveError = null;
    promoteError = null;
  }

  async function handleSaveDraft(): Promise<void> {
    saveMessage = null;
    saveError = null;
    promoteError = null;

    try {
      await onSaveDraft?.(bindings);
      saveMessage = translate("plans.overseer.savedDraft", $locale);
    } catch (error) {
      saveError =
        error instanceof Error
          ? error.message
          : translate("plans.overseer.saveError", $locale);
    }
  }

  async function handlePromote(status: PlanConfigurationStatus): Promise<void> {
    saveMessage = null;
    saveError = null;
    promoteError = null;

    const validation = validateOverseerBindings(template, bindings, status);
    if (!validation.ok) {
      promoteError = formatOverseerPromotionErrors(validation.issues).join(" ");
      return;
    }

    try {
      await onPromote?.(bindings, status);
      saveMessage =
        status === PlanConfigurationStatus.SCHEDULED
          ? translate("plans.overseer.promotedScheduled", $locale)
          : translate("plans.overseer.promotedRunnable", $locale);
    } catch (error) {
      promoteError =
        error instanceof Error
          ? error.message
          : translate("plans.overseer.promoteError", $locale);
    }
  }
</script>

<div class="space-y-4">
  {#if requiredSteps.length === 0}
    <p class="font-body text-sm text-crown-ash">
      {translate("plans.overseer.noAgentSteps", $locale)}
    </p>
  {:else}
    <div class="flex flex-wrap items-center justify-between gap-3">
      <p class="font-body text-sm text-crown-ash">
        {translate("plans.overseer.selfOnlyHint", $locale)}
      </p>
      <button
        type="button"
        onclick={assignSelfToAll}
        class="cursor-pointer rounded-md border border-plumage px-3 py-1.5 font-body text-xs text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
      >
        {translate("plans.overseer.assignAllSelf", $locale)}
      </button>
    </div>

    {#if draftWarning}
      <div
        class="rounded-md border border-talon-gold/30 bg-talon-gold/5 px-3 py-2"
      >
        <p class="flex items-start gap-2 font-body text-sm text-talon-gold">
          <AlertTriangle class="mt-0.5 size-4 shrink-0" />
          <span>
            {translate("plans.overseer.draftWarning", $locale)}
            {draftWarning}
          </span>
        </p>
      </div>
    {/if}

    <ul class="space-y-3">
      {#each requiredSteps as step (step.stepKey)}
        {@const assigned = isSelfAssigned(step.stepKey)}
        <li
          class="rounded-lg border border-plumage bg-obsidian-light/40 px-4 py-3"
        >
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div>
              <p class="font-heading text-sm font-semibold text-cream">
                {step.title}
              </p>
              <p class="font-mono text-[10px] text-crown-ash-dark">
                {step.stepKey}
              </p>
            </div>
            <button
              type="button"
              aria-pressed={assigned}
              onclick={() => toggleSelfAssignment(step.stepKey)}
              class="inline-flex cursor-pointer items-center gap-2 rounded-md border px-3 py-1.5 font-body text-xs transition-colors {assigned
                ? 'border-green-500/40 bg-green-500/10 text-green-400'
                : 'border-plumage text-crown-ash hover:border-talon-gold hover:text-talon-gold'}"
            >
              {#if assigned}
                <CheckCircle2 class="size-3.5" />
                {translate("plans.overseer.assignedSelf", $locale, {
                  name: sessionUser.name,
                })}
              {:else}
                <UserCheck class="size-3.5" />
                {translate("plans.overseer.assignSelf", $locale)}
              {/if}
            </button>
          </div>
        </li>
      {/each}
    </ul>

    <div class="flex flex-wrap gap-3 border-t border-plumage/40 pt-4">
      <button
        type="button"
        onclick={handleSaveDraft}
        class="cursor-pointer rounded-md border border-plumage px-4 py-2 font-body text-sm text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
      >
        {translate("plans.overseer.saveDraft", $locale)}
      </button>
      <button
        type="button"
        onclick={() => handlePromote(PlanConfigurationStatus.RUNNABLE)}
        disabled={!runnableValidation.ok}
        class="cursor-pointer rounded-md border border-green-500/40 bg-green-500/10 px-4 py-2 font-body text-sm text-green-400 transition-colors hover:border-green-400 disabled:cursor-not-allowed disabled:opacity-50"
      >
        {translate("plans.overseer.markRunnable", $locale)}
      </button>
      <button
        type="button"
        onclick={() => handlePromote(PlanConfigurationStatus.SCHEDULED)}
        disabled={!runnableValidation.ok}
        class="cursor-pointer rounded-md border border-talon-gold/40 bg-talon-gold/10 px-4 py-2 font-body text-sm text-talon-gold transition-colors hover:border-talon-gold disabled:cursor-not-allowed disabled:opacity-50"
      >
        {translate("plans.overseer.markScheduled", $locale)}
      </button>
    </div>

    {#if saveMessage}
      <p class="font-body text-sm text-green-400">{saveMessage}</p>
    {/if}
    {#if saveError}
      <p class="font-body text-sm text-red-400">{saveError}</p>
    {/if}
    {#if promoteError}
      <p class="font-body text-sm text-red-400">{promoteError}</p>
    {/if}
  {/if}
</div>
