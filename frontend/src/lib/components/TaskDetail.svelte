<script lang="ts">
  import {
    ArrowLeft,
    Clock,
    UserCheck,
    Bot,
    AlertTriangle,
  } from "lucide-svelte";
  import { TaskStatus, SubtaskStatus, type Task, type Subtask } from "$lib/rpc";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import StatusBadge from "$lib/components/StatusBadge.svelte";
  import { locale, translate } from "$lib/i18n";
  import { formatLocaleDateTime } from "$lib/i18n/format";

  let {
    task,
    onclose,
  }: {
    task: Task | null;
    onclose?: () => void;
  } = $props();

  function subtaskStatusIcon(status: SubtaskStatus): string {
    const icons: Record<number, string> = {
      [SubtaskStatus.PENDING]: "bg-crown-ash",
      [SubtaskStatus.IN_PROGRESS]: "bg-talon-gold",
      [SubtaskStatus.AWAITING_FEEDBACK]: "bg-amber-400",
      [SubtaskStatus.COMPLETED]: "bg-green-500",
      [SubtaskStatus.FAILED]: "bg-red-500",
    };
    return icons[status] ?? "bg-crown-ash";
  }

  function subtaskStatusLabel(status: SubtaskStatus): string {
    const labels: Record<number, string> = {
      [SubtaskStatus.PENDING]: translate("taskStatus.pending", $locale),
      [SubtaskStatus.IN_PROGRESS]: translate("taskStatus.inProgress", $locale),
      [SubtaskStatus.AWAITING_FEEDBACK]: translate(
        "taskStatus.awaitingFeedback",
        $locale,
      ),
      [SubtaskStatus.COMPLETED]: translate("taskStatus.completed", $locale),
      [SubtaskStatus.FAILED]: translate("taskStatus.failed", $locale),
    };
    return labels[status] ?? translate("common.unknown", $locale);
  }

  function isActiveSubtask(subtask: Subtask): boolean {
    return subtask.status === SubtaskStatus.IN_PROGRESS;
  }

  function agentStatusMessage(): string {
    if (!task) return "";
    switch (task.status) {
      case TaskStatus.PLANNING:
        return translate("taskDetail.agentStatus.planning", $locale);
      case TaskStatus.IN_PROGRESS: {
        const activeSubtasks = task.subtasks.filter(
          (s) => s.status === SubtaskStatus.IN_PROGRESS,
        );
        if (activeSubtasks.length > 0) {
          const agentId = activeSubtasks[0].assignedAgentId || "";
          return translate("taskDetail.agentStatus.executing", $locale, {
            agent: agentId
              ? `${agentId.slice(0, 12)}`
              : translate("common.agent", $locale),
          });
        }
        return translate("taskDetail.agentStatus.inProgress", $locale);
      }
      case TaskStatus.AWAITING_FEEDBACK:
        return translate("taskDetail.agentStatus.awaitingFeedback", $locale);
      case TaskStatus.COMPLETED:
        return translate("taskDetail.agentStatus.completed", $locale);
      case TaskStatus.FAILED:
        return translate("taskDetail.agentStatus.failed", $locale);
      case TaskStatus.CANCELLED:
        return translate("taskDetail.agentStatus.cancelled", $locale);
      default:
        return translate("taskDetail.agentStatus.pending", $locale);
    }
  }

  function formattedDate(dateStr: string): string {
    return formatLocaleDateTime(dateStr, $locale);
  }
</script>

