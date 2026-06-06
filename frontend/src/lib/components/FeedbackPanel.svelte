<script lang="ts">
  import { Check, X, Pencil, Loader2 } from "lucide-svelte";
  import { getFeedbackStatus, submitFeedback } from "$lib/client";
  import { FeedbackDecision, type FeedbackRequest } from "$lib/types";

  let {
    feedbackId,
    onResolve,
  }: {
    feedbackId: string;
    onResolve: () => void;
  } = $props();

  let feedback = $state<FeedbackRequest | null>(null);
  let loading = $state(true);
  let submitting = $state(false);
  let submitted = $state(false);
  let submittedDecision = $state<string>("");
  let error = $state<string | null>(null);
  let comment = $state("");

  $effect(() => {
    loadFeedback();
  });

  async function loadFeedback() {
    loading = true;
    error = null;
    try {
      const res = await getFeedbackStatus({
        tenantId: "default",
        feedbackId,
      });
      feedback = res.feedbackRequest;
    } catch (e) {
      error = e instanceof Error ? e.message : "Failed to load feedback";
    } finally {
      loading = false;
    }
  }

  async function handleDecision(decision: FeedbackDecision, label: string) {
    if (submitting) return;
    submitting = true;
    error = null;
    try {
      await submitFeedback({
        tenantId: "default",
        feedbackId,
        decision,
        comment,
      });
      submittedDecision = label;
      submitted = true;
      setTimeout(() => {
        onResolve();
      }, 1500);
    } catch (e) {
      error = e instanceof Error ? e.message : "Failed to submit feedback";
    } finally {
      submitting = false;
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
</script>

{#if loading}
  <div class="flex items-center justify-center py-16">
    <Loader2 class="size-6 animate-spin text-talon-gold" />
  </div>
{:else if error}
  <div class="rounded-lg border border-red-500/20 bg-red-500/10 p-4">
    <p class="font-mono text-sm text-red-400">{error}</p>
  </div>
{:else if submitted}
  <div
    class="animate-in fade-in flex flex-col items-center justify-center py-16 transition-all duration-500"
  >
    <div
      class="flex h-16 w-16 items-center justify-center rounded-full bg-talon-gold/20"
    >
      <Check class="size-8 text-talon-gold" />
    </div>
    <p class="mt-4 font-heading text-xl text-cream">Feedback submitted</p>
    <p class="mt-1 font-body text-sm text-crown-ash">
      {submittedDecision === "Approve"
        ? "Agent work approved."
        : submittedDecision === "Reject"
          ? "Sent back to agent for retry."
          : "Modification feedback sent."}
    </p>
  </div>
{:else if feedback}
  <div class="transition-all duration-300">
    <h2 class="mb-6 font-heading text-2xl font-bold text-cream">Your Review</h2>

    <div class="mb-6 rounded-lg border border-plumage bg-obsidian-light p-5">
      <p
        class="mb-2 font-mono text-[10px] tracking-widest text-crown-ash uppercase"
      >
        Agent Request
      </p>
      <p class="font-body text-base leading-relaxed text-cream">
        {feedback.question}
      </p>
      {#if feedback.createdAt}
        <p class="mt-2 font-mono text-xs text-crown-ash-dark">
          Waiting {timeAgo(feedback.createdAt)}
        </p>
      {/if}
    </div>

    {#if feedback.options && feedback.options.length > 0}
      <div class="mb-6 rounded-lg border border-plumage bg-obsidian p-4">
        <p
          class="mb-3 font-mono text-[10px] tracking-widest text-crown-ash uppercase"
        >
          Agent Output
        </p>
        <div class="max-h-64 overflow-y-auto">
          {#each feedback.options as option (option)}
            <div
              class="mb-2 rounded bg-obsidian-light px-3 py-2 font-mono text-sm leading-relaxed whitespace-pre-wrap text-crown-ash last:mb-0"
            >
              {option}
            </div>
          {/each}
        </div>
      </div>
    {/if}

    {#if feedback.status === 1}
      <div class="mb-6">
        <label
          for="feedback-comment"
          class="mb-2 block font-mono text-[10px] tracking-widest text-crown-ash uppercase"
          >Comment (optional)</label
        >
        <textarea
          id="feedback-comment"
          bind:value={comment}
          placeholder="Provide additional context for the agent..."
          class="w-full resize-none rounded-lg border border-plumage bg-obsidian-light px-4 py-3 font-body text-sm text-cream placeholder:text-crown-ash-dark focus:border-talon-gold focus:ring-1 focus:ring-talon-gold/30 focus:outline-none"
          rows="3"
        ></textarea>
      </div>

      <div class="flex gap-3">
        <button
          onclick={() => handleDecision(FeedbackDecision.APPROVE, "Approve")}
          disabled={submitting}
          class="flex items-center gap-2 rounded-lg bg-talon-gold px-5 py-2.5 font-body text-sm font-medium text-obsidian transition-all hover:bg-talon-gold-bright disabled:cursor-not-allowed disabled:opacity-50"
        >
          {#if submitting}
            <Loader2 class="size-4 animate-spin" />
          {:else}
            <Check class="size-4" />
          {/if}
          Approve
        </button>
        <button
          onclick={() => handleDecision(FeedbackDecision.REJECT, "Reject")}
          disabled={submitting}
          class="flex items-center gap-2 rounded-lg border border-red-500/30 bg-red-500/10 px-5 py-2.5 font-body text-sm font-medium text-red-400 transition-all hover:border-red-500/50 hover:bg-red-500/20 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {#if submitting}
            <Loader2 class="size-4 animate-spin" />
          {:else}
            <X class="size-4" />
          {/if}
          Reject & Retry
        </button>
        <button
          onclick={() => handleDecision(FeedbackDecision.MODIFY, "Modify")}
          disabled={submitting}
          class="flex items-center gap-2 rounded-lg border border-plumage bg-transparent px-5 py-2.5 font-body text-sm font-medium text-crown-ash transition-all hover:border-talon-gold hover:text-talon-gold disabled:cursor-not-allowed disabled:opacity-50"
        >
          {#if submitting}
            <Loader2 class="size-4 animate-spin" />
          {:else}
            <Pencil class="size-4" />
          {/if}
          Modify
        </button>
      </div>
    {/if}

    {#if feedback.status !== 1}
      <div class="rounded-lg border border-crown-ash/20 bg-obsidian-light p-4">
        <p class="font-body text-sm text-crown-ash">
          This feedback has already been resolved.
        </p>
      </div>
    {/if}

    {#if error}
      <div class="mt-4 rounded-lg border border-red-500/20 bg-red-500/10 p-3">
        <p class="font-mono text-xs text-red-400">{error}</p>
      </div>
    {/if}
  </div>
{/if}
