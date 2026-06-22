<script lang="ts">
  import type { Snippet } from "svelte";
  import { X } from "lucide-svelte";
  import { locale, translate } from "$lib/i18n";

  interface Props {
    open: boolean;
    onClose: () => void;
    title: string;
    children: Snippet;
  }
  let { open, onClose, title, children }: Props = $props();
</script>

{#if open}
  <div
    role="presentation"
    onclick={onClose}
    class="fixed inset-0 z-40 bg-black/40"
  ></div>
  <aside
    class="fixed inset-y-0 right-0 z-50 flex w-96 flex-col gap-2 border-l border-plumage bg-obsidian transition-transform"
  >
    <header
      class="flex items-center justify-between border-b border-plumage px-4 py-3"
    >
      <h2 class="font-heading text-[13px] font-semibold text-cream">{title}</h2>
      <button
        type="button"
        onclick={onClose}
        aria-label={translate("canvas.drawer.close", $locale)}
        class="text-crown-ash hover:text-talon-gold"
      >
        <X class="size-4" />
      </button>
    </header>
    <div class="flex-1 overflow-y-auto px-4 py-3">
      {@render children()}
    </div>
  </aside>
{/if}
