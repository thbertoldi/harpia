<script lang="ts">
  import { Plus, Loader2, Sparkles, Bot, AlertTriangle } from "lucide-svelte";
  import { goto } from "$app/navigation";
  import { resolve } from "$app/paths";
  import type { Task } from "$lib/types";
  import { listTasks, watchTask } from "$lib/client";
  import StatusBadge from "$lib/components/StatusBadge.svelte";
  import TaskDetail from "$lib/components/TaskDetail.svelte";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";

  let tasks = $state<Task[]>([]);
  let selectedTask = $state<Task | null>(null);
  let loading = $state(true);
  let loadError = $state<string | null>(null);
  let watchController = $state<AbortController | null>(null);
  let tenantId = $state("default");

  $effect(() => {
    fetchTasks();
  });

  async function fetchTasks() {
    loading = true;
    loadError = null;
    try {
      const res = await listTasks({
        tenantId,
        pageSize: 50,
        pageToken: "",
      });
      tasks = res.tasks;
    } catch (e) {
      loadError = e instanceof Error ? e.message : "Failed to load tasks";
    } finally {
      loading = false;
    }
  }

  function selectTask(task: Task) {
    if (watchController) {
      watchController.abort();
    }
    selectedTask = task;

    watchController = watchTask(
      tenantId,
      task.id,
      (updated) => {
        selectedTask = updated;
        tasks = tasks.map((t) => (t.id === updated.id ? updated : t));
      },
      (err) => {
        console.error("Watch stream error:", err);
      },
    );
  }

  function deselectTask() {
    if (watchController) {
      watchController.abort();
      watchController = null;
    }
    selectedTask = null;
  }

  function handleNewTask() {
    goto(resolve("/"));
  }

  function formattedDate(dateStr: string): string {
    try {
      const d = new Date(dateStr);
      return d.toLocaleDateString("en-US", { month: "short", day: "numeric" });
    } catch {
      return dateStr;
    }
  }

  function descriptionPreview(desc: string): string {
    if (!desc) return "";
    return desc.length > 100 ? desc.slice(0, 100) + "..." : desc;
  }
</script>

<div class="flex h-[calc(100vh-3.5rem)]">
  <div
    class="flex w-full flex-col transition-all duration-300 {selectedTask
      ? 'lg:w-[45%]'
      : 'lg:w-full'}"
  >
    <div class="flex items-center justify-between px-4 py-3 lg:px-6">
      <HarpyHeading tag="h1" class="text-2xl text-cream">Tasks</HarpyHeading>
      <button
        onclick={handleNewTask}
        class="inline-flex cursor-pointer items-center gap-2 rounded-md bg-talon-gold px-4 py-2 font-body text-sm font-medium text-obsidian transition-all hover:bg-talon-gold-bright"
      >
        <Plus class="size-4" />
        New Task
      </button>
    </div>

    {#if loading}
      <div class="flex flex-1 items-center justify-center">
        <div class="flex items-center gap-2 text-crown-ash">
          <Loader2 class="size-5 animate-spin" />
          <span class="font-body text-sm">Loading tasks...</span>
        </div>
      </div>
    {:else if loadError}
      <div class="flex flex-1 items-center justify-center">
        <div class="text-center">
          <AlertTriangle class="mx-auto mb-3 size-10 text-red-400" />
          <p class="font-body text-sm text-red-400">Failed to load tasks</p>
          <p class="mt-1 font-mono text-xs text-crown-ash">{loadError}</p>
          <button
            onclick={() => fetchTasks()}
            class="mt-4 rounded-md border border-plumage px-4 py-2 font-body text-sm text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
          >
            Retry
          </button>
        </div>
      </div>
    {:else if tasks.length === 0}
      <div class="flex flex-1 flex-col items-center justify-center px-4">
        <div class="mb-4 text-talon-gold">
          <Bot class="size-12" />
        </div>
        <HarpyHeading tag="h2" class="mb-2 text-center text-xl text-cream">
          No tasks yet. What do you want to get done?
        </HarpyHeading>
        <p class="mb-6 max-w-md text-center font-body text-sm text-crown-ash">
          Describe your task in natural language and let Harpia's agents handle
          the execution.
        </p>
        <button
          onclick={handleNewTask}
          class="inline-flex cursor-pointer items-center gap-2 rounded-lg bg-talon-gold px-6 py-3 font-heading text-lg font-bold text-obsidian transition-all hover:bg-talon-gold-bright hover:scale-105"
        >
          <Sparkles class="size-5" />
          Create Your First Task
        </button>
      </div>
    {:else}
      <div class="flex-1 overflow-y-auto px-4 py-2 lg:px-6">
        <div class="space-y-2">
          {#each tasks as task (task.id)}
            <button
              onclick={() => selectTask(task)}
              class="w-full rounded-lg border bg-obsidian-light/60 p-4 text-left transition-all duration-200 hover:border-talon-gold/50 hover:bg-obsidian-light {selectedTask?.id ===
              task.id
                ? 'border-talon-gold bg-obsidian-light'
                : 'border-plumage'}"
            >
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0 flex-1">
                  <p
                    class="font-heading text-base font-semibold text-cream truncate"
                  >
                    {task.title}
                  </p>
                  {#if task.description}
                    <p
                      class="mt-1 font-body text-sm text-crown-ash line-clamp-2"
                    >
                      {descriptionPreview(task.description)}
                    </p>
                  {/if}
                </div>
                <StatusBadge status={task.status} pulsing={true} />
              </div>
              <div
                class="mt-2 flex items-center gap-4 font-mono text-[10px] uppercase tracking-wider text-crown-ash-dark"
              >
                <span>{formattedDate(task.createdAt)}</span>
                {#if task.subtasks?.length}
                  <span
                    >{task.subtasks.length} subtask{task.subtasks.length !== 1
                      ? "s"
                      : ""}</span
                  >
                {/if}
              </div>
            </button>
          {/each}
        </div>
      </div>
    {/if}
  </div>

  {#if selectedTask}
    <div
      class="hidden w-[55%] border-l border-plumage bg-obsidian transition-all duration-300 lg:block"
    >
      <TaskDetail task={selectedTask} onclose={deselectTask} />
    </div>
  {/if}
</div>
