<script lang="ts">
  import { requireTenantId } from "$lib/auth";
  import { countPendingElicitations } from "$lib/plans/elicitations";

  let count = $state(0);

  $effect(() => {
    let active = true;
    void (async () => {
      try {
        const tenantId = requireTenantId();
        const pending = await countPendingElicitations(tenantId);
        if (active) {
          count = pending;
        }
      } catch {
        // Badge is best-effort; stay silent when the API or tenant is missing.
      }
    })();
    return () => {
      active = false;
    };
  });
</script>

{#if count > 0}
  <span
    class="ml-auto inline-flex animate-pulse items-center justify-center rounded-full bg-talon-gold px-1.5 py-0.5 font-mono text-[10px] leading-none font-bold text-obsidian"
  >
    {count}
  </span>
{/if}
