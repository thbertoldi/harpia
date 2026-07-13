<script lang="ts">
  import { CheckCircle2, MessageSquare, Send } from "lucide-svelte";
  import type { ChatMessage } from "$lib/chat/types";
  import { toUserMessage } from "$lib/connect-errors";
  import { locale, translate } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";
  import {
    ElicitationStatus,
    loadElicitation,
    respondToElicitation,
    type ElicitationRequest,
  } from "$lib/plans/elicitations";

  interface Props {
    message?: ChatMessage;
    /** Stable request identity from the projected execution interaction. */
    elicitationId?: string;
    tenantId?: string;
    onDecided?: () => void;
  }
  let {
    message,
    elicitationId = "",
    tenantId = "",
    onDecided,
  }: Props = $props();
  const requestId = $derived.by(() => {
    if (elicitationId) return elicitationId;
    try {
      const raw = JSON.parse(message?.payloadJson || "{}") as {
        elicitation_id?: string;
      };
      return raw.elicitation_id ?? "";
    } catch {
      return "";
    }
  });
  let request = $state<ElicitationRequest | null>(null);
  let answer = $state("");
  let submitting = $state(false);
  let error = $state<string | null>(null);

  $effect(() => {
    if (!tenantId || !requestId) return;
    const controller = new AbortController();
    void loadElicitation(tenantId, requestId)
      .then((next) => {
        if (!controller.signal.aborted) request = next;
      })
      .catch(() => {
        if (!controller.signal.aborted)
          error = translate("thread.elicitation.loadError", $locale);
      });
    return () => controller.abort();
  });

  const isAnswered = $derived(
    request?.status === ElicitationStatus.ANSWERED ||
      message?.kind === "ELICITATION_ANSWERED",
  );
  const isActionable = $derived(
    !!tenantId &&
      !!requestId &&
      request?.status === ElicitationStatus.PENDING &&
      !submitting,
  );

  async function submit() {
    if (!isActionable || !answer.trim()) return;
    submitting = true;
    error = null;
    try {
      request = await respondToElicitation(tenantId, requestId, {
        responseText: answer.trim(),
      });
      onDecided?.();
    } catch (cause) {
      error = toUserMessage(cause);
    } finally {
      submitting = false;
    }
  }
</script>

<div
  id={message ? `m-${message.id}` : undefined}
  class="w-full rounded-lg border border-talon-gold/40 bg-talon-gold/10 px-4 py-3"
>
  <div class="flex items-center gap-3">
    {#if isAnswered}
      <CheckCircle2 class="size-4 text-status-done" />
    {:else}
      <MessageSquare class="size-4 text-talon-gold" />
    {/if}
    <p class="min-w-0 flex-1 text-[13px] text-cream">
      {isAnswered
        ? translate("thread.elicitation.answered", $locale)
        : translate("thread.elicitation.raised", $locale)}
    </p>
    {#if message}
      <span class="text-[10px] text-crown-ash-dark"
        >{formatRelativeTime(message.createdAt, $locale)}</span
      >
    {/if}
  </div>

  {#if request}
    <p class="mt-3 text-[12px] whitespace-pre-wrap text-cream">
      {request.prompt}
    </p>
    {#if request.schemaJson}
      <details class="mt-2 text-[11px] text-crown-ash">
        <summary class="cursor-pointer text-crown-ash hover:text-cream">
          {translate("thread.elicitation.schema", $locale)}
        </summary>
        <pre
          class="mt-1 overflow-x-auto rounded border border-plumage/60 bg-obsidian p-2 text-[10px] text-crown-ash">{request.schemaJson}</pre>
      </details>
    {/if}
    {#if isActionable}
      <label
        class="mt-3 block text-[11px] text-crown-ash"
        for={`elicitation-${requestId}`}
      >
        {translate("thread.elicitation.answerLabel", $locale)}
      </label>
      <textarea
        id={`elicitation-${requestId}`}
        bind:value={answer}
        rows="3"
        class="mt-1 w-full rounded-md border border-plumage bg-obsidian px-3 py-2 text-[12px] text-cream outline-none focus:border-talon-gold"
        placeholder={translate("thread.elicitation.answerPlaceholder", $locale)}
      ></textarea>
      <button
        type="button"
        disabled={!answer.trim() || submitting}
        onclick={submit}
        class="mt-2 inline-flex items-center gap-1.5 rounded-md border border-talon-gold bg-talon-gold px-3 py-2 text-[12px] font-semibold text-on-primary disabled:opacity-50"
      >
        <Send class="size-3.5" />
        {translate(
          submitting
            ? "thread.elicitation.pending"
            : "thread.elicitation.answer",
          $locale,
        )}
      </button>
    {/if}
  {:else if tenantId && requestId}
    <p class="mt-2 text-[11px] text-crown-ash">
      {translate("thread.elicitation.loading", $locale)}
    </p>
  {/if}
  {#if error}
    <p class="mt-2 text-[11px] text-danger">{error}</p>
  {/if}
</div>
