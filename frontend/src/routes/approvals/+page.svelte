<script lang="ts">
  import { CheckCircle2, Loader2, ShieldCheck } from "lucide-svelte";
  import { resolve } from "$app/paths";
  import { requireTenantId } from "$lib/auth";
  import { toUserMessage } from "$lib/connect-errors";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import { locale, translate } from "$lib/i18n";
  import {
    loadInboxApprovals,
    watchApprovalRequests,
    type ApprovalRequest,
  } from "$lib/plans/approvals";

  let approvals = $state<ApprovalRequest[]>([]);
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

    void (async () => {
      try {
        approvals = await loadInboxApprovals(tenantId);
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
        for await (const batch of watchApprovalRequests(tenantId)) {
          if (!active) {
            return;
          }
          approvals = batch;
          loading = false;
        }
      } catch {
        // Stream is a live-update enhancement; the initial load already
        // populated the inbox, so failures here are non-fatal.
      }
    })();

    return () => {
      active = false;
    };
  });
</script>

<div class="mx-auto flex w-full max-w-4xl flex-col gap-4 px-4 py-6 lg:px-6">
  <div class="flex items-center justify-between gap-4">
    <div>
      <HarpyHeading tag="h1" class="text-2xl text-cream">
        {translate("approvals.inbox.heading", $locale)}
      </HarpyHeading>
      <p class="mt-1 font-body text-sm text-crown-ash">
        {translate("approvals.inbox.subtitle", $locale)}
      </p>
    </div>
    {#if !loading && approvals.length > 0}
      <span
        class="rounded-full border border-talon-gold/40 bg-talon-gold/10 px-2.5 py-1 font-mono text-[10px] tracking-wider text-talon-gold uppercase"
      >
        {translate("approvals.inbox.pendingCount", $locale, {
          count: approvals.length,
        })}
      </span>
    {/if}
  </div>

  {#if loading}
    <div class="flex min-h-[30vh] items-center justify-center">
      <div class="flex items-center gap-2 text-crown-ash">
        <Loader2 class="size-5 animate-spin" />
        <span class="font-body text-sm">
          {translate("approvals.inbox.loading", $locale)}
        </span>
      </div>
    </div>
  {:else if loadError && approvals.length === 0}
    <div class="rounded-lg border border-red-500/30 bg-red-500/10 p-5">
      <p class="font-body text-sm text-red-300">
        {translate("approvals.inbox.error", $locale, { error: loadError })}
      </p>
    </div>
  {:else if approvals.length === 0}
    <div
      class="flex min-h-[30vh] flex-col items-center justify-center rounded-lg border border-dashed border-plumage bg-obsidian-light/30 px-6 py-12 text-center"
    >
      <CheckCircle2 class="mb-4 size-12 text-talon-gold" />
      <p class="max-w-md font-body text-sm text-crown-ash">
        {translate("approvals.inbox.empty", $locale)}
      </p>
    </div>
  {:else}
    <div class="space-y-3">
      {#each approvals as item (item.id)}
        <a
          href={resolve(
            `/plans/executions/${item.planExecutionId}/approvals/${item.id}`,
          )}
          class="flex flex-col gap-2 rounded-lg border border-plumage bg-obsidian-light/20 px-4 py-3 transition-colors hover:border-talon-gold/60"
        >
          <div class="flex items-start gap-2">
            <ShieldCheck class="mt-0.5 size-4 shrink-0 text-talon-gold" />
            <p class="font-heading text-base text-cream">
              {translate("approvals.inbox.publishDraft", $locale, {
                step: item.planStepKey || item.stepExecutionId,
              })}
            </p>
          </div>
          <div class="font-mono text-[11px] text-crown-ash">
            {translate("approvals.inbox.step", $locale)}:
            {item.planStepKey || item.stepExecutionId}
          </div>
        </a>
      {/each}
    </div>
  {/if}
</div>
