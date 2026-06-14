<script lang="ts">
  import { Plus, Loader2, Sparkles, Bot, AlertTriangle } from "lucide-svelte";
  import { goto } from "$app/navigation";
  import { resolve } from "$app/paths";
  import { onDestroy } from "svelte";
  import { requireTenantId } from "$lib/auth";
  import { toUserMessage } from "$lib/connect-errors";
  import { taskClient, type Task } from "$lib/rpc";
  import StatusBadge from "$lib/components/StatusBadge.svelte";
  import TaskDetail from "$lib/components/TaskDetail.svelte";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import { locale, translate } from "$lib/i18n";

  let tasks = $state<Task[]>([]);
  let selectedTask = $state<Task | null>(null);
  let loading = $state(true);
  let loadError = $state<string | null>(null);
  let watchError = $state<string | null>(null);
  let watchController = $state<AbortController | null>(null);

  $effect(() => {
    fetchTasks();
  });

  async function fetchTasks() {
    loading = true;
    loadError = null;
    try {
      const loadedTasks: Task[] = [];
      const tenantId = requireTenantId();
      for await (const res of taskClient.listTasks({
        tenantId,
        pageSize: 50,
        pageToken: "",
      })) {
        loadedTasks.push(...res.tasks);
      }
      tasks = loadedTasks;
    } catch (e) {
      loadError = toUserMessage(e);
    } finally {
      loading = false;
    }
  }

  function selectTask(task: Task) {
    if (watchController) {
      watchController.abort();
      watchController = null;
    }

    let tenantId: string;
    try {
      tenantId = requireTenantId();
    } catch (e) {
      watchError = toUserMessage(e);
      return;
    }

    selectedTask = task;
    watchError = null;

    const controller = new AbortController();
    watchController = controller;

    void (async () => {
      try {
        for await (const update of taskClient.watchTask(
          { tenantId, taskId: task.id },
          { signal: controller.signal },
        )) {
          if (controller.signal.aborted || update.task?.id !== task.id) {
            break;
          }

          selectedTask = update.task;
          tasks = tasks.map((t) =>
            t.id === update.task?.id ? update.task : t,
          );
        }
      } catch (e) {
        if (!controller.signal.aborted) {
          watchError = toUserMessage(e);
        }
      }
    })();
  }

  function deselectTask() {
    if (watchController) {
      watchController.abort();
      watchController = null;
    }
    selectedTask = null;
    watchError = null;
  }

  function handleNewTask() {
    goto(resolve("/"));
  }

  function formattedDate(dateStr: string): string {
    try {
      const d = new Date(dateStr);
      const localeTag = $locale === "pt-BR" ? "pt-BR" : "en-US";
      return d.toLocaleDateString(localeTag, {
        month: "short",
        day: "numeric",
      });
    } catch {
      return dateStr;
    }
  }

  function subtaskLabel(count: number): string {
    if (count === 1) {
      return translate("tasks.subtask.one", $locale);
    }
    return translate("tasks.subtask.other", $locale, { count });
  }

  function descriptionPreview(desc: string): string {
    if (!desc) return "";
    return desc.length > 100 ? desc.slice(0, 100) + "..." : desc;
  }

  onDestroy(() => {
    watchController?.abort();
  });
</script>

<div class="flex h-[calc(100vh-3.5rem)]">
  <div
    class="flex w-full flex-col transition-all duration-300 {selectedTask
      ? 'lg:w-[45%]'
      : 'lg:w-full'}"
  >
    <div class="flex items-center justify-between px-4 py-3 lg:px-6">
      <HarpyHeading tag="h1" class="text-2xl text-cream"
        >{translate("tasks.heading", $locale)}</HarpyHeading
      >
      <button
        onclick={handleNewTask}
        class="inline-flex cursor-pointer items-center gap-2 rounded-md bg-talon-gold px-4 py-2 font-body text-sm font-medium text-obsidian transition-all hover:bg-talon-gold-bright"
      >
        <Plus class="size-4" />
        {translate("tasks.newTask", $locale)}
      </button>
    </div>

    {#if loading}
      <div class="flex flex-1 items-center justify-center">
        <div class="flex items-center gap-2 text-crown-ash">
          <Loader2 class="size-5 animate-spin" />
          <span class="font-body text-sm"
            >{translate("tasks.loading", $locale)}</span
          >
        </div>
      </div>
    {:else if loadError}
      <div class="flex flex-1 items-center justify-center">
        <div class="text-center">
          <AlertTriangle class="mx-auto mb-3 size-10 text-red-400" />
          <p class="font-body text-sm text-red-400">
            {translate("tasks.loadError", $locale)}
          </p>
          <p class="mt-1 font-mono text-xs text-crown-ash">{loadError}</p>
          <button
            onclick={() => fetchTasks()}
            class="mt-4 rounded-md border border-plumage px-4 py-2 font-body text-sm text-crown-ash transition-colors hover:border-talon-gold hover:text-talon-gold"
          >
            {translate("tasks.retry", $locale)}
          </button>
        </div>
      </div>
    {:else if tasks.length === 0}
      <div class="flex flex-1 flex-col items-center justify-center px-4">
        <div class="mb-4 text-talon-gold">
          <Bot class="size-12" />
        </div>
        <HarpyHeading tag="h2" class="mb-2 text-center text-xl text-cream">
          {translate("tasks.empty.title", $locale)}
        </HarpyHeading>
        <p class="mb-6 max-w-md text-center font-body text-sm text-crown-ash">
          {translate("tasks.empty.description", $locale)}
        </p>
        <button
          onclick={handleNewTask}
          class="inline-flex cursor-pointer items-center gap-2 rounded-lg bg-talon-gold px-6 py-3 font-heading text-lg font-bold text-obsidian transition-all hover:scale-105 hover:bg-talon-gold-bright"
        >
          <Sparkles class="size-5" />
          {translate("tasks.empty.cta", $locale)}
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
                    class="truncate font-heading text-base font-semibold text-cream"
                  >
                    {task.title}
                  </p>
                  {#if task.description}
                    <p
                      class="mt-1 line-clamp-2 font-body text-sm text-crown-ash"
                    >
                      {descriptionPreview(task.description)}
                    </p>
                  {/if}
                </div>
                <StatusBadge status={task.status} pulsing={true} />
              </div>
              <div
                class="mt-2 flex items-center gap-4 font-mono text-[10px] tracking-wider text-crown-ash-dark uppercase"
              >
                <span>{formattedDate(task.createdAt)}</span>
                {#if task.subtasks?.length}
                  <span>{subtaskLabel(task.subtasks.length)}</span>
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
      {#if watchError}
        <div class="border-b border-red-500/20 bg-red-500/10 px-4 py-2">
          <p class="font-mono text-xs text-red-400">{watchError}</p>
        </div>
      {/if}
      <TaskDetail task={selectedTask} onclose={deselectTask} />
    </div>
  {/if}
</div>
