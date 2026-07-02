<script lang="ts">
  import { goto, invalidateAll } from "$app/navigation";
  import { resolve } from "$app/paths";
  import { cubicOut } from "svelte/easing";
  import type { ChatMessage } from "$lib/chat/types";
  import { locale, translate } from "$lib/i18n";
  import { planClient, threadClient } from "$lib/rpc";
  import {
    ThreadMessageKind,
    ThreadMessageRole,
  } from "$lib/gen/harpia/chat/v1/chat_pb";
  import type {
    PlanConfiguration,
    PlanTemplate,
    TemplateInputParameter,
  } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { PlanConfigurationStatus } from "$lib/gen/harpia/plans/v1/plans_pb";
  import TemplateInputsForm from "./TemplateInputsForm.svelte";
  import ScheduleDialog from "$lib/components/canvas/ScheduleDialog.svelte";
  import {
    applyRefinementSelection,
    buildFinalConfirmation,
    confirmPrompt,
    createdActionI18nKey,
    createdActionIds,
    selectBestCandidate,
    type CreatedActionId,
    type PlanProposalCandidate,
    type ProposalStage,
    type RefinementState,
    type RefinementTurn,
  } from "./plan-proposal-logic";
  import {
    genericInputInitialValues,
    genericParameterValuesJson,
    requiredInputsSatisfied,
  } from "$lib/plans/template-inputs";
  import { ensureAggregateRssInstallation } from "$lib/plans/source-groups";
  import { prefersReducedMotion } from "$lib/motion/reducedMotion";
  import {
    clickFlash,
    errorShake,
    hoverCardLift,
    saveCelebration,
  } from "$lib/motion/transitions";

  interface Props {
    message: ChatMessage;
    tenantId: string;
    threadId: string;
  }

  let { message, tenantId, threadId }: Props = $props();

  const confidenceThreshold = 0.6;

  const payload = $derived.by(() => {
    try {
      return JSON.parse(message.payloadJson || "{}") as {
        summary?: string;
        best_candidate_id?: string;
        candidates?: PlanProposalCandidate[];
      };
    } catch {
      return {};
    }
  });

  const summary = $derived(
    typeof payload.summary === "string" ? payload.summary : "",
  );

  const candidates = $derived.by<PlanProposalCandidate[]>(() =>
    Array.isArray(payload.candidates) ? payload.candidates : [],
  );
  const bestCandidate = $derived(
    selectBestCandidate(candidates, payload.best_candidate_id),
  );

  const autoSelected = $derived.by<PlanProposalCandidate | null>(() => {
    if (candidates.length === 1 && candidates[0].confidence >= confidenceThreshold) {
      return candidates[0];
    }
    return null;
  });

  let selected = $state<PlanProposalCandidate | null>(null);
  const active = $derived(selected ?? autoSelected);

  let stage = $state<ProposalStage>("confirm");
  let template = $state<PlanTemplate | null>(null);
  let values = $state<Record<string, unknown>>({});
  let loading = $state(false);
  let creating = $state(false);
  let errorMsg = $state<string | null>(null);
  let showErrorShake = $state(false);
  let createdConfig = $state<PlanConfiguration | null>(null);
  let scheduleOpen = $state(false);
  let busyAction = $state<CreatedActionId | null>(null);
  let refinementState = $state<RefinementState | undefined>();
  let anythingElsePrompted = $state(false);
  // Tracks how the conversation progressed so the transcript keeps every turn
  // visible instead of swapping a card's contents in place.
  let confirmChoice = $state<"yes" | "adjust" | null>(null);
  let wentThroughForm = $state(false);

  // The assistant's opening confirmation sentence, restated in the second person.
  const confirmPromptText = $derived.by(() => {
    const cand = active;
    if (!cand) return "";
    return confirmPrompt(
      $locale,
      summary,
      cand.template_name,
      cand.template_key,
    );
  });

  const createdActions = $derived.by(() => {
    if (!createdConfig) return [];
    return createdActionIds(createdConfig.status);
  });

  function stageTransition(_node: HTMLElement) {
    if (prefersReducedMotion()) {
      return { duration: 0, css: (t: number) => `opacity: ${t};` };
    }
    return {
      duration: 180,
      easing: cubicOut,
      css: (t: number, u: number) =>
        `opacity: ${t}; transform: translateY(${u * 6}px);`,
    };
  }

  $effect(() => {
    const cand = active;
    template = null;
    if (!cand) return;
    loading = true;
    errorMsg = null;
    (async () => {
      try {
        const resp = await planClient.getPlanTemplate({
          planTemplateId: cand.template_id,
        });
        const tpl = resp.planTemplate ?? null;
        template = tpl;
        const params: TemplateInputParameter[] = tpl?.inputParameters ?? [];
        let extracted: Record<string, unknown> = {};
        try {
          extracted = JSON.parse(cand.input_values_json || "{}");
        } catch {
          extracted = {};
        }
        values = genericInputInitialValues(params, extracted);
      } catch {
        errorMsg = translate("thread.propose.loadFailed", $locale);
      } finally {
        loading = false;
      }
    })();
  });

  async function appendSelectionMessage(
    kind: RefinementTurn["kind"],
    label: string,
    value: RefinementTurn["value"],
    summary: string,
  ) {
    try {
      await threadClient.appendThreadMessage({
        tenantId,
        threadId,
        role: ThreadMessageRole.OVERSEER,
        kind: ThreadMessageKind.USER_SELECTION,
        text: `${label}: ${summary}`,
        payloadJson: JSON.stringify({
          in_response_to_message_id: message.id,
          option_id: kind,
          value,
        }),
        executionId: "",
      });
    } catch {
      // Best effort: the local turn still preserves the immediate interaction.
    }
  }

  function rememberSelection(kind: RefinementTurn["kind"], label: string, value: RefinementTurn["value"]) {
    refinementState = applyRefinementSelection(refinementState, {
      kind,
      label,
      value,
    });
    const summary = refinementState.turns[refinementState.turns.length - 1]?.summary ?? "";
    void appendSelectionMessage(kind, label, value, summary);
  }

  function pickCandidate(candidate: PlanProposalCandidate) {
    selected = candidate;
    stage = "confirm";
    rememberSelection(
      "candidate",
      translate("thread.propose.turn.plan", $locale),
      candidate.template_name || candidate.template_key,
    );
  }

  async function savePlan(cand: PlanProposalCandidate): Promise<PlanConfiguration | null> {
    const { savePlanConfigurationRecord } = await import(
      "$lib/plans/plan-configuration"
    );
    if (!template) return null;
    if (Array.isArray(values.source_groups) && values.source_groups.length > 1) {
      const aggregateId = await ensureAggregateRssInstallation({
        tenantId,
        selectedInstallationIds: values.source_groups.filter(
          (id): id is string => typeof id === "string" && id.trim() !== "",
        ),
      });
      if (aggregateId) {
        values.aggregate_source_group = aggregateId;
        values.source_group = aggregateId;
      }
    }
    return savePlanConfigurationRecord({
      tenantId,
      template,
      status: PlanConfigurationStatus.DRAFT,
      threadId,
      parameterValuesJson: genericParameterValuesJson(values),
    });
  }

  async function createAndCelebrate() {
    const cand = active;
    if (!cand || !template || creating) return;
    creating = true;
    errorMsg = null;
    showErrorShake = false;
    try {
      const saved = await savePlan(cand);
      if (!saved) return;
      const res = await planClient.getPlanConfiguration({
        tenantId,
        planConfigurationId: saved.id,
      });
      createdConfig = res.planConfiguration ?? saved;
      stage = "created";
    } catch (e) {
      errorMsg =
        e instanceof Error ? e.message : translate("thread.saveError", $locale);
      showErrorShake = true;
    } finally {
      creating = false;
    }
  }

  async function onConfirmYes() {
    if (!template) return;
    confirmChoice = "yes";
    if (active) {
      rememberSelection(
        "confirmation",
        translate("thread.propose.turn.confirmation", $locale),
        translate("thread.propose.confirmYes", $locale),
      );
    }
    const params = template.inputParameters ?? [];
    if (requiredInputsSatisfied(params, values)) {
      await createAndCelebrate();
    } else {
      wentThroughForm = true;
      stage = "form";
    }
  }

  function onAdjust() {
    confirmChoice = "adjust";
    if (active) {
      rememberSelection(
        "confirmation",
        translate("thread.propose.turn.adjust", $locale),
        translate("thread.propose.adjust", $locale),
      );
    }
    wentThroughForm = true;
    stage = "form";
  }

  async function onFormSubmit() {
    const range = values.date_range as { startDate?: string; endDate?: string } | undefined;
    if (active && typeof values.audience === "string" && range?.startDate && range.endDate) {
      rememberSelection(
        "confirmation",
        translate("thread.propose.turn.confirmation", $locale),
        buildFinalConfirmation($locale, {
          planName: active.template_name || active.template_key,
          audience: values.audience,
          themes: typeof values.theme === "string" ? [values.theme] : [],
          sourceGroups:
            typeof values.source_group === "string" ? [values.source_group] : [],
          dateRange: { startDate: range.startDate, endDate: range.endDate },
        }),
      );
    }
    await createAndCelebrate();
  }

  async function onCreatedAction(action: CreatedActionId) {
    if (!createdConfig || busyAction) return;
    busyAction = action;
    errorMsg = null;
    showErrorShake = false;
    try {
      if (action === "runNow") {
        await planClient.createPlanExecution({
          tenantId,
          planConfigurationId: createdConfig.id,
        });
        // The thread is already open at /chat/[threadId]; re-run its load so the
        // now-attached configuration and the new execution render. goto() to the
        // current URL is a no-op, so invalidateAll() is the correct primitive.
        await invalidateAll();
        return;
      }
      if (action === "finishSetup") {
        await invalidateAll();
        return;
      }
      if (action === "schedule") {
        scheduleOpen = true;
        return;
      }
      if (action === "reviewPlan") {
        await invalidateAll();
        return;
      }
      if (action === "adjustConfiguration") {
        await goto(resolve(`/plans/configurations/${createdConfig.id}/canvas`));
        return;
      }
      if (action === "anythingElse") {
        anythingElsePrompted = true;
      }
    } catch {
      errorMsg = translate("thread.runError", $locale);
      showErrorShake = true;
    } finally {
      busyAction = null;
    }
  }

  function isPrimaryAction(action: CreatedActionId): boolean {
    return action === "runNow" || action === "finishSetup";
  }

  // Shared bubble styling. Assistant turns sit on the left in the neutral
  // surface color; the user's replies sit on the right with a gold tint so the
  // two voices are visually distinct as the transcript grows.
  const agentBubble =
    "max-w-[85%] self-start rounded-2xl rounded-tl-sm border border-plumage bg-obsidian-light px-3 py-2 text-[13px] leading-relaxed text-cream";
  const userBubble =
    "max-w-[85%] self-end rounded-2xl rounded-tr-sm border border-talon-gold/40 bg-talon-gold/10 px-3 py-1.5 text-[12px] font-medium text-cream";
