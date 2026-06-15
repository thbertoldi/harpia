<script lang="ts">
  import { locale } from "$lib/i18n";
  import { activeTheme, type ThemeName } from "$lib/themes";
  import { brandName } from "$lib/themes/branding";

  interface Props {
    variant?: "full" | "symbol";
    size?: "sm" | "md" | "lg";
    theme?: ThemeName;
    class?: string;
  }

  let {
    variant = "full",
    size = "md",
    theme,
    class: className = "",
  }: Props = $props();

  const resolvedTheme = $derived(theme ?? $activeTheme);

  const symbolSizeClass = $derived(
    size === "lg" ? "size-12" : size === "sm" ? "size-6" : "size-8",
  );

  const wordmarkClass = $derived(
    size === "lg"
      ? "text-5xl font-black tracking-tight"
      : size === "sm"
        ? "text-lg font-semibold tracking-wide"
        : "text-xl font-semibold tracking-wide",
  );

  const displayName = $derived(brandName(resolvedTheme, $locale));
</script>

<div class="inline-flex items-center gap-2.5 {className}">
  {#if resolvedTheme === "aiuna"}
    <img
      src="/brand/aiuna-symbol.png"
      alt=""
      aria-hidden="true"
      class="{symbolSizeClass} shrink-0 object-contain"
    />
    {#if variant === "full"}
      <span class="font-wordmark text-primary {wordmarkClass}">
        {displayName}
      </span>
    {:else}
      <span class="sr-only">{displayName}</span>
    {/if}
  {:else if resolvedTheme === "default"}
    {#if variant === "symbol"}
      <span
        class="font-wordmark text-primary {symbolSizeClass} flex shrink-0 items-center justify-center text-lg leading-none"
        aria-hidden="true"
      >
        H
      </span>
      <span class="sr-only">{displayName}</span>
    {:else}
      <span class="font-wordmark text-text {wordmarkClass}">
        <span class="text-primary">Harp</span>ia
      </span>
    {/if}
  {:else if variant === "symbol"}
    <span
      class="flex {symbolSizeClass} shrink-0 items-center justify-center rounded-full border border-primary bg-surface-elevated font-mono text-[10px] text-primary uppercase"
      aria-hidden="true"
    >
      {displayName.slice(0, 1)}
    </span>
    <span class="sr-only">{displayName}</span>
  {:else}
    <span class="font-wordmark text-primary {wordmarkClass}">
      {displayName}
    </span>
  {/if}
</div>
