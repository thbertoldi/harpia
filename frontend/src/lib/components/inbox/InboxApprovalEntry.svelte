<script lang="ts">
  import type { Snippet } from "svelte";
  import { resolve } from "$app/paths";
  import { locale, translate } from "$lib/i18n";
  import type { InboxApprovalItem } from "$lib/inbox/types";
  import InboxRow from "./InboxRow.svelte";
  import CanvasApprovalForm from "$lib/components/canvas/CanvasApprovalForm.svelte";

  interface Props {
    item: InboxApprovalItem;
  }
  let { item }: Props = $props();
</script>

{#snippet inboxShell({
  formActions,
  formFooter,
  hasFooter,
}: {
  formActions: Snippet;
  formFooter: Snippet;
  hasFooter: boolean;
})}
  {#snippet rowActions()}
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
    {@render formActions()}
  {/snippet}

  {#snippet rowFooter()}
    {@render formFooter()}
  {/snippet}

  <InboxRow
    {item}
    actions={rowActions}
    footer={hasFooter ? rowFooter : undefined}
  />
{/snippet}

<CanvasApprovalForm
  approvalRequestId={item.id}
  inputArtifactId={item.inputArtifactId}
  {inboxShell}
/>