</script>

<!--
  The proposal renders as an accumulating conversation transcript: each button
  press leaves the assistant's prompt AND the user's reply on screen and adds the
  next turn below, rather than swapping a single card's contents in place.
-->
<div id={`m-${message.id}`} class="flex w-full flex-col gap-2">
  {#if candidates.length === 0}
    <div class={agentBubble} in:stageTransition>
      {translate("thread.propose.noMatch", $locale)}
      <a class="text-talon-gold underline" href={resolve("/new")}>
        {translate("thread.propose.browseTemplates", $locale)}
      </a>.
    </div>
  {:else if !active}
    <div class={agentBubble} in:stageTransition>
      {translate("thread.propose.whichPlan", $locale)}
    </div>
    <div class="flex flex-wrap gap-2 self-start">
      {#each candidates as c (c.template_id)}
        <button
          type="button"
          use:hoverCardLift
          use:clickFlash
          class="rounded-full border border-plumage px-3 py-1 text-[12px] text-cream hover:border-talon-gold"
          aria-label={c.template_id === bestCandidate?.template_id
            ? translate("thread.propose.bestMatchAria", $locale, { plan: c.template_name || c.template_key })
            : c.template_name || c.template_key}
          onclick={() => pickCandidate(c)}
        >
          {c.template_name || c.template_key}
          {#if c.template_id === bestCandidate?.template_id}
            <span class="ml-1 text-talon-gold">
              {translate("thread.propose.bestMatch", $locale)}
            </span>
          {/if}
        </button>
      {/each}
    </div>
  {:else}
    <!-- Turn 1 — assistant restates the intent and asks to confirm. -->
    <div class={agentBubble} in:stageTransition>
      {confirmPromptText}
    </div>

    {#if stage === "confirm"}
      <div class="flex flex-wrap gap-2 self-start" in:stageTransition>
        <button
          type="button"
          use:clickFlash
          disabled={creating || loading || !template}
          onclick={onConfirmYes}
          class="rounded-md bg-primary px-3 py-1.5 text-[12px] font-semibold text-on-primary hover:opacity-90 disabled:opacity-50"
        >
          {creating
            ? translate("thread.propose.creating", $locale)
            : translate("thread.propose.confirmYes", $locale)}
        </button>
        <button
          type="button"
          use:clickFlash
          disabled={creating}
          onclick={onAdjust}
          class="rounded-md border border-plumage px-3 py-1.5 text-[12px] text-crown-ash hover:border-talon-gold hover:text-cream"
        >
          {translate("thread.propose.adjust", $locale)}
        </button>
      </div>
      {#if candidates.length > 1}
        <div class="self-start">
          <p class="mb-1 text-[11px] text-crown-ash-dark">
            {translate("thread.propose.otherPlan", $locale)}
          </p>
          <div class="flex flex-wrap gap-1.5">
            {#each candidates as c (c.template_id)}
              {@const isBest = c.template_id === bestCandidate?.template_id}
              <button
                type="button"
                use:hoverCardLift
                use:clickFlash
                aria-label={isBest
                  ? translate("thread.propose.bestMatchAria", $locale, { plan: c.template_name || c.template_key })
                  : c.template_name || c.template_key}
                class="rounded-full border px-2.5 py-0.5 text-[11px] {c.template_id === active.template_id
                  ? 'border-talon-gold text-cream'
                  : 'border-plumage text-crown-ash hover:border-talon-gold'}"
                onclick={() => pickCandidate(c)}
              >
                {c.template_name || c.template_key}
                {#if isBest}
                  <span class="ml-1 text-talon-gold">
                    {translate("thread.propose.bestMatch", $locale)}
                  </span>
                {/if}
              </button>
            {/each}
          </div>
        </div>
      {/if}
      {#if loading}
        <p class="self-start text-[12px] text-crown-ash-dark">
          {translate("thread.propose.loadingTemplate", $locale)}
        </p>
      {/if}
    {:else}
      <!-- The user's answer to the confirmation, kept in the transcript. -->
      <div class={userBubble} in:stageTransition>
        {confirmChoice === "adjust"
          ? translate("thread.propose.adjust", $locale)
          : translate("thread.propose.confirmYes", $locale)}
      </div>
    {/if}

    {#if wentThroughForm}
      <!-- Turn 2 — assistant collects the remaining details. -->
      <div class={agentBubble} in:stageTransition>
        {translate("thread.propose.formIntro", $locale)}
      </div>
      {#if stage === "form"}
        {#if loading}
          <p class="self-start text-[12px] text-crown-ash-dark">
            {translate("thread.propose.loadingTemplate", $locale)}
          </p>
        {:else if template}
          <div
            class="w-full max-w-[85%] self-start rounded-2xl rounded-tl-sm border border-plumage bg-obsidian-light px-3 py-3"
            in:stageTransition
          >
            <TemplateInputsForm
              params={template.inputParameters}
              bind:values
              {tenantId}
            />
            <button
              type="button"
              use:clickFlash
              class="mt-3 rounded-md bg-primary px-3 py-1.5 text-[12px] font-semibold text-on-primary hover:opacity-90 disabled:opacity-50"
              onclick={onFormSubmit}
              disabled={creating}
            >
              {creating
                ? translate("thread.propose.creating", $locale)
                : translate("thread.propose.create", $locale)}
            </button>
          </div>
        {/if}
      {:else}
        <div class={userBubble} in:stageTransition>
          {translate("thread.propose.create", $locale)}
        </div>
      {/if}
    {/if}

    {#if stage === "created" && createdConfig}
      <!-- Turn 3 — assistant confirms creation and offers next actions. -->
      <div class="{agentBubble} w-full" in:saveCelebration role="status">
        <div class="flex items-center gap-2">
          <svg
            class="size-6 shrink-0"
            viewBox="0 0 52 52"
            fill="none"
            xmlns="http://www.w3.org/2000/svg"
            aria-hidden="true"
          >
            <circle
              cx="26"
              cy="26"
              r="24"
              stroke="var(--token-primary, #c8920f)"
              stroke-width="2"
              opacity="0.4"
            />
            <path
              class="proposal-check"
              d="M15 27 L23 35 L38 18"
              stroke="var(--token-primary, #c8920f)"
              stroke-width="3"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
          </svg>
          <p class="text-[14px] font-medium text-cream">
            {translate("thread.created.title", $locale)}
          </p>
        </div>
        <div class="mt-3 flex flex-wrap items-center gap-2">
          {#each createdActions as action (action)}
            <button
              type="button"
              use:clickFlash
              disabled={busyAction !== null}
              onclick={() => onCreatedAction(action)}
              class={isPrimaryAction(action)
                ? "cursor-pointer rounded-md bg-primary px-3 py-1.5 text-[12px] font-semibold text-on-primary hover:opacity-90 disabled:opacity-50"
                : "cursor-pointer rounded-md border border-plumage px-3 py-1.5 text-[12px] text-crown-ash hover:border-talon-gold hover:text-cream disabled:opacity-50"}
            >
              {translate(createdActionI18nKey(action), $locale)}
            </button>
          {/each}
        </div>
        {#if anythingElsePrompted}
          <p class="mt-2 text-[12px] leading-relaxed text-crown-ash">
            {translate("thread.created.anythingElsePrompt", $locale)}
          </p>
        {/if}
      </div>
    {/if}
  {/if}

  {#if errorMsg}
    {#if showErrorShake}
      <p class="self-start text-[11px] text-red-400" in:errorShake>{errorMsg}</p>
    {:else}
      <p class="self-start text-[11px] text-red-400">{errorMsg}</p>
    {/if}
  {/if}
</div>

{#if createdConfig}
  <ScheduleDialog
    open={scheduleOpen}
    configuration={createdConfig}
    onClose={() => (scheduleOpen = false)}
    onSaved={(next) => (createdConfig = next)}
  />
{/if}

<style>
  .proposal-check {
    stroke-dasharray: 48;
    stroke-dashoffset: 48;
    animation: proposal-draw 600ms cubic-bezier(0.65, 0, 0.35, 1) 200ms forwards;
  }
  @keyframes proposal-draw {
    to {
      stroke-dashoffset: 0;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .proposal-check {
      animation: none;
      stroke-dashoffset: 0;
    }
  }
</style>
