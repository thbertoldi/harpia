<script lang="ts">
  import {
    ChevronLeft,
    ChevronRight,
    Download,
    FileJson,
    FileSpreadsheet,
    Filter,
    Loader2,
    ScrollText,
  } from "lucide-svelte";
  import {
    AGENT_TYPES,
    FEEDBACK_DECISIONS,
    auditEventsToCsv,
    auditEventsToJson,
    downloadTextFile,
    getAllFilteredAuditEvents,
    getAuditEvents,
    type AuditEvent,
    type AuditEventFilters,
  } from "$lib/audit/audit-store";

  const PAGE_SIZE = 10;

  let events = $state<AuditEvent[]>([]);
  let nextPageToken = $state<string | null>(null);
  let pageHistory = $state<(string | null)[]>([null]);
  let loading = $state(true);
  let exporting = $state(false);

  let taskId = $state("");
  let userId = $state("");
  let agentType = $state("");
  let decision = $state<AuditEventFilters["decision"] | "">("");
  let dateFrom = $state("");
  let dateTo = $state("");

  let appliedFilters = $state<AuditEventFilters>({});

  const currentPageIndex = $derived(pageHistory.length - 1);
  const currentPageToken = $derived(pageHistory[currentPageIndex] ?? null);
  const canGoPrev = $derived(currentPageIndex > 0);
  const canGoNext = $derived(nextPageToken !== null);

  $effect(() => {
    void loadPage(currentPageToken, appliedFilters);
  });

  async function loadPage(
    token: string | null,
    activeFilters: AuditEventFilters,
  ) {
    loading = true;
    try {
      const page = await getAuditEvents(activeFilters, token, PAGE_SIZE);
      events = page.events;
      nextPageToken = page.nextPageToken;
    } finally {
      loading = false;
    }
  }

  function buildFilters(): AuditEventFilters {
    return {
      taskId: taskId || undefined,
      userId: userId || undefined,
      agentType: agentType || undefined,
      decision: decision || undefined,
      dateFrom: dateFrom || undefined,
      dateTo: dateTo || undefined,
    };
  }

  function applyFilters() {
    appliedFilters = buildFilters();
    pageHistory = [null];
  }

  function clearFilters() {
    taskId = "";
    userId = "";
    agentType = "";
    decision = "";
    dateFrom = "";
    dateTo = "";
    appliedFilters = {};
    pageHistory = [null];
  }

  function goNext() {
    if (!nextPageToken) return;
    pageHistory = [...pageHistory, nextPageToken];
  }

  function goPrev() {
    if (pageHistory.length <= 1) return;
    pageHistory = pageHistory.slice(0, -1);
  }

  async function exportCsv() {
    exporting = true;
    try {
      const all = await getAllFilteredAuditEvents(appliedFilters);
      const csv = auditEventsToCsv(all);
      downloadTextFile(
        csv,
        `audit-log-${new Date().toISOString().slice(0, 10)}.csv`,
        "text/csv",
      );
    } finally {
      exporting = false;
    }
  }

  async function exportJson() {
    exporting = true;
    try {
      const all = await getAllFilteredAuditEvents(appliedFilters);
      const json = auditEventsToJson(all);
      downloadTextFile(
        json,
        `audit-log-${new Date().toISOString().slice(0, 10)}.json`,
        "application/json",
      );
    } finally {
      exporting = false;
    }
  }

  function formatTimestamp(iso: string): string {
    return new Date(iso).toLocaleString(undefined, {
      dateStyle: "medium",
      timeStyle: "short",
    });
  }

  function shortId(id: string): string {
    return id.length > 12 ? `${id.slice(0, 8)}…` : id;
  }

  function contextLabel(ctx: AuditEvent["boundedContext"]): string {
    return ctx.replace(/_/g, " ");
  }

  function eventTypeLabel(type: AuditEvent["eventType"]): string {
    return type.replace(/\./g, " · ");
  }
</script>

