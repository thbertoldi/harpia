<script lang="ts">
  import { ChevronDown, ChevronUp, Pencil } from "lucide-svelte";
  import { fade } from "svelte/transition";
  import type { ChatMessage } from "$lib/chat/types";
  import { locale, translate } from "$lib/i18n";
  import {
    appendStepRebound,
    editBinding,
    selectChip,
  } from "$lib/plans/assistant";
  import {
    hydrateMatrixPayload,
    parseBindingStepPayload,
    type BindingStepPayload,
    type MatrixOption,
    type MatrixRow,
  } from "$lib/plans/matrix";
  import {
    buildBindingStepView,
    policiesComplete,
    reflectBindingPayload,
  } from "$lib/plans/configuration-flow";
  import { chipFlash } from "$lib/motion/transitions";
  import { planClient } from "$lib/rpc";
  import { localizedStepTitle } from "$lib/plans/catalog-i18n";
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

  function parsePayloadSafe(json: string): BindingStepPayload | null {
    try {
      return parseBindingStepPayload(json);
    } catch {
      return null;
    }
  }

  let payload = $derived<BindingStepPayload | null>(
    parsePayloadSafe(message.payloadJson),
  );
  let configuration = $state<PlanConfiguration | null>(null);
  let template = $state<PlanTemplate | null>(null);
  let savingRowKey = $state<string | null>(null);
  let saveError = $state(false);
  let editAllOpen = $state(false);
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
        if (payload) {
          payload = hydrateMatrixPayload(payload, {
            slotBindings: cfgRes.planConfiguration.slotBindings,
            overseerBindings: cfgRes.planConfiguration.overseerBindings,
            policiesSet: policiesComplete(cfgRes.planConfiguration),
          });
        }
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
      ? buildBindingStepView(payload, { isLive, editingAnswered, submitted })
      : null,
  );
  const rows = $derived(stepView?.rows ?? []);
  const focusedRow = $derived(stepView?.focusedRow ?? null);
  const boundCount = $derived(stepView?.boundCount ?? 0);
  const totalCount = $derived(stepView?.totalCount ?? 0);
  const showChips = $derived(stepView?.showChips ?? false);

  function selectedOption(row: MatrixRow): MatrixOption | undefined {
    return row.options.find((option) => option.id === row.current_executor_id);
  }

  function optionText(option?: MatrixOption): string {
    if (!option) return "";
    return option.sublabel
      ? `${option.label} · ${option.sublabel}`
      : option.label;
  }

  /**
   * Localize a step title for display. The backend payload carries English
   * `step_title`; resolve through the catalog content keys when the template
   * is loaded so pt-BR users see localized headings, falling back to the
   * backend title (and finally the stepKey) when no entry exists.
   */
  function stepLabel(stepKey: string, fallbackTitle: string): string {
    if (template) return localizedStepTitle(template, stepKey, $locale);
    return fallbackTitle || stepKey;
  }

  async function onPickFocused(option: MatrixOption) {
    if (!payload || !focusedRow || !configuration || !template) return;
    if (savingRowKey || option.id === focusedRow.current_executor_id) return;
    savingRowKey = focusedRow.step_key;
    saveError = false;
    try {
      const next = await selectChip({
        tenantId,
        configurationId,
        promptMessageId: message.id,
        optionId: option.id,
        value: option.value || option.id,
      });
      configuration = next;
      payload = reflectBindingPayload(payload, focusedRow.step_key, option.id);
      submitted = true;
      editingAnswered = false;
    } catch {
      saveError = true;
    } finally {
      savingRowKey = null;
    }
  }

  async function onPickMatrix(row: MatrixRow, optionId: string) {
    if (!optionId || optionId === row.current_executor_id) return;
    if (!payload || !configuration || !template) return;
    const option = row.options.find((candidate) => candidate.id === optionId);
    savingRowKey = row.step_key;
    saveError = false;
    try {
      const previous = row.current_executor_id;
      const next = await editBinding({
        tenantId,
        configurationId,
        existingConfiguration: configuration,
        template,
        stepKey: row.step_key,
        newInstallationId: optionId,
      });
      configuration = next;
      payload = reflectBindingPayload(payload, row.step_key, optionId);
      await appendStepRebound({
        tenantId,
        configurationId,
        threadId: message.threadId,
        stepKey: row.step_key,
        previousInstallationId: previous,
        newInstallationId: optionId,
        label: option?.label ?? optionId,
      });
    } catch {
      saveError = true;
    } finally {
      savingRowKey = null;
    }
  }

  function toggleAnsweredEdit() {
    editingAnswered = !editingAnswered;
    if (editingAnswered) submitted = false;
  }
</script>

<div
  id={`m-${message.id}`}
  class="rounded-lg border border-plumage bg-obsidian-light px-4 py-3 {isLive
    ? 'ring-1 ring-talon-gold'
    : ''}"
