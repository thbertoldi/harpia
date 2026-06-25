<script lang="ts">
  import type { PlanStep } from "$lib/gen/harpia/plans/v1/plans_pb";
  import type { CanvasStepState } from "$lib/plans/canvas-state";
  import { locale, translate } from "$lib/i18n";
  import { formatArtifactTypeLabel } from "$lib/plans/artifact-flow";

  interface Props {
    node: PlanStep;
    state: CanvasStepState;
    /** Node bounds in viewport coordinates (from getBoundingClientRect). */
    anchorRect: DOMRect;
    /** Display name of the bound executor, when known. */
    executorName?: string | null;
    /** Executor tier / SKU label. */
    executorTier?: string | null;
    /** Per-run price in BRL, when known. */
    priceBrl?: number | null;
    /** Overseer label (defaults to the assistant persona upstream). */
    overseerLabel?: string | null;
  }
  let {
    node,
    state,
    anchorRect,
    executorName,
    executorTier,
    priceBrl,
    overseerLabel,
  }: Props = $props();

  const inputLabel = $derived(
    node.inputArtifactTypeId
      ? formatArtifactTypeLabel(node.inputArtifactTypeId, $locale)
      : translate("common.emDash", $locale),
  );
  const outputLabel = $derived(
    node.outputArtifactTypeId
      ? formatArtifactTypeLabel(node.outputArtifactTypeId, $locale)
      : translate("common.emDash", $locale),
  );

  const executorDisplay = $derived(
    executorName ||
      node.defaultExecutorSkuKey ||
      translate("common.emDash", $locale),
  );

  const priceDisplay = $derived(
    priceBrl != null
      ? new Intl.NumberFormat($locale, {
          style: "currency",
          currency: "BRL",
        }).format(priceBrl)
      : null,
  );

  const stateLabel = $derived(
    translate(`canvas.status.${state.status}`, $locale),
  );

  // Anchor ~6px above the node, horizontally centred. Card width is fixed so we
  // can centre on the node midpoint; position:fixed keeps it in viewport space.
  const CARD_WIDTH = 240;
  const GAP = 6;
  const left = $derived(
    Math.max(8, anchorRect.left + anchorRect.width / 2 - CARD_WIDTH / 2),
  );
  const top = $derived(anchorRect.top - GAP);
</script>

<div
  class="harpia-hovercard"
  role="tooltip"
  style:left={`${left}px`}
  style:top={`${top}px`}
  style:width={`${CARD_WIDTH}px`}
>
  <p class="font-heading text-[12px] font-semibold text-text">{node.title}</p>

  <div class="mt-2 flex items-center gap-1.5">
    <span class="harpia-chip">{inputLabel}</span>
    <span class="text-[10px] text-text-muted-dark">→</span>
    <span class="harpia-chip">{outputLabel}</span>
  </div>

  <dl class="mt-2 flex flex-col gap-1 text-[11px]">
    <div class="flex items-baseline justify-between gap-2">
      <dt class="text-text-muted-dark">
        {translate("common.agent", $locale)}
      </dt>
      <dd class="text-right text-text">
        {executorDisplay}{#if executorTier}
          <span class="text-text-muted"> · {executorTier}</span>
        {/if}
      </dd>
    </div>
    {#if priceDisplay}
      <div class="flex items-baseline justify-between gap-2">
        <dt class="text-text-muted-dark">BRL</dt>
        <dd class="text-right text-text">{priceDisplay}</dd>
      </div>
    {/if}
    <div class="flex items-baseline justify-between gap-2">
      <dt class="text-text-muted-dark">
        {translate("login.persona.overseer.label", $locale)}
      </dt>
      <dd class="text-right text-text">
        {overseerLabel || translate("common.emDash", $locale)}
      </dd>
    </div>
    <div class="flex items-baseline justify-between gap-2">
      <dt class="text-text-muted-dark">
        {translate("common.progress", $locale)}
      </dt>
      <dd class="text-right text-text">{stateLabel}</dd>
    </div>
  </dl>
</div>

<style>
  .harpia-hovercard {
    position: fixed;
    z-index: 50;
    transform: translateY(-100%);
    background-color: var(--token-surface-pop);
    border: 1px solid var(--token-border);
    border-radius: 8px;
    padding: 12px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.45);
    pointer-events: none;
  }
  .harpia-chip {
    display: inline-block;
    background-color: var(--token-surface-elevated);
    border: 1px solid var(--token-border);
    border-radius: 4px;
    padding: 1px 5px;
    font-family: var(--font-mono);
    font-size: 9px;
    color: var(--token-text-muted);
  }
</style>
