export const ONGOING_TASKS_POLL_INTERVAL_MS = 30_000;

export type TaskPoller = {
  stop: () => void;
};

export function startTaskPoller(
  fetchTasks: () => void | Promise<void>,
  intervalMs = ONGOING_TASKS_POLL_INTERVAL_MS,
): TaskPoller {
  const timerId = setInterval(() => {
    void fetchTasks();
  }, intervalMs);

  return {
    stop: () => clearInterval(timerId),
  };
}
