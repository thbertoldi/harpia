<script lang="ts">
  import { resolve } from "$app/paths";
  import { locale, translate } from "$lib/i18n";
  import { chipFlash } from "$lib/motion/transitions";
</script>

<a
  href={resolve("/runs")}
  in:chipFlash
  class="rounded border border-primary bg-primary px-3 py-1.5 text-[11px] font-semibold text-primary-foreground hover:opacity-90"
>
  {translate("inbox.actions.openThread", $locale)}
</a>
