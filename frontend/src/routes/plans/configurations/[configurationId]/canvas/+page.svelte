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

  let { data } = $props();

  let messages = $state<ChatMessage[]>([]);
  let runHistoryOpen = $state(false);
  let settingsOpen = $state(false);

  const tenantId = $derived(getTenant()?.id ?? "");

  const runMessages = $derived(
    data.runId ? messages.filter((m) => m.executionId === data.runId) : [],
  );

  const canvasState = $derived(
    buildCanvasState(runMessages, data.template?.steps ?? []),
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
    onAnswerNext={answerNext}
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
</div>
