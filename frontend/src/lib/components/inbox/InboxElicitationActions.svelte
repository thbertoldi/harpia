<script lang="ts">
  import { resolve } from "$app/paths";
  import { locale, translate } from "$lib/i18n";
  import type { InboxElicitationItem } from "$lib/inbox/types";

  interface Props {
    item: InboxElicitationItem;
  }
  let { item }: Props = $props();
</script>

<a
  href={resolve(
    `/plans/executions/${item.planExecutionId}/elicitations/${item.id}`,
  )}
  class="rounded border border-talon-gold bg-talon-gold px-3 py-1.5 text-[11px] font-semibold text-obsidian hover:opacity-90"
>
  {translate("inbox.actions.openThread", $locale)}
</a>
