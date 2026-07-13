<script lang="ts">
  import {
    AlertTriangle,
    CheckCircle2,
    CircleAlert,
    Loader2,
    RotateCcw,
  } from "lucide-svelte";
  import { ExecutionPromptState } from "$lib/chat/types";
  import { locale, translate } from "$lib/i18n";
  import type {
    ExecutionAssistantTurn,
    ExecutionStepView,
  } from "$lib/plans/execution-view";
  import ElicitationRefCard from "./ElicitationRefCard.svelte";
  import ReviewRefCard from "./ReviewRefCard.svelte";
  import ApprovalRefCard from "./ApprovalRefCard.svelte";

  interface Props {
    turn: ExecutionAssistantTurn;
    currentStep?: ExecutionStepView | null;
    tenantId: string;
    onOpenArtifact?: (artifactId: string, artifactVersionId?: string) => void;
    onDecided?: () => void;
    onRetry?: (stepExecutionId: string) => void;
    onRunAgain?: () => void;
  }
  let {
    turn,
    currentStep = null,
    tenantId,
    onOpenArtifact,
    onDecided,
    onRetry,
    onRunAgain,
  }: Props = $props();
  const interaction = $derived(turn.pendingInteraction);
  const stateKey = $derived.by(() => {
    switch (turn.state) {
      case ExecutionPromptState.EXECUTION_QUEUED:
        return "thread.execution.turn.queued";
      case ExecutionPromptState.EXECUTION_RUNNING:
        return "thread.execution.turn.running";
      case ExecutionPromptState.EXECUTION_AWAITING_ELICITATION:
        return "thread.execution.turn.awaitingElicitation";
      case ExecutionPromptState.EXECUTION_AWAITING_REVIEW:
        return "thread.execution.turn.awaitingReview";
      case ExecutionPromptState.EXECUTION_AWAITING_APPROVAL:
        return "thread.execution.turn.awaitingApproval";
      case ExecutionPromptState.EXECUTION_FAILING:
        return "thread.execution.turn.failing";
      case ExecutionPromptState.EXECUTION_FAILED:
        return "thread.execution.turn.failed";
      case ExecutionPromptState.EXECUTION_COMPLETED:
        return "thread.execution.turn.completed";
      case ExecutionPromptState.EXECUTION_CANCELLED:
        return "thread.execution.turn.cancelled";
      default:
        return "thread.execution.turn.needsAttention";
    }
  });
  const isFailed = $derived(
    turn.state === ExecutionPromptState.EXECUTION_FAILED,
  );
  const needsAttention = $derived(
    turn.state === ExecutionPromptState.EXECUTION_NEEDS_ATTENTION,
  );
</script>

<section
  class="mt-2 rounded-md border border-plumage/60 bg-obsidian px-3 py-2"
  aria-live="polite"
>
  <div class="flex items-center gap-2">
    {#if needsAttention}
      <CircleAlert class="size-4 text-danger" />
    {:else if isFailed}
      <AlertTriangle class="size-4 text-danger" />
    {:else if turn.state === ExecutionPromptState.EXECUTION_COMPLETED}
      <CheckCircle2 class="size-4 text-status-done" />
    {:else}
      <Loader2
        class="size-4 text-energy {turn.state ===
        ExecutionPromptState.EXECUTION_RUNNING
          ? 'animate-spin'
          : ''}"
      />
    {/if}
    <p class="text-[12px] font-medium text-cream">
      {translate(stateKey, $locale)}
    </p>
  </div>
  {#if currentStep}
    <p class="mt-1 pl-6 text-[11px] text-crown-ash">
      {translate("thread.execution.turn.currentStep", $locale, {
        step: currentStep.title,
      })}
    </p>
  {/if}

  {#if interaction?.kind === "elicitation"}
    <div class="mt-2">
      <ElicitationRefCard
        elicitationId={interaction.requestId}
        {tenantId}
        {onDecided}
      />
    </div>
  {:else if interaction?.kind === "review"}
    <div class="mt-2">
      <ReviewRefCard
        reviewRequestId={interaction.requestId}
        subjectArtifactRef={interaction.subjectArtifactRef}
        {tenantId}
        {onOpenArtifact}
        {onDecided}
      />
    </div>
  {:else if interaction?.kind === "approval"}
    <div class="mt-2">
      <ApprovalRefCard
        approvalRequestId={interaction.requestId}
        subjectArtifactRef={interaction.subjectArtifactRef}
        {tenantId}
        {onOpenArtifact}
        {onDecided}
      />
    </div>
  {:else if needsAttention}
    <p class="mt-2 text-[11px] text-danger">
      {translate("thread.execution.turn.needsAttentionDetail", $locale)}
    </p>
  {:else if isFailed}
    <div class="mt-2 flex flex-wrap gap-2">
      {#if turn.stepExecutionId && onRetry}
        <button
          type="button"
          onclick={() => onRetry?.(turn.stepExecutionId)}
          class="inline-flex items-center gap-1 rounded-md border border-danger/60 px-2.5 py-1.5 text-[11px] text-danger hover:bg-danger/10"
        >
          <RotateCcw class="size-3.5" />{translate(
            "thread.execution.turn.retry",
            $locale,
          )}
        </button>
      {/if}
      {#if onRunAgain}
        <button
          type="button"
          onclick={onRunAgain}
          class="rounded-md border border-plumage px-2.5 py-1.5 text-[11px] text-crown-ash hover:border-talon-gold hover:text-talon-gold"
        >
          {translate("thread.execution.turn.runAgain", $locale)}
        </button>
      {/if}
    </div>
  {/if}
</section>
