<script lang="ts">
  import { Check, AlertTriangle, Loader2, ChevronDown } from "lucide-svelte";
  import { locale, translate } from "$lib/i18n";
  import type {
    ExecutionViewModel,
    ExecutionStepView,
  } from "$lib/plans/execution-view";

  interface Props {
    vm: ExecutionViewModel;
    initiallyCollapsed?: boolean;
  }
  let { vm, initiallyCollapsed = false }: Props = $props();

  // svelte-ignore state_referenced_locally
  // Seed the initial expanded/collapsed state from the prop; the card is then
  // user-toggled, so only the initial value is needed.
  let collapsed = $state(initiallyCollapsed);

  const subtitle = $derived.by(() => {
    switch (vm.state) {
      case "running":
        return (
          vm.runningStep?.title ??
          translate("thread.execution.status.running", $locale)
        );
      case "completed":
        return translate("thread.execution.completed", $locale);
      case "failed":
        return translate("thread.execution.failed", $locale);
      default:
        return translate("thread.execution.waiting", $locale);
    }
  });

  const barStyle = $derived.by(() => {
    const pct = Math.round(vm.progress * 100);
    if (vm.state === "completed") {
      return `width: 100%; background: var(--token-status-done);`;
    }
    if (vm.state === "failed") {
      return `width: ${pct}%; background: var(--token-danger);`;
    }
    return `width: ${pct}%; background: linear-gradient(90deg, var(--token-energy-bright), var(--token-energy));`;
  });
</script>

<div
  class="rounded-md border border-plumage/60 bg-obsidian-light/80 px-3 py-2.5 text-[12px]"
>
  <button
    type="button"
    onclick={() => (collapsed = !collapsed)}
    class="flex w-full items-center gap-2.5 text-left"
    aria-expanded={!collapsed}
    aria-label={translate("thread.execution.toggle", $locale)}
  >
    <span
      class="flex size-5 shrink-0 items-center justify-center rounded
        {vm.state === 'running'
        ? 'text-energy'
        : vm.state === 'completed'
          ? 'text-status-done'
          : vm.state === 'failed'
            ? 'text-danger'
            : 'text-crown-ash-dark'}"
    >
      {#if vm.state === "running"}
        <Loader2 class="size-4 animate-spin" />
      {:else if vm.state === "completed"}
        <Check class="size-4" />
      {:else if vm.state === "failed"}
        <AlertTriangle class="size-4" />
      {:else}
        <Loader2 class="size-4 opacity-0" />
      {/if}
    </span>

    <span class="min-w-0 flex-1">
      <span class="flex items-center gap-2">
        <span class="truncate font-body font-medium text-cream">
          {translate("thread.execution.runLabel", $locale, { n: vm.runNumber })}
        </span>
        {#if vm.total > 0}
          <span
            class="shrink-0 rounded-full bg-plumage/60 px-1.5 py-0.5 text-[10px] font-medium text-crown-ash tabular-nums"
            aria-label={translate("thread.execution.steps", $locale, {
              done: vm.doneCount,
              total: vm.total,
            })}
          >
            {vm.doneCount}/{vm.total}
          </span>
        {/if}
      </span>
      <span class="mt-0.5 block truncate text-[11px] text-crown-ash"
        >{subtitle}</span
      >
    </span>

    <ChevronDown
      class="size-3.5 shrink-0 text-crown-ash-dark transition-transform duration-150 {!collapsed
        ? ''
        : '-rotate-90'}"
    />
  </button>

  {#if vm.total > 0}
    <div class="mt-2 h-0.5 w-full overflow-hidden rounded-full bg-plumage/50">
      <div
        class="h-full transition-all duration-500 ease-out"
        style={barStyle}
      ></div>
    </div>
  {/if}

  {#if !collapsed && vm.total > 0}
    <ol class="mt-2.5 flex flex-col gap-0.5">
      {#each vm.steps as step, i (step.key)}
        {@const notLast = i < vm.steps.length - 1}
        <li class="relative flex gap-2.5 pb-2 {!notLast ? 'pb-0' : ''}">
          {#if notLast}
            <span
              class="absolute top-5 left-[9px] h-[calc(100%-0.75rem)] w-px
                {step.status === 'done'
                ? 'bg-status-done/40'
                : 'bg-plumage/50'}"
            ></span>
          {/if}
          <span class="relative z-10 pt-0.5">
            {@render stepIcon(step)}
          </span>
          <span
            class="pt-0.5 font-body
              {step.status === 'running'
              ? 'text-energy'
              : step.status === 'done'
                ? 'text-cream'
                : step.status === 'failed'
                  ? 'text-danger'
                  : 'text-crown-ash-dark'}"
          >
            {step.title}
          </span>
          {#if step.status === "running"}
            <span
              class="mt-0.5 ml-auto shrink-0 animate-pulse rounded-full bg-energy/15 px-1.5 py-0.5 text-[9px] font-semibold tracking-wide text-energy uppercase"
            >
              {translate("thread.execution.runningChip", $locale)}
            </span>
          {/if}
        </li>
      {/each}
    </ol>
  {/if}
</div>

{#snippet stepIcon(step: ExecutionStepView)}
  <span
    class="flex size-[18px] items-center justify-center rounded-full
      {step.status === 'done'
      ? 'bg-status-done/15 text-status-done ring-1 ring-status-done/40'
      : step.status === 'running'
        ? 'text-energy'
        : step.status === 'failed'
          ? 'bg-danger/15 text-danger ring-1 ring-danger/40'
          : 'text-status-pending'}"
  >
    {#if step.status === "done"}
      <Check class="size-2.5" />
    {:else if step.status === "running"}
      <Loader2 class="size-3.5 animate-spin" />
    {:else if step.status === "failed"}
      <AlertTriangle class="size-2.5" />
    {:else}
      <span class="size-1.5 rounded-full bg-current opacity-60"></span>
    {/if}
  </span>
{/snippet}
