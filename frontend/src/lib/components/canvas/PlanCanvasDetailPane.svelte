<script lang="ts">
  import { locale, translate } from "$lib/i18n";
  import {
    loadElicitation,
    type ElicitationRequest,
  } from "$lib/plans/elicitations";
  import type { PlanStep } from "$lib/gen/harpia/plans/v1/plans_pb";
  import type { CanvasStepState } from "$lib/plans/canvas-state";
  import CanvasApprovalForm from "./CanvasApprovalForm.svelte";
  import CanvasElicitationForm from "./CanvasElicitationForm.svelte";

  interface Props {
    step: PlanStep | null;
    stepState: CanvasStepState | null;
    tenantId: string;
    approvalInputArtifactId?: string;
  }
  let { step, stepState, tenantId, approvalInputArtifactId }: Props = $props();

  let elicitation = $state<ElicitationRequest | null>(null);
  let elicitationLoadError = $state(false);

  $effect(() => {
    elicitation = null;
    elicitationLoadError = false;
    if (
      !stepState ||
      stepState.status !== "awaiting_elicitation" ||
      !stepState.pendingElicitationId
    ) {
      return;
    }
    const eid = stepState.pendingElicitationId;
    let active = true;
    void (async () => {
      try {
        const fetched = await loadElicitation(tenantId, eid);
        if (active) elicitation = fetched;
      } catch {
        if (active) elicitationLoadError = true;
      }
    })();
    return () => {
      active = false;
    };
  });
</script>

<aside
  class="flex h-full w-[340px] flex-col gap-3 border-l border-plumage bg-obsidian-light px-4 py-3"
>
  {#if !step || !stepState}
    <p class="text-[12px] text-crown-ash-dark">
      {translate("canvas.detail.empty", $locale)}
    </p>
  {:else}
    <header class="flex flex-col gap-1 border-b border-plumage pb-3">
      <h2 class="font-heading text-[14px] font-semibold text-cream">
        {step.title}
      </h2>
      <p class="font-mono text-[10px] text-crown-ash-dark">{step.key}</p>
      <p class="text-[11px] text-crown-ash">
        {translate(`canvas.status.${stepState.status}`, $locale)}
      </p>
    </header>

    {#if stepState.status === "awaiting_approval" && stepState.pendingApprovalId}
      <section>
        <h3
          class="mb-2 text-[11px] font-semibold tracking-wider text-crown-ash uppercase"
        >
          {translate("canvas.detail.answerApproval", $locale)}
        </h3>
        <CanvasApprovalForm
          approvalRequestId={stepState.pendingApprovalId}
          inputArtifactId={approvalInputArtifactId}
        />
      </section>
    {:else if stepState.status === "awaiting_elicitation"}
      <section>
        <h3
          class="mb-2 text-[11px] font-semibold tracking-wider text-crown-ash uppercase"
        >
          {translate("canvas.detail.answerElicitation", $locale)}
        </h3>
        {#if elicitationLoadError}
          <p class="text-[12px] text-red-400">
            {translate("canvas.detail.elicitationLoadError", $locale)}
          </p>
        {:else if !elicitation}
          <p class="text-[12px] text-crown-ash-dark">
            {translate("canvas.detail.elicitationLoading", $locale)}
          </p>
        {:else}
          <CanvasElicitationForm {elicitation} />
        {/if}
      </section>
    {:else}
      <section class="flex flex-col gap-2 text-[12px] text-crown-ash">
        {#if stepState.startedAt}
          <p>
            <span class="text-crown-ash-dark"
              >{translate("canvas.detail.startedAt", $locale)}:
            </span>
            {stepState.startedAt}
          </p>
        {/if}
        {#if stepState.completedAt}
          <p>
            <span class="text-crown-ash-dark"
              >{translate("canvas.detail.completedAt", $locale)}:
            </span>
            {stepState.completedAt}
          </p>
        {/if}
        {#if stepState.outputArtifactId}
          <p>
            <span class="text-crown-ash-dark"
              >{translate("canvas.detail.outputArtifact", $locale)}:
            </span>
            <span class="font-mono">{stepState.outputArtifactId}</span>
          </p>
        {/if}
        {#if stepState.errorMessage}
          <p class="text-red-400">{stepState.errorMessage}</p>
        {/if}
      </section>
    {/if}
  {/if}
</aside>
