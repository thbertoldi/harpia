<script lang="ts">
  import { goto } from "$app/navigation";
  import { resolve } from "$app/paths";
  import { locale, translate } from "$lib/i18n";
  import { loadPlanExecutions } from "$lib/plans/plan-execution";
  import type { PlanExecution } from "$lib/gen/harpia/plans/v1/plans_pb";
  import CanvasDrawer from "./CanvasDrawer.svelte";

  interface Props {
    open: boolean;
    onClose: () => void;
    tenantId: string;
    configurationId: string;
    currentRunId?: string;
  }
  let { open, onClose, tenantId, configurationId, currentRunId }: Props =
    $props();

  let executions = $state<PlanExecution[]>([]);
  let loading = $state(false);
  let loadError = $state(false);

  $effect(() => {
    if (!open) return;
    loading = true;
    loadError = false;
    void (async () => {
      try {
        const result = await loadPlanExecutions();
        executions = result.executions.filter(
          (e) => e.planConfigurationId === configurationId,
        );
      } catch {
        loadError = true;
      } finally {
        loading = false;
      }
    })();
  });

  function go(executionId: string) {
    goto(
      resolve(
        `/plans/configurations/${configurationId}/canvas?run=${executionId}`,
      ),
    );
    onClose();
  }
</script>

<CanvasDrawer
  {open}
  {onClose}
  title={translate("canvas.runHistory.title", $locale)}
>
  {#if loading}
    <p class="text-[12px] text-crown-ash-dark">
      {translate("canvas.runHistory.loading", $locale)}
    </p>
  {:else if loadError}
    <p class="text-[12px] text-red-400">
      {translate("canvas.runHistory.loadError", $locale)}
    </p>
  {:else if executions.length === 0}
    <p class="text-[12px] text-crown-ash-dark">
      {translate("canvas.runHistory.empty", $locale)}
    </p>
  {:else}
    <ul class="flex flex-col gap-2">
      {#each executions as exec, index (exec.id)}
        <li>
          <button
            type="button"
            onclick={() => go(exec.id)}
            class={`flex w-full flex-col gap-0.5 rounded border px-3 py-2 text-left text-[12px] hover:border-talon-gold ${
              exec.id === currentRunId
                ? "border-talon-gold bg-talon-gold/10"
                : "border-plumage bg-obsidian-light"
            }`}
          >
            <span class="font-heading font-semibold text-cream">
              {translate("canvas.runHistory.runLabel", $locale).replace(
                "{n}",
                String(executions.length - index),
              )}
            </span>
            <span class="font-mono text-[10px] text-crown-ash-dark">
              {exec.updatedAt}
            </span>
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</CanvasDrawer>
