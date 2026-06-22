<script lang="ts">
  import { browser } from "$app/environment";
  import { getTenant } from "$lib/auth";
  import { locale, translate } from "$lib/i18n";
  import { loadThreadMessages } from "$lib/chat/client";
  import { watchThreadMessages } from "$lib/chat/watch";
  import type { ChatMessage } from "$lib/chat/types";
  import { buildThreadSections } from "$lib/plans/thread";
  import PlanDagMiniMap from "$lib/components/PlanDagMiniMap.svelte";
  import ThreadMessage from "$lib/components/thread/ThreadMessage.svelte";
  import ExecutionSection from "$lib/components/thread/ExecutionSection.svelte";
  import ThreadComposer from "$lib/components/thread/ThreadComposer.svelte";
  import HintBanner from "$lib/components/thread/HintBanner.svelte";

  let { data } = $props();

  let messages = $state<ChatMessage[]>([]);
  let loadError = $state(false);

  const tenantId = $derived(getTenant()?.id ?? "");

  // Initial load via list-RPC, then live updates via watch-RPC with AbortController.
  $effect(() => {
    if (!tenantId || !data.configurationId) return;
    const controller = new AbortController();
    (async () => {
      try {
        // Initial historical load.
        const initial = await loadThreadMessages(
          tenantId,
          data.configurationId,
        );
        if (controller.signal.aborted) return;
        messages = initial;
        // Live updates from the last known sequence.
        const sinceSeq =
          initial.length > 0 ? initial[initial.length - 1].sequenceNumber : 0n;
        for await (const batch of watchThreadMessages(
          tenantId,
          data.configurationId,
          { sinceSequenceNumber: sinceSeq, signal: controller.signal },
        )) {
          if (controller.signal.aborted) return;
          messages = [...messages, ...batch];
        }
      } catch {
        if (controller.signal.aborted) return;
        loadError = true;
      }
    })();
    return () => {
      controller.abort();
    };
  });

  const sections = $derived(buildThreadSections(messages));
  const mostRecentExecutionId = $derived.by(() => {
    for (let i = sections.length - 1; i >= 0; i--) {
      const s = sections[i];
      if (s.kind === "execution") return s.group.executionId;
    }
    return null;
  });

  // Deep-link anchor: handle three formats and scroll once when target arrives:
  //   #m-<messageId>                — direct chat_message id
  //   #m-elicitation-<elicitationId> — resolve to ELICITATION_RAISED message
  //   #m-approval-<approvalId>       — resolve to APPROVAL_RAISED message
  // The effect re-runs as messages arrive (Svelte tracks the `messages` read).
  // A flag prevents scrolling more than once.
  let didScrollToAnchor = $state(false);
  $effect(() => {
    if (!browser || didScrollToAnchor) return;
    const hash = window.location.hash;
    if (!hash.startsWith("#m-")) return;
    let targetId: string | null = null;
    if (hash.startsWith("#m-elicitation-")) {
      const elicitId = hash.slice("#m-elicitation-".length);
      const match = messages.find((m) => {
        if (m.kind !== "ELICITATION_RAISED") return false;
        try {
          return JSON.parse(m.payloadJson)?.elicitation_id === elicitId;
        } catch {
          return false;
        }
      });
      targetId = match ? `m-${match.id}` : null;
    } else if (hash.startsWith("#m-approval-")) {
      const approvalId = hash.slice("#m-approval-".length);
      const match = messages.find((m) => {
        if (m.kind !== "APPROVAL_RAISED") return false;
        try {
          return JSON.parse(m.payloadJson)?.approval_request_id === approvalId;
        } catch {
          return false;
        }
      });
      targetId = match ? `m-${match.id}` : null;
    } else {
      targetId = hash.slice(1);
    }
    if (!targetId) return; // target message hasn't arrived yet — wait for next reactive update
    requestAnimationFrame(() => {
      const el = document.getElementById(targetId!);
      if (el) {
        el.scrollIntoView({ behavior: "smooth", block: "center" });
        el.classList.add("harpia-pulse-anchor");
        setTimeout(() => el.classList.remove("harpia-pulse-anchor"), 1500);
        didScrollToAnchor = true;
      }
    });
  });
</script>

<svelte:head>
  <title>
    {data.configuration?.workspaceId
      ? "Plan thread"
      : translate("thread.title", $locale)} · Harpia
  </title>
</svelte:head>

<div class="mx-auto flex max-w-3xl flex-col gap-3 px-4 py-6">
  {#if data.template?.steps && data.template.steps.length > 0}
    <PlanDagMiniMap steps={data.template.steps} edges={data.template.edges} />
  {/if}

  <HintBanner threadId={data.configurationId} />

  {#if loadError}
    <p
      class="rounded border border-plumage bg-obsidian-light px-4 py-3 text-sm text-crown-ash"
    >
      {translate("thread.loadError", $locale)}
    </p>
  {:else if sections.length === 0}
    <p
      class="rounded border border-plumage bg-obsidian-light px-4 py-3 text-sm text-crown-ash"
    >
      {translate("thread.empty", $locale)}
    </p>
  {:else}
    <div class="flex flex-col gap-2">
      {#each sections as section (section.kind === "execution" ? section.group.executionId : section.message.id)}
        {#if section.kind === "plan-scope"}
          <ThreadMessage message={section.message} />
        {:else}
          <ExecutionSection
            group={section.group}
            defaultExpanded={section.group.executionId ===
              mostRecentExecutionId}
          />
        {/if}
      {/each}
    </div>
  {/if}

  <ThreadComposer {tenantId} configurationId={data.configurationId} />
</div>

<style>
  :global(.harpia-pulse-anchor) {
    animation: harpia-pulse 1.5s ease-out;
  }
  @keyframes harpia-pulse {
    0% {
      box-shadow: 0 0 0 0 rgba(212, 175, 55, 0.7);
    }
    100% {
      box-shadow: 0 0 0 8px rgba(212, 175, 55, 0);
    }
  }
</style>
