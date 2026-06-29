<script lang="ts">
  import { Send } from "lucide-svelte";
  import { locale, translate } from "$lib/i18n";
  import { appendThreadMessage } from "$lib/chat/client";

  interface Props {
    tenantId: string;
    configurationId: string;
    onSent?: () => void;
  }
  let { tenantId, configurationId, onSent }: Props = $props();

  let value = $state("");
  let sending = $state(false);
  let error = $state<string | null>(null);

  async function send() {
    const trimmed = value.trim();
    if (trimmed.length === 0 || sending) return;
    sending = true;
    error = null;
    try {
      await appendThreadMessage(
        tenantId,
        configurationId,
        "OVERSEER",
        "USER_TEXT",
        trimmed,
      );
      value = "";
      onSent?.();
    } catch (e) {
      error =
        e instanceof Error
          ? e.message
          : translate("thread.composer.error", $locale);
    } finally {
      sending = false;
    }
  }
</script>

<div
  class="flex flex-col gap-2 border-t border-plumage bg-obsidian-light px-3 py-3"
>
  <div class="flex items-end gap-2">
    <textarea
      bind:value
      placeholder={translate("thread.composer.placeholder", $locale)}
      class="flex-1 rounded border border-plumage bg-obsidian px-3 py-2 text-[13px] text-cream focus:border-talon-gold focus:outline-none"
      rows="2"
      disabled={sending}
    ></textarea>
    <button
      type="button"
      onclick={send}
      disabled={sending || value.trim().length === 0}
      class="rounded border border-talon-gold bg-talon-gold px-3 py-2 text-[12px] font-semibold text-on-primary hover:opacity-90 disabled:opacity-50"
    >
      <Send class="size-4" />
    </button>
  </div>
  {#if error}
    <p class="text-[11px] text-red-400">{error}</p>
  {/if}
</div>
