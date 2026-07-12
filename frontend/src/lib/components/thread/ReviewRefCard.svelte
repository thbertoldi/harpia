<script lang="ts">
  import { CheckCircle2, Eye, MessageSquareMore } from "lucide-svelte";
  import { fade } from "svelte/transition";
  import type { ChatMessage } from "$lib/chat/types";
  import { locale, translate } from "$lib/i18n";
  import { toUserMessage } from "$lib/connect-errors";
  import {
    loadReview,
    respondToReview,
    type ReviewResponse,
  } from "$lib/plans/reviews";

  interface ArtifactRef {
    artifactId: string;
    artifactVersionId: string;
    artifactTypeKey: string;
    contentHash: string;
  }
  interface Props {
    message: ChatMessage;
    tenantId?: string;
    onOpenArtifact?: (artifactId: string, artifactVersionId?: string) => void;
    onDecided?: () => void;
  }
  let { message, tenantId = "", onOpenArtifact, onDecided }: Props = $props();
  const reviewId = $derived.by(() => {
    try {
      const raw = JSON.parse(message.payloadJson) as {
        review_request_id?: string;
      };
      return raw.review_request_id ?? "";
    } catch {
      return "";
    }
  });
  let subject = $state<ArtifactRef | null>(null);
  let decision = $state<ReviewResponse | null>(null);
  let feedback = $state("");
  let reviseMode = $state(false);
  let submitting = $state(false);
  let error = $state<string | null>(null);

  $effect(() => {
    if (!tenantId || !reviewId) return;
    const controller = new AbortController();
    void loadReview(tenantId, reviewId)
      .then((review) => {
        if (controller.signal.aborted) return;
        const ref = review.subjectArtifactRef;
        subject = ref
          ? {
              artifactId: ref.artifactId,
              artifactVersionId: ref.artifactVersionId,
              artifactTypeKey: ref.artifactTypeKey,
              contentHash: ref.contentHash,
            }
          : null;
        if (review.status === 2) decision = "accept";
        if (review.status === 3) decision = "revise";
      })
      .catch(() => undefined);
    return () => controller.abort();
  });

  async function submit(next: ReviewResponse) {
    if (!tenantId || !reviewId || submitting) return;
    if (next === "revise" && !feedback.trim()) {
      reviseMode = true;
      return;
    }
    submitting = true;
    error = null;
    try {
      await respondToReview(tenantId, reviewId, next, feedback.trim());
      decision = next;
      onDecided?.();
    } catch (cause) {
      error = toUserMessage(cause);
    } finally {
      submitting = false;
    }
  }
</script>

<div
  id={`m-${message.id}`}
  class="w-full rounded-lg border border-talon-gold/40 bg-talon-gold/10 px-4 py-3"
>
  <div class="flex items-center gap-3">
    <MessageSquareMore class="size-4 text-talon-gold" />
    <p class="min-w-0 flex-1 text-[13px] text-cream">
      {decision === "accept"
        ? translate("thread.review.accepted", $locale)
        : decision === "revise"
          ? translate("thread.review.revisionRequested", $locale)
          : translate("thread.review.raised", $locale)}
    </p>
  </div>
  {#if subject}
    <p class="mt-1 text-[10px] text-crown-ash-dark">
      {translate("thread.review.context.version", $locale, {
        value: subject.artifactVersionId.slice(0, 8),
      })}
    </p>
  {/if}
  {#if !decision && tenantId && reviewId}
    <div class="mt-3 flex flex-wrap gap-2">
      {#if subject && onOpenArtifact}
        <button
          type="button"
          onclick={() =>
            onOpenArtifact?.(subject!.artifactId, subject!.artifactVersionId)}
          class="inline-flex items-center gap-1 rounded-md border border-plumage px-3 py-2 text-[12px] text-crown-ash hover:border-talon-gold hover:text-talon-gold"
          ><Eye class="size-3.5" />{translate(
            "thread.review.previewArtifact",
            $locale,
          )}</button
        >
      {/if}
      <button
        type="button"
        disabled={submitting}
        onclick={() => submit("accept")}
        class="inline-flex items-center gap-1 rounded-md border border-talon-gold bg-talon-gold px-3 py-2 text-[12px] font-semibold text-on-primary disabled:opacity-50"
        ><CheckCircle2 class="size-3.5" />{translate(
          submitting ? "thread.review.pending" : "thread.review.accept",
          $locale,
        )}</button
      >
      <button
        type="button"
        disabled={submitting}
        onclick={() => submit("revise")}
        class="rounded-md border border-danger/60 px-3 py-2 text-[12px] text-danger disabled:opacity-50"
        >{translate("thread.review.revise", $locale)}</button
      >
    </div>
    {#if reviseMode}
      <textarea
        transition:fade
        bind:value={feedback}
        rows="2"
        class="mt-2 w-full rounded-md border border-plumage bg-obsidian px-3 py-2 text-[12px] text-cream outline-none focus:border-talon-gold"
        placeholder={translate("thread.review.feedback", $locale)}
      ></textarea>
    {/if}
  {/if}
  {#if error}<p transition:fade class="mt-2 text-[11px] text-danger">
      {translate("thread.review.error", $locale)}
    </p>{/if}
</div>
