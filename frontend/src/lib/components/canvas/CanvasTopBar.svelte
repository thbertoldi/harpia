<script lang="ts">
  import {
    ArrowLeft,
    History,
    Settings as SettingsIcon,
    Calendar,
    Reply,
  } from "lucide-svelte";
  import { resolve } from "$app/paths";
  import { locale, translate } from "$lib/i18n";

  interface Props {
    configurationId: string;
    runStartedAt?: string;
    pendingAnswerCount: number;
    onOpenRunHistory: () => void;
    onOpenSettings: () => void;
    onAnswerNext: () => void;
    planName?: string;
  }
  let {
    configurationId,
    runStartedAt,
    pendingAnswerCount,
    onOpenRunHistory,
    onOpenSettings,
    onAnswerNext,
    planName,
  }: Props = $props();
</script>

<header
  class="flex items-center gap-3 border-b border-plumage bg-obsidian px-4 py-2"
>
  <a
    href={resolve(`/plans/configurations/${configurationId}`)}
    class="flex items-center gap-1 rounded border border-plumage bg-transparent px-2 py-1 text-[11px] text-crown-ash hover:border-talon-gold hover:text-talon-gold"
  >
    <ArrowLeft class="size-3.5" />
    {translate("canvas.topbar.backToThread", $locale)}
  </a>

  <div class="flex-1">
    {#if planName}
      <span class="font-heading text-[13px] font-semibold text-cream"
        >{planName}</span
      >
    {/if}
    {#if runStartedAt}
      <span class="ml-2 font-mono text-[11px] text-crown-ash-dark"
        >{runStartedAt}</span
      >
    {/if}
  </div>

  <button
    type="button"
    onclick={onOpenRunHistory}
    class="flex items-center gap-1 rounded border border-plumage bg-transparent px-2 py-1 text-[11px] text-crown-ash hover:border-talon-gold hover:text-talon-gold"
  >
    <History class="size-3.5" />
    {translate("canvas.topbar.runHistory", $locale)}
  </button>

  <button
    type="button"
    disabled
    title={translate("canvas.topbar.scheduleComingSoon", $locale)}
    class="flex cursor-not-allowed items-center gap-1 rounded border border-plumage/40 bg-transparent px-2 py-1 text-[11px] text-crown-ash-dark"
  >
    <Calendar class="size-3.5" />
    {translate("canvas.topbar.schedule", $locale)}
  </button>

  <button
    type="button"
    onclick={onOpenSettings}
    class="flex items-center gap-1 rounded border border-plumage bg-transparent px-2 py-1 text-[11px] text-crown-ash hover:border-talon-gold hover:text-talon-gold"
  >
    <SettingsIcon class="size-3.5" />
    {translate("canvas.topbar.settings", $locale)}
  </button>

  {#if pendingAnswerCount > 0}
    <button
      type="button"
      onclick={onAnswerNext}
      class="flex items-center gap-1 rounded border border-talon-gold bg-talon-gold px-3 py-1 text-[11px] font-semibold text-obsidian hover:opacity-90"
    >
      <Reply class="size-3.5" />
      {translate("canvas.topbar.answerPending", $locale, {
        count: pendingAnswerCount,
      })}
    </button>
  {/if}
</header>
