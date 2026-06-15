<script lang="ts">
  import { ArrowRight, Loader2, LayoutDashboard } from "lucide-svelte";
  import { goto } from "$app/navigation";
  import { resolve } from "$app/paths";
  import TaskInput from "$lib/components/TaskInput.svelte";
  import ChatMessage from "$lib/components/ChatMessage.svelte";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import { requireTenantId } from "$lib/auth";
  import { toUserMessage } from "$lib/connect-errors";
  import { taskClient, TaskStatus, type Task } from "$lib/rpc";
  import { locale, translate } from "$lib/i18n";
  import { formatLocaleDateTime } from "$lib/i18n/format";

  let taskCreated = $state(false);
  let task: Task | null = $state(null);
  let chatMessages = $state<
    { role: "user" | "agent" | "system"; content: string; timestamp: string }[]
  >([]);
  let loading = $state(false);
  let error = $state<string | null>(null);
  let taskId = $state<string | null>(null);
  let dismissed = $state(false);

  const exampleTasks = $derived([
    translate("home.example.1", $locale),
    translate("home.example.2", $locale),
    translate("home.example.3", $locale),
    translate("home.example.4", $locale),
  ]);

  function now() {
    return formatLocaleDateTime(new Date(), $locale, {
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
      const res = await taskClient.createTask({
        tenantId: requireTenantId(),
        workspaceId: "default",
        title: text.slice(0, 80),
        description: text,
      });

      if (!res.task)
        throw new Error(translate("home.error.taskMissing", $locale));

      task = res.task;
      taskId = res.task.id;
      taskCreated = true;
      chatMessages = [
        ...chatMessages,
        {
          role: "agent",
          content: translate("home.chat.taskCreated", $locale, {
            title: res.task.title,
          }),
          timestamp: now(),
        },
      ];

      // Redirect to the task dashboard
      goto(resolve(`/tasks#${res.task.id}`));
    } catch (e) {
      error = toUserMessage(e);
      chatMessages = [
        ...chatMessages,
        {
          role: "system",
          content: translate("home.chat.error", $locale, {
            error: error ?? "",
          }),
          timestamp: now(),
        },
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
    class="flex min-h-0 flex-1 flex-col {taskCreated
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

        <HarpyHeading tag="h1" class="mb-3 text-center text-5xl font-bold">
          {translate("home.hero.title", $locale)}
        </HarpyHeading>

        <p class="mb-10 text-center font-body text-lg text-crown-ash">
          {translate("home.hero.subtitle", $locale)}
        </p>

        <div class="mb-12 w-full">
          <TaskInput onsubmit={handleSubmit} disabled={loading} />
        </div>

        {#if loading}
          <div class="flex items-center gap-2 text-crown-ash">
            <Loader2 class="size-4 animate-spin" />
            <span class="font-body text-sm"
              >{translate("home.creating", $locale)}</span
            >
          </div>
        {/if}

        <div class="mt-8 w-full max-w-2xl">
          <p
            class="mb-3 font-mono text-xs tracking-widest text-crown-ash uppercase"
          >
            {translate("home.tryAsking", $locale)}
          </p>
          <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
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
          {translate("home.viewDashboard", $locale)}
        </a>
      </div>
    {:else}
      <!-- Chat state -->
      <div class="flex min-h-0 flex-1 flex-col">
        {#if !dismissed && task}
          <div
            class="mx-4 mt-4 rounded-lg border border-talon-gold/30 bg-talon-gold/10 p-4 lg:mx-6"
          >
            <div class="flex items-center justify-between gap-3">
              <div>
                <p class="font-heading text-base font-semibold text-talon-gold">
                  {translate("home.taskCreated", $locale)}
                </p>
                <p class="mt-1 font-body text-sm text-cream">{task.title}</p>
              </div>
              <div class="flex items-center gap-2">
                <button
                  onclick={viewDashboard}
                  class="inline-flex cursor-pointer items-center gap-2 rounded-md bg-talon-gold px-4 py-2 font-body text-sm font-medium text-obsidian transition-all hover:bg-talon-gold-bright"
                >
                  <LayoutDashboard class="size-4" />
                  {translate("home.openDashboard", $locale)}
                </button>
                <button
                  onclick={() => (dismissed = true)}
                  class="rounded-md p-2 text-crown-ash transition-colors hover:text-cream"
                  aria-label={translate("common.dismiss", $locale)}
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
              placeholder={translate("home.followupPlaceholder", $locale)}
            />
          </div>
        </div>
      </div>
    {/if}
  </div>

  <!-- Task status panel (visible when task is created) -->
  {#if taskCreated && task}
    <div
      class="w-[40%] overflow-y-auto border-l border-plumage bg-obsidian-light transition-all duration-500"
    >
      <div
        class="flex items-center justify-between border-b border-plumage px-4 py-3"
      >
        <HarpyHeading tag="h3" class="text-base"
          >{translate("home.taskStatus", $locale)}</HarpyHeading
        >
        <button
          onclick={viewDashboard}
          class="inline-flex cursor-pointer items-center gap-1.5 rounded-md px-3 py-1.5 font-body text-xs text-talon-gold transition-colors hover:bg-talon-gold/10"
        >
          <LayoutDashboard class="size-3.5" />
          {translate("home.fullDashboard", $locale)}
        </button>
      </div>

      <div class="space-y-4 p-4">
        <div>
          <p
            class="mb-1 font-mono text-[10px] tracking-widest text-crown-ash uppercase"
          >
            {translate("common.task", $locale)}
          </p>
          <p class="font-body text-sm font-medium text-cream">{task.title}</p>
        </div>

        <div>
          <p
            class="mb-1 font-mono text-[10px] tracking-widest text-crown-ash uppercase"
          >
            {translate("common.description", $locale)}
          </p>
          <p class="font-body text-sm text-crown-ash">{task.description}</p>
        </div>

        <div>
          <p
            class="mb-2 font-mono text-[10px] tracking-widest text-crown-ash uppercase"
          >
            {translate("common.progress", $locale)}
          </p>
          <div class="h-2 w-full overflow-hidden rounded-full bg-plumage">
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
                class="h-full w-3/4 animate-pulse rounded-full bg-talon-gold transition-all duration-500"
              ></div>
            {:else if task.status === TaskStatus.PLANNING}
              <div
                class="h-full w-1/4 animate-pulse rounded-full bg-talon-gold transition-all duration-500"
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
              class="mb-2 font-mono text-[10px] tracking-widest text-crown-ash uppercase"
            >
              {translate("common.subtasks", $locale)}
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
                      <p class="mt-0.5 font-mono text-[10px] text-crown-ash">
                        {translate("common.agent", $locale)}:
                        {subtask.assignedAgentId.slice(0, 8)}...
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
