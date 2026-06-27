<script lang="ts">
  let {
    role,
    content,
    timestamp,
  }: {
    role: "user" | "agent" | "system";
    content: string;
    timestamp?: string;
  } = $props();
</script>

<div class="flex gap-3 {role === 'user' ? 'justify-end' : 'justify-start'}">
  {#if role === "agent" || role === "system"}
    <div
      class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-talon-gold/20"
    >
      <span class="font-heading text-xs font-bold text-talon-gold">H</span>
    </div>
  {/if}

  <div
    class="max-w-[75%] rounded-lg px-4 py-3 {role === 'user'
      ? 'ml-12 bg-talon-gold text-on-primary'
      : role === 'system'
        ? 'bg-plumage text-sm text-crown-ash'
        : 'border border-plumage bg-obsidian-light text-cream'}"
  >
    <p class="font-body text-sm leading-relaxed whitespace-pre-wrap">
      {content}
    </p>
    {#if timestamp}
      <span class="mt-1 block font-mono text-xs opacity-60">{timestamp}</span>
    {/if}
  </div>

  {#if role === "user"}
    <div
      class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-crown-ash/20"
    >
      <span class="font-heading text-xs font-bold text-crown-ash">U</span>
    </div>
  {/if}
</div>
