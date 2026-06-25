<script lang="ts">
  import { resolve } from "$app/paths";
  import { locale, translate } from "$lib/i18n";
  import type { InboxFeedbackItem } from "$lib/inbox/types";

  interface Props {
    item: InboxFeedbackItem;
  }
  let { item }: Props = $props();

  // Feedback detail still lives under /oversee in M2; M3 will repoint.
  // Hash anchor omitted: /oversee 302-redirects to /inbox before the fragment is applied.
</script>

<a
  href={resolve("/inbox")}
  data-feedback-id={item.id}
  class="rounded border border-plumage bg-transparent px-3 py-1.5 text-[11px] font-medium text-crown-ash hover:border-talon-gold hover:text-talon-gold"
>
  {translate("inbox.actions.openThread", $locale)}
</a>
