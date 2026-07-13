<script lang="ts">
  import { Eye, ShieldCheck, ShieldX, ShieldQuestion } from "lucide-svelte";
  import { fade } from "svelte/transition";
  import type { ChatMessage } from "$lib/chat/types";
  import type { StepTitleResolver } from "$lib/chat/event-text";
  import { locale, translate } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";
  import { loadApproval, respondToApprovalRequest } from "$lib/plans/approvals";
  import {
    effectiveApprovalDecision,
    parseApprovalPayloadContext,
    shortApprovalContextId,
    type ApprovalDecision,
  } from "$lib/plans/approval-card";
  import { toUserMessage } from "$lib/connect-errors";
  import { approvalAnchorId } from "$lib/inbox/links";
  import { chipFlash } from "$lib/motion/transitions";

  interface Props {
    message?: ChatMessage;
    /** Exact request identity from the execution prompt projection. */
    approvalRequestId?: string;
    subjectArtifactRef?: {
      artifactId: string;
      artifactVersionId: string;
      artifactTypeKey: string;
      contentHash: string;
    } | null;
    /** Required to call the decision RPC. Hidden when absent. */
    tenantId?: string;
    /**
     * Full thread message stream. Used to detect a matching APPROVAL_DECIDED
     * for this approval_request_id that arrived later, so a RAISED card flips
     * to its terminal state instead of re-showing the action buttons.
     */
    messages?: ChatMessage[];
    inputArtifactId?: string;
    onOpenArtifact?: (artifactId: string, artifactVersionId?: string) => void;
    onDecided?: () => void;
    /**
     * Resolves a step_key to its localized template title so the approval
     * context reads "Etapa Publicar no LinkedIn" rather than the raw step key.
     * Defaults to the identity function when no resolver is supplied.
     */
    stepTitleFor?: StepTitleResolver;
  }

  let {
    message,
    approvalRequestId = "",
    subjectArtifactRef = null,
    tenantId = "",
    messages = [],
    inputArtifactId = "",
    onOpenArtifact,
    onDecided,
    stepTitleFor,
  }: Props = $props();

  const parsed = $derived(
    parseApprovalPayloadContext(message?.payloadJson || ""),
  );
  const exactApprovalId = $derived(
    approvalRequestId || parsed?.approvalRequestId || "",
  );

  // Local optimistic decision after a successful RPC, before the
  // APPROVAL_DECIDED message streams in.
  let localDecision = $state<ApprovalDecision>(null);
  let submitting = $state(false);
  let rejectMode = $state(false);
  let rejectReason = $state("");
  let errorMessage = $state<string | null>(null);
  let loadedInputArtifactId = $state("");
  let loadedArtifactVersionId = $state("");

  const effectiveInputArtifactId = $derived(
    subjectArtifactRef?.artifactId ||
      inputArtifactId ||
      parsed?.inputArtifactId ||
      loadedInputArtifactId,
  );
  const effectivePlanExecutionId = $derived(
    parsed?.planExecutionId || message?.executionId || "",
  );

  $effect(() => {
    if (
      !tenantId ||
      !exactApprovalId ||
      (effectiveInputArtifactId && subjectArtifactRef?.artifactVersionId)
    ) {
      return;
    }
    const controller = new AbortController();
    void (async () => {
      try {
        const approval = await loadApproval(tenantId, exactApprovalId);
        if (!controller.signal.aborted) {
          loadedInputArtifactId = approval.inputArtifactId;
          // The projected subjectArtifactRef is authoritative for preview.
          // Older pointer cards may not carry one, so they retain artifact-id
          // only rather than selecting a mutable current version here.
        }
      } catch {
        // The approve/reject controls still work with only approval_request_id.
      }
    })();
    return () => controller.abort();
  });

  // Effective terminal decision, if any: a real APPROVAL_DECIDED payload
  // wins, then a streamed later message, then the local optimistic state.
  const effectiveDecision = $derived(
    message
      ? effectiveApprovalDecision(message, parsed, messages, localDecision)
      : localDecision,
  );

  // The card is actionable only while the approval is still pending: a
  // RAISED pointer, no decision yet (local or streamed), and we have the
  // inputs the RPC needs.
  const isActionable = $derived(
    message?.kind !== "APPROVAL_DECIDED" &&
      effectiveDecision === null &&
      !!exactApprovalId &&
      !!tenantId,
  );

  const Icon = $derived(
    effectiveDecision === "approved"
      ? ShieldCheck
      : effectiveDecision === "rejected"
        ? ShieldX
        : ShieldQuestion,
  );

  const labelText = $derived.by(() => {
    if (effectiveDecision === "approved")
      return translate("thread.approval.granted", $locale);
    if (effectiveDecision === "rejected")
      return translate("thread.approval.rejected", $locale);
    return translate("thread.approval.raised", $locale);
  });

  const approvalContextItems = $derived.by<{ key: string; label: string }[]>(
    () => {
      const items: { key: string; label: string }[] = [];
      if (parsed?.planStepKey) {
        items.push({
          key: "step",
          label: translate("thread.approval.context.step", $locale, {
            value: stepTitleFor?.(parsed.planStepKey) ?? parsed.planStepKey,
          }),
        });
      }
      if (effectiveInputArtifactId) {
        items.push({
          key: "artifact",
          label: translate("thread.approval.context.artifact", $locale, {
            value: shortApprovalContextId(effectiveInputArtifactId),
          }),
        });
      }
      if (effectivePlanExecutionId) {
        items.push({
          key: "execution",
          label: translate("thread.approval.context.execution", $locale, {
            value: shortApprovalContextId(effectivePlanExecutionId),
          }),
        });
      }
      if (exactApprovalId) {
        items.push({
          key: "request",
          label: translate("thread.approval.context.request", $locale, {
            value: shortApprovalContextId(exactApprovalId),
          }),
        });
      }
      return items;
    },
  );

  async function submit(approved: boolean) {
    if (!tenantId || !exactApprovalId || submitting) return;
    // Reject requires a reason (validated server-side). Reveal the reason
    // field on the first reject click, mirroring CanvasApprovalForm.
    if (!approved && rejectReason.trim().length === 0) {
      rejectMode = true;
      return;
    }
    submitting = true;
    errorMessage = null;
    try {
      await respondToApprovalRequest(
        tenantId,
        exactApprovalId,
        approved,
        approved ? "" : rejectReason.trim(),
      );
      localDecision = approved ? "approved" : "rejected";
      rejectMode = false;
      onDecided?.();
    } catch (err) {
      errorMessage = toUserMessage(err);
    } finally {
      submitting = false;
    }
  }
