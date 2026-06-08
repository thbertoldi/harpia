<script lang="ts">
  import { page } from "$app/state";
  import { AlertTriangle } from "lucide-svelte";
  import { toUserMessage } from "$lib/connect-errors";
</script>

<div class="flex h-full items-center justify-center p-6">
  <div class="max-w-md text-center">
    <AlertTriangle class="mx-auto mb-4 size-10 text-red-400" />
    <h1 class="font-heading text-2xl font-bold text-cream">
      Something went wrong
    </h1>
    <p class="mt-2 font-body text-sm text-crown-ash">
      {toUserMessage(page.error)}
    </p>
  </div>
</div>
