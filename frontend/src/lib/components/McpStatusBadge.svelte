<script lang="ts">
  import type { McpServerStatus } from "$lib/mocks/mcp-servers";
  import { statusLabel } from "$lib/mocks/mcp-servers";

  let { status }: { status: McpServerStatus } = $props();

  const statusConfig: Record<
    McpServerStatus,
    { bg: string; text: string; dot: string }
  > = {
    connected: {
      bg: "bg-green-500/20",
      text: "text-green-300",
      dot: "bg-green-400",
    },
    auth_required: {
      bg: "bg-amber-500/20",
      text: "text-amber-300",
      dot: "bg-amber-400",
    },
    error: {
      bg: "bg-red-500/20",
      text: "text-red-300",
      dot: "bg-red-400",
    },
    disconnected: {
      bg: "bg-crown-ash/20",
      text: "text-crown-ash",
      dot: "bg-crown-ash",
    },
  };

  const config = $derived(statusConfig[status]);
</script>

<span
  class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 font-mono text-[10px] tracking-wider uppercase {config.bg} {config.text}"
>
  <span class="relative inline-flex h-2 w-2 rounded-full {config.dot}"></span>
  {statusLabel(status)}
</span>
