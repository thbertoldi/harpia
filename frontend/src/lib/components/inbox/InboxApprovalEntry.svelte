<script lang="ts">
  import { resolve } from "$app/paths";
  import { locale, translate } from "$lib/i18n";
  import { getTenant } from "$lib/auth";
  import { respondToApprovalRequest } from "$lib/plans/approvals";
  import { toUserMessage } from "$lib/connect-errors";
  import type { InboxApprovalItem } from "$lib/inbox/types";
  import InboxRow from "./InboxRow.svelte";
  import ArtifactPreview from "$lib/components/ArtifactPreview.svelte";

  interface Props {
    item: InboxApprovalItem;
  }
  let { item }: Props = $props();

  let expanded = $state(false);
  let submitting = $state(false);
  let decision = $state<"approved" | "rejected" | null>(null);
  let rejectReason = $state("");
  let rejectMode = $state(false);
  let errorMessage = $state<string | null>(null);

  const tenantId = $derived(getTenant()?.id ?? "");

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
        item.id,
        approved,
        approved ? "" : rejectReason.trim(),
      );
      decision = approved ? "approved" : "rejected";
      expanded = false;
      rejectMode = false;
    } catch (e) {
      errorMessage = toUserMessage(e);
    } finally {
      submitting = false;
    }
  }

  const hasFooter = $derived(expanded || rejectMode || errorMessage !== null);
</script>

{#snippet approvalActions()}
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
    <a
      href={resolve(
        item.configurationId
          ? `/plans/configurations/${item.configurationId}#m-approval-${item.id}`
          : `/plans/executions/${item.planExecutionId}/approvals/${item.id}`,
      )}
      class="rounded border border-plumage bg-transparent px-3 py-1.5 text-[11px] font-medium text-crown-ash hover:border-talon-gold hover:text-talon-gold"
    >
      {translate("inbox.actions.openThread", $locale)}
    </a>
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

{#snippet approvalFooter()}
  {#if expanded && tenantId && item.inputArtifactId}
    <div class="mt-3">
      <ArtifactPreview {tenantId} artifactId={item.inputArtifactId} />
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

<InboxRow
  {item}
  actions={approvalActions}
  footer={hasFooter ? approvalFooter : undefined}
/>
