<script lang="ts">
  import { Send } from 'lucide-svelte';

  let { onsubmit, disabled = false, placeholder = "What do you want to get done?" }: {
    onsubmit: (text: string) => void | Promise<void>;
    disabled?: boolean;
    placeholder?: string;
  } = $props();

  let value = $state('');
  let loading = $state(false);

  async function handleSubmit(e: Event) {
    e.preventDefault();
    const trimmed = value.trim();
    if (!trimmed || disabled || loading) return;
    loading = true;
    try {
      await onsubmit(trimmed);
      value = '';
    } finally {
      loading = false;
    }
  }
</script>

<form onsubmit={handleSubmit} class="w-full max-w-2xl mx-auto">
  <div class="flex items-center gap-2 rounded-lg border border-plumage bg-obsidian-light p-1 transition-all focus-within:border-talon-gold focus-within:ring-2 focus-within:ring-talon-gold/20">
    <input
      type="text"
      bind:value
      {disabled}
      {placeholder}
      class="flex-1 bg-transparent px-4 py-3 text-cream placeholder:text-crown-ash outline-none font-body text-lg"
    />
    <button
      type="submit"
      disabled={disabled || loading || !value.trim()}
      class="flex h-10 w-10 items-center justify-center rounded-md bg-talon-gold text-obsidian transition-all hover:bg-talon-gold-bright disabled:opacity-40 disabled:cursor-not-allowed"
    >
      <Send class="size-4" />
    </button>
  </div>
</form>
