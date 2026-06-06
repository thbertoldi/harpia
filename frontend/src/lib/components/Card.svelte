<script lang="ts">
  let {
    children,
    ...rest
  }: {
    children: import("svelte").Snippet;
    [key: string]: unknown;
  } = $props();
</script>

<div
  class="rounded-lg border border-plumage bg-obsidian-light p-6 text-cream shadow-sm"
  {...rest}
>
  {@render children()}
</div>
