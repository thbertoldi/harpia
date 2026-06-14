import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  ONGOING_TASKS_POLL_INTERVAL_MS,
  startTaskPoller,
} from "./task-refresh";

describe("startTaskPoller", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("uses the default 30 second interval", () => {
    expect(ONGOING_TASKS_POLL_INTERVAL_MS).toBe(30_000);
  });

  it("calls fetch on each interval and stops when requested", () => {
    const fetchTasks = vi.fn();

    const poller = startTaskPoller(fetchTasks, 1_000);

    expect(fetchTasks).not.toHaveBeenCalled();

    vi.advanceTimersByTime(1_000);
    expect(fetchTasks).toHaveBeenCalledTimes(1);

    vi.advanceTimersByTime(2_000);
    expect(fetchTasks).toHaveBeenCalledTimes(3);

    poller.stop();

    vi.advanceTimersByTime(5_000);
    expect(fetchTasks).toHaveBeenCalledTimes(3);
  });
});