{#if !task}
  <div class="flex h-full items-center justify-center">
    <div class="text-center">
      <Bot class="mx-auto mb-3 size-10 text-crown-ash" />
      <p class="font-body text-sm text-crown-ash">
        {translate("taskDetail.empty", $locale)}
      </p>
    </div>
  </div>
{:else}
  <div class="flex h-full flex-col">
    <div class="flex items-center justify-between px-4 py-3">
      <div class="flex items-center gap-3">
        {#if onclose}
          <button
            onclick={onclose}
            class="rounded-md p-1 text-crown-ash transition-colors hover:text-cream"
            aria-label={translate("taskDetail.close", $locale)}
          >
            <ArrowLeft class="size-4" />
          </button>
        {/if}
        <HarpyHeading tag="h3" class="text-base text-cream"
          >{translate("taskDetail.title", $locale)}</HarpyHeading
        >
      </div>
      <StatusBadge status={task.status} pulsing={true} />
    </div>

    <div class="flex-1 overflow-y-auto px-4 py-4">
      <div class="space-y-5">
        <div class="transition-opacity duration-300">
          <HarpyHeading tag="h2" class="mb-2 text-xl text-cream"
            >{task.title}</HarpyHeading
          >
          <p class="font-body text-sm leading-relaxed text-crown-ash">
            {task.description}
          </p>
        </div>

        <div class="flex items-center gap-6 font-mono text-xs text-crown-ash">
          <span class="flex items-center gap-1.5">
            <Clock class="size-3.5" />
            {formattedDate(task.createdAt)}
          </span>
          <span class="flex items-center gap-1.5">
            <Bot class="size-3.5" />
            {translate("tasks.subtask.other", $locale, {
              count: task.subtasks?.length ?? 0,
            })}
          </span>
        </div>

        {#if task.status !== TaskStatus.PENDING && task.status !== TaskStatus.CANCELLED}
          <div
            class="rounded-lg border border-plumage bg-obsidian-light p-4 transition-all duration-300"
          >
            <p
              class="mb-1 font-mono text-[10px] tracking-widest text-crown-ash uppercase"
            >
              {translate("taskDetail.agentStatus.heading", $locale)}
            </p>
            <div class="flex items-center gap-2">
              {#if task.status === TaskStatus.PLANNING || task.status === TaskStatus.IN_PROGRESS}
                <span class="flex h-2.5 w-2.5">
                  <span
                    class="absolute inline-flex h-2.5 w-2.5 animate-ping rounded-full bg-talon-gold opacity-75"
                  ></span>
                  <span
                    class="relative inline-flex h-2.5 w-2.5 rounded-full bg-talon-gold"
                  ></span>
                </span>
              {:else if task.status === TaskStatus.COMPLETED}
                <UserCheck class="size-4 text-green-400" />
              {:else if task.status === TaskStatus.FAILED}
                <AlertTriangle class="size-4 text-red-400" />
              {/if}
              <span
                class="font-body text-sm text-cream"
                class:animate-pulse={task.status === TaskStatus.IN_PROGRESS}
              >
                {agentStatusMessage()}
              </span>
            </div>
          </div>
        {/if}

        {#if task.status === TaskStatus.AWAITING_FEEDBACK}
          <div
            class="rounded-lg border border-amber-500/30 bg-amber-500/10 p-4 transition-all duration-300"
          >
            <p
              class="mb-2 font-mono text-[10px] tracking-widest text-amber-400 uppercase"
            >
              {translate("taskDetail.feedbackRequired.heading", $locale)}
            </p>
            <p class="font-body text-sm text-cream">
              {translate("taskDetail.feedbackRequired.description", $locale)}
            </p>
            <div class="mt-3 flex gap-2">
              <button
                class="rounded-md bg-talon-gold px-4 py-2 font-body text-sm text-obsidian transition-colors hover:bg-talon-gold-bright"
              >
                {translate("feedback.approve", $locale)}
              </button>
              <button
                class="rounded-md border border-crown-ash px-4 py-2 font-body text-sm text-crown-ash transition-colors hover:border-cream hover:text-cream"
              >
                {translate("feedback.modify", $locale)}
              </button>
              <button
                class="rounded-md border border-red-500/30 px-4 py-2 font-body text-sm text-red-400 transition-colors hover:border-red-400"
              >
                {translate("feedback.reject", $locale)}
              </button>
            </div>
          </div>
        {/if}

        {#if task.subtasks && task.subtasks.length > 0}
          <div>
            <p
              class="mb-3 font-mono text-[10px] tracking-widest text-crown-ash uppercase"
            >
              {translate("common.subtasks", $locale)}
            </p>
            <div class="space-y-0">
              {#each task.subtasks as subtask, i (subtask.id)}
                <div
                  class="relative ml-4 flex items-start gap-3 py-2.5 pl-4 transition-all duration-300"
                  class:border-l-2={isActiveSubtask(subtask)}
                  class:border-l-talon-gold={isActiveSubtask(subtask)}
                  class:border-l-transparent={!isActiveSubtask(subtask)}
                  class:opacity-60={subtask.status === SubtaskStatus.COMPLETED}
                >
                  <div class="flex flex-col items-center">
                    <span
                      class="mt-0.5 block h-2.5 w-2.5 shrink-0 rounded-full {subtaskStatusIcon(
                        subtask.status,
                      )} transition-colors duration-300"
                      class:animate-pulse={subtask.status ===
                        SubtaskStatus.IN_PROGRESS}
                    ></span>
                    {#if i < task.subtasks.length - 1}
                      <span class="mt-0.5 h-full w-px bg-plumage"></span>
                    {/if}
                  </div>
                  <div class="min-w-0 flex-1">
                    <div class="flex items-center gap-2">
                      <p
                        class="font-body text-sm {isActiveSubtask(subtask)
                          ? 'text-talon-gold'
                          : 'text-cream'} transition-colors duration-300"
                        class:font-medium={isActiveSubtask(subtask)}
                      >
                        {subtask.description}
                      </p>
                    </div>
                    <div class="mt-1 flex items-center gap-2">
                      <span
                        class="font-mono text-[10px] tracking-wider text-crown-ash uppercase"
                      >
                        {subtaskStatusLabel(subtask.status)}
                      </span>
                      {#if subtask.assignedAgentId}
                        <span class="font-mono text-[10px] text-crown-ash-dark">
                          {subtask.assignedAgentId.slice(0, 10)}...
                        </span>
                      {/if}
                    </div>
                  </div>
                </div>
              {/each}
            </div>
          </div>
        {/if}

        {#if task.status === TaskStatus.FAILED}
          <div class="rounded-lg border border-red-500/20 bg-red-500/10 p-3">
            <p
              class="mb-1 font-mono text-[10px] tracking-widest text-red-400 uppercase"
            >
              {translate("common.error", $locale)}
            </p>
            <p class="font-body text-sm text-red-300">
              {translate("taskDetail.errorDescription", $locale)}
            </p>
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}
