<script lang="ts">
  import type { Snippet } from "svelte";

  let { tag = "h1", children }: { tag?: string; children: Snippet } = $props();
</script>

<svelte:element this={tag} class="font-heading tracking-tight text-talon-gold">
  {@render children()}
</svelte:element>
