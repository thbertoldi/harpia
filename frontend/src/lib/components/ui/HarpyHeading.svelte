<script lang="ts">
  let { tag = 'h1', children, class: className = '' }: {
    tag?: 'h1' | 'h2' | 'h3' | 'h4' | 'h5' | 'h6';
    children: import('svelte').Snippet;
    class?: string;
  } = $props();
</script>

<svelte:element this={tag} class="font-heading tracking-tight text-talon-gold {className}" style="font-family: 'Bodoni Moda', serif">
  {@render children()}
</svelte:element>
