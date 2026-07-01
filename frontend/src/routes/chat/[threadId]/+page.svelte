<script lang="ts">
  import { browser } from "$app/environment";
  import { resolve } from "$app/paths";
  import { listArtifacts } from "$lib/artifacts/artifacts";
  import { getTenant } from "$lib/auth";
  import { locale, translate } from "$lib/i18n";
  import { loadThreadMessages } from "$lib/chat/client";
  import { watchThreadMessages } from "$lib/chat/watch";
  import type { ChatMessage } from "$lib/chat/types";
  import { buildThreadSections } from "$lib/plans/thread";
  import PlanDagMiniMap from "$lib/components/PlanDagMiniMap.svelte";
  import ConversationalWorkspace from "$lib/components/thread/ConversationalWorkspace.svelte";
  import ThreadComposer from "$lib/components/thread/ThreadComposer.svelte";
  import PlanProposalCard from "$lib/components/thread/PlanProposalCard.svelte";
  import HintBanner from "$lib/components/thread/HintBanner.svelte";
  import PlanThreadTopBar from "$lib/components/PlanThreadTopBar.svelte";
  import ScheduleDialog from "$lib/components/canvas/ScheduleDialog.svelte";
  import { computeRunCost, type ExecutorPriceLookup } from "$lib/plans/cost";
  import {
    PlanConfigurationStatus,
    type PlanConfiguration,
  } from "$lib/gen/harpia/plans/v1/plans_pb";
  import type { Artifact } from "$lib/gen/harpia/artifacts/v1/artifacts_pb";
  import {
    mapStepRowsToActivity,
    type PlanActivityItem,
  } from "$lib/plans/activity";
  import { loadPlanExecutionDetail } from "$lib/plans/plan-execution-detail";
  import { planClient, threadClient } from "$lib/rpc";
  import { Expand } from "lucide-svelte";

  let { data } = $props();

  // Path B: the chat route is thread-first. Message history, live watch, and the
  // composer are keyed by the thread id, while plan-scoped surfaces (config,
  // executions, artifacts, canvas links) remain keyed by the attached
  // configuration id during this foundation slice.
  const routeThreadId = $derived(data.threadId);
  const routeConfigurationId = $derived(data.configurationId);

  let messages = $state<ChatMessage[]>([]);
  let proposing = $state(false);
  let loadError = $state(false);
  let scheduleOpen = $state(false);
  // Server is the source of truth for configuration mutations performed by
  // the assistant. Use load-time data until an incoming chat message indicates
  // a mutation happened, then keep the refetched snapshot as an override.
  let liveConfigurationOverride = $state<PlanConfiguration | undefined>();
  const liveConfiguration = $derived(
    liveConfigurationOverride ?? data.configuration,
  );
  let workspaceArtifacts = $state<Artifact[]>([]);
  let workspaceActivityItems = $state<PlanActivityItem[]>([]);
  let workspaceLoadError = $state(false);

  $effect(() => {
    const configurationId = routeConfigurationId;
    liveConfigurationOverride = undefined;
    void configurationId;
  });

  const tenantId = $derived(getTenant()?.id ?? "");

  // Id of the most recent USER_TEXT that has no PLAN_PROPOSED after it, else
  // null. This is the message a proposal would answer.
  const unansweredUserMessageId = $derived.by<string | null>(() => {
    for (let i = messages.length - 1; i >= 0; i--) {
      if (messages[i].kind === "PLAN_PROPOSED") return null;
      if (messages[i].kind === "USER_TEXT") return messages[i].id;
    }
    return null;
  });

  // Guards against re-proposing for the same source message during the window
  // between proposePlan resolving and the PLAN_PROPOSED arriving via the watch
  // stream (which would otherwise fire a duplicate proposal + LLM call).
  let lastProposedSourceId = $state<string | null>(null);

  async function triggerProposal() {
    if (proposing || routeConfigurationId || !tenantId || !routeThreadId) return;
    const sourceId = unansweredUserMessageId;
    if (!sourceId || sourceId === lastProposedSourceId) return;
    proposing = true;
    lastProposedSourceId = sourceId;
    try {
      // Reads the thread's latest user message server-side and appends
      // PLAN_PROPOSED, which arrives back via the existing watch stream.
      await threadClient.proposePlan({ tenantId, threadId: routeThreadId });
    } catch {
      // Allow a retry (next send or effect run) if the proposal failed.
      lastProposedSourceId = null;
    } finally {
      proposing = false;
    }
  }

  // Auto-propose when a config-less thread has an unanswered opening message.
  $effect(() => {
    if (routeConfigurationId) return;
    if (!unansweredUserMessageId) return;
    void triggerProposal();
  });

  const pricing: ExecutorPriceLookup = (id) =>
    data.executorCatalog?.get(id) ?? null;
  const cost = $derived(
    data.template && liveConfiguration
      ? computeRunCost(data.template, liveConfiguration, pricing)
      : {
          totalPerRunBrl: 0,
          currency: "BRL" as const,
          unboundStepCount: 0,
          breakdown: [],
        },
  );
  const statusLabel = $derived(
    liveConfiguration?.status === PlanConfigurationStatus.RUNNABLE
      ? translate("plans.configure.status.runnable", $locale)
      : liveConfiguration?.status === PlanConfigurationStatus.SCHEDULED
        ? translate("plans.configure.status.scheduled", $locale)
        : translate("plans.configure.status.draft", $locale),
  );

  // Initial load via list-RPC, then live updates via watch-RPC with AbortController.
  $effect(() => {
    if (!tenantId || !routeThreadId) return;
    const controller = new AbortController();
    (async () => {
      try {
        // Initial historical load.
        const initial = await loadThreadMessages(tenantId, routeThreadId);
        if (controller.signal.aborted) return;
        messages = initial;
        // Live updates from the last known sequence.
        const sinceSeq =
          initial.length > 0 ? initial[initial.length - 1].sequenceNumber : 0n;
        for await (const batch of watchThreadMessages(tenantId, routeThreadId, {
          sinceSequenceNumber: sinceSeq,
          signal: controller.signal,
        })) {
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

  // Refetch the configuration when an incoming message indicates the
  // assistant (or schedule dialog) mutated it server-side. Debounced so
  // a batch of kinds arriving together collapses to one round-trip.
  const MUTATING_KINDS = new Set([
    "USER_SELECTION",
    "STEP_REBOUND",
    "SCHEDULE_SET",
    "CONFIGURATION_SAVED",
  ]);
  let refetchSeq = $state(0n);
  $effect(() => {
    // Find the highest-sequence mutating message in the stream and bump
    // refetchSeq to it. Scanning (not just looking at messages[last]) is
    // important because NextTurn appends an ASSISTANT_PROMPT right after
    // the USER_SELECTION it answers, so the LATEST kind is non-mutating
    // even though the previous one mutated the configuration.
    let maxSeq = refetchSeq;
    for (let i = messages.length - 1; i >= 0; i--) {
      const m = messages[i];
      if (m.sequenceNumber <= maxSeq) break; // older than already-seen
      if (MUTATING_KINDS.has(m.kind) && m.sequenceNumber > maxSeq) {
        maxSeq = m.sequenceNumber;
      }
    }
    if (maxSeq !== refetchSeq) refetchSeq = maxSeq;
  });
  $effect(() => {
    if (!tenantId || !routeConfigurationId || refetchSeq === 0n) return;
    const controller = new AbortController();
    const timer = setTimeout(async () => {
      try {
        const res = await planClient.getPlanConfiguration(
          { tenantId, planConfigurationId: routeConfigurationId },
          { signal: controller.signal },
        );
        if (res.planConfiguration)
          liveConfigurationOverride = res.planConfiguration;
      } catch {
        // best-effort; pill stays stale until the next refetch
      }
    }, 200);
    return () => {
      controller.abort();
      clearTimeout(timer);
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
  const planScopeMessages = $derived(
    sections.flatMap((section) =>
      section.kind === "plan-scope" ? [section.message] : [],
    ),
  );
  const latestExecutionSequence = $derived.by(() => {
    if (!mostRecentExecutionId) return 0n;
    let latest = 0n;
    for (const message of messages) {
      if (
        message.executionId === mostRecentExecutionId &&
        message.sequenceNumber > latest
      ) {
        latest = message.sequenceNumber;
      }
    }
    return latest;
  });

  $effect(() => {
    const executionId = mostRecentExecutionId;
    const executionSequence = latestExecutionSequence;
    void executionSequence;
    if (!tenantId || !executionId) {
      workspaceArtifacts = [];
      workspaceActivityItems = [];
      workspaceLoadError = false;
      return;
    }
    const controller = new AbortController();
    workspaceLoadError = false;
    (async () => {
      try {
        const [detail, artifacts] = await Promise.all([
          loadPlanExecutionDetail(tenantId, executionId),
          listArtifacts({ tenantId, planExecutionId: executionId }),
        ]);
        if (controller.signal.aborted) return;
        workspaceActivityItems = mapStepRowsToActivity(detail.rows);
        workspaceArtifacts = artifacts;
      } catch {
        if (controller.signal.aborted) return;
        workspaceLoadError = true;
      }
    })();
    return () => {
      controller.abort();
    };
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
  <title>{translate("thread.title", $locale)} · Harpia</title>
</svelte:head>

<div class="mx-auto flex max-w-7xl flex-col gap-3 px-4 py-6">
  {#if !routeConfigurationId}
    <div class="flex flex-col gap-3">
      {#if messages.length === 0}
        <div class="rounded border border-plumage bg-obsidian-light px-4 py-3 text-sm text-crown-ash">
          Tell Aiuna what you want to create, then a plan will appear here.
        </div>
      {/if}
      {#each messages as m (m.id)}
        {#if m.kind === "PLAN_PROPOSED"}
          <PlanProposalCard message={m} {tenantId} threadId={routeThreadId} />
        {:else if m.kind === "USER_TEXT"}
          <div
            class="max-w-[85%] self-end rounded-lg border border-plumage bg-obsidian-light px-3 py-2 text-[13px] whitespace-pre-wrap text-cream"
          >
            {m.text}
          </div>
        {/if}
      {/each}
      {#if proposing}
        <div class="text-[12px] text-crown-ash-dark">Thinking about a plan…</div>
      {/if}
      <ThreadComposer
        {tenantId}
        configurationId={routeThreadId}
        onSent={triggerProposal}
      />
    </div>
  {:else}
    {#if data.template && liveConfiguration}
      <PlanThreadTopBar
        planName={data.template.name}
        {statusLabel}
        {cost}
        onOpenSchedule={() => (scheduleOpen = true)}
      />
    {/if}

    {#if data.template?.steps && data.template.steps.length > 0}
      <div class="flex items-center justify-between gap-2">
        <PlanDagMiniMap
          steps={data.template.steps}
          edges={data.template.edges}
        />
        <a
          href={resolve(
            `/plans/configurations/${routeConfigurationId}/canvas`,
          )}
          class="flex items-center gap-1 rounded border border-plumage bg-transparent px-2 py-1 text-[10px] text-crown-ash hover:border-talon-gold hover:text-talon-gold"
          aria-label={translate("canvas.expandLink", $locale)}
        >
          <Expand class="size-3" />
          {translate("canvas.expandLink", $locale)}
        </a>
      </div>
    {/if}

    <HintBanner threadId={routeThreadId} />

    {#if workspaceLoadError}
      <p
        class="rounded border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-300"
      >
        {translate("artifacts.loadError", $locale)}
      </p>
    {/if}

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
      <ConversationalWorkspace
        {tenantId}
        configurationId={routeConfigurationId}
        messages={planScopeMessages}
        activityItems={workspaceActivityItems}
        artifacts={workspaceArtifacts}
      />
    {/if}

    <ThreadComposer {tenantId} configurationId={routeThreadId} />
  {/if}
</div>

{#if routeConfigurationId && liveConfiguration}
  <ScheduleDialog
    open={scheduleOpen}
    configuration={liveConfiguration}
    onClose={() => (scheduleOpen = false)}
  />
{/if}

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
