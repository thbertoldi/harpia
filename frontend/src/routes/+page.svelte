<script lang="ts">
  import { ArrowRight, Loader2, LayoutDashboard } from "lucide-svelte";
  import { goto } from "$app/navigation";
  import { resolve } from "$app/paths";
  import TaskInput from "$lib/components/TaskInput.svelte";
  import ChatMessage from "$lib/components/ChatMessage.svelte";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import { createTask, watchTask } from "$lib/client";
  import type { Task } from "$lib/types";
  import { TaskStatus } from "$lib/types";

  let taskCreated = $state(false);
  let task: Task | null = $state(null);
  let chatMessages = $state<
    { role: "user" | "agent" | "system"; content: string; timestamp: string }[]
  >([]);
  let loading = $state(false);
  let error = $state<string | null>(null);
  let taskId = $state<string | null>(null);
  let dismissed = $state(false);

  const exampleTasks = [
    "Create a social media post about our product launch",
    "Analyze my calendar for this week and suggest optimizations",
    "Generate a report on Q2 sales performance",
    "Draft a response to the client inquiry about pricing",
  ];

  function now() {
    return new Date().toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
    });
  }

  async function handleSubmit(text: string) {
    loading = true;
    error = null;
    dismissed = false;

    chatMessages = [{ role: "user", content: text, timestamp: now() }];

    try {
      const res = await createTask({
        tenantId: "default",
        workspaceId: "default",
        title: text.slice(0, 80),
        description: text,
      });

      task = res.task;
      taskId = res.task.id;
      taskCreated = true;
      chatMessages = [
        ...chatMessages,
        {
          role: "agent",
          content: `Task created: "${res.task.title}". Opening dashboard...`,
          timestamp: now(),
        },
      ];

      // Start streaming updates
      watchTask(
        "default",
        res.task.id,
        (updated) => {
          task = updated;
        },
        (err) => {
          error = err.message;
        },
      );

      // Redirect to the task dashboard
      goto(resolve(`/tasks#${res.task.id}`));
    } catch (e) {
      error = e instanceof Error ? e.message : "Failed to create task";
      chatMessages = [
        ...chatMessages,
        { role: "system", content: `Error: ${error}`, timestamp: now() },
      ];
    } finally {
      loading = false;
    }
  }

  function viewDashboard() {
    if (taskId) {
      goto(resolve(`/tasks#${taskId}`));
    } else {
      goto(resolve("/tasks"));
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      taskCreated = false;
      task = null;
      taskId = null;
      chatMessages = [];
      error = null;
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="flex flex-1">
  <!-- Chat area -->
  <div
    class="flex flex-1 flex-col min-h-0 {taskCreated
      ? 'max-w-[60%]'
      : ''} transition-all duration-500"
  >
    {#if !taskCreated}
      <!-- Landing state -->
      <div class="flex flex-1 flex-col items-center justify-center px-4">
        <div class="mb-4 text-talon-gold">
          <svg class="size-12" viewBox="0 0 48 48" fill="none">
            <path
              d="M24 4L6 18v12l18 14 18-14V18L24 4z"
              stroke="currentColor"
              stroke-width="2"
              fill="none"
            />
            <path
              d="M24 16l-6 4v6l6 4 6-4v-6l-6-4z"
              fill="currentColor"
              opacity="0.3"
            />
          </svg>
        </div>

        <HarpyHeading tag="h1" class="text-5xl font-bold mb-3 text-center">
          What do you want to get done?
        </HarpyHeading>

        <p class="font-body text-lg text-crown-ash mb-10 text-center">
          Describe your task in natural language. Harpia handles the rest.
        </p>

        <div class="w-full mb-12">
          <TaskInput onsubmit={handleSubmit} disabled={loading} />
        </div>

        {#if loading}
          <div class="flex items-center gap-2 text-crown-ash">
            <Loader2 class="size-4 animate-spin" />
            <span class="font-body text-sm">Creating task...</span>
          </div>
        {/if}

        <div class="mt-8 w-full max-w-2xl">
          <p
            class="font-mono text-xs uppercase tracking-widest text-crown-ash mb-3"
          >
            Try asking
          </p>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
            {#each exampleTasks as example (example)}
              <button
                onclick={() => handleSubmit(example)}
                disabled={loading}
                class="flex items-center gap-2 rounded-lg border border-plumage bg-obsidian-light/60 px-4 py-3 text-left font-body text-sm text-cream transition-all hover:border-talon-gold hover:bg-obsidian-light disabled:opacity-50"
              >
                <ArrowRight class="size-3.5 shrink-0 text-talon-gold" />
                <span>{example}</span>
              </button>
            {/each}
          </div>
        </div>

        <a
          href={resolve("/tasks")}
          class="mt-6 inline-flex items-center gap-2 font-body text-sm text-crown-ash transition-colors hover:text-talon-gold"
        >
          <LayoutDashboard class="size-4" />
          View Task Dashboard
        </a>
      </div>
    {:else}
      <!-- Chat state -->
      <div class="flex flex-1 flex-col min-h-0">
        {#if !dismissed && task}
          <div
            class="mx-4 mt-4 rounded-lg border border-talon-gold/30 bg-talon-gold/10 p-4 lg:mx-6"
          >
            <div class="flex items-center justify-between gap-3">
              <div>
                <p class="font-heading text-base font-semibold text-talon-gold">
                  Task created!
                </p>
                <p class="mt-1 font-body text-sm text-cream">{task.title}</p>
              </div>
              <div class="flex items-center gap-2">
                <button
                  onclick={viewDashboard}
                  class="inline-flex cursor-pointer items-center gap-2 rounded-md bg-talon-gold px-4 py-2 font-body text-sm font-medium text-obsidian transition-all hover:bg-talon-gold-bright"
                >
                  <LayoutDashboard class="size-4" />
                  Open Dashboard
                </button>
                <button
                  onclick={() => (dismissed = true)}
                  class="rounded-md p-2 text-crown-ash transition-colors hover:text-cream"
                  aria-label="Dismiss"
                >
                  <span class="text-lg">&times;</span>
                </button>
              </div>
            </div>
          </div>
        {/if}

        <div class="flex-1 overflow-y-auto px-4 py-6 lg:px-6">
          <div class="mx-auto max-w-2xl space-y-4">
            {#each chatMessages as msg (msg.timestamp + msg.content.slice(0, 10))}
              <ChatMessage
                role={msg.role}
                content={msg.content}
                timestamp={msg.timestamp}
              />
            {/each}
          </div>
        </div>

        <div class="border-t border-plumage p-4">
          <div class="mx-auto max-w-2xl">
            <TaskInput
              onsubmit={handleSubmit}
              disabled={loading}
              placeholder="Follow up or refine your task..."
            />
          </div>
        </div>
      </div>
    {/if}
  </div>

  <!-- Task status panel (visible when task is created) -->
  {#if taskCreated && task}
    <div
      class="w-[40%] border-l border-plumage bg-obsidian-light overflow-y-auto transition-all duration-500"
    >
      <div
        class="flex items-center justify-between border-b border-plumage px-4 py-3"
      >
        <HarpyHeading tag="h3" class="text-base">Task Status</HarpyHeading>
        <button
          onclick={viewDashboard}
          class="inline-flex cursor-pointer items-center gap-1.5 rounded-md px-3 py-1.5 font-body text-xs text-talon-gold transition-colors hover:bg-talon-gold/10"
        >
          <LayoutDashboard class="size-3.5" />
          Full Dashboard
        </button>
      </div>

      <div class="p-4 space-y-4">
        <div>
          <p
            class="font-mono text-[10px] uppercase tracking-widest text-crown-ash mb-1"
          >
            Task
          </p>
          <p class="font-body text-sm font-medium text-cream">{task.title}</p>
        </div>

        <div>
          <p
            class="font-mono text-[10px] uppercase tracking-widest text-crown-ash mb-1"
          >
            Description
          </p>
          <p class="font-body text-sm text-crown-ash">{task.description}</p>
        </div>

        <div>
          <p
            class="font-mono text-[10px] uppercase tracking-widest text-crown-ash mb-2"
          >
            Progress
          </p>
          <div class="h-2 w-full rounded-full bg-plumage overflow-hidden">
            {#if task.status === TaskStatus.COMPLETED}
              <div
                class="h-full w-full rounded-full bg-green-500 transition-all duration-500"
              ></div>
            {:else if task.status === TaskStatus.FAILED}
              <div
                class="h-full w-full rounded-full bg-red-500 transition-all duration-500"
              ></div>
            {:else if task.status === TaskStatus.IN_PROGRESS}
              <div
                class="h-full w-3/4 rounded-full bg-talon-gold animate-pulse transition-all duration-500"
              ></div>
            {:else if task.status === TaskStatus.PLANNING}
              <div
                class="h-full w-1/4 rounded-full bg-talon-gold animate-pulse transition-all duration-500"
              ></div>
            {:else if task.status === TaskStatus.AWAITING_FEEDBACK}
              <div
                class="h-full w-1/2 rounded-full bg-yellow-500 transition-all duration-500"
              ></div>
            {:else if task.status === TaskStatus.PENDING}
              <div
                class="h-full w-[10%] rounded-full bg-crown-ash transition-all duration-500"
              ></div>
            {:else if task.status === TaskStatus.CANCELLED}
              <div
                class="h-full w-full rounded-full bg-crown-ash transition-all duration-500"
              ></div>
            {/if}
          </div>
        </div>

        {#if task.subtasks && task.subtasks.length > 0}
          <div>
            <p
              class="font-mono text-[10px] uppercase tracking-widest text-crown-ash mb-2"
            >
              Subtasks
            </p>
            <ul class="space-y-2">
              {#each task.subtasks as subtask (subtask.id)}
                <li
                  class="flex items-start gap-2 rounded-lg border border-plumage bg-obsidian/60 p-2.5"
                >
                  <span
                    class="mt-0.5 h-2 w-2 shrink-0 rounded-full {subtask.status ===
                    4
                      ? 'bg-green-500'
                      : subtask.status === 5
                        ? 'bg-red-500'
                        : subtask.status === 2
                          ? 'bg-talon-gold'
                          : 'bg-crown-ash'}"
                  ></span>
                  <div class="min-w-0 flex-1">
                    <p class="font-body text-xs text-cream">
                      {subtask.description}
                    </p>
                    {#if subtask.assignedAgentId}
                      <p class="font-mono text-[10px] text-crown-ash mt-0.5">
                        Agent: {subtask.assignedAgentId.slice(0, 8)}...
                      </p>
                    {/if}
                  </div>
                </li>
              {/each}
            </ul>
          </div>
        {/if}

        {#if error}
          <div class="rounded-lg border border-red-500/20 bg-red-500/10 p-3">
            <p class="font-mono text-xs text-red-600 dark:text-red-400">
              {error}
            </p>
          </div>
        {/if}
      </div>
    </div>
  {/if}
</div>
