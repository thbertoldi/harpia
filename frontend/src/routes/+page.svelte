<script lang="ts">
  import { ArrowRight, Loader2 } from "lucide-svelte";
  import { goto } from "$app/navigation";
  import { resolve } from "$app/paths";
  import TaskInput from "$lib/components/TaskInput.svelte";
  import BrandLockup from "$lib/components/BrandLockup.svelte";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import { requireTenantId } from "$lib/auth";
  import { toUserMessage } from "$lib/connect-errors";
  import { threadClient } from "$lib/rpc";
  import { locale, translate } from "$lib/i18n";
  import { activeTheme } from "$lib/themes";
  import { brandTranslateParams } from "$lib/themes/branding";
  import { resolveLocalizedContent } from "$lib/i18n/content";

  let loading = $state(false);
  let error = $state<string | null>(null);

  const exampleTasks = $derived([
    resolveLocalizedContent("home.example.1", $locale),
    resolveLocalizedContent("home.example.2", $locale),
    resolveLocalizedContent("home.example.3", $locale),
    resolveLocalizedContent("home.example.4", $locale),
  ]);

  // The home screen is the chat-first entry: a prompt creates a thread and
  // routes to /chat/[threadId], where the plan proposal appears. Task listing
  // lives on /inbox.
  async function handleSubmit(text: string) {
    const trimmed = text.trim();
    if (loading || trimmed.length === 0) return;
    loading = true;
    error = null;
    try {
      const response = await threadClient.createThread({
        tenantId: requireTenantId(),
        title: trimmed.slice(0, 60),
        initialMessageText: trimmed,
      });
      const threadId = response.thread?.id;
      if (!threadId) throw new Error("createThread returned no id");
      await goto(resolve(`/chat/${threadId}`));
    } catch (e) {
      error = toUserMessage(e);
    } finally {
      loading = false;
    }
  }
</script>

<div class="flex flex-1">
  <div class="flex min-h-0 flex-1 flex-col">
    <div class="flex flex-1 flex-col items-center justify-center px-4">
      <div class="mb-4 flex justify-center">
        <BrandLockup variant="symbol" size="lg" />
      </div>

      <HarpyHeading
        tag="h1"
        class="mb-3 text-center text-5xl font-bold text-text"
      >
        {translate("home.hero.title", $locale)}
      </HarpyHeading>

      <p class="mb-10 text-center font-body text-lg text-crown-ash">
        {translate(
          "home.hero.subtitle",
          $locale,
          brandTranslateParams($activeTheme, $locale),
        )}
      </p>

      <div class="mb-12 w-full">
        <TaskInput onsubmit={handleSubmit} disabled={loading} />
      </div>

      {#if loading}
        <div class="flex items-center gap-2 text-crown-ash">
          <Loader2 class="size-4 animate-spin" />
          <span class="font-body text-sm"
            >{translate("home.creating", $locale)}</span
          >
        </div>
      {/if}

      {#if error}
        <div
          class="mt-4 w-full max-w-2xl rounded-lg border border-red-500/20 bg-red-500/10 p-3"
        >
          <p class="font-mono text-xs text-red-400">{error}</p>
        </div>
      {/if}

      <div class="mt-8 w-full max-w-2xl">
        <p
          class="mb-3 font-mono text-xs tracking-widest text-crown-ash uppercase"
        >
          {translate("home.tryAsking", $locale)}
        </p>
        <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
          {#each exampleTasks as example (example)}
            <button
              onclick={() => handleSubmit(example)}
              disabled={loading}
              class="flex items-center gap-2 rounded-lg border border-plumage bg-obsidian-light/60 px-4 py-3 text-left font-body text-sm text-cream transition-all hover:border-talon-gold hover:bg-obsidian-light disabled:opacity-50"
            >
              <ArrowRight class="size-3.5 shrink-0 text-talon-gold" />
              <span>{example}</span>
            </button>
          {/each}
        </div>
      </div>
    </div>
  </div>
</div>
