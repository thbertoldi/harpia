<script lang="ts">
  import type { Snippet } from "svelte";
  import { locale, translate } from "$lib/i18n";
  import { getTenant } from "$lib/auth";
  import { respondToApprovalRequest } from "$lib/plans/approvals";
  import { toUserMessage } from "$lib/connect-errors";
  import ArtifactPreview from "$lib/components/ArtifactPreview.svelte";

  interface Props {
    approvalRequestId: string;
    inputArtifactId?: string;
    onDecided?: (decision: "approved" | "rejected") => void;
    inboxShell?: Snippet<
      [
        {
          formActions: Snippet;
          formFooter: Snippet;
          hasFooter: boolean;
        },
      ]
    >;
  }
  let { approvalRequestId, inputArtifactId, onDecided, inboxShell }: Props =
    $props();

  let expanded = $state(false);
  let submitting = $state(false);
  let decision = $state<"approved" | "rejected" | null>(null);
  let rejectReason = $state("");
  let rejectMode = $state(false);
  let errorMessage = $state<string | null>(null);

  const tenantId = $derived(getTenant()?.id ?? "");
  const hasFooter = $derived(expanded || rejectMode || errorMessage !== null);

  async function submit(approved: boolean) {
    const tenant = getTenant();
    if (!tenant?.id || submitting) return;
    if (!approved && rejectReason.trim().length === 0) {
      rejectMode = true;
      return;
    }
    submitting = true;
    errorMessage = null;
    try {
      await respondToApprovalRequest(
        tenant.id,
        approvalRequestId,
        approved,
        approved ? "" : rejectReason.trim(),
      );
      decision = approved ? "approved" : "rejected";
      expanded = false;
      rejectMode = false;
      onDecided?.(decision);
    } catch (e) {
      errorMessage = toUserMessage(e);
    } finally {
      submitting = false;
    }
  }
</script>

{#snippet formActions()}
  {#if decision}
    <span class="text-[11px] font-semibold text-talon-gold">
      {translate(
        decision === "approved"
          ? "inbox.decision.approved"
          : "inbox.decision.rejected",
        $locale,
      )}
    </span>
  {:else}
    <button
      type="button"
      onclick={() => (expanded = !expanded)}
      disabled={submitting}
      class="rounded border border-plumage bg-transparent px-3 py-1.5 text-[11px] font-medium text-crown-ash hover:border-talon-gold hover:text-talon-gold disabled:opacity-50"
    >
      {translate(
        expanded ? "inbox.actions.hide" : "inbox.actions.preview",
        $locale,
      )}
    </button>
    <button
      type="button"
      onclick={() => submit(false)}
      disabled={submitting}
      class="rounded border border-plumage bg-transparent px-3 py-1.5 text-[11px] font-medium text-crown-ash hover:border-red-400 hover:text-red-400 disabled:opacity-50"
    >
      {translate("inbox.actions.reject", $locale)}
    </button>
    <button
      type="button"
      onclick={() => submit(true)}
      disabled={submitting}
      class="rounded border border-talon-gold bg-talon-gold px-3 py-1.5 text-[11px] font-semibold text-obsidian hover:opacity-90 disabled:opacity-50"
    >
      {translate(
        submitting ? "inbox.actions.submitting" : "inbox.actions.approve",
        $locale,
      )}
    </button>
  {/if}
{/snippet}

{#snippet formFooter()}
  {#if expanded && tenantId && inputArtifactId}
    <div class="mt-3">
      <ArtifactPreview {tenantId} artifactId={inputArtifactId} />
    </div>
  {/if}
  {#if rejectMode}
    <textarea
      class="mt-3 w-full rounded border border-plumage bg-obsidian px-3 py-2 text-[12px] text-cream focus:border-talon-gold focus:outline-none"
      rows="2"
      placeholder={translate("inbox.rejectReason.placeholder", $locale)}
      bind:value={rejectReason}
    ></textarea>
  {/if}
  {#if errorMessage}
    <p class="mt-2 text-[11px] text-red-400">{errorMessage}</p>
  {/if}
{/snippet}

{#if inboxShell}
  {@render inboxShell({ formActions, formFooter, hasFooter })}
{:else}
  <div class="flex flex-col gap-2">
    <div class="flex gap-1.5">
      {@render formActions()}
    </div>
    {#if hasFooter}
      <div class="mt-3 flex flex-col gap-2 border-t border-plumage pt-3">
        {@render formFooter()}
      </div>
    {/if}
  </div>
{/if}
