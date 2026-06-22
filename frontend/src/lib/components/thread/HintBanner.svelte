<script lang="ts">
  import { X } from "lucide-svelte";
  import { browser } from "$app/environment";
  import { locale, translate } from "$lib/i18n";

  interface Props {
    threadId: string;
  }
  let { threadId }: Props = $props();

  const storageKey = $derived(
    `harpia.thread.${threadId}.composerHintDismissed`,
  );

  let visible = $state(false);

  $effect(() => {
    if (!browser) return;
    visible = localStorage.getItem(storageKey) !== "1";
  });

  function dismiss() {
    visible = false;
    if (browser) {
      localStorage.setItem(storageKey, "1");
    }
  }
</script>

{#if visible}
  <div
    class="flex items-start gap-3 rounded-md border border-talon-gold/40 bg-talon-gold/5 px-3 py-2 text-[12px] text-crown-ash"
  >
    <p class="flex-1">
      {translate("thread.composerHint", $locale)}
    </p>
    <button
      type="button"
      onclick={dismiss}
      aria-label={translate("thread.dismiss", $locale)}
      class="text-crown-ash-dark hover:text-talon-gold"
    >
      <X class="size-4" />
    </button>
  </div>
{/if}
