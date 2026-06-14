<script lang="ts">
  import { Loader2, Save, ShieldCheck } from "lucide-svelte";
  import {
    ELICITATION_TIMEOUT_BEHAVIOR_OPTIONS,
    PUBLISH_APPROVAL_MODE_OPTIONS,
    elicitationBehaviorDescriptionKey,
    elicitationBehaviorLabelKey,
    publishModeDescriptionKey,
    publishModeLabelKey,
    validateBehaviorPolicies,
    type BehaviorPoliciesFormValues,
    type PlanConfigurationSource,
  } from "$lib/plans/behavior-policies";
  import {
    ElicitationTimeoutBehavior,
    PublishApprovalMode,
  } from "$lib/gen/harpia/plans/v1/plans_pb";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import { locale, translate } from "$lib/i18n";

  let {
    values = $bindable(),
    saving = false,
    saveError = null,
    saveNotice = null,
    dataSource = "api",
    onSave,
  }: {
    values: BehaviorPoliciesFormValues;
    saving?: boolean;
    saveError?: string | null;
    saveNotice?: string | null;
    dataSource?: PlanConfigurationSource;
    onSave: () => void | Promise<void>;
  } = $props();

  let validationError = $derived(validateBehaviorPolicies(values));

  function behaviorOptionId(behavior: ElicitationTimeoutBehavior): string {
    return `elicitation-${behavior}`;
  }

  function publishOptionId(mode: PublishApprovalMode): string {
    return `publish-${mode}`;
  }

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault();
    if (validationError || saving) {
      return;
    }
    await onSave();
  }
</script>

