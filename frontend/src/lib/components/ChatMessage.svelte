<script lang="ts">
  let { role, content, timestamp }: {
    role: 'user' | 'agent' | 'system';
    content: string;
    timestamp?: string;
  } = $props();
</script>

<div class="flex gap-3 {role === 'user' ? 'justify-end' : 'justify-start'}">
  {#if role === 'agent' || role === 'system'}
    <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-talon-gold/20">
      <span class="font-heading text-xs text-talon-gold font-bold">H</span>
    </div>
  {/if}

  <div class="max-w-[75%] rounded-lg px-4 py-3 {role === 'user'
    ? 'bg-talon-gold text-obsidian ml-12'
    : role === 'system'
      ? 'bg-muted text-muted-foreground text-sm'
      : 'bg-card border text-card-foreground'}">
    <p class="whitespace-pre-wrap font-body text-sm leading-relaxed">{content}</p>
    {#if timestamp}
      <span class="mt-1 block text-xs opacity-60 font-mono">{timestamp}</span>
    {/if}
  </div>

  {#if role === 'user'}
    <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-crown-ash/20">
      <span class="font-heading text-xs text-crown-ash font-bold">U</span>
    </div>
  {/if}
</div>
