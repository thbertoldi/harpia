<script lang="ts">
  import { browser } from "$app/environment";
  import { invalidateAll, replaceState } from "$app/navigation";
  import { page } from "$app/stores";
  import { resolve } from "$app/paths";
  import { listArtifacts, getArtifact } from "$lib/artifacts/artifacts";
  import {
    primaryPreviewArtifact,
    resolvePreviewArtifact,
    shouldAutoOpenFinalArtifact,
  } from "$lib/artifacts/preview";
  import { getTenant } from "$lib/auth";
  import { locale, translate } from "$lib/i18n";
  import {
    loadThreadMessages,
    appendThreadMessage,
    mergeThreadMessages,
  } from "$lib/chat/client";
  import { watchThreadMessages } from "$lib/chat/watch";
  import type { ChatMessage } from "$lib/chat/types";
  import { buildThreadSections } from "$lib/plans/thread";
  import { buildExecutionViewModel } from "$lib/plans/execution-view";
  import { finalArtifactTypeKeys } from "$lib/plans/artifact-flow";
  import { PanelRightOpen } from "lucide-svelte";
  import PlanDagMiniMap from "$lib/components/PlanDagMiniMap.svelte";
  import ConversationalWorkspace from "$lib/components/thread/ConversationalWorkspace.svelte";
  import ThreadComposer from "$lib/components/thread/ThreadComposer.svelte";
  import PlanProposalCard from "$lib/components/thread/PlanProposalCard.svelte";
  import PlanExecutionCard from "$lib/components/thread/PlanExecutionCard.svelte";
  import { matchingConfigurationForProposal } from "$lib/components/thread/plan-proposal-logic";
  import ArtifactPreviewSheet from "$lib/components/artifacts/ArtifactPreviewSheet.svelte";
  import SuggestionChips from "$lib/components/SuggestionChips.svelte";
  import HintBanner from "$lib/components/thread/HintBanner.svelte";
  import PlanThreadTopBar from "$lib/components/PlanThreadTopBar.svelte";
  import ScheduleDialog from "$lib/components/canvas/ScheduleDialog.svelte";
  import { computeRunCost, type ExecutorPriceLookup } from "$lib/plans/cost";
  import {
    PlanConfigurationStatus,
    type PlanConfiguration,
    type PlanTemplate,
  } from "$lib/gen/harpia/plans/v1/plans_pb";
  import type { Artifact } from "$lib/gen/harpia/artifacts/v1/artifacts_pb";
  import { buildPlanSummary } from "$lib/plans/config-summary";
  import {
    localizedPlanName,
    localizedStepTitle,
  } from "$lib/plans/catalog-i18n";
  import { planClient, threadClient } from "$lib/rpc";
  import { chatEnter } from "$lib/motion/transitions";

  function chatEnterStaggered(node: HTMLElement, params: { delay?: number }) {
    return { ...chatEnter(node), delay: params.delay ?? 0 };
  }

  type ChatPageData = {
    threadId: string;
    configurationId: string;
    configuration?: PlanConfiguration;
    configurations: PlanConfiguration[];
    template?: PlanTemplate;
    templateByConfigurationId: Map<string, PlanTemplate>;
    executorCatalog?: Map<
      string,
      { displayName: string; pricePerRunBrl: number | null }
    >;
  };

  let { data }: { data: ChatPageData } = $props();

  // Path B: the chat route is thread-first. Message history, live watch, and the
  // composer are keyed by the thread id, while plan-scoped surfaces (config,
  // executions, artifacts, canvas links) are keyed by the selected plan
  // configuration for this thread.
  // routeThreadId is derived from the ROUTE PARAM, not load `data`, so that
  // invalidateAll() (which replaces `data` after mutations) can never re-key
  // or restart the message watch / composer mid-session. The load function
  // sets data.threadId = params.threadId, so this is behaviorally identical
  // but free of any data dependency.
  const routeThreadId = $derived($page.params.threadId ?? "");
  let selectedConfigurationId = $state("");
  const activeConfigurationId = $derived.by(() => {
    // The chat route is the canonical place for structural edits, and deep
    // links from the Runs panel (and elsewhere) carry a `?plan=` query param
    // to preselect the matching plan tab.
    const queryPlanId = $page.url.searchParams.get("plan");
    const candidateConfigurationId =
      selectedConfigurationId || queryPlanId || data.configurationId;
    if (
      candidateConfigurationId &&
      data.configurations.some(
        (config) => config.id === candidateConfigurationId,
      )
    ) {
      return candidateConfigurationId;
    }
    return data.configurationId;
  });

  let messages = $state<ChatMessage[]>([]);
  let proposing = $state(false);
  let loadError = $state(false);
  let scheduleOpen = $state(false);
  // Server is the source of truth for configuration mutations performed by
  // the assistant. Use load-time data until an incoming chat message indicates
  // a mutation happened, then keep the refetched snapshot as an override.
  let liveConfigurationOverride = $state<PlanConfiguration | undefined>();
  const focusedConfiguration = $derived.by(() => {
    if (liveConfigurationOverride?.id === activeConfigurationId) {
      return liveConfigurationOverride;
    }
    return (
      data.configurations.find(
        (config) => config.id === activeConfigurationId,
      ) ?? data.configuration
    );
  });
  // Mirrors liveConfigurationOverride: right after a config is created or
  // flipped to, data.templateByConfigurationId may not yet contain it (the
  // load hasn't repopulated after invalidateAll). Fetch the template directly
  // so PlanThreadTopBar renders immediately and the graph appears once the
  // steps arrive. See the lazy-fetch effect below.
  let liveTemplateOverride = $state<PlanTemplate | undefined>();
  const focusedTemplate = $derived(
    liveTemplateOverride ??
      data.templateByConfigurationId.get(activeConfigurationId) ??
      data.template,
  );
  // Maps a step_key to its human template title for STEP_STARTED / STEP_BOUND
  // system events. Threads down to SystemEventCard so the event text reads
  // "Step Write draft started." rather than the raw step id. Resolved via the
  // catalog content keys (catalog.plan.<key>.step.<stepKey>.title) so Brazilian
  // users see pt-BR titles; falls back to the stepKey when no template is loaded.
  const stepTitleFor = $derived((stepKey: string) =>
    focusedTemplate
      ? localizedStepTitle(focusedTemplate, stepKey, $locale)
      : stepKey,
  );
  let workspaceArtifacts = $state<Artifact[]>([]);
  let workspaceLoadError = $state(false);

  // Canonical artifact preview state — single source of truth for BOTH the
  // auto-surfaced final artifact and any manually-opened artifact (final-
  // artifact button, inline STEP_BOUND cards). Intermediate
  // artifacts are never auto-promoted here (spec: artifact-side-preview) —
  // only opened explicitly by the user.
  let activeArtifactId = $state<string | null>(null);
  let previewOpen = $state(false);
  let previewArtifactLoading = $state(false);
  // Populated on demand when the id isn't in workspaceArtifacts (e.g. an
  // artifact from an older execution in this thread).
  let previewFallbackArtifact = $state<Artifact | null>(null);
  // Dismissal memory scoped ONLY to the final-artifact auto-open flow below —
  // never touched by manually opening/closing an unrelated artifact.
  let previewDismissedFinalArtifactId = $state<string | null>(null);

  const finalArtifactTypeKeySet = $derived(
    focusedTemplate
      ? finalArtifactTypeKeys(focusedTemplate.steps, focusedTemplate.edges)
      : new Set<string>(),
  );
  // Prefer the human-readable generated draft over a terminal publish receipt.
  // In the LinkedIn plan the PublishConfirmation is useful metadata, but the
  // artifact users need to review is the LinkedInPostDraft.
  const finalArtifact = $derived(
    primaryPreviewArtifact(workspaceArtifacts, finalArtifactTypeKeySet),
  );

  const previewArtifact = $derived(
    resolvePreviewArtifact(
      activeArtifactId,
      workspaceArtifacts,
      previewFallbackArtifact,
    ),
  );

  // The one callback threaded down through ConversationalWorkspace to inline
  // STEP_BOUND artifact cards.
  function openArtifact(artifactId: string) {
    activeArtifactId = artifactId;
    previewOpen = true;
    previewFallbackArtifact = null;
    previewArtifactLoading = false;
    if (workspaceArtifacts.some((a) => a.id === artifactId)) return;
    previewArtifactLoading = true;
    void (async () => {
      try {
        const artifact = await getArtifact(tenantId, artifactId);
        if (activeArtifactId === artifactId) previewFallbackArtifact = artifact;
      } finally {
        if (activeArtifactId === artifactId) previewArtifactLoading = false;
      }
    })();
  }

  function closePreview() {
    // Only feed the dismissal memory when the thing being closed IS the final
    // artifact — closing a manually-opened, unrelated artifact must never
    // suppress the next final-artifact auto-open.
    if (
      previewArtifact &&
      finalArtifact &&
      previewArtifact.id === finalArtifact.id
    ) {
      previewDismissedFinalArtifactId = finalArtifact.id;
    }
    previewOpen = false;
    previewArtifactLoading = false;
  }

  // Re-open the panel automatically when a new final artifact appears (e.g. a
  // fresh run completes). Stays dismissed for the artifact the user closed.
  $effect(() => {
    const artifactId = finalArtifact?.id ?? null;
    if (
      shouldAutoOpenFinalArtifact(artifactId, previewDismissedFinalArtifactId)
    ) {
      activeArtifactId = artifactId;
      previewFallbackArtifact = null;
      previewArtifactLoading = false;
      previewOpen = true;
    }
  });

  const tenantId = $derived(getTenant()?.id ?? "");

  // Looks up the PlanConfiguration that already exists for a PLAN_PROPOSED
  // message, if any. Computed against the server-supplied data.configurations
  // (newest-first) so PlanProposalCard renders read-only instead of re-offering
  // confirm/adjust on reload.
  function existingConfigurationFor(
    message: ChatMessage,
  ): PlanConfiguration | undefined {
    let candidateIds: { template_id: string }[] = [];
    try {
      const parsed = JSON.parse(message.payloadJson || "{}") as {
        candidates?: unknown;
      };
      const rawCandidates = Array.isArray(parsed.candidates)
        ? parsed.candidates
        : [];
      candidateIds = rawCandidates
        .map((candidate: unknown) => {
          if (
            typeof candidate === "object" &&
            candidate !== null &&
            "template_id" in candidate &&
            typeof candidate.template_id === "string"
          ) {
            return { template_id: candidate.template_id };
          }
          return null;
        })
        .filter(
          (candidate): candidate is { template_id: string } =>
            candidate !== null,
        );
    } catch {
      // malformed payload — treat as no candidates
    }
    return (
      matchingConfigurationForProposal(candidateIds, data.configurations) ??
      undefined
    );
  }

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
    if (proposing || !tenantId || !routeThreadId) return;
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
    if (activeConfigurationId) return;
    if (!unansweredUserMessageId) return;
    void triggerProposal();
  });

  // An empty config-less thread is actionable: a suggestion chip seeds the
  // opening user message on the current thread (same path the composer uses),
  // then the auto-propose effect fires once it arrives.
  let openingPromptError = $state<string | null>(null);

  async function sendOpeningPrompt(prompt: string) {
    const trimmed = prompt.trim();
    if (!tenantId || !routeThreadId || trimmed.length === 0) return;
    openingPromptError = null;
    try {
      await appendThreadMessage(
        tenantId,
        routeThreadId,
        "OVERSEER",
        "USER_TEXT",
        trimmed,
      );
    } catch {
      // The composer remains available for manual entry either way, but the
      // user needs to know the chip's message didn't actually go through.
      openingPromptError = translate("thread.propose.sendError", $locale);
    }
  }

  // Execute the configured plan from within the chat. The PlanExecutionCard
  // and the artifact grid pick up the run as RUN_*/STEP_* events stream in.
  let runError = $state<string | null>(null);
  let runErrorNeedsIntegration = $state(false);

  function clearFocusedPlanUi() {
    scheduleOpen = false;
    runError = null;
    runErrorNeedsIntegration = false;
    workspaceArtifacts = [];
    workspaceLoadError = false;
    previewOpen = false;
    activeArtifactId = null;
    previewFallbackArtifact = null;
    previewArtifactLoading = false;
    previewDismissedFinalArtifactId = null;
  }

  function selectConfiguration(configurationId: string) {
    selectedConfigurationId = configurationId;
    liveConfigurationOverride = undefined;
    liveTemplateOverride = undefined;
    clearFocusedPlanUi();
    // Keep the URL's ?plan= param in sync with the selected chip so a reload
    // or bookmark lands back on the plan the user actually switched to,
    // instead of whichever plan the page was first opened with.
    if (browser) {
      replaceState(
        resolve(
          `/chat/[threadId]?plan=${encodeURIComponent(configurationId)}`,
          {
            threadId: routeThreadId,
          },
        ),
        {},
      );
    }
  }

  function shortConfigurationId(id: string) {
    return id.length <= 8 ? id : id.slice(0, 8);
  }

  function statusTextForConfiguration(configuration?: PlanConfiguration) {
    return configuration?.status === PlanConfigurationStatus.RUNNABLE
      ? translate("plans.configure.status.runnable", $locale)
      : configuration?.status === PlanConfigurationStatus.SCHEDULED
        ? translate("plans.configure.status.scheduled", $locale)
        : translate("plans.configure.status.draft", $locale);
  }

  async function startRun() {
    if (!tenantId || !activeConfigurationId || anyExecutionRunning) return;
    runError = null;
    runErrorNeedsIntegration = false;
    try {
      await planClient.createPlanExecution({
        tenantId,
        planConfigurationId: activeConfigurationId,
      });
      await invalidateAll();
      requestAnimationFrame(() =>
        window.scrollTo(0, document.body.scrollHeight),
      );
    } catch (e) {
      const raw = e instanceof Error ? e.message : String(e);
      if (/requires enabled executor installation/i.test(raw)) {
        runErrorNeedsIntegration = true;
        runError = translate("thread.run.missingExecutor", $locale, {
          detail: raw,
          sku: raw.match(/sku "([^"]+)"/)?.[1] ?? "",
        });
      } else {
        runError = raw;
      }
    }
  }

  const pricing: ExecutorPriceLookup = (id) =>
    data.executorCatalog?.get(id) ?? null;
  const cost = $derived(
    focusedTemplate && focusedConfiguration
      ? computeRunCost(focusedTemplate, focusedConfiguration, pricing)
      : {
          totalPerRunBrl: 0,
          currency: "BRL" as const,
          unboundStepCount: 0,
          breakdown: [],
        },
  );
  const statusLabel = $derived(
    statusTextForConfiguration(focusedConfiguration),
  );
  const planSummary = $derived(
    focusedTemplate && focusedConfiguration
      ? buildPlanSummary(focusedTemplate, focusedConfiguration)
      : null,
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
          // mergeThreadMessages dedupes by id and re-sorts by sequenceNumber,
          // hardening against stream replay dupes.
          messages = mergeThreadMessages(messages, batch);
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

  // Fetch-on-flip: when the thread transitions from no-config (proposal flow)
  // to configured (e.g. after "Finish setup" calls nextTurn + invalidateAll),
  // the seeded ASSISTANT_PROMPT may not have arrived over the watch stream
  // yet. Refetch the full history and merge it in so the configured branch
  // (ConversationalBindingCard etc.) has something to render immediately.
  // The existing watch stream is NOT reset or aborted; this only supplements
  // it. Guarded per-thread so it fires once per flip, not on every tick.
  let flippedFetchThread = $state("");
  $effect(() => {
    const configId = activeConfigurationId;
    const threadId = routeThreadId;
    if (!configId) {
      // Re-arm so a later flip (e.g. proposal -> configure) re-fires.
      if (flippedFetchThread !== "") flippedFetchThread = "";
      return;
    }
    if (!tenantId || !threadId) return;
    if (flippedFetchThread === threadId) return;
    flippedFetchThread = threadId;
    void (async () => {
      try {
        const fresh = await loadThreadMessages(tenantId, threadId);
        messages = mergeThreadMessages(messages, fresh);
      } catch {
        // best-effort; the watch stream still delivers in due course
      }
    })();
  });

  // Lazy template fetch fallback: when focusedConfiguration exists but its
  // template isn't in data.templateByConfigurationId (typical right after a
  // config is created or flipped to, before invalidateAll repopulates data),
  // fetch it directly so PlanThreadTopBar renders immediately and the graph
  // (PlanDagMiniMap) appears once steps arrive. If the fetch fails, the top
  // bar still renders with the fallback name and the graph stays hidden.
  $effect(() => {
    const configId = activeConfigurationId;
    const templateId = focusedConfiguration?.planTemplateId;
    if (!configId || !templateId) {
      liveTemplateOverride = undefined;
      return;
    }
    // data already has the template — clear any stale override and rely on it
    // so we don't shadow server data once the load catches up.
    if (data.templateByConfigurationId.get(configId)) {
      liveTemplateOverride = undefined;
      return;
    }
    // Clear any override left over from a previously focused config so its
    // steps don't briefly render for this one, then fetch this config's.
    liveTemplateOverride = undefined;
    const controller = new AbortController();
    void (async () => {
      try {
        const res = await planClient.getPlanTemplate(
          { planTemplateId: templateId },
          { signal: controller.signal },
        );
        // Guard against a resolve racing with a config switch (the cleanup
        // aborts the controller); otherwise a stale template from the previous
        // config could overwrite this one's.
        if (!controller.signal.aborted && res.planTemplate)
          liveTemplateOverride = res.planTemplate;
      } catch {
        // best-effort; top bar still renders with the fallback name
      }
    })();
    return () => controller.abort();
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
    if (!tenantId || !activeConfigurationId || refetchSeq === 0n) return;
    const controller = new AbortController();
    const timer = setTimeout(async () => {
      try {
        const res = await planClient.getPlanConfiguration(
          { tenantId, planConfigurationId: activeConfigurationId },
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

  // ADR-016 / live-execution-chat — ordered template steps feed the execution
  // view model; execution sections render as live progress cards. Step titles
  // are resolved through the catalog content keys so the timeline renders in
  // the active locale.
  const orderedSteps = $derived(
    focusedTemplate
      ? focusedTemplate.steps.map((s) => ({
          key: s.key,
          title: localizedStepTitle(focusedTemplate, s.key, $locale),
        }))
      : [],
  );
  const executionGroups = $derived(
    sections.flatMap((section) =>
      section.kind === "execution" ? [section.group] : [],
    ),
  );
  const executionViewModels = $derived(
    executionGroups.map((group) =>
      buildExecutionViewModel(group, orderedSteps),
    ),
  );
  const anyExecutionRunning = $derived(
    executionViewModels.some((vm) => vm.state === "running"),
  );

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
      workspaceLoadError = false;
      return;
    }
    const controller = new AbortController();
    workspaceLoadError = false;
    (async () => {
      try {
        const artifacts = await listArtifacts({
          tenantId,
          planExecutionId: executionId,
        });
        if (controller.signal.aborted) return;
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

  // On resume (no deep-link hash), land at the latest message instead of the
  // top of the thread. Re-arms per thread so switching threads re-scrolls.
  let scrolledForThread = "";
  $effect(() => {
    void routeThreadId;
    void messages.length;
    if (!browser) return;
    if (scrolledForThread === routeThreadId) return;
    if (window.location.hash.startsWith("#m-")) return;
    if (messages.length === 0) return;
    requestAnimationFrame(() => {
      window.scrollTo(0, document.body.scrollHeight);
      scrolledForThread = routeThreadId;
    });
  });
</script>

<svelte:head>
  <title>{translate("thread.title", $locale)} · Harpia</title>
</svelte:head>

<div class="mx-auto flex max-w-7xl flex-col gap-3 px-4 py-6">
  {#if !activeConfigurationId}
    <div class="flex flex-col gap-3">
      {#if messages.length === 0}
        <SuggestionChips onSelect={(p) => void sendOpeningPrompt(p)} />
      {:else}
        <div
          class="rounded border border-plumage bg-obsidian-light px-4 py-3 text-sm text-crown-ash"
        >
          {translate("thread.propose.homeHint", $locale)}
        </div>
      {/if}
      {#each messages as m, i (m.id)}
        {#if m.kind === "PLAN_PROPOSED"}
          <div in:chatEnterStaggered={{ delay: Math.min(i * 40, 200) }}>
            <PlanProposalCard
              message={m}
              {tenantId}
              threadId={routeThreadId}
              existingConfiguration={existingConfigurationFor(m)}
            />
          </div>
        {:else if m.kind === "USER_TEXT"}
          <div
            in:chatEnterStaggered={{ delay: Math.min(i * 40, 200) }}
            class="max-w-[85%] self-end rounded-2xl rounded-tr-sm border border-talon-gold/40 bg-talon-gold/10 px-3 py-2 text-[13px] whitespace-pre-wrap text-cream"
          >
            {m.text}
          </div>
        {/if}
      {/each}
      {#if proposing}
        <div
          class="flex items-center gap-2 text-[12px] text-crown-ash-dark"
          aria-live="polite"
        >
          <span class="typing-dots flex items-center gap-1">
            <span class="typing-dot"></span>
            <span class="typing-dot"></span>
            <span class="typing-dot"></span>
          </span>
          <span>{translate("thread.propose.thinking", $locale)}</span>
        </div>
      {/if}
      {#if openingPromptError}
        <p
          class="rounded border border-danger/40 bg-danger/10 px-3 py-2 text-[12px] text-danger"
        >
          {openingPromptError}
        </p>
      {/if}
      <ThreadComposer
        {tenantId}
        configurationId={routeThreadId}
        onSent={triggerProposal}
      />
    </div>
  {:else}
    <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:gap-4">
      <section class="flex min-w-0 flex-1 flex-col gap-3">
        {#if data.configurations.length > 1}
          <div
            class="flex flex-wrap gap-2 rounded border border-plumage/60 bg-obsidian-light/30 p-2"
          >
            {#each data.configurations as configuration (configuration.id)}
              {@const template = data.templateByConfigurationId.get(
                configuration.id,
              )}
              {@const selected = configuration.id === activeConfigurationId}
              <button
                type="button"
                class="rounded border px-3 py-2 text-left {selected
                  ? 'border-talon-gold/60 bg-talon-gold/10 text-cream'
                  : 'border-plumage/60 bg-obsidian text-crown-ash hover:border-talon-gold/40 hover:text-cream'}"
                aria-current={selected ? "true" : undefined}
                onclick={() => selectConfiguration(configuration.id)}
              >
                <span class="block text-[12px] font-medium">
                  {template
                    ? localizedPlanName(template, $locale)
                    : shortConfigurationId(configuration.id)}
                </span>
                <span class="mt-0.5 block text-[10px] text-crown-ash-dark">
                  {statusTextForConfiguration(configuration)} · {shortConfigurationId(
                    configuration.id,
                  )}
                </span>
              </button>
            {/each}
          </div>
        {/if}

        {#if focusedConfiguration}
          <PlanThreadTopBar
            planName={(planSummary?.intent &&
            planSummary.intent !== focusedTemplate?.name
              ? planSummary.intent
              : focusedTemplate
                ? localizedPlanName(focusedTemplate, $locale)
                : "") || shortConfigurationId(focusedConfiguration.id)}
            {statusLabel}
            {cost}
            onOpenSchedule={() => (scheduleOpen = true)}
            canRun={focusedConfiguration.status ===
              PlanConfigurationStatus.RUNNABLE}
            running={anyExecutionRunning}
            onRun={startRun}
          />
        {/if}

        {#if runError}
          <div
            class="flex flex-wrap items-center gap-2 rounded border border-danger/40 bg-danger/10 px-3 py-2 text-[12px] text-danger"
          >
            <span class="flex-1">{runError}</span>
            {#if runErrorNeedsIntegration}
              <a
                href={resolve("/admin/integrations")}
                class="shrink-0 underline-offset-2 hover:underline"
              >
                {translate("nav.integrations", $locale)}
              </a>
            {/if}
          </div>
        {/if}

        <div class="flex flex-wrap items-center gap-1.5 text-[11px]">
          <span
            class="h-1.5 w-1.5 rounded-full
              {anyExecutionRunning
              ? 'animate-pulse bg-energy'
              : 'bg-status-done/70'}"
          ></span>
          <span class={anyExecutionRunning ? "text-energy" : "text-crown-ash"}>
            {anyExecutionRunning
              ? translate("thread.status.executing", $locale)
              : translate("thread.status.ready", $locale)}
          </span>
          {#if finalArtifact && !(previewOpen && previewArtifact?.id === finalArtifact.id)}
            <button
              type="button"
              class="ml-auto inline-flex items-center gap-1.5 rounded-md border border-plumage px-2.5 py-1 text-[11px] text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
              onclick={() => openArtifact(finalArtifact.id)}
            >
              <PanelRightOpen class="size-3.5" />
              {translate("thread.sidePreview.open", $locale)}
            </button>
          {/if}
        </div>

        {#if focusedTemplate?.steps && focusedTemplate.steps.length > 0}
          <div
            class="rounded border border-plumage/60 bg-obsidian-light/30 px-3 py-2"
          >
            <PlanDagMiniMap
              steps={focusedTemplate.steps}
              edges={focusedTemplate.edges}
              template={focusedTemplate}
            />
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
            configurationId={activeConfigurationId}
            messages={planScopeMessages}
            artifacts={workspaceArtifacts}
            onOpenArtifact={openArtifact}
            {existingConfigurationFor}
            {stepTitleFor}
          />
        {/if}

        {#if executionViewModels.length > 0}
          <div class="flex flex-col gap-2">
            {#each executionViewModels as vm (vm.executionId)}
              <div in:chatEnterStaggered={{ delay: 0 }}>
                <PlanExecutionCard
                  {vm}
                  initiallyCollapsed
                  {tenantId}
                  onOpenArtifact={openArtifact}
                  onApprovalDecided={() => void invalidateAll()}
                />
              </div>
            {/each}
          </div>
        {/if}

        <ThreadComposer
          {tenantId}
          configurationId={routeThreadId}
          onSent={triggerProposal}
        />
      </section>

      <ArtifactPreviewSheet
        open={previewOpen}
        artifact={previewArtifact}
        artifactLoading={previewArtifactLoading}
        {tenantId}
        onClose={closePreview}
      />
    </div>
  {/if}
</div>

{#if activeConfigurationId && focusedConfiguration}
  <ScheduleDialog
    open={scheduleOpen}
    configuration={focusedConfiguration}
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
  .typing-dot {
    width: 4px;
    height: 4px;
    border-radius: 9999px;
    background-color: var(--token-energy);
    opacity: 0.4;
    animation: typing-bounce 1.1s ease-in-out infinite;
  }
  .typing-dot:nth-child(2) {
    animation-delay: 0.15s;
  }
  .typing-dot:nth-child(3) {
    animation-delay: 0.3s;
  }
  @keyframes typing-bounce {
    0%,
    60%,
    100% {
      transform: translateY(0);
      opacity: 0.4;
    }
    30% {
      transform: translateY(-3px);
      opacity: 1;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .typing-dot {
      animation: none;
      opacity: 0.7;
    }
  }
</style>
