<script lang="ts">
  import { AlertTriangle, Bot, Loader2, Shield } from "lucide-svelte";
  import { loadAgentCatalog, type AgentCatalogEntry } from "$lib/agent-catalog";
  import AgentCatalogDetail from "$lib/components/AgentCatalogDetail.svelte";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";

  let entries = $state<AgentCatalogEntry[]>([]);
  let loading = $state(true);
  let loadError = $state<string | null>(null);
  let dataSource = $state<"api" | "mock">("mock");
  let selectedEntry = $state<AgentCatalogEntry | null>(null);
  let toast = $state<string | null>(null);
  let toastTimer: ReturnType<typeof setTimeout> | null = null;

  $effect(() => {
    void fetchCatalog();
  });

  async function fetchCatalog() {
    loading = true;
    loadError = null;
    try {
      const result = await loadAgentCatalog();
      entries = result.entries;
      dataSource = result.source;
      if (result.error && result.source === "mock") {
        loadError = result.error;
      }
    } catch (e) {
      loadError = e instanceof Error ? e.message : "Failed to load agents";
    } finally {
      loading = false;
    }
  }

  function selectEntry(entry: AgentCatalogEntry) {
    selectedEntry = entry;
  }

  function deselectEntry() {
    selectedEntry = null;
  }

  function showToast(message: string) {
    toast = message;
    if (toastTimer) clearTimeout(toastTimer);
    toastTimer = setTimeout(() => {
      toast = null;
    }, 4000);
  }

  function handleAction(_action: string, message: string) {
    showToast(message);
  }

  function formattedDate(dateStr: string): string {
    try {
      return new Date(dateStr).toLocaleDateString("en-US", {
        month: "short",
        day: "numeric",
        year: "numeric",
      });
    } catch {
      return dateStr;
    }
  }

  function trustScoreClass(score: number): string {
    if (score >= 90) return "text-green-400";
    if (score >= 75) return "text-talon-gold";
    return "text-red-400";
  }

  function visibilityLabel(entry: AgentCatalogEntry): string {
    if (entry.tenantVisibility === "global") return "All tenants";
    const count = entry.visibleTenantIds?.length ?? 0;
    return `${count} tenant${count === 1 ? "" : "s"}`;
  }
</script>

<div class="flex h-[calc(100vh-3.5rem)]">
  <div
    class="flex w-full flex-col transition-all duration-300 {selectedEntry
      ? 'lg:w-[45%]'
      : 'lg:w-full'}"
  >
    <div class="flex items-center justify-between px-4 py-3 lg:px-6">
      <div>
        <HarpyHeading tag="h1" class="text-2xl text-cream"
          >Agent Catalog</HarpyHeading
        >
        <p class="mt-1 font-body text-sm text-crown-ash">
          Browse registered agent types, versions, and trust scores.
        </p>
      </div>
      <span
        class="rounded-full border border-plumage px-2.5 py-1 font-mono text-[10px] tracking-wider text-crown-ash uppercase"
      >
        {dataSource === "api" ? "Live API" : "Mock data"}
      </span>
    </div>

    {#if loading}
      <div class="flex flex-1 items-center justify-center">
        <div class="flex items-center gap-2 text-crown-ash">
          <Loader2 class="size-5 animate-spin" />
          <span class="font-body text-sm">Loading agents...</span>
        </div>
      </div>
    {:else if entries.length === 0 && loadError}
      <div class="flex flex-1 items-center justify-center">
        <div class="text-center">
          <AlertTriangle class="mx-auto mb-3 size-10 text-red-400" />
          <p class="font-body text-sm text-red-400">Failed to load agents</p>
          <p class="mt-1 font-mono text-xs text-crown-ash">{loadError}</p>
          <button
            onclick={() => fetchCatalog()}
            class="mt-4 cursor-pointer rounded-md border border-plumage px-4 py-2 font-body text-sm text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
          >
            Retry
          </button>
        </div>
      </div>
    {:else if entries.length === 0}
      <div class="flex flex-1 flex-col items-center justify-center px-4">
        <Bot class="mb-4 size-12 text-talon-gold" />
        <HarpyHeading tag="h2" class="mb-2 text-center text-xl text-cream">
          No agent types registered
        </HarpyHeading>
        <p class="max-w-md text-center font-body text-sm text-crown-ash">
          Register agent manifests to populate the catalog.
        </p>
      </div>
    {:else}
      {#if loadError}
        <div
          class="mx-4 mb-2 rounded-md border border-talon-gold/30 bg-talon-gold/5 px-3 py-2 lg:mx-6"
        >
          <p class="font-mono text-xs text-talon-gold">
            API unavailable — showing mock catalog. {loadError}
          </p>
        </div>
      {/if}

      <div class="flex-1 overflow-x-auto px-4 pb-4 lg:px-6">
        <table class="w-full min-w-[720px] border-collapse">
          <thead>
            <tr
              class="border-b border-plumage font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase"
            >
              <th class="px-3 py-2 text-left">Agent type</th>
              <th class="px-3 py-2 text-left">Active</th>
              <th class="px-3 py-2 text-left">Canary</th>
              <th class="px-3 py-2 text-left">Trust</th>
              <th class="px-3 py-2 text-left">Visibility</th>
              <th class="px-3 py-2 text-left">Updated</th>
            </tr>
          </thead>
          <tbody>
            {#each entries as entry (entry.agentType.id)}
              <tr
                onclick={() => selectEntry(entry)}
                class="cursor-pointer border-b border-plumage/40 transition-colors hover:bg-obsidian-light/60 {selectedEntry
                  ?.agentType.id === entry.agentType.id
                  ? 'bg-obsidian-light'
                  : ''}"
              >
                <td class="px-3 py-3">
                  <p class="font-heading text-sm font-semibold text-cream">
                    {entry.agentType.displayName || entry.agentType.name}
                  </p>
                  <p class="font-mono text-[10px] text-crown-ash-dark">
                    {entry.agentType.id}
                  </p>
                </td>
                <td class="px-3 py-3 font-mono text-xs text-cream">
                  {entry.activeVersion}
                </td>
                <td class="px-3 py-3 font-mono text-xs text-crown-ash">
                  {entry.canaryVersion ?? "—"}
                </td>
                <td class="px-3 py-3">
                  <span
                    class="inline-flex items-center gap-1 font-mono text-xs {trustScoreClass(
                      entry.trustScore,
                    )}"
                  >
                    <Shield class="size-3" />
                    {entry.trustScore}
                  </span>
                </td>
                <td class="px-3 py-3 font-body text-xs text-crown-ash">
                  {visibilityLabel(entry)}
                </td>
                <td class="px-3 py-3 font-mono text-[10px] text-crown-ash-dark">
                  {formattedDate(entry.lastUpdated)}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>

  {#if selectedEntry}
    <div
      class="hidden w-[55%] border-l border-plumage bg-obsidian transition-all duration-300 lg:block"
    >
      <AgentCatalogDetail
        entry={selectedEntry}
        onclose={deselectEntry}
        onAction={handleAction}
      />
    </div>
  {/if}
</div>

{#if toast}
  <div
    class="fixed right-6 bottom-6 z-50 max-w-sm rounded-lg border border-plumage bg-obsidian-light px-4 py-3 shadow-lg"
    role="status"
  >
    <p class="font-body text-sm text-cream">{toast}</p>
  </div>
{/if}
