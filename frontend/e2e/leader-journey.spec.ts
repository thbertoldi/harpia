import { expect, test } from "@playwright/test";
import { loginAsAna } from "./fixtures/personas";
import { installTaskApiStub } from "./fixtures/tasks";

test("Ana submits a task and lands in the M6 inbox", async ({
  page,
  baseURL,
}) => {
  const taskApi = await installTaskApiStub(page);

  await loginAsAna(page, baseURL);
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

  await expect(page).toHaveURL(/\/inbox$/);
  await expect(page.getByRole("heading", { name: "Needs you" })).toBeVisible();
});
