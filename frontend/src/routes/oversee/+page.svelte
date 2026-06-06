<script lang="ts">
  import { Check, Loader2, Clock, Eye } from "lucide-svelte";
  import { listPendingFeedback } from "$lib/client";
  import type { FeedbackRequest } from "$lib/types";
  import FeedbackPanel from "$lib/components/FeedbackPanel.svelte";

  let feedbacks = $state<FeedbackRequest[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);
  let selectedId = $state<string | null>(null);

  $effect(() => {
    loadPending();
  });

  async function loadPending() {
    loading = true;
    error = null;
    try {
      const res = await listPendingFeedback({
        tenantId: "default",
        pageSize: 50,
        pageToken: "",
      });
      feedbacks = res.feedbackRequests ?? [];
    } catch (e) {
      error = e instanceof Error ? e.message : "Failed to load feedback";
    } finally {
      loading = false;
    }
  }

  function timeAgo(dateStr: string): string {
    const diff = Date.now() - new Date(dateStr).getTime();
    const mins = Math.floor(diff / 60000);
    if (mins < 1) return "just now";
    if (mins < 60) return `${mins}m ago`;
    const hours = Math.floor(mins / 60);
    if (hours < 24) return `${hours}h ago`;
    const days = Math.floor(hours / 24);
    return `${days}d ago`;
  }

  function shortId(id: string): string {
    return id.slice(0, 8);
  }

  function handleResolve() {
    selectedId = null;
    loadPending();
  }
</script>

<div class="flex h-full">
  <div class="flex-1 overflow-y-auto p-6 lg:p-8">
    <div class="mx-auto max-w-3xl">
      <h1 class="mb-2 font-heading text-3xl font-bold text-cream">Oversee</h1>
      <p class="mb-8 font-body text-crown-ash">
        Review agent outputs that need your attention.
      </p>

      {#if loading}
        <div class="flex items-center justify-center py-16">
          <Loader2 class="size-6 animate-spin text-talon-gold" />
        </div>
      {:else if error}
        <div class="rounded-lg border border-red-500/20 bg-red-500/10 p-4">
          <p class="font-mono text-sm text-red-400">{error}</p>
          <button
            onclick={loadPending}
            class="mt-3 rounded-md border border-red-500/30 px-3 py-1.5 font-body text-xs text-red-400 transition-colors hover:bg-red-500/10"
          >
            Retry
          </button>
        </div>
      {:else if feedbacks.length === 0}
        <div
          class="flex flex-col items-center justify-center py-20 text-center"
        >
          <div
            class="flex h-16 w-16 items-center justify-center rounded-full bg-green-500/10"
          >
            <Check class="size-8 text-green-400" />
          </div>
          <p class="mt-4 font-heading text-xl text-cream">All caught up</p>
          <p class="mt-1 font-body text-sm text-crown-ash">
            Nothing needs your review.
          </p>
        </div>
      {:else}
        <div class="space-y-3">
          {#each feedbacks as fb (fb.id)}
            <button
              onclick={() => (selectedId = fb.id)}
              class="w-full rounded-lg border text-left transition-all {selectedId ===
              fb.id
                ? 'border-talon-gold bg-obsidian-light'
                : 'border-plumage bg-obsidian hover:border-talon-gold/50'}"
            >
              <div class="flex items-start gap-4 p-4">
                <div class="min-w-0 flex-1">
                  <div class="mb-1 flex items-center gap-2">
                    <span
                      class="inline-flex animate-pulse items-center rounded-full bg-talon-gold/10 px-2 py-0.5 font-mono text-[10px] tracking-wider text-talon-gold uppercase"
                    >
                      Awaiting Your Review
                    </span>
                    {#if fb.createdAt}
                      <span
                        class="flex items-center gap-1 font-mono text-[10px] text-crown-ash-dark"
                      >
                        <Clock class="size-3" />
                        {timeAgo(fb.createdAt)}
                      </span>
                    {/if}
                  </div>
                  <p class="truncate font-body text-sm font-medium text-cream">
                    {fb.question}
                  </p>
                  <div class="mt-2 flex items-center gap-3">
                    <span class="font-mono text-[10px] text-crown-ash-dark"
                      >Task: {shortId(fb.taskId)}</span
                    >
                    {#if fb.agentInstanceId}
                      <span class="font-mono text-[10px] text-crown-ash-dark"
                        >Agent: {shortId(fb.agentInstanceId)}</span
                      >
                    {/if}
                  </div>
                </div>
                <Eye class="mt-0.5 size-4 shrink-0 text-crown-ash" />
              </div>
            </button>
          {/each}
        </div>
      {/if}
    </div>
  </div>

  {#if selectedId}
    <div
      class="w-[480px] shrink-0 overflow-y-auto border-l border-plumage bg-obsidian-light transition-all duration-300"
    >
      <div
        class="flex items-center justify-between border-b border-plumage px-4 py-3"
      >
        <span
          class="font-mono text-[10px] tracking-widest text-crown-ash uppercase"
          >Feedback Details</span
        >
        <button
          onclick={() => (selectedId = null)}
          class="rounded-md p-1 text-crown-ash transition-colors hover:text-cream"
          aria-label="Close feedback details"
        >
          <svg
            class="size-4"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <path d="M18 6L6 18M6 6l12 12" />
          </svg>
        </button>
      </div>
      <div class="p-4">
        <FeedbackPanel feedbackId={selectedId} onResolve={handleResolve} />
      </div>
    </div>
  {/if}
</div>
