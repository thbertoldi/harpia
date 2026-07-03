<script lang="ts">
  import { onMount } from "svelte";
  import { getTenant } from "$lib/auth";
  import { locale, translate } from "$lib/i18n";
  import { watchInbox } from "$lib/inbox/aggregator";
  import type { InboxItem } from "$lib/inbox/types";
  import InboxRow from "$lib/components/inbox/InboxRow.svelte";
  import InboxElicitationActions from "$lib/components/inbox/InboxElicitationActions.svelte";
  import InboxApprovalEntry from "$lib/components/inbox/InboxApprovalEntry.svelte";

  type Filter = "all" | "elicitation" | "approval";
  type SupportedInboxItem = Extract<
    InboxItem,
    { kind: "elicitation" | "approval" }
  >;

  let items = $state<InboxItem[]>([]);
  let filter = $state<Filter>("all");
  let loadError = $state(false);

  onMount(() => {
    const tenant = getTenant();
    if (!tenant?.id) return;
    const controller = new AbortController();
    (async () => {
      try {
        for await (const batch of watchInbox(tenant.id, undefined, {
          signal: controller.signal,
        })) {
          if (controller.signal.aborted) return;
          items = batch;
        }
      } catch {
        if (controller.signal.aborted) return; // expected on unmount
        loadError = true;
      }
    })();
    return () => {
      controller.abort();
    };
  });

  const supportedItems = $derived(
    items.filter(
      (it): it is SupportedInboxItem =>
        it.kind === "elicitation" || it.kind === "approval",
    ),
  );

  const visible = $derived(
    filter === "all"
      ? supportedItems
      : supportedItems.filter((it) => it.kind === filter),
  );

  const counts = $derived({
    all: supportedItems.length,
    elicitation: supportedItems.filter((it) => it.kind === "elicitation")
      .length,
    approval: supportedItems.filter((it) => it.kind === "approval").length,
  });

  // M2 ships only the live "needs you" list. The selectEarlierTodayItems
  // helper exists in lib/inbox/buckets but is not wired here yet — M3 will
  // feed it recently-completed items from a separate query and render the
  // "Earlier today" section below this one.
</script>

<svelte:head>
  <title>{translate("inbox.title", $locale)} · Harpia</title>
</svelte:head>

<div class="mx-auto max-w-5xl px-6 py-6">
  <header class="mb-6 flex items-baseline justify-between">
    <h1 class="font-heading text-2xl text-text">
      {translate("inbox.title", $locale)}
      <span class="ml-2 text-xs font-normal text-text-muted">
        {translate("inbox.pendingCount", $locale).replace(
          "{count}",
          String(counts.all),
        )}
      </span>
    </h1>
  </header>

  <div class="mb-5 flex flex-wrap gap-2">
    {#each [{ key: "all" as Filter, label: "inbox.filters.all", count: counts.all }, { key: "elicitation" as Filter, label: "inbox.filters.elicitations", count: counts.elicitation }, { key: "approval" as Filter, label: "inbox.filters.approvals", count: counts.approval }] as chip (chip.key)}
      <button
        type="button"
        onclick={() => (filter = chip.key)}
        class={`flex items-center gap-1.5 rounded-full border px-3 py-1.5 text-[12px] transition ${
          filter === chip.key
            ? "border-primary bg-primary/10 text-primary"
            : "border-border text-text-muted hover:bg-surface-hover hover:text-text"
        }`}
      >
        {translate(chip.label, $locale)}
        <span
          class={`rounded-full px-1.5 py-0.5 text-[10px] font-semibold ${
            filter === chip.key
              ? "bg-primary text-primary-foreground"
              : "bg-surface-hover text-text"
          }`}
        >
          {chip.count}
        </span>
      </button>
    {/each}
  </div>

  {#if loadError}
    <p
      class="rounded border border-border bg-surface-elevated px-4 py-3 text-sm text-text-muted"
    >
      {translate("inbox.error", $locale)}
    </p>
  {:else if visible.length === 0}
    <p
      class="rounded border border-border bg-surface-elevated px-4 py-3 text-sm text-text-muted"
    >
      {translate("inbox.empty", $locale)}
    </p>
  {:else}
    <ul class="flex flex-col gap-2">
      {#each visible as item (item.id)}
        <li>
          {#if item.kind === "approval"}
            <InboxApprovalEntry {item} />
          {:else}
            <InboxRow {item}>
              {#snippet actions()}
                {#if item.kind === "elicitation"}
                  <InboxElicitationActions />
                {/if}
              {/snippet}
            </InboxRow>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</div>
