import { expect, test } from "@playwright/test";
import { TaskStatus } from "../src/lib/gen/harpia/tasks/v1/tasks_pb";
import { loginAsLeader } from "./fixtures/personas";
import { installTaskApiStub } from "./fixtures/tasks";

test("Leader completes submit-to-result journey via watch stream", async ({
  page,
  baseURL,
}) => {
  const taskApi = await installTaskApiStub(page);
  const seededTask = taskApi.seedTask({
    tenantId: "dev",
    title: "Leader seeded task",
    description: "Seeded task visible in dashboard list",
  });

  await loginAsLeader(page, baseURL);
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "What do you want to get done?" }),
  ).toBeVisible();

  const taskInput = page.getByRole("textbox", {
    name: /what do you want to get done/i,
  });
  await taskInput.fill("Leader journey stub task");
  const submitButton = page.locator("form button[type='submit']").first();
  await expect(submitButton).toBeEnabled();
  await submitButton.click();

  expect(taskApi.getCreateCallCount()).toBeGreaterThan(0);

  await expect(page).toHaveURL(/\/tasks(#|%23|$)/);
  const createdTaskId = page.url().split("#").at(1);
  expect(createdTaskId).toBeTruthy();

  await expect
    .poll(() => taskApi.getListCallCount(), {
      message: "Expected TaskService.ListTasks to be called",
      timeout: 5_000,
    })
    .toBeGreaterThan(0);
  await expect(
    page.getByRole("button", { name: /Leader journey stub task/i }).first(),
  ).toBeVisible();
  await expect(page.getByText(seededTask.title)).toBeVisible();

  await page.getByRole("button", { name: /Leader journey stub task/i }).click();

  const completedTask = await taskApi.waitForTaskStatus(
    createdTaskId as string,
    TaskStatus.COMPLETED,
    { timeoutMs: 10_000, pollMs: 50 },
  );

  await expect(
    page.getByRole("heading", { name: completedTask.title }),
  ).toBeVisible();

  const watchEvents = taskApi.getWatchEventCount(completedTask.id);
  expect(watchEvents).toBeGreaterThan(0);
  expect(watchEvents).toBeGreaterThanOrEqual(3);
});
