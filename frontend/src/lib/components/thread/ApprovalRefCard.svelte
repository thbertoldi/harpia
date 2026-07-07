<script lang="ts">
  import { Eye, ShieldCheck, ShieldX, ShieldQuestion } from "lucide-svelte";
  import { fade } from "svelte/transition";
  import type { ChatMessage } from "$lib/chat/types";
  import { locale, translate } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";
  import { loadApproval, respondToApprovalRequest } from "$lib/plans/approvals";
  import { toUserMessage } from "$lib/connect-errors";
  import { chipFlash } from "$lib/motion/transitions";

  interface Props {
    message: ChatMessage;
    /** Required to call the decision RPC. Hidden when absent. */
    tenantId?: string;
    /**
     * Full thread message stream. Used to detect a matching APPROVAL_DECIDED
     * for this approval_request_id that arrived later, so a RAISED card flips
     * to its terminal state instead of re-showing the action buttons.
     */
    messages?: ChatMessage[];
    inputArtifactId?: string;
    onOpenArtifact?: (artifactId: string) => void;
    onDecided?: () => void;
  }

  let {
    message,
    tenantId = "",
    messages = [],
    inputArtifactId = "",
    onOpenArtifact,
    onDecided,
  }: Props = $props();

  const isDecidedMessage = $derived(message.kind === "APPROVAL_DECIDED");

  // Parse approval_request_id (and, for APPROVAL_DECIDED, the `approved`
  // flag) out of the per-kind payload. Backend builders live in
  // control-plane/internal/chat/messages.go:
  //   BuildApprovalRaisedPayload  -> { "approval_request_id": string }
  //   BuildApprovalDecidedPayload -> { "approval_request_id": string, "approved": bool }
  const parsed = $derived.by<{
    approvalRequestId: string;
    approved: boolean | null;
    inputArtifactId: string;
    planStepKey: string;
  } | null>(() => {
    try {
      const raw = JSON.parse(message.payloadJson);
      const id =
        typeof raw.approval_request_id === "string"
          ? raw.approval_request_id
          : "";
      if (!id) return null;
      const approved = typeof raw.approved === "boolean" ? raw.approved : null;
      return {
        approvalRequestId: id,
        approved,
        inputArtifactId:
          typeof raw.input_artifact_id === "string"
            ? raw.input_artifact_id
            : "",
        planStepKey:
          typeof raw.plan_step_key === "string" ? raw.plan_step_key : "",
      };
    } catch {
      return null;
    }
  });

  // Whether a later APPROVAL_DECIDED message in this thread already closed
  // this same approval request. Returns the decision (true/false) or null.
  // Prevents double-rendering the buttons once the decision has streamed in.
  const decidedElsewhere = $derived.by<boolean | null>(() => {
    if (!parsed) return null;
    for (const candidate of messages) {
      if (candidate.kind !== "APPROVAL_DECIDED") continue;
      if (candidate.id === message.id) continue;
      try {
        const raw = JSON.parse(candidate.payloadJson);
        if (raw.approval_request_id === parsed.approvalRequestId) {
          return typeof raw.approved === "boolean" ? raw.approved : null;
        }
      } catch {
        continue;
      }
    }
    return null;
  });

  // Local optimistic decision after a successful RPC, before the
  // APPROVAL_DECIDED message streams in.
  let localDecision = $state<"approved" | "rejected" | null>(null);
  let submitting = $state(false);
  let rejectMode = $state(false);
  let rejectReason = $state("");
  let errorMessage = $state<string | null>(null);
  let loadedInputArtifactId = $state("");

  const effectiveInputArtifactId = $derived(
    inputArtifactId || parsed?.inputArtifactId || loadedInputArtifactId,
  );

  $effect(() => {
    if (!tenantId || !parsed?.approvalRequestId || effectiveInputArtifactId) {
      return;
    }
    const controller = new AbortController();
    void (async () => {
      try {
        const approval = await loadApproval(tenantId, parsed.approvalRequestId);
        if (!controller.signal.aborted) {
          loadedInputArtifactId = approval.inputArtifactId;
        }
      } catch {
        // The approve/reject controls still work with only approval_request_id.
      }
    })();
    return () => controller.abort();
  });

  // Effective terminal decision, if any: a real APPROVAL_DECIDED payload
  // wins, then a streamed later message, then the local optimistic state.
  const effectiveDecision = $derived.by<"approved" | "rejected" | null>(() => {
    if (isDecidedMessage) {
      if (parsed?.approved === true) return "approved";
      if (parsed?.approved === false) return "rejected";
      return null;
    }
    if (decidedElsewhere === true) return "approved";
    if (decidedElsewhere === false) return "rejected";
    return localDecision;
  });

  // The card is actionable only while the approval is still pending: a
  // RAISED pointer, no decision yet (local or streamed), and we have the
  // inputs the RPC needs.
  const isActionable = $derived(
    !isDecidedMessage &&
      effectiveDecision === null &&
      !!parsed?.approvalRequestId &&
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

  async function submit(approved: boolean) {
    if (!tenantId || !parsed?.approvalRequestId || submitting) return;
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
        parsed.approvalRequestId,
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
  id={`m-${message.id}`}
  class="w-full rounded-lg border px-4 py-3 {effectiveDecision === 'rejected'
    ? 'border-danger/40 bg-danger/10'
    : 'border-talon-gold/40 bg-talon-gold/10'}"
>
  <div class="flex items-center gap-3">
    <Icon
      class="size-4 {effectiveDecision === 'rejected'
        ? 'text-danger'
        : 'text-talon-gold'}"
    />
    <span class="flex-1 text-[13px] text-cream">{labelText}</span>
    <span class="text-[10px] text-crown-ash-dark">
      {formatRelativeTime(message.createdAt, $locale)}
    </span>
  </div>

  {#if isActionable}
    <div class="mt-3 flex flex-wrap items-center gap-2">
      {#if effectiveInputArtifactId && onOpenArtifact}
        <button
          type="button"
          onclick={() => onOpenArtifact(effectiveInputArtifactId)}
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
