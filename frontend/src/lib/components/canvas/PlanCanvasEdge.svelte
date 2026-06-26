<script lang="ts">
  /**
   * Anchor-point edge for the plan canvas. Draws a horizontal bezier from the
   * source node's right-edge midpoint to the target node's left-edge midpoint
   * so the line visibly touches both boxes, with the artifact type label riding
   * the curve midpoint as a small mono chip.
   *
   * All coordinates are in the SVG overlay space (canvas-content relative).
   */
  interface Anchor {
    /** Right-edge midpoint of the source node. */
    sourceRight: number;
    sourceMidY: number;
    /** Left-edge midpoint of the target node. */
    targetLeft: number;
    targetMidY: number;
  }

  interface Props {
    label: string;
    anchor: Anchor;
  }
  let { label, anchor }: Props = $props();

  const ctl = $derived(
    Math.max(16, (anchor.targetLeft - anchor.sourceRight) * 0.4),
  );

  const path = $derived(
    `M ${anchor.sourceRight} ${anchor.sourceMidY} ` +
      `C ${anchor.sourceRight + ctl} ${anchor.sourceMidY}, ` +
      `${anchor.targetLeft - ctl} ${anchor.targetMidY}, ` +
      `${anchor.targetLeft} ${anchor.targetMidY}`,
  );

  // Bezier midpoint (t = 0.5) for the type-label chip. Closed form of a cubic
  // bezier evaluated at the half mark.
  const mid = $derived.by(() => {
    const t = 0.5;
    const mt = 1 - t;
    const p0x = anchor.sourceRight;
    const p0y = anchor.sourceMidY;
    const p1x = anchor.sourceRight + ctl;
    const p1y = anchor.sourceMidY;
    const p2x = anchor.targetLeft - ctl;
    const p2y = anchor.targetMidY;
    const p3x = anchor.targetLeft;
    const p3y = anchor.targetMidY;
    const x =
      mt * mt * mt * p0x +
      3 * mt * mt * t * p1x +
      3 * mt * t * t * p2x +
      t * t * t * p3x;
    const y =
      mt * mt * mt * p0y +
      3 * mt * mt * t * p1y +
      3 * mt * t * t * p2y +
      t * t * t * p3y;
    return { x, y };
  });

  // Chip sizing: ~9px mono glyphs, 4px horizontal padding. Approximated so the
  // background rect hugs the text without a DOM measurement round-trip.
  const chipPadX = 4;
  const chipPadY = 2;
  const charW = 5.4;
  const chipW = $derived(label.length * charW + chipPadX * 2);
  const chipH = 13;
</script>

<g class="harpia-edge">
  <path
    d={path}
    fill="none"
    stroke="var(--token-text-muted-dark)"
    stroke-width="1.5"
  />
  {#if label}
    <rect
      x={mid.x - chipW / 2}
      y={mid.y - chipH / 2}
      width={chipW}
      height={chipH}
      rx="3"
      fill="var(--token-surface-elevated)"
      stroke="var(--token-border)"
      stroke-width="0.5"
    />
    <text
      x={mid.x}
      y={mid.y + chipPadY}
      text-anchor="middle"
      class="harpia-edge-label"
    >
      {label}
    </text>
  {/if}
</g>

<style>
  .harpia-edge-label {
    font-family: var(--font-mono);
    font-size: 9px;
    fill: var(--token-text-muted);
  }
</style>
