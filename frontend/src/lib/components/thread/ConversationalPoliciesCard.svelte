<script lang="ts">
  import { Pencil } from "lucide-svelte";
  import { fade } from "svelte/transition";
  import type { ChatMessage } from "$lib/chat/types";
  import { locale, translate } from "$lib/i18n";
  import { selectChip } from "$lib/plans/assistant";
  import {
    isPolicyOptionSelected,
    parsePoliciesStepPayload,
    policyFieldChipsShown,
    type MatrixOption,
    type PoliciesStepPayload,
    type PolicyField,
  } from "$lib/plans/matrix";
  import {
    buildPoliciesStepView,
    hydratePoliciesPayloadFromConfiguration,
    reflectPolicyPayload,
  } from "$lib/plans/configuration-flow";
  import { chipFlash } from "$lib/motion/transitions";
  import { planClient } from "$lib/rpc";
  import type {
    PlanConfiguration,
    PlanTemplate,
  } from "$lib/gen/harpia/plans/v1/plans_pb";

  interface Props {
    message: ChatMessage;
    configurationId: string;
    tenantId: string;
    isLive?: boolean;
    isAnswered?: boolean;
  }

  let {
    message,
    configurationId,
    tenantId,
    isLive = false,
    isAnswered = false,
  }: Props = $props();

  function parsePayloadSafe(json: string): PoliciesStepPayload | null {
    try {
      return parsePoliciesStepPayload(json);
    } catch {
      return null;
    }
  }

  let payload = $derived<PoliciesStepPayload | null>(
    parsePayloadSafe(message.payloadJson),
  );
  let configuration = $state<PlanConfiguration | null>(null);
  let template = $state<PlanTemplate | null>(null);
  let savingFieldKey = $state<string | null>(null);
  let saveError = $state(false);
  let editingAnswered = $state(false);
  let submitted = $state(false);

  $effect(() => {
    if (!tenantId || !configurationId) return;
    const controller = new AbortController();
    (async () => {
      try {
        const cfgRes = await planClient.getPlanConfiguration(
          { tenantId, planConfigurationId: configurationId },
          { signal: controller.signal },
        );
        if (controller.signal.aborted || !cfgRes.planConfiguration) return;
        configuration = cfgRes.planConfiguration;
        hydrateCurrentValues(cfgRes.planConfiguration);

        const tplRes = await planClient.getPlanTemplate(
          { planTemplateId: cfgRes.planConfiguration.planTemplateId },
          { signal: controller.signal },
        );
        if (controller.signal.aborted) return;
        template = tplRes.planTemplate ?? null;
      } catch {
        saveError = true;
      }
    })();
    return () => controller.abort();
  });

  const stepView = $derived(
    payload
      ? buildPoliciesStepView(payload, { isLive, editingAnswered, submitted })
      : null,
  );
  const fields = $derived(stepView?.fields ?? []);
  const selectedCount = $derived(stepView?.selectedCount ?? 0);
  const totalCount = $derived(stepView?.totalCount ?? 0);
  const complete = $derived(stepView?.complete ?? false);
  const showChips = $derived(stepView?.showChips ?? false);

  function hydrateCurrentValues(config: PlanConfiguration) {
    if (!payload) return;
    payload = hydratePoliciesPayloadFromConfiguration(payload, config);
    submitted = payload.fields.every((field) => field.current_value !== "");
  }

  function reflectPolicy(fieldKey: string, value: string) {
    if (!payload) return;
    payload = reflectPolicyPayload(payload, fieldKey, value);
    submitted = payload.fields.every((field) => field.current_value !== "");
    editingAnswered = !submitted && editingAnswered;
  }

  function fieldLabel(field: PolicyField): string {
    return translate(`assistant.policiesStep.field.${field.key}`, $locale);
  }

  function optionLabel(field: PolicyField, option: MatrixOption): string {
    return translate(
      `assistant.policiesStep.option.${field.key}.${option.value || option.id}`,
      $locale,
    );
  }

  function selectedOption(field: PolicyField): MatrixOption | undefined {
    return field.options.find(
      (option) =>
        option.value === field.current_value ||
        option.id === field.current_value,
    );
  }

  function selectedLabel(field: PolicyField): string {
    const selected = selectedOption(field);
    return selected ? optionLabel(field, selected) : "";
  }

  async function onPick(field: PolicyField, option: MatrixOption) {
    if (!payload || !configuration || !template) return;
    const value = option.value || option.id;
    if (!field.parameter_key || !value || value === field.current_value) return;
    if (savingFieldKey) return;
    savingFieldKey = field.key;
    saveError = false;
    try {
      const next = await selectChip({
        tenantId,
        configurationId,
        promptMessageId: message.id,
        optionId: option.id,
        value,
      });
      configuration = next;
      reflectPolicy(field.key, value);
    } catch {
      saveError = true;
    } finally {
      savingFieldKey = null;
    }
  }

  function toggleAnsweredEdit() {
    editingAnswered = !editingAnswered;
    if (editingAnswered) submitted = false;
  }
