<script lang="ts">
  let { tag = 'h1', children }: { tag?: string; children: any } = $props();
</script>

<svelte:element this={tag} class="font-heading tracking-tight text-talon-gold">
  {@render children()}
</svelte:element>
