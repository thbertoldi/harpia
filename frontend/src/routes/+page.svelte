<script lang="ts">
  import { Loader2 } from "lucide-svelte";
  import { goto } from "$app/navigation";
  import { resolve } from "$app/paths";
  import TaskInput from "$lib/components/TaskInput.svelte";
  import BrandLockup from "$lib/components/BrandLockup.svelte";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import SuggestionChips from "$lib/components/SuggestionChips.svelte";
  import RecentConversations from "$lib/components/RecentConversations.svelte";
  import { requireTenantId } from "$lib/auth";
  import { toUserMessage } from "$lib/connect-errors";
  import { threadClient } from "$lib/rpc";
  import { locale, translate } from "$lib/i18n";
  import { activeTheme } from "$lib/themes";
  import { brandTranslateParams } from "$lib/themes/branding";

  let loading = $state(false);
  let error = $state<string | null>(null);

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
        <SuggestionChips onSelect={handleSubmit} disabled={loading} />
      </div>

      <div class="mt-10">
        <RecentConversations tenantId={requireTenantId()} />
      </div>
    </div>
  </div>
</div>
