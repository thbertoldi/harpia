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

  const zitadelReady = isZitadelConfigured();
  const devLoginEnabled = isDevLoginEnabled();
  import { Sun, Moon, Crown, Eye, Wrench } from "lucide-svelte";

  let email = $state("");
  let dark = $state(true);

  $effect(() => {
    if (getSession()) {
      goto(resolve("/"));
    }
  });

  function toggleDark() {
    dark = !dark;
    document.documentElement.classList.toggle("dark", dark);
    localStorage.setItem("harpia-theme", dark ? "dark" : "light");
  }

  async function handleLogin() {
    await login(email ? { login_hint: email } : undefined);
  }

  function handleDevLogin(role: "Leader" | "Overseer" | "Engineer") {
    if (!devLogin(role)) return;
    goto(resolve("/"));
  }

  const personas = [
    {
      role: "Leader" as const,
      label: "Leader",
      subtitle: "Direct tasks",
      icon: Crown,
    },
    {
      role: "Overseer" as const,
      label: "Overseer",
      subtitle: "Approve & feedback",
      icon: Eye,
    },
    {
      role: "Engineer" as const,
      label: "Platform Engineer",
      subtitle: "Configure agents",
      icon: Wrench,
    },
  ];
</script>

<div class="flex min-h-screen items-center justify-center bg-obsidian">
  <div
    class="fixed top-0 left-0 h-full w-[5px] bg-gradient-to-b from-talon-gold-bright via-talon-gold to-talon-gold/30"
  ></div>

  <button
    onclick={toggleDark}
    class="fixed top-4 right-4 z-50 cursor-pointer rounded-md border border-plumage p-2 text-crown-ash transition-colors hover:border-talon-gold hover:text-cream"
    aria-label="Toggle dark mode"
  >
    {#if dark}
      <Sun class="size-4" />
    {:else}
      <Moon class="size-4" />
    {/if}
  </button>

  <div class="w-full max-w-md space-y-8 p-8">
    <div class="text-center">
      <h1
        class="text-6xl font-black tracking-tight text-talon-gold"
        style="font-family: 'Bodoni Moda', serif"
      >
        Harpia
      </h1>
      <p
        class="mt-3 font-body text-cream/70"
        style="font-family: 'DM Sans', sans-serif"
      >
        AI operations for your business
      </p>
    </div>

    <div class="space-y-4">
      <button
        onclick={handleLogin}
        disabled={!zitadelReady}
        title={zitadelReady
          ? ""
          : "Sign-in is not configured. Contact your administrator."}
        class="w-full cursor-pointer rounded-lg bg-talon-gold px-6 py-3 font-medium text-obsidian transition-all hover:bg-talon-gold-bright disabled:cursor-not-allowed disabled:bg-talon-gold/30 disabled:text-obsidian/60"
        style="font-family: 'DM Sans', sans-serif"
      >
        Sign in with Zitadel{zitadelReady ? "" : " (not configured)"}
      </button>

      <div class="relative">
        <div class="absolute inset-0 flex items-center">
          <div class="w-full border-t border-plumage"></div>
        </div>
        <div class="relative flex justify-center text-xs">
          <span
            class="bg-obsidian px-2 font-mono tracking-wider text-crown-ash uppercase"
            style="font-family: 'JetBrains Mono', monospace"
          >
            or
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
          class="w-full rounded-lg border border-plumage bg-obsidian-light px-4 py-3 font-body text-cream transition-colors placeholder:text-crown-ash-dark focus:border-talon-gold focus:ring-1 focus:ring-talon-gold focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
          style="font-family: 'DM Sans', sans-serif"
        />
        <button
          type="submit"
          disabled={!zitadelReady}
          class="w-full cursor-pointer rounded-lg border border-plumage px-6 py-3 font-body text-sm text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:border-plumage disabled:hover:text-crown-ash"
          style="font-family: 'DM Sans', sans-serif"
        >
          Continue with email
        </button>
      </form>

      {#if devLoginEnabled}
        <div class="relative pt-2">
          <div class="absolute inset-0 flex items-center pt-2">
            <div class="w-full border-t border-plumage"></div>
          </div>
          <div class="relative flex justify-center text-xs">
            <span
              class="bg-obsidian px-2 font-mono tracking-wider text-crown-ash-dark uppercase"
              style="font-family: 'JetBrains Mono', monospace"
            >
              dev login
            </span>
          </div>
        </div>

        <div class="grid grid-cols-1 gap-2">
          {#each personas as persona (persona.role)}
            <button
              onclick={() => handleDevLogin(persona.role)}
              class="group flex cursor-pointer items-center gap-3 rounded-lg border border-plumage bg-obsidian-light/40 px-4 py-3 text-left transition-colors hover:border-talon-gold hover:bg-obsidian-light"
            >
              <persona.icon
                class="size-5 shrink-0 text-crown-ash transition-colors group-hover:text-talon-gold"
              />
              <div class="min-w-0 flex-1">
                <div
                  class="font-body text-sm font-medium text-cream"
                  style="font-family: 'DM Sans', sans-serif"
                >
                  {persona.label}
                </div>
                <div
                  class="font-mono text-[10px] tracking-wider text-crown-ash-dark uppercase"
                  style="font-family: 'JetBrains Mono', monospace"
                >
                  {persona.subtitle}
                </div>
              </div>
            </button>
          {/each}
        </div>
      {/if}
    </div>
  </div>
</div>
