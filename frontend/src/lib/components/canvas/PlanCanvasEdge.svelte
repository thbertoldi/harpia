<script lang="ts">
  import { ArrowRight } from "lucide-svelte";

  interface Props {
    label: string;
  }
  let { label }: Props = $props();
</script>

<div class="flex flex-col items-center gap-0.5 px-2">
  <span class="font-mono text-[9px] text-crown-ash-dark">{label}</span>
  <ArrowRight class="size-4 text-crown-ash-dark" />
</div>
