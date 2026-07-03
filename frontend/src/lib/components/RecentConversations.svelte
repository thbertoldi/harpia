<script lang="ts">
  import { resolve } from "$app/paths";
  import { timestampDate } from "@bufbuild/protobuf/wkt";
  import { MessageSquare } from "lucide-svelte";
  import { locale, translate } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";
  import { listRecentThreads } from "$lib/chat/client";
  import type { Thread } from "$lib/gen/harpia/chat/v1/chat_pb";

  let { tenantId }: { tenantId: string } = $props();

  let threads = $state<Thread[]>([]);

  $effect(() => {
    if (!tenantId) return;
    let cancelled = false;
    listRecentThreads(tenantId)
      .then((result) => {
        if (!cancelled) threads = result;
      })
      .catch(() => {
        if (!cancelled) threads = [];
      });
    return () => {
      cancelled = true;
    };
  });
</script>

{#if threads.length > 0}
  <div class="w-full max-w-2xl">
    <p class="mb-3 font-mono text-xs tracking-widest text-crown-ash uppercase">
      {translate("home.recent.title", $locale)}
    </p>
    <ul
      class="flex flex-col divide-y divide-plumage/50 overflow-hidden rounded-lg border border-plumage"
    >
      {#each threads as thread (thread.id)}
        <li>
          <a
            href={resolve(`/chat/${thread.id}`)}
            class="flex items-center gap-3 bg-obsidian-light/40 px-4 py-2.5 transition-colors hover:bg-obsidian-light"
          >
            <MessageSquare class="size-4 shrink-0 text-crown-ash-dark" />
            <span class="min-w-0 flex-1 truncate font-body text-sm text-cream">
              {thread.title?.trim() ||
                translate("home.recent.untitled", $locale)}
            </span>
            {#if thread.activePlanConfigurationId}
              <span
                class="shrink-0 rounded-full bg-energy/15 px-2 py-0.5 text-[10px] font-medium text-energy"
              >
                {translate("home.recent.hasPlan", $locale)}
              </span>
            {/if}
            <span class="shrink-0 font-mono text-[10px] text-crown-ash-dark">
              {formatRelativeTime(
                thread.updatedAt
                  ? timestampDate(thread.updatedAt).toISOString()
                  : "",
                $locale,
              )}
            </span>
          </a>
        </li>
      {/each}
    </ul>
  </div>
{/if}
