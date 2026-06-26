<script lang="ts">
  import { Construction } from "lucide-svelte";
  import { locale, translate } from "$lib/i18n";

  interface Props {
    title: string;
    milestone: string;
  }

  let { title, milestone }: Props = $props();
</script>

<div
  class="mx-auto flex max-w-3xl flex-col items-center gap-4 px-4 py-24 text-center"
>
  <Construction class="size-10 text-text-muted" />
  <h1 class="font-heading text-2xl text-text">{title}</h1>
  <p class="text-text-muted">
    {translate("nav.stubComingSoon", $locale)} ({milestone})
  </p>
</div>
