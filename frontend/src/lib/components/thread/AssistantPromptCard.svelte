<script lang="ts">
  import { Pencil } from "lucide-svelte";
  import type { ChatMessage } from "$lib/chat/types";
  import { selectChip } from "$lib/plans/assistant";
  import { locale, translate } from "$lib/i18n";

  interface Option {
    id: string;
    label: string;
    sublabel?: string;
    value: string;
    price_brl?: number;
  }
  interface Props {
    message: ChatMessage;
    configurationId: string;
    tenantId: string;
    isAnswered: boolean;
    isLive: boolean;
  }
  let { message, configurationId, tenantId, isAnswered, isLive }: Props =
    $props();

  let editing = $state(false);
  let pending = $state(false);
  // `submitted` flips true on the first successful chip click and stays
  // true. Chips are hidden as soon as it flips, so a rapid second click
  // before the next ASSISTANT_PROMPT arrives via the 2 s watch poll
  // cannot fire a duplicate USER_SELECTION. Reset on edit-pencil reopen.
  let submitted = $state(false);

  const payload = $derived.by(() => {
    try {
      return JSON.parse(message.payloadJson) as {
        state?: string;
        step_key?: string;
        options?: Option[];
      };
    } catch {
      return { options: [] as Option[] };
    }
  });

  // Localized heading: the AWAITING_TEMPLATE state has a canonical key; any
  // other unexpected prompt state falls back to the backend's English text.
  const headingText = $derived(
    payload.state === "AWAITING_TEMPLATE"
      ? translate("assistant.prompt.awaitingTemplate", $locale)
      : message.text,
  );

  const showChips = $derived((isLive || editing) && !submitted);

  async function onSelect(option: Option) {
    if (pending || submitted) return;
    pending = true;
    try {
      await selectChip({
        tenantId,
        configurationId,
        promptMessageId: message.id,
        optionId: option.id,
        value: option.value,
      });
      submitted = true;
    } finally {
      pending = false;
      editing = false;
    }
  }

  function toggleEditing() {
    editing = !editing;
    if (editing) submitted = false;
  }
</script>

<div
  id={`m-${message.id}`}
  class="rounded-lg border border-plumage bg-obsidian-light px-4 py-3 {isLive
    ? 'ring-1 ring-talon-gold'
    : ''}"
>
  <div class="flex items-start justify-between gap-2">
    <p class="font-body text-[13px] text-cream">{headingText}</p>
    {#if isAnswered && !isLive}
      <button
        type="button"
        class="cursor-pointer rounded p-1 text-crown-ash hover:text-talon-gold"
        onclick={toggleEditing}
        aria-label={translate("assistant.edit", $locale)}
      >
        <Pencil class="size-3.5" />
      </button>
    {/if}
  </div>

  {#if showChips && payload.options && payload.options.length > 0}
    <div class="mt-3 flex flex-wrap gap-2">
      {#each payload.options as opt (opt.id)}
        <button
          type="button"
          disabled={pending || submitted}
          onclick={() => onSelect(opt)}
          class="cursor-pointer rounded-md border border-plumage bg-obsidian px-3 py-1.5 text-left text-[12px] text-cream hover:border-talon-gold hover:text-talon-gold disabled:cursor-not-allowed disabled:opacity-50"
        >
          <span class="font-medium">{opt.label}</span>
          {#if opt.sublabel}
            <span class="ml-2 text-[10px] text-crown-ash-dark"
              >{opt.sublabel}</span
            >
          {/if}
        </button>
      {/each}
    </div>
  {/if}
</div>
