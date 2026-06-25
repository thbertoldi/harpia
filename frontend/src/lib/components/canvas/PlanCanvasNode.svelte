<script lang="ts">
  import {
    Loader2,
    Check,
    AlertTriangle,
    MessageSquare,
    ShieldQuestion,
    Clock,
    CircleDashed,
    Bot,
  } from "lucide-svelte";
  import type { PlanStep } from "$lib/gen/harpia/plans/v1/plans_pb";
  import type { CanvasStepState } from "$lib/plans/canvas-state";
  import { locale, translate } from "$lib/i18n";
  import { createNodeStatePulse } from "$lib/motion/springs";
  import { prefersReducedMotion } from "$lib/motion/reducedMotion";

  interface Props {
    step: PlanStep;
    nodeState: CanvasStepState;
    selected: boolean;
    onSelect: () => void;
    /** Fired when the node should reveal its hover card; passes the node bounds. */
    onHoverShow?: (rect: DOMRect) => void;
    /** Fired when the node should hide its hover card. */
    onHoverHide?: () => void;
  }
  let {
    step,
    nodeState,
    selected,
    onSelect,
    onHoverShow,
    onHoverHide,
  }: Props = $props();

  let buttonEl = $state<HTMLButtonElement | null>(null);

  // `waiting` collapses the awaiting_* runtime states into the spec's halo
  // treatment; the runtime model keeps them distinct for the detail pane.
  const isWaiting = $derived(
    nodeState.status === "awaiting_elicitation" ||
      nodeState.status === "awaiting_approval",
  );

  const StatusIcon = $derived(
    nodeState.status === "running"
      ? Loader2
      : nodeState.status === "done"
        ? Check
        : nodeState.status === "failed"
          ? AlertTriangle
          : nodeState.status === "awaiting_elicitation"
            ? MessageSquare
            : nodeState.status === "awaiting_approval"
              ? ShieldQuestion
              : nodeState.status === "bound"
                ? Bot
                : nodeState.status === "unbound"
                  ? CircleDashed
                  : Clock,
  );

  const statusLabelKey = $derived(`canvas.status.${nodeState.status}`);

  // --- running pulse (opacity 1.0 ↔ 0.55 over ~1.4s loop) -----------------
  const pulse = createNodeStatePulse();
  let pulseLoop = false;
  // Map the 0→1 tween onto the 1.0↔0.55 opacity band; midpoint is the dip.
  const pulseOpacity = $derived(1 - $pulse * 0.45);

  async function runPulse() {
    while (pulseLoop) {
      await pulse.set(1);
      if (!pulseLoop) break;
      await pulse.set(0);
    }
  }

  $effect(() => {
    const shouldPulse = nodeState.status === "running" && !prefersReducedMotion();
    if (shouldPulse && !pulseLoop) {
      pulseLoop = true;
      void runPulse();
    } else if (!shouldPulse && pulseLoop) {
      pulseLoop = false;
      void pulse.set(0);
    }
  });

  // --- hover intent timers ------------------------------------------------
  let showTimer: ReturnType<typeof setTimeout> | null = null;
  let hideTimer: ReturnType<typeof setTimeout> | null = null;

  function clearTimers() {
    if (showTimer) {
      clearTimeout(showTimer);
      showTimer = null;
    }
    if (hideTimer) {
      clearTimeout(hideTimer);
      hideTimer = null;
    }
  }

  function handleEnter() {
    if (hideTimer) {
      clearTimeout(hideTimer);
      hideTimer = null;
    }
    if (showTimer) return;
    showTimer = setTimeout(() => {
      showTimer = null;
      if (buttonEl && onHoverShow) {
        onHoverShow(buttonEl.getBoundingClientRect());
      }
    }, 150);
  }

  function handleLeave() {
    if (showTimer) {
      clearTimeout(showTimer);
      showTimer = null;
    }
    if (hideTimer) return;
    hideTimer = setTimeout(() => {
      hideTimer = null;
      onHoverHide?.();
    }, 80);
  }

  function handleClick() {
    // Hover popover is hover-only: dismiss on click, then open detail pane.
    clearTimers();
    onHoverHide?.();
    onSelect();
  }

  // Teardown on unmount (Svelte 5 idiom — onDestroy must not be used here:
  // it failed with a null lifecycle context and crashed every node, blanking
  // the canvas). A dep-free $effect runs its cleanup exactly on destroy.
  $effect(() => {
    return () => {
      pulseLoop = false;
      clearTimers();
    };
  });
</script>

<button
  bind:this={buttonEl}
  type="button"
  id={`node-${step.key}`}
  onclick={handleClick}
  onmouseenter={handleEnter}
  onmouseleave={handleLeave}
  onfocus={handleEnter}
  onblur={handleLeave}
  class="harpia-node flex w-44 flex-col gap-1.5 rounded-lg px-3 py-2 text-left transition-colors"
  class:is-unbound={nodeState.status === "unbound"}
  class:is-bound={nodeState.status === "bound"}
  class:is-running={nodeState.status === "running"}
  class:is-waiting={isWaiting}
  class:is-done={nodeState.status === "done"}
  class:is-failed={nodeState.status === "failed"}
  class:is-selected={selected}
  style:opacity={nodeState.status === "running" ? pulseOpacity : 1}
>
  <div class="flex items-center gap-1.5">
    <StatusIcon
      class={`size-3.5 ${
        nodeState.status === "running"
          ? "animate-spin text-primary"
          : nodeState.status === "bound" || nodeState.status === "done"
            ? "text-primary"
            : nodeState.status === "failed"
              ? "text-[#d9534f]"
              : "text-text-muted"
      }`}
    />
    <span class="font-heading text-[12px] font-semibold text-text">
      {step.title}
    </span>
  </div>
  <span class="font-mono text-[9px] text-text-muted-dark">{step.key}</span>
  <span class="text-[10px] text-text-muted">
    {translate(statusLabelKey, $locale)}
  </span>
</button>

<style>
  .harpia-node {
    background-color: var(--token-surface-elevated);
    border: 1px solid var(--token-border);
  }
  .harpia-node:hover {
    background-color: var(--token-surface-hover);
  }

  /* unbound: muted fill, dashed 1px border */
  .harpia-node.is-unbound {
    border-style: dashed;
    border-color: var(--token-text-muted-dark);
  }

  /* bound: 2px gold inner ring + solid border */
  .harpia-node.is-bound {
    border-color: var(--token-primary);
    box-shadow: inset 0 0 0 2px var(--token-primary);
  }

  /* running: solid gold border (opacity pulse driven inline) */
  .harpia-node.is-running {
    border-color: var(--token-primary);
  }

  /* waiting: 3px outer gold halo */
  .harpia-node.is-waiting {
    border-color: var(--token-primary);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--token-primary) 55%, transparent);
  }

  /* done: gold border, full opacity (check glyph supplied by icon) */
  .harpia-node.is-done {
    border-color: var(--token-primary);
  }

  /* failed: 1px red border */
  .harpia-node.is-failed {
    border-color: #d9534f;
  }

  /* selection ring sits outside the state treatment */
  .harpia-node.is-selected {
    outline: 2px solid var(--token-primary);
    outline-offset: 2px;
  }
</style>
