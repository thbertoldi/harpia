<script lang="ts">
  import { Check, X, Pencil } from "lucide-svelte";
  import { requireTenantId } from "$lib/auth";
  import { toUserMessage } from "$lib/connect-errors";
  import Skeleton from "$lib/components/Skeleton.svelte";
  import {
    feedbackClient,
    FeedbackDecision,
    FeedbackStatus,
    type FeedbackRequest,
  } from "$lib/rpc";
  import { locale, translate } from "$lib/i18n";
  import { chipFlash } from "$lib/motion/transitions";

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

  type FeedbackClientLike = Pick<
    typeof feedbackClient,
    "getFeedbackStatus" | "submitFeedback"
  >;

  function resolveFeedbackClient(): FeedbackClientLike {
    const scope = globalThis as typeof globalThis & {
      __HARPIA_E2E_OVERSEER__?: { feedbackClient?: FeedbackClientLike };
    };
    return scope.__HARPIA_E2E_OVERSEER__?.feedbackClient ?? feedbackClient;
  }

  $effect(() => {
    loadFeedback();
  });

  async function loadFeedback() {
    loading = true;
    error = null;
    try {
      const client = resolveFeedbackClient();
      const res = await client.getFeedbackStatus({
        tenantId: requireTenantId(),
        feedbackId,
      });
      if (!res.feedbackRequest)
        throw new Error(translate("feedback.error.requestMissing", $locale));
      feedback = res.feedbackRequest;
    } catch (e) {
      error = toUserMessage(e);
    } finally {
      loading = false;
    }
  }

  async function handleDecision(decision: FeedbackDecision, label: string) {
    if (submitting) return;
    submitting = true;
    error = null;
    try {
      const client = resolveFeedbackClient();
      await client.submitFeedback({
        tenantId: requireTenantId(),
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
      error = toUserMessage(e);
    } finally {
      submitting = false;
    }
  }

  function timeAgo(dateStr: string): string {
    const diff = Date.now() - new Date(dateStr).getTime();
    const mins = Math.floor(diff / 60000);
    if (mins < 1) return translate("common.time.justNow", $locale);
    if (mins < 60)
      return translate("common.time.minutesAgo", $locale, { count: mins });
    const hours = Math.floor(mins / 60);
    if (hours < 24)
      return translate("common.time.hoursAgo", $locale, { count: hours });
    const days = Math.floor(hours / 24);
    return translate("common.time.daysAgo", $locale, { count: days });
  }
</script>

{#if loading}
  <div class="space-y-3 py-8">
    <Skeleton width="40%" height="1.5rem" />
    <Skeleton width="100%" height="4rem" />
    <Skeleton width="60%" height="2.5rem" />
  </div>
{:else if error}
  <div class="rounded-lg border border-danger/20 bg-danger/10 p-4">
    <p class="font-mono text-sm text-danger">{error}</p>
  </div>
{:else if submitted}
  <div
    data-testid="feedback-submitted"
    class="animate-in fade-in flex flex-col items-center justify-center py-16 transition-all duration-500"
  >
    <div
      class="flex h-16 w-16 items-center justify-center rounded-full bg-primary/20"
    >
      <Check class="size-8 text-primary" />
    </div>
    <p class="mt-4 font-heading text-xl text-text">
      {translate("feedback.submitted", $locale)}
    </p>
    <p class="mt-1 font-body text-sm text-text-muted">
      {submittedDecision === "Approve"
        ? translate("feedback.submitted.approved", $locale)
        : submittedDecision === "Reject"
          ? translate("feedback.submitted.rejected", $locale)
          : translate("feedback.submitted.modified", $locale)}
    </p>
  </div>
{:else if feedback}
  <div class="transition-all duration-300">
    <h2
      data-testid="feedback-panel-title"
      class="mb-6 font-heading text-2xl font-bold text-text"
    >
      {translate("feedback.yourReview", $locale)}
    </h2>

    <div class="mb-6 rounded-lg border border-border bg-surface-elevated p-5">
      <p
        class="mb-2 font-mono text-[10px] tracking-widest text-text-muted uppercase"
      >
        {translate("feedback.agentRequest", $locale)}
      </p>
      <p
        data-testid="feedback-question"
        class="font-body text-base leading-relaxed text-text"
      >
        {feedback.question}
      </p>
      {#if feedback.createdAt}
        <p class="mt-2 font-mono text-xs text-text-muted-dark">
          {translate("feedback.waiting", $locale)}
          {timeAgo(feedback.createdAt)}
        </p>
      {/if}
    </div>

    {#if feedback.options && feedback.options.length > 0}
      <div class="mb-6 rounded-lg border border-border bg-surface p-4">
        <p
          class="mb-3 font-mono text-[10px] tracking-widest text-text-muted uppercase"
        >
          {translate("feedback.agentOutput", $locale)}
        </p>
        <div class="max-h-64 overflow-y-auto">
          {#each feedback.options as option, idx (option)}
            <div
              data-testid={`feedback-option-${idx}`}
              class="mb-2 rounded bg-surface-hover px-3 py-2 font-mono text-sm leading-relaxed whitespace-pre-wrap text-text-muted last:mb-0"
            >
              {option}
            </div>
          {/each}
        </div>
      </div>
    {/if}

    {#if feedback.status === FeedbackStatus.PENDING}
      <div class="mb-6">
        <label
          for="feedback-comment"
          class="mb-2 block font-mono text-[10px] tracking-widest text-text-muted uppercase"
          >{translate("feedback.commentOptional", $locale)}</label
        >
        <textarea
          id="feedback-comment"
          bind:value={comment}
          placeholder={translate("feedback.commentPlaceholder", $locale)}
          class="w-full resize-none rounded-lg border border-border bg-surface-elevated px-4 py-3 font-body text-sm text-text placeholder:text-text-muted-dark focus:border-primary focus:ring-1 focus:ring-primary/30 focus:outline-none"
          rows="3"
        ></textarea>
      </div>

      <div class="flex gap-3">
        <button
          onclick={() => handleDecision(FeedbackDecision.APPROVE, "Approve")}
          data-testid="feedback-approve"
          disabled={submitting}
          in:chipFlash
          class="flex items-center gap-2 rounded-lg bg-primary px-5 py-2.5 font-body text-sm font-medium text-primary-foreground transition-all hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {#if submitting}
            <Skeleton shape="circle" width="1rem" height="1rem" />
          {:else}
            <Check class="size-4" />
          {/if}
          {translate("feedback.approve", $locale)}
        </button>
        <button
          onclick={() => handleDecision(FeedbackDecision.REJECT, "Reject")}
          data-testid="feedback-reject-retry"
          disabled={submitting}
          in:chipFlash
          class="flex items-center gap-2 rounded-lg border border-danger/30 bg-danger/10 px-5 py-2.5 font-body text-sm font-medium text-danger transition-all hover:border-danger/50 hover:bg-danger/20 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {#if submitting}
            <Skeleton shape="circle" width="1rem" height="1rem" />
          {:else}
            <X class="size-4" />
          {/if}
          {translate("feedback.rejectRetry", $locale)}
        </button>
        <button
          onclick={() => handleDecision(FeedbackDecision.MODIFY, "Modify")}
          data-testid="feedback-modify"
          disabled={submitting}
          in:chipFlash
          class="flex items-center gap-2 rounded-lg border border-border bg-transparent px-5 py-2.5 font-body text-sm font-medium text-text-muted transition-all hover:bg-surface-hover hover:text-text disabled:cursor-not-allowed disabled:opacity-50"
        >
          {#if submitting}
            <Skeleton shape="circle" width="1rem" height="1rem" />
          {:else}
            <Pencil class="size-4" />
          {/if}
          {translate("feedback.modify", $locale)}
        </button>
      </div>
    {/if}

    {#if feedback.status !== FeedbackStatus.PENDING}
      <div class="rounded-lg border border-border bg-surface-elevated p-4">
        <p class="font-body text-sm text-text-muted">
          {translate("feedback.alreadyResolved", $locale)}
        </p>
      </div>
    {/if}

    {#if error}
      <div class="mt-4 rounded-lg border border-danger/20 bg-danger/10 p-3">
        <p class="font-mono text-xs text-danger">{error}</p>
      </div>
    {/if}
  </div>
{/if}
