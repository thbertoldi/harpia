<script lang="ts">
  import type { ChatMessage } from "$lib/chat/types";
  import { selectChip } from "$lib/plans/assistant";
  import { locale, translate } from "$lib/i18n";

  interface Props {
    message: ChatMessage;
    configurationId: string;
    tenantId: string;
    isLive: boolean;
  }
  let { message, configurationId, tenantId, isLive }: Props = $props();
  let saving = $state(false);
  // Once the save lands, hide the button so a rapid second click can't
  // re-fire selectChip before the next assistant turn arrives. Mirrors
  // the submitted-flag pattern in AssistantPromptCard.
  let submitted = $state(false);

  const saveLabel = $derived(translate("confirm.save", $locale));

  async function onSave() {
    if (saving || submitted) return;
    saving = true;
    try {
      await selectChip({
        tenantId,
        configurationId,
        promptMessageId: message.id,
        optionId: "save",
        value: "save",
        label: saveLabel,
      });
      submitted = true;
    } finally {
      saving = false;
    }
  }
</script>

<div
  id={`m-${message.id}`}
  class="rounded-lg border border-plumage bg-obsidian-light px-4 py-3 {isLive ? 'ring-1 ring-talon-gold' : ''}"
>
  <p class="text-[13px] font-body text-cream">{message.text}</p>
  {#if isLive && !submitted}
    <div class="mt-3 flex items-center gap-2">
      <button
        type="button"
        disabled={saving || submitted}
        onclick={onSave}
        class="cursor-pointer rounded-md border border-talon-gold bg-talon-gold px-3 py-1.5 text-[12px] font-semibold text-obsidian hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
      >
        {saveLabel}
      </button>
    </div>
  {/if}
</div>
