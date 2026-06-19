<script lang="ts">
  import { AlertTriangle, Check, Loader2, X } from "lucide-svelte";
  import { page } from "$app/state";
  import { resolve } from "$app/paths";
  import { requireTenantId } from "$lib/auth";
  import { toUserMessage } from "$lib/connect-errors";
  import ArtifactPreview from "$lib/components/ArtifactPreview.svelte";
  import { locale, translate } from "$lib/i18n";
  import {
    ApprovalRequestStatus,
    approvalStatusLabelKey,
    isApprovalActionDisabled,
    loadApproval,
    respondToApprovalRequest,
    watchApprovalRequests,
    type ApprovalRequest,
  } from "$lib/plans/approvals";

  let approval = $state<ApprovalRequest | null>(null);
  let loading = $state(true);
  let loadError = $state<string | null>(null);
  let rejectReason = $state("");
  let submitting = $state(false);
  let submitError = $state<string | null>(null);
  let decided = $state(false);

  const formDisabled = $derived(
    !approval || isApprovalActionDisabled(approval.status),
  );

  $effect(() => {
    const approvalId = page.params.approvalId;
    if (!approvalId) {
      loadError = translate("approvals.detail.error", $locale, {
        error: "missing id",
      });
      loading = false;
      return;
    }

    let tenantId: string;
    try {
      tenantId = requireTenantId();
    } catch (error) {
      loadError = toUserMessage(error);
      loading = false;
      return;
    }

    let active = true;

    void (async () => {
      try {
        const loaded = await loadApproval(tenantId, approvalId);
        if (active) {
          approval = loaded;
        }
      } catch (error) {
        if (active) {
          loadError = toUserMessage(error);
        }
      } finally {
        if (active) {
          loading = false;
        }
      }
    })();

    void (async () => {
      try {
        for await (const batch of watchApprovalRequests(tenantId)) {
          if (!active) {
            return;
          }
          const match = batch.find((item) => item.id === approvalId);
          if (match) {
            approval = match;
          }
        }
      } catch {
        // Live updates are best-effort.
      }
    })();

    return () => {
      active = false;
    };
  });

  async function submitDecision(approved: boolean): Promise<void> {
    if (!approval || formDisabled || submitting) {
      return;
    }
    if (!approved && rejectReason.trim() === "") {
      submitError = translate("approvals.detail.rejectReasonRequired", $locale);
      return;
    }

    submitting = true;
    submitError = null;

    try {
      const tenantId = requireTenantId();
      const updated = await respondToApprovalRequest(
        tenantId,
        approval.id,
        approved,
        rejectReason.trim(),
      );
      approval = updated;
      decided = true;
      rejectReason = "";
    } catch (error) {
      submitError = translate("approvals.detail.submitError", $locale, {
        error: toUserMessage(error),
      });
    } finally {
      submitting = false;
    }
  }
</script>

