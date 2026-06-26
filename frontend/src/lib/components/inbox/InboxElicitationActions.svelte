<script lang="ts">
  import { resolve } from "$app/paths";
  import { locale, translate } from "$lib/i18n";
  import type { InboxElicitationItem } from "$lib/inbox/types";
  import { chipFlash } from "$lib/motion/transitions";

  interface Props {
    item: InboxElicitationItem;
  }
  let { item }: Props = $props();
</script>

<a
  href={resolve(
    item.configurationId
      ? `/plans/configurations/${item.configurationId}#m-elicitation-${item.id}`
      : `/plans/executions/${item.planExecutionId}/elicitations/${item.id}`,
  )}
  in:chipFlash
  class="rounded border border-primary bg-primary px-3 py-1.5 text-[11px] font-semibold text-primary-foreground hover:opacity-90"
>
  {translate("inbox.actions.openThread", $locale)}
</a>
