<script lang="ts">
  import { AlertTriangle, Bot, Clock, Loader2, User } from "lucide-svelte";
  import { page } from "$app/state";
  import { resolve } from "$app/paths";
  import { requireTenantId } from "$lib/auth";
  import { toUserMessage } from "$lib/connect-errors";
  import { locale, translate } from "$lib/i18n";
  import CanvasElicitationForm from "$lib/components/canvas/CanvasElicitationForm.svelte";
  import {
    ElicitationStatus,
    ThreadMessageRole,
    formatCountdown,
    isExpired,
    isFormDisabled,
    loadElicitation,
    roleLabelKey,
    statusLabelKey,
    timeoutPolicyKey,
    watchElicitations,
    type ElicitationRequest,
  } from "$lib/plans/elicitations";

  let elicitation = $state<ElicitationRequest | null>(null);
  let loading = $state(true);
  let loadError = $state<string | null>(null);

  let now = $state(Date.now());

  const expired = $derived(
    elicitation ? isExpired(elicitation.expiresAt, now) : false,
  );

  const formDisabled = $derived(
    !elicitation || isFormDisabled(elicitation.status) || expired,
  );

  $effect(() => {
    const elicitationId = page.params.elicitationId;
    if (!elicitationId) {
      loadError = translate("elicitations.thread.error", $locale, {
        error: "missing id",
      });
      loading = false;
      return;
    }

    let tenantId: string;
    try {
      tenantId = requireTenantId();
    } catch (error) {
      loadError = toUserMessage(error);
      loading = false;
      return;
    }

    let active = true;

    void (async () => {
      try {
        const loaded = await loadElicitation(tenantId, elicitationId);
        if (active) {
          elicitation = loaded;
        }
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
        for await (const batch of watchElicitations(tenantId)) {
          if (!active) {
            return;
          }
          const match = batch.find((item) => item.id === elicitationId);
          if (match) {
            elicitation = match;
          }
        }
      } catch {
        // Live updates are best-effort; manual reload still reflects state.
      }
    })();

    return () => {
      active = false;
    };
  });

  $effect(() => {
    const interval = setInterval(() => {
      now = Date.now();
    }, 30_000);
    return () => clearInterval(interval);
  });

  function roleIcon(role: ThreadMessageRole) {
    return role === ThreadMessageRole.AGENT ? Bot : User;
  }

  function formatDateTime(value: string): string {
    if (!value) return translate("common.emDash", $locale);
    const parsed = Date.parse(value);
    if (Number.isNaN(parsed)) return value;
    const localeTag = $locale === "pt-BR" ? "pt-BR" : "en-US";
    return new Intl.DateTimeFormat(localeTag, {
      dateStyle: "medium",
      timeStyle: "short",
    }).format(new Date(parsed));
  }
</script>

<div class="mx-auto flex w-full max-w-3xl flex-col gap-4 px-4 py-6 lg:px-6">
  <a
    href={resolve("/inbox")}
    class="font-body text-sm text-crown-ash transition-colors hover:text-talon-gold"
  >
    {translate("nav.needsYou", $locale)}
  </a>

  {#if loading}
    <div class="flex min-h-[16rem] items-center justify-center">
      <div class="flex items-center gap-2 text-crown-ash">
        <Loader2 class="size-5 animate-spin" />
        <span class="font-body text-sm">
          {translate("elicitations.thread.loading", $locale)}
        </span>
      </div>
    </div>
  {:else if !elicitation}
    <div class="rounded-lg border border-red-500/30 bg-red-500/10 p-5">
      <div class="flex items-start gap-3">
        <AlertTriangle class="mt-0.5 size-5 text-red-400" />
        <p class="font-body text-sm text-red-300">
          {translate("elicitations.thread.error", $locale, {
            error: loadError ?? "",
          })}
        </p>
      </div>
    </div>
  {:else}
    <section class="rounded-lg border border-plumage bg-obsidian-light/50 p-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 class="font-heading text-xl font-semibold text-cream">
            {translate("elicitations.thread.heading", $locale)}
          </h1>
          <p class="mt-1 font-mono text-[11px] text-crown-ash">
            {translate("elicitations.thread.stepLabel", $locale)}:
            {elicitation.planStepKey || elicitation.stepExecutionId}
          </p>
        </div>
        <span
          class="rounded-full border border-plumage px-2.5 py-1 font-mono text-[10px] tracking-wider text-crown-ash uppercase"
        >
          {translate(statusLabelKey(elicitation.status), $locale)}
        </span>
      </div>

      <div
        class="mt-3 flex flex-wrap items-center gap-3 border-t border-plumage/40 pt-3 font-mono text-[11px] text-crown-ash"
      >
        <span class="inline-flex items-center gap-1">
          <Clock class="size-3" />
          {translate("elicitations.timeout.label", $locale)}:
          {elicitation.expiresAt
            ? formatCountdown(elicitation.expiresAt, now)
            : translate("elicitations.inbox.noDeadline", $locale)}
        </span>
        <span>
          {translate(timeoutPolicyKey(elicitation.timeoutBehavior), $locale)}
        </span>
      </div>
    </section>

    <section class="space-y-3">
      {#if elicitation.thread.length === 0}
        <p class="font-body text-sm text-crown-ash">
          {translate("elicitations.thread.empty", $locale)}
        </p>
      {/if}
      {#each elicitation.thread as message, index (index)}
        {@const Icon = roleIcon(message.role)}
        <div
          class="rounded-lg border border-plumage bg-obsidian-light/20 p-3 {message.role ===
          ThreadMessageRole.OVERSEER
            ? 'border-talon-gold/40'
            : ''}"
        >
          <div
            class="mb-1 flex items-center gap-2 font-mono text-[10px] tracking-wider text-crown-ash uppercase"
          >
            <Icon class="size-3" />
            {translate(roleLabelKey(message.role), $locale)}
            {#if message.createdAt}
              <span class="text-crown-ash-dark">
                · {formatDateTime(message.createdAt)}
              </span>
            {/if}
          </div>
          <p class="font-body text-sm whitespace-pre-wrap text-cream">
            {message.text}
          </p>
          {#if message.payloadJson && message.payloadJson !== "{}"}
            <pre
              class="mt-2 overflow-x-auto rounded-md border border-plumage/40 bg-obsidian/60 p-2 font-mono text-[11px] text-crown-ash">{message.payloadJson}</pre>
          {/if}
        </div>
      {/each}
    </section>

    <section class="rounded-lg border border-plumage bg-obsidian-light/30 p-4">
      <h2 class="mb-3 font-heading text-lg font-semibold text-cream">
        {translate("elicitations.form.heading", $locale)}
      </h2>

      {#if expired && elicitation.status === ElicitationStatus.PENDING}
        <p
          class="mb-3 rounded-md border border-talon-gold/30 bg-talon-gold/5 px-3 py-2 font-body text-xs text-talon-gold"
        >
          {translate("elicitations.timeout.expired", $locale)}
        </p>
      {/if}

      <CanvasElicitationForm
        {elicitation}
        disabled={formDisabled}
        onAnswered={(updated) => {
          elicitation = updated;
        }}
      />
    </section>
  {/if}
</div>