<div class="mx-auto flex w-full max-w-3xl flex-col gap-4 px-4 py-6 lg:px-6">
  <a
    href={resolve("/approvals")}
    class="font-body text-sm text-crown-ash transition-colors hover:text-talon-gold"
  >
    {translate("approvals.detail.back", $locale)}
  </a>

  {#if loading}
    <div class="flex min-h-[16rem] items-center justify-center">
      <div class="flex items-center gap-2 text-crown-ash">
        <Loader2 class="size-5 animate-spin" />
        <span class="font-body text-sm">
          {translate("approvals.detail.loading", $locale)}
        </span>
      </div>
    </div>
  {:else if !approval}
    <div class="rounded-lg border border-red-500/30 bg-red-500/10 p-5">
      <div class="flex items-start gap-3">
        <AlertTriangle class="mt-0.5 size-5 text-red-400" />
        <p class="font-body text-sm text-red-300">
          {translate("approvals.detail.error", $locale, {
            error: loadError ?? "",
          })}
        </p>
      </div>
    </div>
  {:else}
    <section class="rounded-lg border border-plumage bg-obsidian-light/50 p-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 class="font-heading text-xl font-semibold text-cream">
            {translate("approvals.detail.heading", $locale)}
          </h1>
          <p class="mt-1 font-mono text-[11px] text-crown-ash">
            {translate("approvals.detail.stepLabel", $locale)}:
            {approval.planStepKey || approval.stepExecutionId}
          </p>
        </div>
        <span
          class="rounded-full border border-plumage px-2.5 py-1 font-mono text-[10px] tracking-wider text-crown-ash uppercase"
        >
          {translate(approvalStatusLabelKey(approval.status), $locale)}
        </span>
      </div>
    </section>

    {#if approval.inputArtifactId}
      <section
        class="rounded-lg border border-plumage bg-obsidian-light/30 p-4"
      >
        <h2 class="mb-3 font-heading text-lg font-semibold text-cream">
          {translate("approvals.detail.previewHeading", $locale)}
        </h2>
        <ArtifactPreview
          tenantId={approval.tenantId}
          artifactId={approval.inputArtifactId}
        />
      </section>
    {/if}

    <section class="rounded-lg border border-plumage bg-obsidian-light/30 p-4">
      {#if formDisabled}
        <p class="font-body text-sm text-crown-ash">
          {#if approval.status === ApprovalRequestStatus.APPROVED}
            {translate("approvals.detail.approvedMessage", $locale)}
          {:else if approval.status === ApprovalRequestStatus.REJECTED}
            {translate("approvals.detail.rejectedMessage", $locale, {
              reason: approval.decisionReason,
            })}
          {:else}
            {translate("approvals.detail.closed", $locale)}
          {/if}
        </p>
      {:else}
        <h2 class="mb-3 font-heading text-lg font-semibold text-cream">
          {translate("approvals.detail.actionHeading", $locale)}
        </h2>
        <p class="mb-4 font-body text-sm text-crown-ash">
          {translate("approvals.detail.actionDescription", $locale)}
        </p>

        <div class="space-y-3">
          <label
            for="reject-reason"
            class="block font-body text-xs text-crown-ash"
          >
            {translate("approvals.detail.rejectReasonLabel", $locale)}
          </label>
          <textarea
            id="reject-reason"
            bind:value={rejectReason}
            rows="3"
            placeholder={translate(
              "approvals.detail.rejectReasonPlaceholder",
              $locale,
            )}
            class="w-full rounded-md border border-plumage bg-obsidian px-3 py-2 font-body text-sm text-cream focus:border-talon-gold focus:outline-none"
          ></textarea>

          {#if submitError}
            <p class="font-body text-xs text-red-400">{submitError}</p>
          {/if}
          {#if decided}
            <p class="font-body text-xs text-green-400">
              {translate("approvals.detail.submitted", $locale)}
            </p>
          {/if}

          <div class="flex flex-wrap gap-3 pt-1">
            <button
              type="button"
              disabled={submitting}
              onclick={() => submitDecision(true)}
              class="inline-flex cursor-pointer items-center gap-2 rounded-md border border-green-500/50 bg-green-500/10 px-4 py-2 font-body text-sm text-green-400 transition-colors hover:bg-green-500/20 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {#if submitting}
                <Loader2 class="size-4 animate-spin" />
              {:else}
                <Check class="size-4" />
              {/if}
              {translate("approvals.detail.approve", $locale)}
            </button>
            <button
              type="button"
              disabled={submitting}
              onclick={() => submitDecision(false)}
              class="inline-flex cursor-pointer items-center gap-2 rounded-md border border-red-500/50 bg-red-500/10 px-4 py-2 font-body text-sm text-red-400 transition-colors hover:bg-red-500/20 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {#if submitting}
                <Loader2 class="size-4 animate-spin" />
              {:else}
                <X class="size-4" />
              {/if}
              {translate("approvals.detail.reject", $locale)}
            </button>
          </div>
        </div>
      {/if}
    </section>
  {/if}
</div>
