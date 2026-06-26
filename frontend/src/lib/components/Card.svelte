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
  class="rounded-lg border border-border bg-surface-elevated p-6 text-text shadow-sm"
  {...rest}
>
  {@render children()}
</div>
