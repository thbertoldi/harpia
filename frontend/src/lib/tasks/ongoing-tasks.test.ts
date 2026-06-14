import { describe, expect, it } from "vitest";
import { SubtaskStatus, TaskStatus, type Subtask, type Task } from "$lib/rpc";
import {
  classifyAwaitingFeedbackTask,
  formatElapsed,
  getCurrentSubtask,
  getOngoingSection,
  groupOngoingTasks,
  isOngoingTask,
} from "./ongoing-tasks";

function makeSubtask(
  overrides: Partial<Subtask> & { id: string; description: string },
): Subtask {
  return {
    taskId: "task-1",
    status: SubtaskStatus.PENDING,
    assignedAgentId: "",
    createdAt: "2025-01-01T00:00:00Z",
    updatedAt: "2025-01-01T00:00:00Z",
    ...overrides,
  } as Subtask;
}

function makeTask(overrides: Partial<Task> & { id: string }): Task {
  return {
    tenantId: "dev",
    workspaceId: "ws-1",
    title: "Task",
    description: "",
    status: TaskStatus.PENDING,
    subtasks: [],
    createdAt: "2025-01-01T00:00:00Z",
    updatedAt: "2025-01-01T00:00:00Z",
    ...overrides,
  } as Task;
}

describe("isOngoingTask", () => {
  it("includes active lifecycle statuses", () => {
    expect(isOngoingTask(makeTask({ id: "1", status: TaskStatus.PLANNING }))).toBe(
      true,
    );
    expect(
      isOngoingTask(makeTask({ id: "2", status: TaskStatus.IN_PROGRESS })),
    ).toBe(true);
    expect(
      isOngoingTask(
        makeTask({ id: "3", status: TaskStatus.AWAITING_FEEDBACK }),
      ),
    ).toBe(true);
  });

  it("excludes terminal and pending tasks", () => {
    expect(isOngoingTask(makeTask({ id: "4", status: TaskStatus.PENDING }))).toBe(
      false,
    );
    expect(
      isOngoingTask(makeTask({ id: "5", status: TaskStatus.COMPLETED })),
    ).toBe(false);
    expect(isOngoingTask(makeTask({ id: "6", status: TaskStatus.FAILED }))).toBe(
      false,
    );
    expect(
      isOngoingTask(makeTask({ id: "7", status: TaskStatus.CANCELLED })),
    ).toBe(false);
  });
});

describe("classifyAwaitingFeedbackTask", () => {
  it("routes to Gate 1 when no subtasks have completed", () => {
    const task = makeTask({
      id: "gate-1",
      status: TaskStatus.AWAITING_FEEDBACK,
      subtasks: [
        makeSubtask({
          id: "s1",
          description: "Plan review",
          status: SubtaskStatus.AWAITING_FEEDBACK,
        }),
      ],
    });

    expect(classifyAwaitingFeedbackTask(task)).toBe("awaiting_approval");
  });

  it("routes to Gate 2 when at least one subtask completed", () => {
    const task = makeTask({
      id: "gate-2",
      status: TaskStatus.AWAITING_FEEDBACK,
      subtasks: [
        makeSubtask({
          id: "s1",
          description: "Research",
          status: SubtaskStatus.COMPLETED,
        }),
        makeSubtask({
          id: "s2",
          description: "Draft review",
          status: SubtaskStatus.AWAITING_FEEDBACK,
        }),
      ],
    });

    expect(classifyAwaitingFeedbackTask(task)).toBe("awaiting_review");
  });
});

describe("groupOngoingTasks", () => {
  it("maps tasks into the four ongoing sections", () => {
    const tasks = [
      makeTask({ id: "planning", status: TaskStatus.PLANNING }),
      makeTask({ id: "running", status: TaskStatus.IN_PROGRESS }),
      makeTask({
        id: "gate-1",
        status: TaskStatus.AWAITING_FEEDBACK,
        subtasks: [
          makeSubtask({
            id: "s1",
            description: "Approve plan",
            status: SubtaskStatus.AWAITING_FEEDBACK,
          }),
        ],
      }),
      makeTask({
        id: "gate-2",
        status: TaskStatus.AWAITING_FEEDBACK,
        subtasks: [
          makeSubtask({
            id: "s1",
            description: "Research",
            status: SubtaskStatus.COMPLETED,
          }),
          makeSubtask({
            id: "s2",
            description: "Review draft",
            status: SubtaskStatus.AWAITING_FEEDBACK,
          }),
        ],
      }),
      makeTask({ id: "done", status: TaskStatus.COMPLETED }),
    ];

    const grouped = groupOngoingTasks(tasks);

    expect(grouped.planning.map((task) => task.id)).toEqual(["planning"]);
    expect(grouped.in_progress.map((task) => task.id)).toEqual(["running"]);
    expect(grouped.awaiting_approval.map((task) => task.id)).toEqual(["gate-1"]);
    expect(grouped.awaiting_review.map((task) => task.id)).toEqual(["gate-2"]);
  });
});

describe("getOngoingSection", () => {
  it("returns null for non-ongoing tasks", () => {
    expect(
      getOngoingSection(makeTask({ id: "done", status: TaskStatus.COMPLETED })),
    ).toBeNull();
  });
});

describe("getCurrentSubtask", () => {
  it("prefers in-progress, then awaiting feedback, then pending", () => {
    const task = makeTask({
      id: "task",
      subtasks: [
        makeSubtask({
          id: "done",
          description: "Done",
          status: SubtaskStatus.COMPLETED,
        }),
        makeSubtask({
          id: "active",
          description: "Running step",
          status: SubtaskStatus.IN_PROGRESS,
        }),
        makeSubtask({
          id: "next",
          description: "Next",
          status: SubtaskStatus.PENDING,
        }),
      ],
    });

    expect(getCurrentSubtask(task)?.id).toBe("active");
  });
});

describe("formatElapsed", () => {
  it("formats elapsed durations", () => {
    const now = Date.parse("2025-01-01T02:15:00Z");
    expect(formatElapsed("2025-01-01T00:00:00Z", now)).toBe("2h 15m");
    expect(formatElapsed("2025-01-01T02:10:00Z", now)).toBe("5m");
    expect(formatElapsed("2025-01-01T02:14:30Z", now)).toBe("<1m");
  });
});
