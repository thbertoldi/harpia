<script lang="ts">
  import { Pencil } from "lucide-svelte";
  import { fade } from "svelte/transition";
  import type { ChatMessage } from "$lib/chat/types";
  import { locale, translate } from "$lib/i18n";
  import { editBinding } from "$lib/plans/assistant";
  import {
    parseMatrixPayload,
    computeRunCostBRL,
    isMatrixComplete,
    type MatrixPayload,
    type MatrixRow,
  } from "$lib/plans/matrix";
  import { cardLift, chipFlash } from "$lib/motion/transitions";
  import { planClient } from "$lib/rpc";
  import {
    PlanConfigurationStatus,
    type PlanConfiguration,
    type PlanTemplate,
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
    currentUserDisplayName?: string;
  }
  let {
    message,
    configurationId,
    tenantId,
    // Reserved for parent price overrides; cost currently derived from matrix rows.
    executorCatalog,
    currentUserDisplayName,
  }: Props = $props();
  void executorCatalog;

  // The matrix payload is the visible source of truth for rows, options,
  // contracts and current bindings. We seed it once from the (immutable)
  // message, then mutate `current_executor_id` locally for instant feedback
  // while the server is updated via editBinding.
  function parseMatrixPayloadSafe(json: string): MatrixPayload | null {
    try {
      return parseMatrixPayload(json);
    } catch {
      return null;
    }
  }

  let payload = $state<MatrixPayload | null>(
    parseMatrixPayloadSafe(message.payloadJson),
  );
  const parseError = $derived(payload === null);

  // The configuration + template are needed for editBinding (which writes a
  // STEP_REBOUND and calls UpdatePlanConfiguration) and for the Save action's
  // status promotion + policy writes. Loaded lazily off the ids.
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

  function optionLabel(row: MatrixRow): string {
    const opt = row.options.find((o) => o.id === row.current_executor_id);
    return opt?.label ?? "";
  }
  function optionSublabel(row: MatrixRow): string {
    const opt = row.options.find((o) => o.id === row.current_executor_id);
    return opt?.sublabel ?? "";
  }

  function overseerInitial(label: string): string {
    const trimmed = (label || currentUserDisplayName || "A").trim();
    return trimmed ? trimmed.charAt(0).toUpperCase() : "A";
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

  async function onSave(promote: boolean) {
    if (saving || submitted || !configuration) return;
    saving = true;
    saveError = false;
    try {
      const hasCron = !!configuration.schedule?.cronExpression?.trim();
      // "Save draft" persists the current bindings without promoting status.
      // "Save & make runnable" promotes DRAFT → RUNNABLE (or SCHEDULED when a
      // cron is already set), which triggers the server's NextTurn → LandingCard.
      const status = !promote
        ? configuration.status
        : hasCron
          ? PlanConfigurationStatus.SCHEDULED
          : PlanConfigurationStatus.RUNNABLE;
      const res = await planClient.updatePlanConfiguration({
        tenantId,
        planConfigurationId: configurationId,
        status,
        seedArtifacts: configuration.seedArtifacts,
        slotBindings: configuration.slotBindings,
        overseerBindings: configuration.overseerBindings,
        behaviorPolicies: configuration.behaviorPolicies,
        schedule: configuration.schedule,
      });
      if (res.planConfiguration) configuration = res.planConfiguration;
      submitted = true;
    } catch {
      saveError = true;
    } finally {
      saving = false;
    }
  }

  const introText = $derived(message.text);
  const templateName = $derived(template?.name ?? "");
</script>

<div id={`m-${message.id}`} class="flex flex-col gap-3">
  {#if introText}
    <p class="font-body text-[14px] leading-relaxed text-cream">{introText}</p>
  {/if}

  {#if parseError || !payload}
    <p
      class="rounded-lg border border-plumage bg-surface-elevated px-4 py-3 text-[13px] text-crown-ash"
    >
      {introText}
    </p>
  {:else}
    <div
      class="overflow-hidden rounded-lg border border-plumage bg-surface-elevated shadow-[0_8px_24px_rgba(0,0,0,0.25)]"
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
        <span
          class="rounded border border-talon-gold/35 bg-talon-gold/[0.06] px-2 py-1 text-[10px] font-semibold tracking-[0.1em] text-talon-gold uppercase"
        >
          {boundCount}/{rows.length}
          {translate("assistant.bindingMatrix.progress", $locale)}
        </span>
      </div>

      <!-- Rows -->
      {#each rows as row (row.step_key)}
        {@const bound = row.current_executor_id !== ""}
        <div
          in:cardLift
          class="grid grid-cols-[20px_1fr_240px_120px] items-center gap-3.5 border-b border-obsidian-light px-5 py-3.5 hover:bg-surface-hover"
        >
          <!-- Step number -->
          <div
            class="flex size-5 items-center justify-center rounded-full border text-[10px] font-semibold {bound
              ? 'border-talon-gold bg-talon-gold text-obsidian'
              : 'border-plumage text-crown-ash-dark'}"
          >
            {rows.indexOf(row) + 1}
          </div>

          <!-- Task name + contract -->
          <div class="min-w-0">
            <p class="truncate text-[13px] font-medium text-cream">
              {row.step_title}
            </p>
            <div
              class="mt-0.5 font-mono text-[10px] tracking-[0.02em] text-crown-ash-dark"
            >
              ({row.contracts.input}) → {row.contracts.output}
            </div>
          </div>

          <!-- Executor picker -->
          <div class="relative">
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
            {#if bound}
              {@const sub = optionSublabel(row)}
              {#if sub || optionLabel(row)}
                <div class="mt-1 font-mono text-[10px] text-crown-ash-dark">
                  {sub}
                </div>
              {/if}
            {/if}
          </div>

          <!-- Overseer cell -->
          <div
            class="group flex items-center gap-1.5 text-[11px] text-crown-ash"
          >
            <div
              class="flex size-[18px] items-center justify-center rounded-full border border-plumage bg-surface-pop text-[10px] font-semibold text-talon-gold"
            >
              {overseerInitial(row.current_overseer_label)}
            </div>
            <span class="truncate"
              >{row.current_overseer_label ||
                currentUserDisplayName ||
                ""}</span
            >
            <button
              type="button"
              class="ml-auto cursor-pointer text-crown-ash-dark opacity-0 transition-opacity group-hover:opacity-100 hover:text-talon-gold"
              aria-label={translate("assistant.edit", $locale)}
            >
              <Pencil class="size-3" />
            </button>
          </div>
        </div>
      {/each}

      <!-- Policies footer -->
      <div
        class="flex flex-wrap gap-6 border-b border-plumage bg-surface-deep px-5 py-3.5"
      >
        <div class="flex flex-col gap-1">
          <span
            class="text-[10px] font-semibold tracking-[0.1em] text-crown-ash-dark uppercase"
            >{translate("assistant.bindingMatrix.timeout", $locale)}</span
          >
          <span class="text-[12px] text-crown-ash">
            {configuration?.behaviorPolicies?.elicitationTimeoutHours ?? "—"} h
          </span>
        </div>
        <div class="flex flex-col gap-1">
          <span
            class="text-[10px] font-semibold tracking-[0.1em] text-crown-ash-dark uppercase"
            >{translate("assistant.bindingMatrix.approval", $locale)}</span
          >
          <span class="text-[12px] text-crown-ash">
            {payload.policies_set ? "✓" : "—"}
          </span>
        </div>
        <div class="flex flex-col gap-1">
          <span
            class="text-[10px] font-semibold tracking-[0.1em] text-crown-ash-dark uppercase"
            >{translate("assistant.bindingMatrix.schedule", $locale)}</span
          >
          <span class="text-[12px] text-crown-ash">
            {configuration?.schedule?.cronExpression?.trim() || "Manual"}
          </span>
        </div>
      </div>

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
              {cost.unboundCount} pending
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
              class="text-on-primary cursor-pointer rounded-md bg-primary px-3.5 py-2 text-[12px] font-semibold shadow-[0_1px_0_rgba(255,255,255,0.15)_inset] hover:opacity-90 disabled:cursor-not-allowed disabled:bg-surface-pop disabled:text-crown-ash-dark disabled:opacity-100 disabled:shadow-none"
            >
              {translate("assistant.bindingMatrix.savePrimary", $locale)}
            </button>
          </div>
        {/if}
      </div>
    </div>

    {#if saveError}
      <p transition:fade class="text-[11px] text-red-400">
        {translate("thread.loadError", $locale)}
      </p>
    {/if}
  {/if}
</div>
