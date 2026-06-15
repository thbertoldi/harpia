import { SubtaskStatus, TaskStatus, type Subtask, type Task } from "$lib/rpc";
import { translate, type Locale } from "$lib/i18n";

export type OngoingSectionId =
  | "planning"
  | "in_progress"
  | "awaiting_approval"
  | "awaiting_review";

export type OngoingSections = Record<OngoingSectionId, Task[]>;

export const ONGOING_SECTION_ORDER: OngoingSectionId[] = [
  "in_progress",
  "awaiting_approval",
  "awaiting_review",
  "planning",
];

export const ONGOING_SECTION_LABEL_KEYS: Record<OngoingSectionId, string> = {
  planning: "ongoing.section.planning",
  in_progress: "ongoing.section.inProgress",
  awaiting_approval: "ongoing.section.awaitingApproval",
  awaiting_review: "ongoing.section.awaitingReview",
};

const TERMINAL_STATUSES = new Set([
  TaskStatus.COMPLETED,
  TaskStatus.FAILED,
  TaskStatus.CANCELLED,
]);

export function isOngoingTask(task: Task): boolean {
  return (
    !TERMINAL_STATUSES.has(task.status) &&
    task.status !== TaskStatus.PENDING &&
    task.status !== TaskStatus.UNSPECIFIED
  );
}

export function classifyAwaitingFeedbackTask(
  task: Task,
): "awaiting_approval" | "awaiting_review" {
  const completedCount = task.subtasks.filter(
    (subtask) => subtask.status === SubtaskStatus.COMPLETED,
  ).length;

  if (completedCount === 0) {
    return "awaiting_approval";
  }

  return "awaiting_review";
}

export function getOngoingSection(task: Task): OngoingSectionId | null {
  if (!isOngoingTask(task)) {
    return null;
  }

  switch (task.status) {
    case TaskStatus.PLANNING:
      return "planning";
    case TaskStatus.IN_PROGRESS:
      return "in_progress";
    case TaskStatus.AWAITING_FEEDBACK:
      return classifyAwaitingFeedbackTask(task);
    default:
      return null;
  }
}

export function groupOngoingTasks(tasks: Task[]): OngoingSections {
  const sections: OngoingSections = {
    planning: [],
    in_progress: [],
    awaiting_approval: [],
    awaiting_review: [],
  };

  for (const task of tasks) {
    const section = getOngoingSection(task);
    if (section) {
      sections[section].push(task);
    }
  }

  return sections;
}

export function getCurrentSubtask(task: Task): Subtask | null {
  if (!task.subtasks?.length) {
    return null;
  }

  const inProgress = task.subtasks.find(
    (subtask) => subtask.status === SubtaskStatus.IN_PROGRESS,
  );
  if (inProgress) {
    return inProgress;
  }

  const awaiting = task.subtasks.find(
    (subtask) => subtask.status === SubtaskStatus.AWAITING_FEEDBACK,
  );
  if (awaiting) {
    return awaiting;
  }

  const pending = task.subtasks.find(
    (subtask) => subtask.status === SubtaskStatus.PENDING,
  );
  if (pending) {
    return pending;
  }

  return task.subtasks[task.subtasks.length - 1] ?? null;
}

export function getAgentLabel(
  task: Task,
  subtask: Subtask | null,
  locale: Locale,
): string | null {
  if (subtask?.assignedAgentId) {
    return subtask.assignedAgentId;
  }

  if (task.status === TaskStatus.PLANNING) {
    return translate("ongoing.agent.planner", locale);
  }

  return null;
}

export function formatElapsed(
  since: string,
  locale: Locale,
  now = Date.now(),
): string {
  const start = Date.parse(since);
  if (Number.isNaN(start)) {
    return translate("common.emDash", locale);
  }

  const elapsedMs = Math.max(0, now - start);
  const totalMinutes = Math.floor(elapsedMs / 60_000);
  const hours = Math.floor(totalMinutes / 60);

  if (hours > 0) {
    return translate("common.time.hoursMinutes", locale, {
      hours,
      minutes: totalMinutes % 60,
    });
  }

  if (totalMinutes > 0) {
    return translate("common.time.minutes", locale, { count: totalMinutes });
  }

  return translate("common.time.lessThanMinute", locale);
}

export function formatEstimatedCost(locale: Locale): string {
  return translate("common.emDash", locale);
}
