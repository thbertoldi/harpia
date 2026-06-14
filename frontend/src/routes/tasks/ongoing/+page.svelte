<script lang="ts">
  import { goto } from "$app/navigation";
  import { resolve } from "$app/paths";
  import { onDestroy } from "svelte";
  import { AlertTriangle, Loader2, Radar } from "lucide-svelte";
  import { requireTenantId } from "$lib/auth";
  import { toUserMessage } from "$lib/connect-errors";
  import OngoingTaskCard from "$lib/components/OngoingTaskCard.svelte";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import { taskClient, type Task } from "$lib/rpc";
  import {
    groupOngoingTasks,
    ONGOING_SECTION_LABELS,
    ONGOING_SECTION_ORDER,
    type OngoingSectionId,
  } from "$lib/tasks/ongoing-tasks";
  import { startTaskPoller } from "$lib/tasks/task-refresh";

  let tasks = $state<Task[]>([]);
  let loading = $state(true);
  let refreshing = $state(false);
  let loadError = $state<string | null>(null);

  const sections = $derived(groupOngoingTasks(tasks));
  const ongoingCount = $derived(
    ONGOING_SECTION_ORDER.reduce(
      (count, sectionId) => count + sections[sectionId].length,
      0,
    ),
  );

  async function fetchTasks({ background = false } = {}) {
    if (background) {
      refreshing = true;
    } else {
      loading = true;
    }
    loadError = null;

    try {
      const loadedTasks: Task[] = [];
      const tenantId = requireTenantId();

      for await (const response of taskClient.listTasks({
        tenantId,
        pageSize: 50,
        pageToken: "",
      })) {
        loadedTasks.push(...response.tasks);
      }

      tasks = loadedTasks;
    } catch (error) {
      loadError = toUserMessage(error);
    } finally {
      loading = false;
      refreshing = false;
    }
  }

  function openTask(task: Task) {
    goto(`${resolve("/tasks")}?id=${encodeURIComponent(task.id)}`);
  }

  function emptyMessage(sectionId: OngoingSectionId): string {
    switch (sectionId) {
      case "in_progress":
        return "No tasks are executing right now.";
      case "awaiting_approval":
        return "No tasks waiting for plan or agent approval.";
      case "awaiting_review":
        return "No tasks waiting for output review.";
      case "planning":
        return "No tasks are being planned.";
    }
  }

  $effect(() => {
    void fetchTasks();
  });

  const poller = startTaskPoller(() => fetchTasks({ background: true }));

  onDestroy(() => {
    poller.stop();
  });
</script>

<div class="px-4 py-6 lg:px-6">
  <div class="mb-6 flex items-center justify-between gap-4">
    <div>
      <HarpyHeading tag="h1" class="text-2xl text-cream">
        Ongoing Tasks
      </HarpyHeading>
      <p class="mt-1 font-body text-sm text-crown-ash">
        Live view of in-flight work across your workspace.
      </p>
    </div>
    {#if refreshing}
      <span
        class="inline-flex items-center gap-2 font-mono text-[10px] tracking-wider text-crown-ash uppercase"
      >
        <Loader2 class="size-3 animate-spin" />
        Refreshing
      </span>
    {/if}
  </div>

  {#if loading}
    <div class="flex min-h-[40vh] items-center justify-center">
      <div class="flex items-center gap-2 text-crown-ash">
        <Loader2 class="size-5 animate-spin" />
        <span class="font-body text-sm">Loading ongoing tasks...</span>
      </div>
    </div>
  {:else if loadError}
    <div class="flex min-h-[40vh] items-center justify-center">
      <div class="text-center">
        <AlertTriangle class="mx-auto mb-3 size-10 text-red-400" />
        <p class="font-body text-sm text-red-400">Failed to load tasks</p>
        <p class="mt-1 font-mono text-xs text-crown-ash">{loadError}</p>
        <button
          type="button"
          onclick={() => fetchTasks()}
          class="mt-4 rounded-md border border-plumage px-4 py-2 font-body text-sm text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
        >
          Retry
        </button>
      </div>
    </div>
  {:else if ongoingCount === 0}
    <div
      class="flex min-h-[40vh] flex-col items-center justify-center rounded-lg border border-dashed border-plumage bg-obsidian-light/30 px-6 py-12 text-center"
    >
      <Radar class="mb-4 size-12 text-talon-gold" />
      <HarpyHeading tag="h2" class="mb-2 text-xl text-cream">
        Nothing in flight
      </HarpyHeading>
      <p class="max-w-md font-body text-sm text-crown-ash">
        When tasks are planning, executing, or waiting on your feedback, they
        will appear here.
      </p>
    </div>
  {:else}
    <div class="grid gap-4 xl:grid-cols-2 2xl:grid-cols-4">
      {#each ONGOING_SECTION_ORDER as sectionId (sectionId)}
        <section
          class="flex min-h-[280px] flex-col rounded-lg border border-plumage bg-obsidian-light/20"
        >
          <header
            class="border-b border-plumage/60 px-4 py-3"
          >
            <h2 class="font-heading text-sm font-semibold text-cream">
              {ONGOING_SECTION_LABELS[sectionId]}
            </h2>
            <p class="mt-0.5 font-mono text-[10px] tracking-wider text-crown-ash uppercase">
              {sections[sectionId].length} task{sections[sectionId].length === 1
                ? ""
                : "s"}
            </p>
          </header>

          <div class="flex-1 space-y-2 overflow-y-auto p-3">
            {#if sections[sectionId].length === 0}
              <p class="px-1 py-6 text-center font-body text-xs text-crown-ash-dark">
                {emptyMessage(sectionId)}
              </p>
            {:else}
              {#each sections[sectionId] as task (task.id)}
                <OngoingTaskCard task={task} onclick={() => openTask(task)} />
              {/each}
            {/if}
          </div>
        </section>
      {/each}
    </div>
  {/if}
</div>
