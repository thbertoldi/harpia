<script lang="ts">
  import { fade } from "svelte/transition";
  import type { ChatMessage } from "$lib/chat/types";
  import { locale, translate } from "$lib/i18n";
  import { applyLinkedInSuggestion, editBinding } from "$lib/plans/assistant";
  import {
    buildDefaultLinkedInInputValues,
    linkedInInputValuesFromParameterValuesJson,
  } from "$lib/plans/linkedin-template-inputs";
  import { localizedStepTitleByKey } from "$lib/plans/catalog-i18n";
  import {
    parseMatrixPayload,
    computeRunCostBRL,
    hydrateMatrixPayload,
    isMatrixComplete,
    type MatrixPayload,
    type MatrixOption,
    type MatrixRow,
  } from "$lib/plans/matrix";
  import type { LinkedInTemplateInputValues } from "$lib/plans/template-inputs";
  import { cardLift, chipFlash } from "$lib/motion/transitions";
  import { planClient } from "$lib/rpc";
  import type {
    PlanConfiguration,
    PlanTemplate,
  } from "$lib/gen/harpia/plans/v1/plans_pb";

  interface ExecutorCatalogEntry {
    displayName: string;
    pricePerRunBrl: number | null;
  }

  interface Props {
    message: ChatMessage;
    configurationId: string;
    tenantId: string;
    executorCatalog?: Map<string, ExecutorCatalogEntry>;
  }
  let { message, configurationId, tenantId }: Props = $props();

  // The matrix payload is the visible source of truth for rows, options and
  // current bindings. We seed it once from the (immutable)
  // message, then mutate `current_executor_id` locally for instant feedback
  // while the server is updated via editBinding.
  function parseMatrixPayloadSafe(json: string): MatrixPayload | null {
    try {
      return parseMatrixPayload(json);
    } catch {
      return null;
    }
  }

  // svelte-ignore state_referenced_locally
  let payload = $state<MatrixPayload | null>(
    parseMatrixPayloadSafe(message.payloadJson),
  );
  const parseError = $derived(payload === null);

  // The configuration + template are needed for editBinding (which calls
  // UpdatePlanConfiguration silently) and for the Save action's status
  // promotion + policy writes. Loaded lazily off the ids.
  let configuration = $state<PlanConfiguration | null>(null);
  let template = $state<PlanTemplate | null>(null);

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
            policiesSet: !!cfgRes.planConfiguration.behaviorPolicies,
          });
        }
        inputValues = linkedInInputValuesFromParameterValuesJson(
          cfgRes.planConfiguration.parameterValuesJson,
        );
        const tplRes = await planClient.getPlanTemplate(
          { planTemplateId: cfgRes.planConfiguration.planTemplateId },
          { signal: controller.signal },
        );
        if (controller.signal.aborted) return;
        template = tplRes.planTemplate ?? null;
      } catch {
        // best-effort; pickers stay read-only until the load lands
      }
    })();
    return () => controller.abort();
  });

  const rows = $derived(payload?.rows ?? []);
  const boundCount = $derived(
    rows.filter((r) => r.current_executor_id !== "").length,
  );
  const cost = $derived(computeRunCostBRL(rows));
  const complete = $derived(payload ? isMatrixComplete(payload) : false);

  let savingRowKey = $state<string | null>(null);
  let saving = $state(false);
  let submitted = $state(false);
  let saveError = $state(false);
  let suggestOpen = $state(false);
  let inputValues = $state<LinkedInTemplateInputValues>(
    buildDefaultLinkedInInputValues(),
  );

  function stepTitle(row: MatrixRow): string {
    return localizedStepTitleByKey(
      template?.key ?? "",
      row.step_key,
      $locale,
      row.step_title || row.step_key,
    );
  }

  function translatedInputLabel(inputKey: string, fallback: string): string {
    const key = `plans.inputs.${inputKey}.label`;
    const translated = translate(key, $locale);
    return translated === key ? fallback : translated;
  }

  function translatedInputOption(
    inputKey: string,
    value: string,
    fallback: string,
  ): string {
    const key = `plans.inputs.${inputKey}.option.${value}`;
    const translated = translate(key, $locale);
    return translated === key ? fallback : translated;
  }

  async function onPickExecutor(row: MatrixRow, optionId: string) {
    if (!optionId || optionId === row.current_executor_id) return;
    if (!configuration || !template) return;
    savingRowKey = row.step_key;
    try {
      const next = await editBinding({
        tenantId,
        configurationId,
        existingConfiguration: configuration,
        template,
        stepKey: row.step_key,
        newInstallationId: optionId,
      });
      configuration = next;
      // Reflect the pick locally so the row + cost update immediately.
      if (payload) {
        payload = {
          ...payload,
          rows: payload.rows.map((r) =>
            r.step_key === row.step_key
              ? { ...r, current_executor_id: optionId }
              : r,
          ),
        };
      }
    } catch {
      saveError = true;
    } finally {
      savingRowKey = null;
    }
  }

  function optionMatchesTopic(option: MatrixOption, topic: string): boolean {
    if (!topic) return false;
    return [option.label, option.sublabel ?? "", option.value]
      .join(" ")
      .toLowerCase()
      .includes(topic);
  }

  function installationIdsByStep(): Record<string, string> {
    const ids: Record<string, string> = {};
    const topic = inputValues.theme.trim().toLowerCase();
    for (const row of rows) {
      const preferred = row.options.find((option) =>
        optionMatchesTopic(option, topic),
      );
      const fallback = row.options.find((option) => option.id.trim() !== "");
      const selected = preferred ?? fallback;
      if (selected?.id) ids[row.step_key] = selected.id;
    }
    return ids;
  }

  async function onSuggest() {
    if (!configuration || !template) return;
    saving = true;
    saveError = false;
    try {
      const next = await applyLinkedInSuggestion({
        tenantId,
        configurationId,
        existingConfiguration: configuration,
        template,
        topic: inputValues.theme,
        values: inputValues,
        installationIdsByStep: installationIdsByStep(),
      });
      configuration = next;
      const nextBindings = new Map(
        next.slotBindings.map((binding) => [
          binding.stepKey,
          binding.executorInstallationId,
        ]),
      );
      if (payload) {
        payload = {
          ...payload,
          policies_set: true,
          rows: payload.rows.map((row) => ({
            ...row,
            current_executor_id:
              nextBindings.get(row.step_key) ?? row.current_executor_id,
          })),
        };
      }
      suggestOpen = false;
    } catch {
      saveError = true;
    } finally {
      saving = false;
    }
  }

  async function onSave(promote: boolean) {
    if (saving || submitted || !configuration) return;
    saving = true;
    saveError = false;
    try {
      const res = promote
        ? await planClient.submitConfigurationSelection({
            tenantId,
            planConfigurationId: configurationId,
            assistantPromptMessageId: message.id,
            selection: {
              optionId: "save-runnable",
              value: "save_runnable",
            },
          })
        : await planClient.updatePlanConfiguration({
            tenantId,
            planConfigurationId: configurationId,
            status: configuration.status,
            overseerBindings: configuration.overseerBindings,
            schedule: configuration.schedule,
            parameterValuesJson: configuration.parameterValuesJson,
            // Explicit user save — announce it in the thread.
            announceSaved: true,
          });
      if (res.planConfiguration) configuration = res.planConfiguration;
      submitted = true;
    } catch {
      saveError = true;
    } finally {
      saving = false;
    }
  }

  const introText = $derived(
    translate("assistant.prompt.bindingMatrix", $locale),
  );
  const templateName = $derived(template?.name ?? "");