<form class="space-y-8" onsubmit={handleSubmit}>
  <section class="rounded-lg border border-plumage bg-obsidian-light/40 p-4">
    <HarpyHeading tag="h2" class="mb-1 text-lg text-cream">
      {translate("plans.policies.elicitationHeading", $locale)}
    </HarpyHeading>
    <p class="mb-4 font-body text-sm text-crown-ash">
      {translate("plans.policies.elicitationDescription", $locale)}
    </p>

    <fieldset class="space-y-3">
      <legend class="sr-only">
        {translate("plans.policies.elicitationHeading", $locale)}
      </legend>
      {#each ELICITATION_TIMEOUT_BEHAVIOR_OPTIONS as behavior (behavior)}
        <label
          for={behaviorOptionId(behavior)}
          class="flex cursor-pointer gap-3 rounded-md border px-3 py-3 transition-colors {values.elicitationTimeoutBehavior ===
          behavior
            ? 'border-talon-gold/60 bg-talon-gold/5'
            : 'border-plumage hover:border-plumage/80'}"
        >
          <input
            id={behaviorOptionId(behavior)}
            type="radio"
            name="elicitation-timeout-behavior"
            class="mt-1 accent-talon-gold"
            checked={values.elicitationTimeoutBehavior === behavior}
            onchange={() => {
              values = { ...values, elicitationTimeoutBehavior: behavior };
            }}
          />
          <span>
            <span class="font-heading text-sm font-semibold text-cream">
              {translate(elicitationBehaviorLabelKey(behavior), $locale)}
            </span>
            <span class="mt-1 block font-body text-xs text-crown-ash">
              {translate(elicitationBehaviorDescriptionKey(behavior), $locale)}
            </span>
          </span>
        </label>
      {/each}
    </fieldset>

    <div class="mt-4">
      <label
        for="elicitation-timeout-hours"
        class="mb-1 block font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase"
      >
        {translate("plans.policies.timeoutHoursLabel", $locale)}
      </label>
      <input
        id="elicitation-timeout-hours"
        type="number"
        min="1"
        max="720"
        step="1"
        class="w-full max-w-[12rem] rounded-md border border-plumage bg-obsidian px-3 py-2 font-mono text-sm text-cream focus:border-talon-gold focus:outline-none"
        bind:value={values.elicitationTimeoutHours}
      />
      <p class="mt-1 font-body text-xs text-crown-ash">
        {translate("plans.policies.timeoutHoursHint", $locale)}
      </p>
    </div>
  </section>

  <section class="rounded-lg border border-plumage bg-obsidian-light/40 p-4">
    <HarpyHeading tag="h2" class="mb-1 text-lg text-cream">
      {translate("plans.policies.publishHeading", $locale)}
    </HarpyHeading>
    <p class="mb-4 font-body text-sm text-crown-ash">
      {translate("plans.policies.publishDescription", $locale)}
    </p>

    <fieldset class="space-y-3">
      <legend class="sr-only">
        {translate("plans.policies.publishHeading", $locale)}
      </legend>
      {#each PUBLISH_APPROVAL_MODE_OPTIONS as mode (mode)}
        <label
          for={publishOptionId(mode)}
          class="flex cursor-pointer gap-3 rounded-md border px-3 py-3 transition-colors {values.publishApprovalMode ===
          mode
            ? 'border-talon-gold/60 bg-talon-gold/5'
            : 'border-plumage hover:border-plumage/80'}"
        >
          <input
            id={publishOptionId(mode)}
            type="radio"
            name="publish-approval-mode"
            class="mt-1 accent-talon-gold"
            checked={values.publishApprovalMode === mode}
            onchange={() => {
              values = { ...values, publishApprovalMode: mode };
            }}
          />
          <span>
            <span class="font-heading text-sm font-semibold text-cream">
              {translate(publishModeLabelKey(mode), $locale)}
            </span>
            <span class="mt-1 block font-body text-xs text-crown-ash">
              {translate(publishModeDescriptionKey(mode), $locale)}
            </span>
          </span>
        </label>
      {/each}
    </fieldset>
  </section>

  <section class="rounded-lg border border-plumage/80 bg-obsidian/60 p-4">
    <div class="mb-3 flex items-center gap-2">
      <ShieldCheck class="size-4 text-talon-gold" />
      <HarpyHeading tag="h2" class="text-lg text-cream">
        {translate("plans.policies.summaryHeading", $locale)}
      </HarpyHeading>
    </div>
    <p class="mb-4 font-body text-sm text-crown-ash">
      {translate("plans.policies.summaryDescription", $locale)}
    </p>
    <dl class="grid gap-3 sm:grid-cols-2">
      <div class="rounded-md border border-plumage/60 px-3 py-2">
        <dt
          class="font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase"
        >
          {translate("plans.policies.summaryElicitation", $locale)}
        </dt>
        <dd class="mt-1 font-heading text-sm font-semibold text-cream">
          {translate(
            elicitationBehaviorLabelKey(values.elicitationTimeoutBehavior),
            $locale,
          )}
        </dd>
        <dd class="font-body text-xs text-crown-ash">
          {translate("plans.policies.summaryTimeout", $locale, {
            hours: values.elicitationTimeoutHours,
          })}
        </dd>
      </div>
      <div class="rounded-md border border-plumage/60 px-3 py-2">
        <dt
          class="font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase"
        >
          {translate("plans.policies.summaryPublish", $locale)}
        </dt>
        <dd class="mt-1 font-heading text-sm font-semibold text-cream">
          {translate(publishModeLabelKey(values.publishApprovalMode), $locale)}
        </dd>
      </div>
    </dl>
  </section>

  {#if validationError}
    <p class="font-body text-sm text-red-400">
      {translate(validationError, $locale)}
    </p>
  {/if}

  {#if saveError}
    <p class="font-body text-sm text-red-400">{saveError}</p>
  {/if}

  {#if saveNotice}
    <p class="font-body text-sm text-green-400">{saveNotice}</p>
  {/if}

  {#if dataSource === "mock"}
    <p class="font-mono text-xs text-talon-gold">
      {translate("plans.policies.mockFallback", $locale)}
    </p>
  {/if}

  <div class="flex justify-end">
    <button
      type="submit"
      disabled={saving || Boolean(validationError)}
      class="inline-flex cursor-pointer items-center gap-2 rounded-md border border-talon-gold/60 bg-talon-gold/10 px-4 py-2 font-body text-sm text-talon-gold transition-colors hover:bg-talon-gold/20 disabled:cursor-not-allowed disabled:opacity-50"
    >
      {#if saving}
        <Loader2 class="size-4 animate-spin" />
        {translate("plans.policies.saving", $locale)}
      {:else}
        <Save class="size-4" />
        {translate("plans.policies.save", $locale)}
      {/if}
    </button>
  </div>
</form>
