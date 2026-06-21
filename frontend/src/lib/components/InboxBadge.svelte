<script lang="ts">
  import { onDestroy } from "svelte";
  import { getTenant } from "$lib/auth";
  import { countInbox } from "$lib/inbox/aggregator";

  let count = $state(0);
  let timer: ReturnType<typeof setInterval> | undefined;

  async function refresh() {
    const tenant = getTenant();
    if (!tenant?.id) return;
    try {
      count = await countInbox(tenant.id);
    } catch {
      // Keep the last-known count if the load fails.
    }
  }

  $effect(() => {
    void refresh();
    timer = setInterval(refresh, 30_000);
    return () => {
      if (timer) clearInterval(timer);
    };
  });

  onDestroy(() => {
    if (timer) clearInterval(timer);
  });
</script>

{#if count > 0}
  <span
    class="ml-auto inline-flex animate-pulse items-center justify-center rounded-full bg-talon-gold px-1.5 py-0.5 font-mono text-[10px] leading-none font-bold text-obsidian"
  >
    {count}
  </span>
{/if}
