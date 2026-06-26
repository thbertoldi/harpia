<script lang="ts">
  type Props = {
    shape?: "rect" | "circle";
    width: string;
    height: string;
    count?: number;
  };
  let { shape = "rect", width, height, count = 1 }: Props = $props();
</script>

<div class="stack" style="--gap: 8px;">
  {#each Array.from({ length: count }, (v, i) => i) as i (i)}
    <div
      class="skeleton"
      class:circle={shape === "circle"}
      style="width: {width}; height: {height};"
    >
      <div class="shimmer"></div>
    </div>
  {/each}
</div>

<style>
  .stack {
    display: flex;
    flex-direction: column;
    gap: var(--gap);
  }
  .skeleton {
    position: relative;
    overflow: hidden;
    background: var(--token-surface-elevated, #1a1b24);
    border-radius: 6px;
  }
  .skeleton.circle {
    border-radius: 50%;
  }
  .shimmer {
    position: absolute;
    inset: 0;
    transform: translateX(-100%);
    background: linear-gradient(
      90deg,
      transparent 0%,
      rgba(255, 255, 255, 0.04) 50%,
      transparent 100%
    );
    animation: shimmer 1.4s linear infinite;
  }
  @media (prefers-reduced-motion: reduce) {
    .shimmer {
      animation: none;
    }
  }
  @keyframes shimmer {
    to {
      transform: translateX(100%);
    }
  }
</style>