</script>

<div
  id={`m-${message.id}`}
  class="w-full rounded-lg border border-plumage bg-obsidian-light px-4 py-3 {isLive
    ? 'ring-1 ring-talon-gold'
    : ''}"
>
  {#if !payload}
    <p class="text-[13px] text-cream">{message.text}</p>
  {:else}
    <div class="flex items-start justify-between gap-3">
      <div class="min-w-0">
        <p class="font-body text-[13px] text-cream">
          {translate("assistant.prompt.policiesStep", $locale)}
        </p>
        <p class="mt-2 text-[11px] text-crown-ash">
          {translate("assistant.policiesStep.progress", $locale, {
            selected: selectedCount,
            total: totalCount,
          })}
        </p>
      </div>
      {#if isAnswered && !isLive}
        <button
          type="button"
          class="cursor-pointer rounded p-1 text-crown-ash hover:text-talon-gold"
          onclick={toggleAnsweredEdit}
          aria-label={translate("assistant.edit", $locale)}
        >
          <Pencil class="size-3.5" />
        </button>
      {/if}
    </div>

    <div class="mt-3 grid gap-3">
      {#each fields as field (field.key)}
        <section
          class="border-t border-plumage/70 pt-3 first:border-t-0 first:pt-0"
        >
          <div class="flex flex-wrap items-center justify-between gap-2">
            <p class="text-[12px] font-medium text-cream">
              {fieldLabel(field)}
            </p>
            {#if selectedLabel(field)}
              <p class="text-[11px] text-crown-ash">
                {translate("assistant.policiesStep.selected", $locale, {
                  value: selectedLabel(field),
                })}
              </p>
            {/if}
          </div>

          {#if policyFieldChipsShown( field, { isLive, editingAnswered, submitted }, )}
            <div class="mt-2 flex flex-wrap gap-2">
              {#each field.options as opt (opt.id)}
                {@const selected = isPolicyOptionSelected(field, opt)}
                <button
                  type="button"
                  in:chipFlash
                  aria-pressed={selected ? "true" : "false"}
                  disabled={savingFieldKey !== null ||
                    !configuration ||
                    !template ||
                    !field.parameter_key}
                  onclick={() => onPick(field, opt)}
                  class="cursor-pointer rounded-md border px-3 py-2 text-left text-[12px] disabled:cursor-not-allowed disabled:opacity-50 {selected
                    ? 'border-talon-gold bg-talon-gold/10 text-talon-gold ring-1 ring-talon-gold'
                    : 'border-plumage bg-obsidian text-cream hover:border-talon-gold hover:text-talon-gold'}"
                >
                  <span class="font-medium">{optionLabel(field, opt)}</span>
                </button>
              {/each}
              {#if field.options.length === 0}
                <p class="text-[12px] text-crown-ash">
                  {translate("assistant.policiesStep.noOptions", $locale)}
                </p>
              {/if}
            </div>
          {/if}
        </section>
      {/each}
    </div>

    {#if complete && !showChips}
      <p class="mt-3 text-[12px] text-crown-ash">
        {translate("assistant.policiesStep.complete", $locale)}
      </p>
    {/if}
  {/if}

  {#if saveError}
    <p transition:fade class="mt-2 text-[11px] text-red-400">
      {translate("thread.saveError", $locale)}
    </p>
  {/if}
</div>
