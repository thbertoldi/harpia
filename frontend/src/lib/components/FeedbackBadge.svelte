<script lang="ts">
  let { count = 0 }: { count?: number } = $props();
</script>

{#if count > 0}
  <span
    class="ml-auto inline-flex animate-pulse items-center justify-center rounded-full bg-talon-gold px-1.5 py-0.5 font-mono text-[10px] leading-none font-bold text-obsidian"
  >
    {count}
  </span>
{/if}