<div class="p-6 lg:p-8">
  <div class="mx-auto max-w-6xl">
    <div
      class="mb-8 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between"
    >
      <div>
        <div class="mb-2 flex items-center gap-2">
          <ScrollText class="size-6 text-talon-gold" />
          <h1 class="font-heading text-3xl font-bold text-cream">Audit Log</h1>
        </div>
        <p class="font-body text-crown-ash">
          Tamper-evident trail of agent actions and human decisions.
        </p>
      </div>

      <div class="flex shrink-0 gap-2">
        <button
          onclick={exportCsv}
          disabled={exporting}
          class="inline-flex cursor-pointer items-center gap-2 rounded-md border border-plumage px-3 py-2 font-body text-xs text-crown-ash transition-colors hover:border-talon-gold hover:text-cream disabled:opacity-50"
        >
          <FileSpreadsheet class="size-3.5" />
          Export CSV
        </button>
        <button
          onclick={exportJson}
          disabled={exporting}
          class="inline-flex cursor-pointer items-center gap-2 rounded-md border border-plumage px-3 py-2 font-body text-xs text-crown-ash transition-colors hover:border-talon-gold hover:text-cream disabled:opacity-50"
        >
          <FileJson class="size-3.5" />
          Export JSON
        </button>
      </div>
    </div>

    <section
      class="mb-6 rounded-lg border border-plumage bg-obsidian-light p-4"
    >
      <div class="mb-4 flex items-center gap-2">
        <Filter class="size-4 text-talon-gold" />
        <h2 class="font-body text-sm font-medium text-cream">Filters</h2>
      </div>

      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <label class="block">
          <span
            class="mb-1 block font-mono text-[10px] tracking-wider text-crown-ash-dark uppercase"
            >Task ID</span
          >
          <input
            type="text"
            bind:value={taskId}
            placeholder="task-a1b2…"
            class="w-full rounded-md border border-plumage bg-obsidian px-3 py-2 font-mono text-xs text-cream placeholder:text-crown-ash-dark focus:border-talon-gold focus:outline-none"
          />
        </label>

        <label class="block">
          <span
            class="mb-1 block font-mono text-[10px] tracking-wider text-crown-ash-dark uppercase"
            >User</span
          >
          <input
            type="text"
            bind:value={userId}
            placeholder="dev-overseer"
            class="w-full rounded-md border border-plumage bg-obsidian px-3 py-2 font-mono text-xs text-cream placeholder:text-crown-ash-dark focus:border-talon-gold focus:outline-none"
          />
        </label>

        <label class="block">
          <span
            class="mb-1 block font-mono text-[10px] tracking-wider text-crown-ash-dark uppercase"
            >Agent Type</span
          >
          <select
            bind:value={agentType}
            class="w-full rounded-md border border-plumage bg-obsidian px-3 py-2 font-mono text-xs text-cream focus:border-talon-gold focus:outline-none"
          >
            <option value="">All</option>
            {#each AGENT_TYPES as type (type)}
              <option value={type}>{type}</option>
            {/each}
          </select>
        </label>

        <label class="block">
          <span
            class="mb-1 block font-mono text-[10px] tracking-wider text-crown-ash-dark uppercase"
            >Decision</span
          >
          <select
            bind:value={decision}
            class="w-full rounded-md border border-plumage bg-obsidian px-3 py-2 font-mono text-xs text-cream focus:border-talon-gold focus:outline-none"
          >
            <option value="">All</option>
            {#each FEEDBACK_DECISIONS as d (d)}
              <option value={d}>{d}</option>
            {/each}
          </select>
        </label>

        <label class="block">
          <span
            class="mb-1 block font-mono text-[10px] tracking-wider text-crown-ash-dark uppercase"
            >From</span
          >
          <input
            type="date"
            bind:value={dateFrom}
            class="w-full rounded-md border border-plumage bg-obsidian px-3 py-2 font-mono text-xs text-cream focus:border-talon-gold focus:outline-none"
          />
        </label>

        <label class="block">
          <span
            class="mb-1 block font-mono text-[10px] tracking-wider text-crown-ash-dark uppercase"
            >To</span
          >
          <input
            type="date"
            bind:value={dateTo}
            class="w-full rounded-md border border-plumage bg-obsidian px-3 py-2 font-mono text-xs text-cream focus:border-talon-gold focus:outline-none"
          />
        </label>
      </div>

      <div class="mt-4 flex gap-2">
        <button
          onclick={applyFilters}
          class="cursor-pointer rounded-md bg-talon-gold/10 px-4 py-2 font-body text-xs font-medium text-talon-gold transition-colors hover:bg-talon-gold/20"
        >
          Apply
        </button>
        <button
          onclick={clearFilters}
          class="cursor-pointer rounded-md border border-plumage px-4 py-2 font-body text-xs text-crown-ash transition-colors hover:text-cream"
        >
          Clear
        </button>
      </div>
    </section>

    {#if loading}
      <div class="flex items-center justify-center py-16">
        <Loader2 class="size-6 animate-spin text-talon-gold" />
      </div>
    {:else if events.length === 0}
      <div
        class="flex flex-col items-center justify-center rounded-lg border border-plumage py-20 text-center"
      >
        <Download class="size-8 text-crown-ash-dark" />
        <p class="mt-4 font-heading text-xl text-cream">No events found</p>
        <p class="mt-1 font-body text-sm text-crown-ash">
          Try adjusting your filters.
        </p>
      </div>
    {:else}
      <div class="space-y-3">
        {#each events as event (event.eventId)}
          <article
            class="rounded-lg border border-plumage bg-obsidian transition-colors hover:border-talon-gold/40"
          >
            <div class="border-b border-plumage/50 px-4 py-3">
              <div class="flex flex-wrap items-center gap-2">
                <span
                  class="rounded-full bg-talon-gold/10 px-2 py-0.5 font-mono text-[10px] tracking-wider text-talon-gold uppercase"
                >
                  {eventTypeLabel(event.eventType)}
                </span>
                <span
                  class="rounded-full bg-plumage/50 px-2 py-0.5 font-mono text-[10px] text-crown-ash uppercase"
                >
                  {contextLabel(event.boundedContext)}
                </span>
                {#if event.decision}
                  <span
                    class="rounded-full bg-green-500/10 px-2 py-0.5 font-mono text-[10px] text-green-400 uppercase"
                  >
                    {event.decision}
                  </span>
                {/if}
              </div>

              <div
                class="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 font-mono text-[10px] text-crown-ash-dark"
              >
                <span>{formatTimestamp(event.timestamp)}</span>
                <span>Task: {shortId(event.taskId)}</span>
                <span>Trace: {shortId(event.traceId)}</span>
                <span>Event: {shortId(event.eventId)}</span>
              </div>
            </div>

            <div
              class="grid gap-4 px-4 py-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]"
            >
              <div>
                <p
                  class="mb-1 font-mono text-[10px] tracking-wider text-crown-ash-dark uppercase"
                >
                  Actor
                </p>
                <p class="font-body text-sm text-cream">
                  {event.actor.displayName}
                </p>
                <p class="font-mono text-[10px] text-crown-ash">
                  {event.actor.kind}
                  {#if event.actor.agentType}
                    · {event.actor.agentType}
                  {/if}
                  · {shortId(event.actor.id)}
                </p>
              </div>

              <div>
                <p
                  class="mb-1 font-mono text-[10px] tracking-wider text-crown-ash-dark uppercase"
                >
                  Payload Diff
                </p>
                <div class="space-y-1">
                  {#each event.payloadDiff as diff (diff.field)}
                    <div
                      class="rounded border border-plumage/40 bg-obsidian-light px-2 py-1 font-mono text-[10px]"
                    >
                      <span class="text-crown-ash">{diff.field}:</span>
                      {#if diff.before !== null}
                        <span class="text-red-400/80 line-through"
                          >{diff.before}</span
                        >
                        <span class="text-crown-ash-dark"> → </span>
                      {/if}
                      <span class="text-green-400/90">{diff.after}</span>
                    </div>
                  {/each}
                </div>
              </div>
            </div>
          </article>
        {/each}
      </div>

      <div
        class="mt-6 flex items-center justify-between rounded-lg border border-plumage px-4 py-3"
      >
        <span class="font-mono text-[10px] text-crown-ash">
          Page {currentPageIndex + 1} · {events.length} events
        </span>
        <div class="flex gap-2">
          <button
            onclick={goPrev}
            disabled={!canGoPrev}
            class="inline-flex cursor-pointer items-center gap-1 rounded-md border border-plumage px-3 py-1.5 font-body text-xs text-crown-ash transition-colors hover:text-cream disabled:cursor-not-allowed disabled:opacity-40"
          >
            <ChevronLeft class="size-3.5" />
            Previous
          </button>
          <button
            onclick={goNext}
            disabled={!canGoNext}
            class="inline-flex cursor-pointer items-center gap-1 rounded-md border border-plumage px-3 py-1.5 font-body text-xs text-crown-ash transition-colors hover:text-cream disabled:cursor-not-allowed disabled:opacity-40"
          >
            Next
            <ChevronRight class="size-3.5" />
          </button>
        </div>
      </div>
    {/if}
  </div>
</div>
