<script lang="ts">
  import { TaskStatus } from "$lib/types";

  let {
    status,
    pulsing = false,
  }: {
    status: TaskStatus;
    pulsing?: boolean;
  } = $props();

  const statusConfig: Record<
    number,
    { label: string; bg: string; text: string; dot: string }
  > = {
    [TaskStatus.PENDING]: {
      label: "Pending",
      bg: "bg-crown-ash/20",
      text: "text-crown-ash",
      dot: "bg-crown-ash",
    },
    [TaskStatus.PLANNING]: {
      label: "Planning",
      bg: "bg-talon-gold/20",
      text: "text-obsidian",
      dot: "bg-talon-gold",
    },
    [TaskStatus.IN_PROGRESS]: {
      label: "Running",
      bg: "bg-blue-500/20",
      text: "text-blue-300",
      dot: "bg-blue-400",
    },
    [TaskStatus.AWAITING_FEEDBACK]: {
      label: "Awaiting You",
      bg: "bg-amber-500/20",
      text: "text-amber-300",
      dot: "bg-amber-400",
    },
    [TaskStatus.COMPLETED]: {
      label: "Done",
      bg: "bg-green-500/20",
      text: "text-green-300",
      dot: "bg-green-400",
    },
    [TaskStatus.FAILED]: {
      label: "Failed",
      bg: "bg-red-500/20",
      text: "text-white",
      dot: "bg-red-400",
    },
    [TaskStatus.CANCELLED]: {
      label: "Cancelled",
      bg: "bg-crown-ash/20",
      text: "text-crown-ash",
      dot: "bg-crown-ash",
    },
  };

  const config = $derived(
    statusConfig[status] ?? statusConfig[TaskStatus.UNSPECIFIED],
  );
  const isActive = $derived(
    pulsing &&
      (status === TaskStatus.PLANNING || status === TaskStatus.IN_PROGRESS),
  );
</script>

<span
  class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 font-mono text-[10px] uppercase tracking-wider {config.bg} {config.text}"
>
  {#if pulsing}
    <span class="relative flex h-2 w-2">
      <span
        class="{isActive
          ? 'animate-ping'
          : ''} absolute inline-flex h-full w-full rounded-full opacity-75 {config.dot}"
      ></span>
      <span class="relative inline-flex h-2 w-2 rounded-full {config.dot}"
      ></span>
    </span>
  {/if}
  {config.label}
</span>