</script>

<div
  id={message ? `m-${message.id}` : undefined}
  class="w-full rounded-lg border px-4 py-3 {effectiveDecision === 'rejected'
    ? 'border-danger/40 bg-danger/10'
    : 'border-talon-gold/40 bg-talon-gold/10'}"
>
  {#if exactApprovalId}
    <!-- Deep-link target for inbox/runs approval links (#m-approval-<id>). -->
    <span id={approvalAnchorId(exactApprovalId)} aria-hidden="true"></span>
  {/if}
  <div class="flex items-center gap-3">
    <Icon
      class="size-4 {effectiveDecision === 'rejected'
        ? 'text-danger'
        : 'text-talon-gold'}"
    />
    <div class="min-w-0 flex-1">
      <div class="text-[13px] text-cream">{labelText}</div>
      {#if approvalContextItems.length > 0}
        <div
          class="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-[10px] text-crown-ash-dark"
        >
          {#each approvalContextItems as contextItem (contextItem.key)}
            <span>{contextItem.label}</span>
          {/each}
        </div>
      {/if}
    </div>
    {#if message}
      <span class="text-[10px] text-crown-ash-dark">
        {formatRelativeTime(message.createdAt, $locale)}
      </span>
    {/if}
  </div>

  {#if isActionable}
    <div class="mt-3 flex flex-wrap items-center gap-2">
      {#if effectiveInputArtifactId && onOpenArtifact}
        <button
          type="button"
          onclick={() =>
            onOpenArtifact(
              effectiveInputArtifactId,
              subjectArtifactRef?.artifactVersionId ||
                loadedArtifactVersionId ||
                undefined,
            )}
          class="cursor-pointer rounded-md border border-plumage bg-transparent px-3 py-2 text-[12px] font-medium text-crown-ash hover:border-talon-gold hover:text-talon-gold disabled:cursor-not-allowed disabled:opacity-50"
        >
          <span class="inline-flex items-center gap-1.5">
            <Eye class="size-3.5" />
            {translate("thread.approval.previewArtifact", $locale)}
          </span>
        </button>
      {/if}
      <button
        type="button"
        in:chipFlash
        onclick={() => submit(true)}
        disabled={submitting}
        class="cursor-pointer rounded-md border border-talon-gold bg-talon-gold px-3 py-2 text-[12px] font-semibold text-on-primary hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
      >
        {translate(
          submitting ? "thread.approval.pending" : "thread.approval.approve",
          $locale,
        )}
      </button>
      <button
        type="button"
        onclick={() => submit(false)}
        disabled={submitting}
        class="cursor-pointer rounded-md border border-danger/60 bg-transparent px-3 py-2 text-[12px] font-medium text-danger hover:bg-danger/10 disabled:cursor-not-allowed disabled:opacity-50"
      >
        {translate("thread.approval.reject", $locale)}
      </button>
    </div>

    {#if rejectMode}
      <textarea
        transition:fade
        class="mt-2 w-full rounded-md border border-plumage bg-obsidian px-3 py-2 text-[12px] text-cream outline-none focus:border-talon-gold"
        rows="2"
        placeholder={translate("thread.approval.rejectReason", $locale)}
        bind:value={rejectReason}
      ></textarea>
    {/if}
  {/if}

  {#if errorMessage}
    <p transition:fade class="mt-2 text-[11px] text-danger">
      {translate("thread.approval.error", $locale)}
    </p>
  {/if}
</div>
