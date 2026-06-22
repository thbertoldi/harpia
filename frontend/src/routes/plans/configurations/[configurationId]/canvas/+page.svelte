<script lang="ts">
  import { goto } from "$app/navigation";
  import { resolve } from "$app/paths";
  import { getTenant } from "$lib/auth";
  import { locale, translate } from "$lib/i18n";
  import { loadThreadMessages } from "$lib/chat/client";
  import { watchThreadMessages } from "$lib/chat/watch";
  import type { ChatMessage } from "$lib/chat/types";
  import { buildCanvasState } from "$lib/plans/canvas-state";
  import PlanCanvas from "$lib/components/canvas/PlanCanvas.svelte";
  import CanvasTopBar from "$lib/components/canvas/CanvasTopBar.svelte";
  import RunHistoryDrawer from "$lib/components/canvas/RunHistoryDrawer.svelte";
  import SettingsDrawer from "$lib/components/canvas/SettingsDrawer.svelte";
  import ScheduleDialog from "$lib/components/canvas/ScheduleDialog.svelte";
  import { computeRunCost, type ExecutorPriceLookup } from "$lib/plans/cost";
  import { planClient } from "$lib/rpc";
  import type { PlanConfiguration } from "$lib/gen/harpia/plans/v1/plans_pb";

  let { data } = $props();

  let messages = $state<ChatMessage[]>([]);
  let runHistoryOpen = $state(false);
  let settingsOpen = $state(false);
  let scheduleOpen = $state(false);
  // Server is the source of truth for binding/schedule mutations; the
  // load-time snapshot goes stale during an in-progress walk. Refetch
  // when the chat stream emits a kind that mutates configuration.
  let liveConfiguration = $state<PlanConfiguration | undefined>(
    data.configuration,
  );

  const tenantId = $derived(getTenant()?.id ?? "");

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

  const runMessages = $derived(
    data.runId ? messages.filter((m) => m.executionId === data.runId) : [],
  );

  const canvasState = $derived(
    buildCanvasState(
      runMessages,
      data.template?.steps ?? [],
      liveConfiguration?.slotBindings,
    ),
  );

  const approvalInputArtifactByStep = $derived<Record<string, string>>({});

  const pendingAnswerCount = $derived(
    Object.values(canvasState).filter(
      (s) =>
        s.status === "awaiting_elicitation" || s.status === "awaiting_approval",
    ).length,
  );

  function answerNext() {
    const firstPending = Object.entries(canvasState).find(
      ([, s]) =>
        s.status === "awaiting_elicitation" || s.status === "awaiting_approval",
    );
    if (firstPending) {
      goto(
        resolve(
          `/plans/configurations/${data.configurationId}/canvas?run=${data.runId}#node-${firstPending[0]}`,
        ),
      );
    }
  }

  $effect(() => {
    if (!tenantId || !data.configurationId) return;
    const controller = new AbortController();
    void (async () => {
      try {
        const initial = await loadThreadMessages(
          tenantId,
          data.configurationId,
        );
        if (controller.signal.aborted) return;
        messages = initial;
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
      }
    })();
    return () => {
      controller.abort();
    };
  });

  // Refetch the configuration when a mutating message kind arrives.
  const MUTATING_KINDS = new Set([
    "USER_SELECTION",
    "STEP_REBOUND",
    "SCHEDULE_SET",
    "CONFIGURATION_SAVED",
  ]);
  let refetchSeq = $state(0n);
  $effect(() => {
    // Scan for the highest-seq mutating message; the LATEST kind right
    // after a USER_SELECTION is the next ASSISTANT_PROMPT (non-mutating).
    let maxSeq = refetchSeq;
    for (let i = messages.length - 1; i >= 0; i--) {
      const m = messages[i];
      if (m.sequenceNumber <= maxSeq) break;
      if (MUTATING_KINDS.has(m.kind) && m.sequenceNumber > maxSeq) {
        maxSeq = m.sequenceNumber;
      }
    }
    if (maxSeq !== refetchSeq) refetchSeq = maxSeq;
  });
  $effect(() => {
    if (!tenantId || !data.configurationId || refetchSeq === 0n) return;
    const controller = new AbortController();
    const timer = setTimeout(async () => {
      try {
        const res = await planClient.getPlanConfiguration(
          { tenantId, planConfigurationId: data.configurationId },
          { signal: controller.signal },
        );
        if (res.planConfiguration) liveConfiguration = res.planConfiguration;
      } catch {
        // best-effort
      }
    }, 200);
    return () => {
      controller.abort();
      clearTimeout(timer);
    };
  });

  const runStartedAt = $derived(
    runMessages.find((m) => m.kind === "RUN_STARTED")?.createdAt,
  );
</script>

<svelte:head>
  <title>{translate("nav.planCanvas", $locale)} · Harpia</title>
</svelte:head>

<div class="flex h-screen flex-col">
  <CanvasTopBar
    configurationId={data.configurationId}
    {runStartedAt}
    {pendingAnswerCount}
    planName={data.template?.name}
    onOpenRunHistory={() => (runHistoryOpen = true)}
    onOpenSettings={() => (settingsOpen = true)}
    onOpenSchedule={() => (scheduleOpen = true)}
    onAnswerNext={answerNext}
    {cost}
  />

  <div class="flex-1 overflow-hidden">
    {#if data.template?.steps && data.template.steps.length > 0}
      <PlanCanvas
        steps={data.template.steps}
        edges={data.template.edges}
        {canvasState}
        {tenantId}
        {approvalInputArtifactByStep}
      />
    {:else}
      <p class="px-6 py-6 text-[12px] text-crown-ash-dark">
        {translate("canvas.template.empty", $locale)}
      </p>
    {/if}
  </div>

  <RunHistoryDrawer
    open={runHistoryOpen}
    onClose={() => (runHistoryOpen = false)}
    {tenantId}
    configurationId={data.configurationId}
    currentRunId={data.runId || undefined}
  />

  <SettingsDrawer
    open={settingsOpen}
    onClose={() => (settingsOpen = false)}
    configurationId={data.configurationId}
    {tenantId}
  />

  {#if liveConfiguration}
    <ScheduleDialog
      open={scheduleOpen}
      configuration={liveConfiguration}
      onClose={() => (scheduleOpen = false)}
    />
  {/if}
</div>
