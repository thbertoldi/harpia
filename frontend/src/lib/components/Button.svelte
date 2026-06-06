<script lang="ts">
  let {
    variant = "default",
    size = "md",
    onclick,
    children,
    ...rest
  }: {
    variant?: "default" | "outline" | "ghost";
    size?: "sm" | "md" | "lg";
    onclick?: () => void;
    children: import("svelte").Snippet;
    [key: string]: unknown;
  } = $props();
</script>

<button
  class="inline-flex items-center justify-center rounded-md font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50 {variant ===
  'default'
    ? 'bg-primary text-primary-foreground hover:bg-primary/90'
    : ''} {variant === 'outline'
    ? 'border border-input bg-background hover:bg-accent hover:text-accent-foreground'
    : ''} {variant === 'ghost'
    ? 'hover:bg-accent hover:text-accent-foreground'
    : ''} {size === 'sm' ? 'h-8 px-3 text-xs' : ''} {size === 'md'
    ? 'h-10 px-4 text-sm'
    : ''} {size === 'lg' ? 'h-11 px-6 text-base' : ''}"
  {onclick}
  {...rest}
>
  {@render children()}
</button>