</script>

<div id={`m-${message.id}`} class="flex flex-col gap-3">
  {#if introText}
    <p class="font-body text-[14px] leading-relaxed text-cream">{introText}</p>
  {/if}

  {#if parseError || !payload}
    <p
      class="w-full rounded-lg border border-plumage bg-surface-elevated px-4 py-3 text-[13px] text-crown-ash"
    >
      {introText}
    </p>
  {:else}
    <div
      class="w-full overflow-hidden rounded-lg border border-plumage bg-surface-elevated shadow-[0_8px_24px_rgba(0,0,0,0.25)]"
    >
      <!-- Header -->
      <div
        class="flex items-center justify-between border-b border-plumage px-5 py-4"
      >
        <div>
          <h3 class="font-heading text-[15px] font-semibold text-cream">
            {templateName}
          </h3>
        </div>
        <div class="flex items-center gap-2">
          <button
            type="button"
            disabled={!configuration || !template || saving || submitted}
            onclick={() => (suggestOpen = true)}
            class="cursor-pointer rounded-md border border-plumage px-3 py-1.5 text-[12px] font-medium text-crown-ash hover:border-talon-gold hover:text-talon-gold disabled:cursor-not-allowed disabled:opacity-50"
          >
            {translate("assistant.bindingMatrix.suggest", $locale)}
          </button>
          <span
            class="rounded border border-talon-gold/35 bg-talon-gold/[0.06] px-2 py-1 text-[10px] font-semibold tracking-[0.1em] text-talon-gold uppercase"
          >
            {translate("assistant.bindingMatrix.progress", $locale, {
              bound: boundCount,
              total: rows.length,
            })}
          </span>
        </div>
      </div>

      {#if suggestOpen}
        <div class="border-b border-plumage bg-surface-deep px-5 py-4">
          <div class="grid gap-3 md:grid-cols-2">
            <label class="block">
              <span class="mb-1 block text-[11px] font-medium text-crown-ash"
                >{translatedInputLabel("theme", "Theme")}</span
              >
              <input
                value={inputValues.theme}
                oninput={(event) =>
                  (inputValues = {
                    ...inputValues,
                    theme: (event.currentTarget as HTMLInputElement).value,
                  })}
                class="w-full rounded-md border border-plumage bg-surface-hover px-3 py-2 text-[13px] text-cream outline-none focus:border-talon-gold"
              />
            </label>
            <label class="block">
              <span class="mb-1 block text-[11px] font-medium text-crown-ash"
                >{translatedInputLabel("language", "Language")}</span
              >
              <select
                value={inputValues.language}
                onchange={(event) =>
                  (inputValues = {
                    ...inputValues,
                    language: (event.currentTarget as HTMLSelectElement)
                      .value as LinkedInTemplateInputValues["language"],
                  })}
                class="w-full rounded-md border border-plumage bg-surface-hover px-3 py-2 text-[13px] text-cream outline-none focus:border-talon-gold"
              >
                <option value="pt-BR"
                  >{translatedInputOption(
                    "language",
                    "pt-BR",
                    "Portuguese",
                  )}</option
                >
                <option value="en-US"
                  >{translatedInputOption(
                    "language",
                    "en-US",
                    "English",
                  )}</option
                >
                <option value="es"
                  >{translatedInputOption("language", "es", "Spanish")}</option
                >
              </select>
            </label>
            <label class="block">
              <span class="mb-1 block text-[11px] font-medium text-crown-ash"
                >{translatedInputLabel("tone", "Tone")}</span
              >
              <select
                value={inputValues.tone}
                onchange={(event) =>
                  (inputValues = {
                    ...inputValues,
                    tone: (event.currentTarget as HTMLSelectElement).value,
                  })}
                class="w-full rounded-md border border-plumage bg-surface-hover px-3 py-2 text-[13px] text-cream outline-none focus:border-talon-gold"
              >
                <option value="analytical, concise, and practical">
                  {translatedInputOption(
                    "tone",
                    "analytical, concise, and practical",
                    "Analytical",
                  )}
                </option>
                <option value="friendly and clear"
                  >{translatedInputOption(
                    "tone",
                    "friendly and clear",
                    "Friendly",
                  )}</option
                >
                <option value="executive and direct"
                  >{translatedInputOption(
                    "tone",
                    "executive and direct",
                    "Executive",
                  )}</option
                >
              </select>
            </label>
            <label class="block">
              <span class="mb-1 block text-[11px] font-medium text-crown-ash"
                >{translatedInputLabel("audience", "Audience")}</span
              >
              <input
                value={inputValues.audience}
                oninput={(event) =>
                  (inputValues = {
                    ...inputValues,
                    audience: (event.currentTarget as HTMLInputElement).value,
                  })}
                class="w-full rounded-md border border-plumage bg-surface-hover px-3 py-2 text-[13px] text-cream outline-none focus:border-talon-gold"
              />
            </label>
            <label class="block md:col-span-2">
              <span class="mb-1 block text-[11px] font-medium text-crown-ash"
                >{translatedInputLabel(
                  "topics_to_avoid",
                  "Topics to avoid",
                )}</span
              >
              <textarea
                value={inputValues.topicsToAvoid}
                oninput={(event) =>
                  (inputValues = {
                    ...inputValues,
                    topicsToAvoid: (event.currentTarget as HTMLTextAreaElement)
                      .value,
                  })}
                rows="2"
                class="w-full resize-none rounded-md border border-plumage bg-surface-hover px-3 py-2 text-[13px] text-cream outline-none focus:border-talon-gold"
              ></textarea>
            </label>
          </div>
          <div class="mt-3 flex justify-end">
            <button
              type="button"
              disabled={saving}
              onclick={onSuggest}
              class="cursor-pointer rounded-md bg-talon-gold px-3 py-2 text-[12px] font-semibold text-on-primary hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {translate("assistant.bindingMatrix.applySuggestion", $locale)}
            </button>
          </div>
        </div>
      {/if}

      <!-- Rows -->
      {#each rows as row (row.step_key)}
        {@const bound = row.current_executor_id !== ""}
        <div
          in:cardLift
          class="grid grid-cols-[20px_minmax(0,1fr)] items-center gap-3.5 border-b border-obsidian-light px-5 py-3.5 hover:bg-surface-hover md:grid-cols-[20px_minmax(10rem,1fr)_minmax(14rem,260px)]"
        >
          <!-- Step number -->
          <div
            class="flex size-5 items-center justify-center rounded-full border text-[10px] font-semibold {bound
              ? 'border-talon-gold bg-talon-gold text-on-primary'
              : 'border-plumage text-crown-ash-dark'}"
          >
            {rows.indexOf(row) + 1}
          </div>

          <!-- Task name -->
          <div class="min-w-0">
            <p class="truncate text-[13px] font-medium text-cream">
              {stepTitle(row)}
            </p>
          </div>

          <!-- Executor picker -->
          <div class="relative col-start-2 md:col-start-auto">
            <select
              value={row.current_executor_id}
              disabled={savingRowKey !== null ||
                submitted ||
                !configuration ||
                !template}
              onchange={(e) =>
                onPickExecutor(
                  row,
                  (e.currentTarget as HTMLSelectElement).value,
                )}
              aria-label={translate(
                "assistant.bindingMatrix.pickExecutor",
                $locale,
              )}
              class="w-full cursor-pointer appearance-none rounded-md border border-plumage bg-surface-hover py-2 pr-7 pl-3 text-[12px] text-cream hover:border-talon-gold focus:border-talon-gold focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
            >
              <option value="" disabled>
                {translate("assistant.bindingMatrix.pickExecutor", $locale)}
              </option>
              {#each row.options as opt (opt.id)}
                <option value={opt.id}>
                  {opt.label}{opt.sublabel ? ` · ${opt.sublabel}` : ""}
                </option>
              {/each}
            </select>
            <span
              class="pointer-events-none absolute top-1/2 right-2.5 -translate-y-1/2 text-[10px] text-crown-ash-dark"
              aria-hidden="true">▾</span
            >
          </div>
        </div>
      {/each}

      <!-- Footer: cost + actions -->
      <div
        class="flex items-center justify-between gap-3 bg-surface-deep px-5 py-3.5"
      >
        <div class="flex items-center gap-2 text-[13px] text-crown-ash">
          <span>{translate("assistant.bindingMatrix.costPerRun", $locale)}</span
          >
          <span class="font-heading text-[14px] font-semibold text-talon-gold">
            R$ {cost.totalBrl.toFixed(2)}
          </span>
          {#if cost.unboundCount > 0}
            <span
              class="rounded border border-talon-gold/30 px-1.5 py-0.5 text-[10px] tracking-[0.08em] text-talon-gold uppercase"
            >
              {translate("assistant.bindingMatrix.pending", $locale, {
                count: cost.unboundCount,
              })}
            </span>
          {/if}
        </div>
        {#if !submitted}
          <div class="flex items-center gap-2">
            <button
              type="button"
              in:chipFlash
              disabled={saving || !configuration}
              onclick={() => onSave(false)}
              class="cursor-pointer rounded-md border border-plumage bg-transparent px-3.5 py-2 text-[12px] font-medium text-crown-ash hover:border-talon-gold hover:text-cream disabled:cursor-not-allowed disabled:opacity-50"
            >
              {translate("assistant.bindingMatrix.saveSecondary", $locale)}
            </button>
            <button
              type="button"
              in:chipFlash
              disabled={!complete || saving || !configuration}
              onclick={() => onSave(true)}
              class="cursor-pointer rounded-md bg-primary px-3.5 py-2 text-[12px] font-semibold text-on-primary shadow-[0_1px_0_rgba(255,255,255,0.15)_inset] hover:opacity-90 disabled:cursor-not-allowed disabled:bg-surface-pop disabled:text-crown-ash-dark disabled:opacity-100 disabled:shadow-none"
            >
              {translate("assistant.bindingMatrix.savePrimary", $locale)}
            </button>
          </div>
        {/if}
      </div>
    </div>

    {#if saveError}
      <p transition:fade class="text-[11px] text-red-400">
        {translate("thread.saveError", $locale)}
      </p>
    {/if}
  {/if}
</div>
