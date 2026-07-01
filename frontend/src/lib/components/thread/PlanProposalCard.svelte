<script lang="ts">
  import { goto } from "$app/navigation";
  import { resolve } from "$app/paths";
  import { cubicOut } from "svelte/easing";
  import type { ChatMessage } from "$lib/chat/types";
  import { locale, translate } from "$lib/i18n";
  import { planClient } from "$lib/rpc";
  import type {
    PlanConfiguration,
    PlanTemplate,
    TemplateInputParameter,
  } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { PlanConfigurationStatus } from "$lib/gen/harpia/plans/v1/plans_pb";
  import TemplateInputsForm from "./TemplateInputsForm.svelte";
  import ScheduleDialog from "$lib/components/canvas/ScheduleDialog.svelte";
  import {
    confirmSummaryLabel,
    createdActionI18nKey,
    createdActionIds,
    type CreatedActionId,
    type ProposalStage,
  } from "./plan-proposal-logic";
  import {
    genericInputInitialValues,
    genericParameterValuesJson,
    requiredInputsSatisfied,
  } from "$lib/plans/template-inputs";
  import { prefersReducedMotion } from "$lib/motion/reducedMotion";
  import {
    clickFlash,
    errorShake,
    hoverCardLift,
    saveCelebration,
  } from "$lib/motion/transitions";

  interface Candidate {
    template_id: string;
    template_key: string;
    template_name: string;
    confidence: number;
    input_values_json: string;
  }

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
        candidates?: Candidate[];
      };
    } catch {
      return {};
    }
  });

  const summary = $derived(
    typeof payload.summary === "string" ? payload.summary : "",
  );

  const candidates = $derived.by<Candidate[]>(() =>
    Array.isArray(payload.candidates) ? payload.candidates : [],
  );

  const autoSelected = $derived.by<Candidate | null>(() => {
    if (candidates.length === 1 && candidates[0].confidence >= confidenceThreshold) {
      return candidates[0];
    }
    return null;
  });

  let selected = $state<Candidate | null>(null);
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

  const confirmLabel = $derived.by(() => {
    const cand = active;
    if (!cand) return "";
    return confirmSummaryLabel(
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

  function pickCandidate(candidate: Candidate) {
    selected = candidate;
    stage = "confirm";
  }

  async function savePlan(cand: Candidate): Promise<PlanConfiguration | null> {
    const { savePlanConfigurationRecord } = await import(
      "$lib/plans/plan-configuration"
    );
    if (!template) return null;
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
    const params = template.inputParameters ?? [];
    if (requiredInputsSatisfied(params, values)) {
      await createAndCelebrate();
    } else {
      stage = "form";
    }
  }

  function onAdjust() {
    stage = "form";
  }

  async function onFormSubmit() {
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
        await goto(resolve(`/chat/${threadId}`), { invalidateAll: true });
        return;
      }
      if (action === "finishSetup") {
        await goto(resolve(`/chat/${threadId}`), { invalidateAll: true });
        return;
      }
      if (action === "schedule") {
        scheduleOpen = true;
        return;
      }
      if (action === "anythingElse") {
        await goto(resolve("/"), { invalidateAll: true });
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
</script>

<div
  id={`m-${message.id}`}
  class="max-w-[85%] self-start rounded-lg border border-plumage bg-obsidian-light px-3 py-3"
>
  {#if candidates.length === 0}
    <p class="text-[13px] text-crown-ash">
      {translate("thread.propose.noMatch", $locale)}
      <a class="text-talon-gold underline" href={resolve("/new")}>
        {translate("thread.propose.browseTemplates", $locale)}
      </a>.
    </p>
  {:else if !active}
    <p class="mb-2 text-[13px] text-cream">
      {translate("thread.propose.whichPlan", $locale)}
    </p>
    <div class="flex flex-wrap gap-2">
      {#each candidates as c (c.template_id)}
        <button
          type="button"
          use:hoverCardLift
          class="rounded-full border border-plumage px-3 py-1 text-[12px] text-cream hover:border-talon-gold"
          onclick={() => pickCandidate(c)}
        >
          {c.template_name || c.template_key}
        </button>
      {/each}
    </div>
  {:else if stage === "created" && createdConfig}
    <div in:saveCelebration role="status" class="flex flex-col items-center gap-3 py-2 text-center">
      <svg
        class="size-10"
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
      <div class="flex flex-wrap items-center justify-center gap-2">
        {#each createdActions as action (action)}
          <button
            type="button"
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
    </div>
  {:else}
    {#key stage}
      <div in:stageTransition>
        {#if stage === "confirm"}
          <p class="mb-3 text-[13px] leading-relaxed text-cream">
            {translate("thread.propose.confirm", $locale, { summary: confirmLabel })}
          </p>
          <div class="flex flex-wrap gap-2">
            <button
              type="button"
              use:clickFlash
              disabled={creating || loading || !template}
              onclick={onConfirmYes}
              class="rounded-md bg-primary px-3 py-1.5 text-[12px] font-semibold text-on-primary disabled:opacity-50"
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
          {#if loading}
            <p class="mt-2 text-[12px] text-crown-ash-dark">
              {translate("thread.propose.loadingTemplate", $locale)}
            </p>
          {/if}
        {:else}
          {#if candidates.length > 1}
            <p class="mb-1 text-[11px] text-crown-ash-dark">
              {translate("thread.propose.otherPlan", $locale)}
            </p>
            <div class="mb-3 flex flex-wrap gap-1.5">
              {#each candidates as c (c.template_id)}
                <button
                  type="button"
                  use:hoverCardLift
                  use:clickFlash
                  class="rounded-full border px-2.5 py-0.5 text-[11px] {c.template_id === active.template_id
                    ? 'border-talon-gold text-cream'
                    : 'border-plumage text-crown-ash hover:border-talon-gold'}"
                  onclick={() => pickCandidate(c)}
                >
                  {c.template_name || c.template_key}
                </button>
              {/each}
            </div>
          {/if}
          {#if loading}
            <p class="text-[12px] text-crown-ash-dark">
              {translate("thread.propose.loadingTemplate", $locale)}
            </p>
          {:else if template}
            <TemplateInputsForm
              params={template.inputParameters}
              bind:values
              {tenantId}
            />
            <button
              type="button"
              class="mt-3 rounded bg-talon-gold px-3 py-1 text-[12px] font-medium text-obsidian disabled:opacity-50"
              onclick={onFormSubmit}
              disabled={creating}
            >
              {creating
                ? translate("thread.propose.creating", $locale)
                : translate("thread.propose.create", $locale)}
            </button>
          {/if}
        {/if}
      </div>
    {/key}
  {/if}

  {#if errorMsg}
    {#if showErrorShake}
      <p class="mt-2 text-[11px] text-red-400" in:errorShake>{errorMsg}</p>
    {:else}
      <p class="mt-2 text-[11px] text-red-400">{errorMsg}</p>
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
