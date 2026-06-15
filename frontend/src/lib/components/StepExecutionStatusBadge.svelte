<script lang="ts">
  import { StepExecutionStatus } from "$lib/rpc";
  import { locale, translate } from "$lib/i18n";
  import { statusKeyForStepExecution } from "$lib/plans/plan-execution";

  let {
    status,
    pulsing = false,
  }: {
    status: StepExecutionStatus;
    pulsing?: boolean;
  } = $props();

  const statusConfig: Record<
    number,
    { bg: string; text: string; dot: string; active: boolean }
  > = {
    [StepExecutionStatus.UNSPECIFIED]: {
      bg: "bg-crown-ash/20",
      text: "text-crown-ash",
      dot: "bg-crown-ash",
      active: false,
    },
    [StepExecutionStatus.PENDING]: {
      bg: "bg-crown-ash/20",
      text: "text-crown-ash",
      dot: "bg-crown-ash",
      active: false,
    },
    [StepExecutionStatus.RUNNING]: {
      bg: "bg-blue-500/20",
      text: "text-blue-300",
      dot: "bg-blue-400",
      active: true,
    },
    [StepExecutionStatus.AWAITING_ELICITATION]: {
      bg: "bg-amber-500/20",
      text: "text-amber-300",
      dot: "bg-amber-400",
      active: true,
    },
    [StepExecutionStatus.AWAITING_APPROVAL]: {
      bg: "bg-amber-500/20",
      text: "text-amber-300",
      dot: "bg-amber-400",
      active: true,
    },
    [StepExecutionStatus.COMPLETED]: {
      bg: "bg-green-500/20",
      text: "text-green-300",
      dot: "bg-green-400",
      active: false,
    },
    [StepExecutionStatus.FAILED]: {
      bg: "bg-red-500/20",
      text: "text-red-200",
      dot: "bg-red-400",
      active: false,
    },
  };

  const config = $derived(
    statusConfig[status] ?? statusConfig[StepExecutionStatus.UNSPECIFIED],
  );
  const labelKey = $derived(statusKeyForStepExecution(status));
</script>

<span
  class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 font-mono text-[10px] tracking-wider uppercase {config.bg} {config.text}"
>
  <span class="relative flex h-2 w-2">
    <span
      class="{pulsing && config.active
        ? 'animate-ping'
        : ''} absolute inline-flex h-full w-full rounded-full opacity-75 {config.dot}"
    ></span>
    <span class="relative inline-flex h-2 w-2 rounded-full {config.dot}"></span>
  </span>
  {translate(labelKey, $locale)}
</span>
