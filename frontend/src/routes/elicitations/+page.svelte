<script lang="ts">
  import { Clock, Loader2, MessagesSquare } from "lucide-svelte";
  import { resolve } from "$app/paths";
  import { requireTenantId } from "$lib/auth";
  import { toUserMessage } from "$lib/connect-errors";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import { locale, translate } from "$lib/i18n";
  import {
    formatCountdown,
    loadInboxElicitations,
    watchElicitations,
    type ElicitationRequest,
  } from "$lib/plans/elicitations";

  let elicitations = $state<ElicitationRequest[]>([]);
  let loading = $state(true);
  let loadError = $state<string | null>(null);

  $effect(() => {
    let tenantId: string;
    try {
      tenantId = requireTenantId();
    } catch (error) {
      loadError = toUserMessage(error);
      loading = false;
      return;
    }

    let active = true;
    const controller = new AbortController();

    void (async () => {
      try {
        elicitations = await loadInboxElicitations(tenantId);
      } catch (error) {
        if (active) {
          loadError = toUserMessage(error);
        }
      } finally {
        if (active) {
          loading = false;
        }
      }
    })();

    void (async () => {
      try {
        for await (const batch of watchElicitations(tenantId, {
          addressedToMe: true,
        })) {
          if (!active) {
            return;
          }
          elicitations = batch;
          loading = false;
        }
      } catch {
        // Stream is a live-update enhancement; the initial load already
        // populated the inbox, so failures here are non-fatal.
      }
    })();

    return () => {
      active = false;
      controller.abort();
    };
  });
</script>

<div class="mx-auto flex w-full max-w-4xl flex-col gap-4 px-4 py-6 lg:px-6">
  <div class="flex items-center justify-between gap-4">
    <div>
      <HarpyHeading tag="h1" class="text-2xl text-cream">
        {translate("elicitations.inbox.heading", $locale)}
      </HarpyHeading>
      <p class="mt-1 font-body text-sm text-crown-ash">
        {translate("elicitations.inbox.subtitle", $locale)}
      </p>
    </div>
    {#if !loading && elicitations.length > 0}
      <span
        class="rounded-full border border-talon-gold/40 bg-talon-gold/10 px-2.5 py-1 font-mono text-[10px] tracking-wider text-talon-gold uppercase"
      >
        {translate("elicitations.inbox.pendingCount", $locale, {
          count: elicitations.length,
        })}
      </span>
    {/if}
  </div>

  {#if loading}
    <div class="flex min-h-[30vh] items-center justify-center">
      <div class="flex items-center gap-2 text-crown-ash">
        <Loader2 class="size-5 animate-spin" />
        <span class="font-body text-sm">
          {translate("elicitations.inbox.loading", $locale)}
        </span>
      </div>
    </div>
  {:else if loadError && elicitations.length === 0}
    <div class="rounded-lg border border-red-500/30 bg-red-500/10 p-5">
      <p class="font-body text-sm text-red-300">
        {translate("elicitations.inbox.error", $locale, { error: loadError })}
      </p>
    </div>
  {:else if elicitations.length === 0}
    <div
      class="flex min-h-[30vh] flex-col items-center justify-center rounded-lg border border-dashed border-plumage bg-obsidian-light/30 px-6 py-12 text-center"
    >
      <MessagesSquare class="mb-4 size-12 text-talon-gold" />
      <p class="max-w-md font-body text-sm text-crown-ash">
        {translate("elicitations.inbox.empty", $locale)}
      </p>
    </div>
  {:else}
    <div class="space-y-3">
      {#each elicitations as item (item.id)}
        <a
          href={resolve(
            `/plans/executions/${item.planExecutionId}/elicitations/${item.id}`,
          )}
          class="flex flex-col gap-2 rounded-lg border border-plumage bg-obsidian-light/20 px-4 py-3 transition-colors hover:border-talon-gold/60"
        >
          <p class="font-heading text-base text-cream">
            {item.prompt}
          </p>
          <div
            class="flex flex-wrap items-center gap-3 font-mono text-[11px] text-crown-ash"
          >
            <span>
              {translate("elicitations.inbox.step", $locale)}: {item.planStepKey ||
                item.stepExecutionId}
            </span>
            <span class="inline-flex items-center gap-1">
              <Clock class="size-3" />
              {#if item.expiresAt}
                {translate("elicitations.inbox.expiresIn", $locale, {
                  countdown: formatCountdown(item.expiresAt),
                })}
              {:else}
                {translate("elicitations.inbox.noDeadline", $locale)}
              {/if}
            </span>
          </div>
        </a>
      {/each}
    </div>
  {/if}
</div>
