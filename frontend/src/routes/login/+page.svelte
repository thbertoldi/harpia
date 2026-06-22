<script lang="ts">
  import { goto } from "$app/navigation";
  import { resolve } from "$app/paths";
  import {
    login,
    getSession,
    devLogin,
    isDevLoginEnabled,
    isZitadelConfigured,
  } from "$lib/auth";
  import BrandLockup from "$lib/components/BrandLockup.svelte";
  import { applyColorScheme, initTheme, resolveColorScheme } from "$lib/themes";
  import { initLocale, locale, translate } from "$lib/i18n";

  import { Sun, Moon, Crown, Eye, Wrench } from "lucide-svelte";

  const zitadelReady = isZitadelConfigured();
  const devLoginEnabled = isDevLoginEnabled();

  let email = $state("");
  let dark = $state(true);

  $effect(() => {
    initTheme();
    initLocale();
    dark = resolveColorScheme() === "dark";
  });

  $effect(() => {
    if (!devLoginEnabled && getSession()) {
      goto(resolve("/"));
    }
  });

  function toggleDark() {
    dark = !dark;
    applyColorScheme(dark ? "dark" : "light");
  }

  async function handleLogin() {
    await login(email ? { login_hint: email } : undefined);
  }

  function handleDevLogin(role: "Leader" | "Overseer" | "Engineer") {
    if (!devLogin(role)) return;
    goto(resolve("/"));
  }

  const personas = $derived([
    {
      role: "Leader" as const,
      labelKey: "login.persona.leader.label",
      subtitleKey: "login.persona.leader.subtitle",
      icon: Crown,
    },
    {
      role: "Overseer" as const,
      labelKey: "login.persona.overseer.label",
      subtitleKey: "login.persona.overseer.subtitle",
      icon: Eye,
    },
    {
      role: "Engineer" as const,
      labelKey: "login.persona.engineer.label",
      subtitleKey: "login.persona.engineer.subtitle",
      icon: Wrench,
    },
  ]);
</script>

<div class="flex min-h-screen items-center justify-center bg-surface">
  <div class="brand-stripe fixed top-0 left-0 h-full w-[5px]"></div>

  <button
    onclick={toggleDark}
    class="fixed top-4 right-4 z-50 cursor-pointer rounded-md border border-border p-2 text-text-muted transition-colors hover:border-primary hover:text-text"
    aria-label={translate("nav.toggleDark", $locale)}
  >
    {#if dark}
      <Sun class="size-4" />
    {:else}
      <Moon class="size-4" />
    {/if}
  </button>

  <div class="w-full max-w-md space-y-8 p-8">
    <div class="text-center">
      <BrandLockup size="lg" class="justify-center" />
      <p class="mt-3 font-body text-text-muted">
        {translate("login.tagline", $locale)}
      </p>
    </div>

    <div class="space-y-4">
      <button
        onclick={handleLogin}
        disabled={!zitadelReady}
        title={zitadelReady
          ? ""
          : translate("login.signInNotConfiguredTitle", $locale)}
        class="w-full cursor-pointer rounded-lg bg-primary px-6 py-3 font-body font-medium text-primary-foreground transition-all hover:bg-primary-bright disabled:cursor-not-allowed disabled:bg-primary/30 disabled:text-primary-foreground/60"
      >
        {zitadelReady
          ? translate("login.signInZitadel", $locale)
          : translate("login.signInZitadelNotConfigured", $locale)}
      </button>

      <div class="relative">
        <div class="absolute inset-0 flex items-center">
          <div class="w-full border-t border-border"></div>
        </div>
        <div class="relative flex justify-center text-xs">
          <span
            class="bg-surface px-2 font-mono tracking-wider text-text-muted uppercase"
          >
            {translate("login.or", $locale)}
          </span>
        </div>
      </div>

      <form
        onsubmit={(e) => {
          e.preventDefault();
          if (zitadelReady) handleLogin();
        }}
        class="space-y-3"
      >
        <input
          type="email"
          bind:value={email}
          placeholder="admin@harpia.local"
          disabled={!zitadelReady}
          class="w-full rounded-lg border border-border bg-surface-elevated px-4 py-3 font-body text-text transition-colors placeholder:text-text-muted-dark focus:border-primary focus:ring-1 focus:ring-primary focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
        />
        <button
          type="submit"
          disabled={!zitadelReady}
          class="w-full cursor-pointer rounded-lg border border-border px-6 py-3 font-body text-sm text-text-muted transition-colors hover:border-primary hover:text-primary disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:border-border disabled:hover:text-text-muted"
        >
          {translate("login.continueEmail", $locale)}
        </button>
      </form>

      {#if devLoginEnabled}
        <div class="relative pt-2">
          <div class="absolute inset-0 flex items-center pt-2">
            <div class="w-full border-t border-border"></div>
          </div>
          <div class="relative flex justify-center text-xs">
            <span
              class="bg-surface px-2 font-mono tracking-wider text-text-muted-dark uppercase"
              style="font-family: 'JetBrains Mono', monospace"
            >
              {translate("login.devLogin", $locale)}
            </span>
          </div>
        </div>

        <div class="grid grid-cols-1 gap-2">
          {#each personas as persona (persona.role)}
            <button
              onclick={() => handleDevLogin(persona.role)}
              class="group flex cursor-pointer items-center gap-3 rounded-lg border border-border bg-surface-elevated/40 px-4 py-3 text-left transition-colors hover:border-primary hover:bg-surface-elevated"
            >
              <persona.icon
                class="size-5 shrink-0 text-text-muted transition-colors group-hover:text-primary"
              />
              <div class="min-w-0 flex-1">
                <div class="font-body text-sm font-medium text-text">
                  {translate(persona.labelKey, $locale)}
                </div>
                <div
                  class="font-mono text-[10px] tracking-wider text-text-muted-dark uppercase"
                >
                  {translate(persona.subtitleKey, $locale)}
                </div>
              </div>
            </button>
          {/each}
        </div>
      {/if}
    </div>
  </div>
</div>