>
  {#if !payload || !focusedRow}
    <p class="text-[13px] text-cream">{message.text}</p>
  {:else}
    <div class="flex items-start justify-between gap-3">
      <div class="min-w-0">
        <p class="font-body text-[13px] text-cream">
          {translate("assistant.prompt.bindingStep", $locale, {
            step: stepLabel(
              focusedRow.step_key,
              focusedRow.step_title || payload.step_key || "",
            ),
          })}
        </p>
        <div
          class="mt-2 flex flex-wrap items-center gap-2 text-[11px] text-crown-ash"
        >
          <span
            class="rounded border border-plumage bg-surface-hover px-2 py-1 font-mono text-[10px] text-crown-ash"
          >
            {focusedRow.contracts.input || "—"} → {focusedRow.contracts
              .output || "—"}
          </span>
          <span>
            {translate("assistant.bindingStep.progress", $locale, {
              bound: boundCount,
              total: totalCount,
            })}
          </span>
        </div>
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

    {#if selectedOption(focusedRow) && !showChips}
      <p class="mt-3 text-[12px] text-crown-ash">
        {translate("assistant.bindingStep.selected", $locale, {
          executor: optionText(selectedOption(focusedRow)),
        })}
      </p>
    {/if}

    {#if showChips}
      <div class="mt-3 flex flex-wrap gap-2">
        {#each payload.options as opt (opt.id)}
          <button
            type="button"
            in:chipFlash
            disabled={savingRowKey !== null || !configuration || !template}
            onclick={() => onPickFocused(opt)}
            class="cursor-pointer rounded-md border border-plumage bg-obsidian px-3 py-2 text-left text-[12px] text-cream hover:border-talon-gold hover:text-talon-gold disabled:cursor-not-allowed disabled:opacity-50"
          >
            <span class="font-medium">{opt.label}</span>
            {#if opt.sublabel}
              <span class="ml-2 text-[10px] text-crown-ash-dark">
                {opt.sublabel}
              </span>
            {/if}
          </button>
        {/each}
        {#if payload.options.length === 0}
          <p class="text-[12px] text-crown-ash">
            {translate("assistant.bindingStep.noOptions", $locale)}
          </p>
        {/if}
      </div>
    {/if}

    <div class="mt-3 border-t border-plumage/70 pt-3">
      <button
        type="button"
        class="flex cursor-pointer items-center gap-1.5 text-[12px] font-medium text-crown-ash hover:text-talon-gold"
        onclick={() => (editAllOpen = !editAllOpen)}
        aria-expanded={editAllOpen}
      >
        {#if editAllOpen}
          <ChevronUp class="size-3.5" />
          {translate("assistant.bindingStep.hideAll", $locale)}
        {:else}
          <ChevronDown class="size-3.5" />
          {translate("assistant.bindingStep.editAll", $locale)}
        {/if}
      </button>

      {#if editAllOpen}
        <div transition:fade class="mt-3 overflow-x-auto">
          <table class="w-full min-w-[560px] border-collapse text-left">
            <thead>
              <tr
                class="border-b border-plumage text-[10px] text-crown-ash-dark uppercase"
              >
                <th class="py-2 pr-3 font-semibold">
                  {translate("assistant.bindingStep.step", $locale)}
                </th>
                <th class="py-2 pr-3 font-semibold">
                  {translate("assistant.bindingStep.executor", $locale)}
                </th>
              </tr>
            </thead>
            <tbody>
              {#each rows as row (row.step_key)}
                <tr class="border-b border-obsidian-light">
                  <td class="py-2 pr-3 align-top">
                    <p class="text-[12px] font-medium text-cream">
                      {stepLabel(row.step_key, row.step_title)}
                    </p>
                    <p class="mt-0.5 font-mono text-[10px] text-crown-ash-dark">
                      {row.contracts.input || "—"} → {row.contracts.output ||
                        "—"}
                    </p>
                  </td>
                  <td class="py-2 pr-3 align-top">
                    <select
                      value={row.current_executor_id}
                      disabled={savingRowKey !== null ||
                        !configuration ||
                        !template}
                      onchange={(event) =>
                        onPickMatrix(
                          row,
                          (event.currentTarget as HTMLSelectElement).value,
                        )}
                      aria-label={translate(
                        "assistant.bindingMatrix.pickExecutor",
                        $locale,
                      )}
                      class="w-full cursor-pointer rounded-md border border-plumage bg-surface-hover px-3 py-2 text-[12px] text-cream outline-none hover:border-talon-gold focus:border-talon-gold disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      <option value="" disabled>
                        {translate(
                          "assistant.bindingMatrix.pickExecutor",
                          $locale,
                        )}
                      </option>
                      {#each row.options as option (option.id)}
                        <option value={option.id}>
                          {optionText(option)}
                        </option>
                      {/each}
                    </select>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </div>
  {/if}

  {#if saveError}
    <p transition:fade class="mt-2 text-[11px] text-red-400">
      {translate("thread.saveError", $locale)}
    </p>
  {/if}
</div>
