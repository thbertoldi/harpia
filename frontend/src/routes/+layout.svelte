<script lang="ts">
  import "../app.css";
  import { Menu, X, Sun, Moon } from "lucide-svelte";
  import TenantSelector from "$lib/components/TenantSelector.svelte";
  import FeedbackBadge from "$lib/components/FeedbackBadge.svelte";
  import BrandLockup from "$lib/components/BrandLockup.svelte";
  import { logout, getTenant } from "$lib/auth";
  import { page } from "$app/state";
  import { resolve } from "$app/paths";
  import {
    applyColorScheme,
    initTheme,
    resolveColorScheme,
    activeTheme,
  } from "$lib/themes";
  import {
    brandFavicon,
    brandMetaDescription,
    brandName,
  } from "$lib/themes/branding";
  import {
    initLocale,
    locale,
    setLocale,
    translate,
    translateRole,
    type Locale,
  } from "$lib/i18n";
  import { isNavSectionActive } from "$lib/nav/active";
  import { resolveNavSections } from "$lib/nav/sections";

  let { data, children } = $props();

  let navOpen = $state(false);
  let dark = $state(false);

  $effect(() => {
    initTheme({ tenantThemeKey: getTenant()?.themeKey ?? null });
    initLocale();
    dark = resolveColorScheme() === "dark";
  });

  function toggleDark() {
    dark = !dark;
    applyColorScheme(dark ? "dark" : "light");
  }

  const sections = $derived(
    resolveNavSections(data?.user?.role, (key) => translate(key, $locale)),
  );

  function isActive(path: string) {
    return isNavSectionActive(path, page.url.pathname);
  }
</script>

<svelte:head>
  <title>{brandName($activeTheme, $locale)}</title>
  <meta
    name="description"
    content={brandMetaDescription($activeTheme, $locale)}
  />
  <link rel="icon" href={brandFavicon($activeTheme)} />
  <script>
    (() => {
      const themeKey = "aiuna-theme";
      const schemeKey = "aiuna-color-scheme";
      const legacyKey = "harpia-theme";
      const themes = ["default", "aiuna", "tenant-base"];

      const theme = localStorage.getItem(themeKey);
      if (theme && themes.includes(theme)) {
        document.documentElement.setAttribute("data-theme", theme);
      } else {
        document.documentElement.setAttribute("data-theme", "aiuna");
      }

      const storedScheme =
        localStorage.getItem(schemeKey) ?? localStorage.getItem(legacyKey);
      const prefersDark = window.matchMedia(
        "(prefers-color-scheme: dark)",
      ).matches;
      const isDark =
        storedScheme === "dark" || (storedScheme !== "light" && prefersDark);
      document.documentElement.classList.toggle("dark", isDark);
    })();
  </script>
</svelte:head>

<div class="flex min-h-screen bg-obsidian">
  <!-- Left brand stripe -->
  <div class="brand-stripe fixed inset-y-0 left-0 z-50 w-[5px]"></div>

  <!-- Hidden lateral nav -->
  <div class="fixed inset-y-0 left-[5px] z-40 flex">
    <div
      class="h-full overflow-hidden transition-all duration-300 {navOpen
        ? 'w-64'
        : 'w-0'}"
    >
      <nav class="h-full w-64 border-r border-plumage bg-obsidian px-4 py-6">
        <div class="mb-8 flex items-center justify-between">
          <BrandLockup size="sm" />
          <button
            onclick={() => (navOpen = false)}
            class="cursor-pointer rounded-md p-1 text-crown-ash transition-colors hover:text-cream"
          >
            <X class="size-5" />
          </button>
        </div>

        <div class="space-y-1">
          {#each sections as section (section.href)}
            <a
              href={resolve(section.href)}
              onclick={() => (navOpen = false)}
              class="flex cursor-pointer items-center gap-3 rounded-md px-3 py-2.5 transition-all {isActive(
                section.href,
              )
                ? 'bg-obsidian-light text-talon-gold'
                : 'text-crown-ash hover:bg-obsidian-light hover:text-cream'}"
            >
              <section.icon class="size-4" />
              <span
                class="text-sm font-medium"
                style="font-family: 'DM Sans', sans-serif">{section.label}</span
              >
              {#if section.href === "/oversee"}
                <FeedbackBadge />
              {/if}
            </a>
          {/each}
        </div>

        <div class="mt-8 border-t border-plumage pt-6">
          <p
            class="font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase"
            style="font-family: 'JetBrains Mono', monospace"
          >
            {translate("nav.role", $locale)}
          </p>
          <p
            class="mt-1 text-sm text-cream"
            style="font-family: 'DM Sans', sans-serif"
          >
            {translateRole(data?.user?.role ?? "Leader", $locale)}
          </p>
        </div>
      </nav>
    </div>
  </div>

  <!-- Main content -->
  <div
    class="flex flex-1 flex-col transition-all duration-300 {navOpen
      ? 'ml-[calc(5px+16rem)]'
      : 'ml-[5px]'}"
  >
    {#if data?.user}
      <header
        class="sticky top-0 z-30 border-b border-plumage/30 bg-obsidian/80 backdrop-blur"
      >
        <div class="flex h-14 items-center justify-between px-4 lg:px-6">
          <div class="flex items-center gap-3">
            <button
              onclick={() => (navOpen = !navOpen)}
              class="cursor-pointer rounded-md p-1.5 text-crown-ash transition-colors hover:text-cream"
              aria-label={translate("nav.toggleNav", $locale)}
            >
              <Menu class="size-5" />
            </button>
            <BrandLockup size="sm" />
          </div>

          <div class="flex items-center gap-4">
            <label class="sr-only" for="locale-switcher">
              {translate("nav.language", $locale)}
            </label>
            <select
              id="locale-switcher"
              value={$locale}
              onchange={(e) => setLocale(e.currentTarget.value as Locale)}
              class="cursor-pointer rounded-md border border-plumage bg-obsidian px-2 py-1 text-xs text-crown-ash transition-colors hover:border-talon-gold hover:text-cream"
              style="font-family: 'DM Sans', sans-serif"
            >
              <option value="en">EN</option>
              <option value="pt-BR">PT</option>
            </select>
            <button
              onclick={toggleDark}
              class="cursor-pointer rounded-md border border-plumage p-1.5 text-crown-ash transition-colors hover:border-talon-gold hover:text-cream"
              aria-label={translate("nav.toggleDark", $locale)}
            >
              {#if dark}
                <Sun class="size-4" />
              {:else}
                <Moon class="size-4" />
              {/if}
            </button>
            <TenantSelector />
            <span
              class="text-sm text-crown-ash"
              style="font-family: 'DM Sans', sans-serif">{data.user.name}</span
            >
            <button
              onclick={logout}
              class="cursor-pointer rounded-md border border-plumage px-3 py-1 text-xs text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
              style="font-family: 'DM Sans', sans-serif"
            >
              {translate("nav.logout", $locale)}
            </button>
          </div>
        </div>
      </header>
    {/if}

    <main class="flex-1">
      {@render children()}
    </main>
  </div>
</div>
