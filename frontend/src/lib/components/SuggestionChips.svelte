<script lang="ts">
  import {
    ArrowRight,
    BarChart3,
    Edit3,
    Megaphone,
    Newspaper,
    Share2,
    Sparkles,
  } from "lucide-svelte";
  import { locale, translate } from "$lib/i18n";
  import { resolveLocalizedContent } from "$lib/i18n/content";
  import {
    loadCatalogSuggestions,
    type CatalogSuggestion,
  } from "$lib/plans/suggestions";

  type IconComponent = typeof ArrowRight;

  let {
    onSelect,
    disabled = false,
    headingKey = "home.tryAsking",
  }: {
    onSelect: (prompt: string) => void;
    disabled?: boolean;
    /** Flat i18n key for the section heading. */
    headingKey?: string;
  } = $props();

  let suggestions = $state<CatalogSuggestion[]>([]);

  // Load catalog-driven suggestions client-side; fall back to the generic
  // example prompts while loading or when the catalog is empty.
  $effect(() => {
    const loc = $locale;
    loadCatalogSuggestions(loc)
      .then((s) => (suggestions = s))
      .catch(() => (suggestions = []));
  });

  const fallback = $derived(
    [1, 2, 3, 4].map((n) =>
      resolveLocalizedContent(`home.example.${n}`, $locale),
    ),
  );

  type Chip = { icon: IconComponent; label: string; prompt: string };

  function iconFor(iconKey: string): IconComponent {
    switch (iconKey) {
      case "share":
        return Share2;
      case "newspaper":
        return Newspaper;
      case "edit":
        return Edit3;
      case "chart":
        return BarChart3;
      case "megaphone":
        return Megaphone;
      case "sparkles":
        return Sparkles;
      default:
        return ArrowRight;
    }
  }

  const chips = $derived<Chip[]>(
    suggestions.length > 0
      ? suggestions.map((s) => ({
          icon: iconFor(s.iconKey),
          label: s.label,
          prompt: s.prompt,
        }))
      : fallback.map((prompt) => ({ icon: ArrowRight, label: prompt, prompt })),
  );
</script>

<div class="w-full max-w-2xl">
  <p class="mb-3 font-mono text-xs tracking-widest text-crown-ash uppercase">
    {translate(headingKey, $locale)}
  </p>
  <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
    {#each chips as chip, i (chip.prompt + i)}
      {@const Icon = chip.icon}
      <button
        type="button"
        onclick={() => onSelect(chip.prompt)}
        {disabled}
        class="flex items-center gap-2 rounded-lg border border-plumage bg-obsidian-light/60 px-4 py-3 text-left font-body text-sm text-cream transition-all hover:border-talon-gold hover:bg-obsidian-light disabled:opacity-50"
      >
        <Icon class="size-3.5 shrink-0 text-energy" />
        <span class="truncate">{chip.label}</span>
      </button>
    {/each}
  </div>
</div>
