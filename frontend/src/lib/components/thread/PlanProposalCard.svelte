<script lang="ts">
  import { goto } from "$app/navigation";
  import { resolve } from "$app/paths";
  import type { ChatMessage } from "$lib/chat/types";
  import { planClient } from "$lib/rpc";
  import type {
    PlanTemplate,
    TemplateInputParameter,
  } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { PlanConfigurationStatus } from "$lib/gen/harpia/plans/v1/plans_pb";
  import TemplateInputsForm from "./TemplateInputsForm.svelte";
  import {
    genericInputInitialValues,
    genericParameterValuesJson,
  } from "$lib/plans/template-inputs";

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

  const candidates = $derived.by<Candidate[]>(() => {
    try {
      const parsed = JSON.parse(message.payloadJson || "{}");
      return Array.isArray(parsed.candidates) ? parsed.candidates : [];
    } catch {
      return [];
    }
  });

  // Auto-select the single confident candidate; otherwise wait for a pick.
  const autoSelected = $derived.by<Candidate | null>(() => {
    if (candidates.length === 1 && candidates[0].confidence >= confidenceThreshold) {
      return candidates[0];
    }
    return null;
  });

  let selected = $state<Candidate | null>(null);
  const active = $derived(selected ?? autoSelected);

  let template = $state<PlanTemplate | null>(null);
  let values = $state<Record<string, string>>({});
  let loading = $state(false);
  let creating = $state(false);
  let errorMsg = $state<string | null>(null);

  // Load the active candidate's template and seed the form values.
  $effect(() => {
    const cand = active;
    template = null;
    if (!cand) return;
    loading = true;
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
        errorMsg = "Failed to load template.";
      } finally {
        loading = false;
      }
    })();
  });

  async function confirm() {
    const cand = active;
    if (!cand || !template || creating) return;
    creating = true;
    errorMsg = null;
    try {
      const saved = await savePlan(cand);
      if (saved) await goto(resolve(`/chat/${threadId}`), { invalidateAll: true });
    } catch (e) {
      errorMsg = e instanceof Error ? e.message : "Failed to create plan.";
    } finally {
      creating = false;
    }
  }

  async function savePlan(cand: Candidate) {
    const { savePlanConfigurationRecord } = await import(
      "$lib/plans/plan-configuration"
    );
    if (!template) return null;
    return savePlanConfigurationRecord({
      tenantId,
      template,
      status: PlanConfigurationStatus.DRAFT,
      threadId,
      // parameter values flow through the shared helper below
      parameterValuesJson: genericParameterValuesJson(values),
    });
  }
</script>

<div
  id={`m-${message.id}`}
  class="max-w-[85%] self-start rounded-lg border border-plumage bg-obsidian-light px-3 py-3"
>
  {#if candidates.length === 0}
    <p class="text-[13px] text-crown-ash">
      I couldn't match your request to a plan.
      <a class="text-talon-gold underline" href={resolve("/new")}>Browse templates</a>.
    </p>
  {:else if !active}
    <p class="mb-2 text-[13px] text-cream">Which plan did you mean?</p>
    <div class="flex flex-col gap-2">
      {#each candidates as c (c.template_id)}
        <button
          class="rounded border border-plumage px-2 py-1 text-left text-[12px] text-cream hover:border-talon-gold"
          onclick={() => (selected = c)}
        >
          {c.template_name || c.template_key}
        </button>
      {/each}
    </div>
  {:else}
    <p class="mb-2 text-[13px] font-medium text-cream">
      {active.template_name || active.template_key}
    </p>
    {#if loading}
      <p class="text-[12px] text-crown-ash-dark">Loading template…</p>
    {:else if template}
      <TemplateInputsForm
        params={template.inputParameters}
        bind:values
      />
      <button
        class="mt-3 rounded bg-talon-gold px-3 py-1 text-[12px] font-medium text-obsidian disabled:opacity-50"
        onclick={confirm}
        disabled={creating}
      >
        {creating ? "Creating…" : "Create this plan"}
      </button>
    {/if}
  {/if}
  {#if errorMsg}
    <p class="mt-2 text-[11px] text-red-400">{errorMsg}</p>
  {/if}
</div>
