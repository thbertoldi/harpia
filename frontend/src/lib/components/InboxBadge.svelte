<script lang="ts">
  import { getTenant } from "$lib/auth";
  import { countInbox } from "$lib/inbox/aggregator";

  let count = $state(0);

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
    const timer = setInterval(refresh, 30_000);
    return () => clearInterval(timer);
  });
</script>

{#if count > 0}
  <span
    class="ml-auto inline-flex items-center justify-center rounded-full bg-primary px-1.5 py-0.5 font-mono text-[10px] leading-none font-bold text-primary-foreground"
  >
    {count}
  </span>
{/if}
