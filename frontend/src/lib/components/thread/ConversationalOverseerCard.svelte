<script lang="ts">
  import { Pencil } from "lucide-svelte";
  import { fade } from "svelte/transition";
  import type { ChatMessage } from "$lib/chat/types";
  import { getSession } from "$lib/auth";
  import { locale, translate } from "$lib/i18n";
  import { selectChip } from "$lib/plans/assistant";
  import {
    hydrateMatrixPayload,
    parseOverseerStepPayload,
    type MatrixOption,
    type MatrixRow,
    type OverseerStepPayload,
  } from "$lib/plans/matrix";
  import {
    buildOverseerStepView,
    policiesComplete,
    reflectOverseerPayload,
  } from "$lib/plans/configuration-flow";
  import { localizedOverseerLabel } from "$lib/plans/overseer-label";
  import { chipFlash } from "$lib/motion/transitions";
  import { planClient } from "$lib/rpc";
  import type { PlanConfiguration } from "$lib/gen/harpia/plans/v1/plans_pb";

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

  function parsePayloadSafe(json: string): OverseerStepPayload | null {
    try {
      return parseOverseerStepPayload(json);
    } catch {
      return null;
    }
  }

  let payload = $derived<OverseerStepPayload | null>(
    parsePayloadSafe(message.payloadJson),
  );
  let configuration = $state<PlanConfiguration | null>(null);
  let saving = $state(false);
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
        const tplRes = await planClient.getPlanTemplate(
          { planTemplateId: cfgRes.planConfiguration.planTemplateId },
          { signal: controller.signal },
        );
        if (controller.signal.aborted) return;
        if (payload && tplRes.planTemplate) {
          payload = hydrateMatrixPayload(payload, {
            slotBindings: cfgRes.planConfiguration.slotBindings,
            overseerBindings: cfgRes.planConfiguration.overseerBindings,
            policiesSet: policiesComplete(cfgRes.planConfiguration),
            steps: tplRes.planTemplate.steps,
            includedOptionalCapabilities:
              cfgRes.planConfiguration.includedOptionalCapabilities,
          });
        }
      } catch {
        saveError = true;
      }
    })();
    return () => controller.abort();
  });

  const stepView = $derived(
    payload
      ? buildOverseerStepView(payload, { isLive, editingAnswered, submitted })
      : null,
  );
  const focusedRow = $derived(stepView?.focusedRow ?? null);
  const requiredRows = $derived(stepView?.requiredRows ?? []);
  const overseerCount = $derived(stepView?.overseerCount ?? 0);
  const totalRequired = $derived(stepView?.totalRequired ?? 0);
  const showChips = $derived(stepView?.showChips ?? false);

  function selectedOverseer(row: MatrixRow): string {
    const sessionUser = getSession()?.user;
    const selected = payload?.options.find(
      (option) => option.value === row.current_overseer_id,
    );
    if (selected) {
      return displayLabel(selected);
    }
    // No matching option (e.g. after hydration the row only carries the id):
    // resolve the id to a localized label rather than rendering it raw.
    return row.current_overseer_id
      ? localizedOverseerLabel(
          row.current_overseer_id,
          sessionUser?.sub,
          $locale,
        )
      : "";
  }

  function currentUserFallback(option: MatrixOption): MatrixOption {
    const sessionUser = getSession()?.user;
    if (!sessionUser || option.value !== sessionUser.sub) return option;
    return {
      ...option,
      label: sessionUser.name || sessionUser.email || option.label,
    };
  }

  // The backend labels the current-user overseer option "You" (a locale-agnostic
  // fallback). Localize it for display by matching the option's value (the user
  // id) against the session user — never by string-matching the label. The
  // shared helper owns the id→label resolution so every "linked things" surface
  // stays consistent.
  function displayLabel(option: MatrixOption): string {
    const sessionUser = getSession()?.user;
    if (sessionUser && option.value === sessionUser.sub) {
      return localizedOverseerLabel(option.value, sessionUser.sub, $locale);
    }
    return option.label;
  }

  async function onPickFocused(option: MatrixOption) {
    if (!payload || !focusedRow || !configuration) return;
    const picked = currentUserFallback(option);
    if (saving || picked.value === focusedRow.current_overseer_id) return;
    saving = true;
    saveError = false;
    try {
      const next = await selectChip({
        tenantId,
        configurationId,
        promptMessageId: message.id,
        optionId: picked.id,
        value: picked.value || picked.id,
      });
      configuration = next;
      payload = reflectOverseerPayload(
        payload,
        focusedRow.step_key,
        picked.value || picked.id,
        picked.label,
      );
      submitted = true;
      editingAnswered = false;
    } catch {
      saveError = true;
    } finally {
      saving = false;
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
  {#if !payload || !focusedRow}
    <p class="text-[13px] text-cream">{message.text}</p>
  {:else}
    <div class="flex items-start justify-between gap-3">
      <div class="min-w-0">
        <p class="font-body text-[13px] text-cream">
          {translate("assistant.prompt.overseerStep", $locale, {
            step:
              focusedRow.step_title ||
              translate("assistant.overseerStep.thisStep", $locale),
          })}
        </p>
        <p class="mt-2 text-[11px] text-crown-ash">
          {translate("assistant.overseerStep.progress", $locale, {
            bound: overseerCount,
            total: totalRequired,
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

    {#if selectedOverseer(focusedRow) && !showChips}
      <p class="mt-3 text-[12px] text-crown-ash">
        {translate("assistant.overseerStep.selected", $locale, {
          overseer: selectedOverseer(focusedRow),
        })}
      </p>
    {/if}

    {#if showChips}
      <div class="mt-3 flex flex-wrap gap-2">
        {#each payload.options as opt (opt.id)}
          <button
            type="button"
            in:chipFlash
            disabled={saving || !configuration}
            onclick={() => onPickFocused(opt)}
            class="cursor-pointer rounded-md border border-plumage bg-obsidian px-3 py-2 text-left text-[12px] text-cream hover:border-talon-gold hover:text-talon-gold disabled:cursor-not-allowed disabled:opacity-50"
          >
            <span class="font-medium">{displayLabel(opt)}</span>
            {#if opt.sublabel}
              <span class="ml-2 text-[10px] text-crown-ash-dark">
                {opt.sublabel}
              </span>
            {/if}
          </button>
        {/each}
        {#if payload.options.length === 0}
          <p class="text-[12px] text-crown-ash">
            {translate("assistant.overseerStep.noOptions", $locale)}
          </p>
        {/if}
      </div>
    {/if}

    <div class="mt-3 border-t border-plumage/70 pt-3">
      <p class="text-[10px] font-semibold text-crown-ash-dark uppercase">
        {translate("assistant.overseerStep.required", $locale)}
      </p>
      <div class="mt-2 grid gap-2">
        {#each requiredRows as row, i (row.step_key)}
          <div
            class="grid grid-cols-[minmax(0,1fr)_minmax(120px,180px)] gap-3 text-[12px]"
          >
            <div class="min-w-0">
              <p class="truncate text-cream">
                {row.step_title ||
                  translate("assistant.overseerStep.agentStep", $locale, {
                    n: i + 1,
                  })}
              </p>
            </div>
            <p class="truncate text-crown-ash">
              {selectedOverseer(row) ||
                translate("assistant.overseerStep.unassigned", $locale)}
            </p>
          </div>
        {/each}
      </div>
    </div>
  {/if}

  {#if saveError}
    <p transition:fade class="mt-2 text-[11px] text-red-400">
      {translate("thread.saveError", $locale)}
    </p>
  {/if}
</div>
