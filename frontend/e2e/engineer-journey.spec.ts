import { expect, test } from "@playwright/test";
import { loginAsEngineer } from "./fixtures/personas";

test("Platform Engineer can complete agent integration journey", async ({
  page,
  baseURL,
}) => {
  await loginAsEngineer(page, baseURL);

  await page.goto("/agents");
  await expect(
    page.getByRole("heading", { name: "Agent Catalog" }),
  ).toBeVisible();
  const sampleAgentRow = page.getByRole("row", { name: /Email Drafter/i });
  await expect(sampleAgentRow).toBeVisible({ timeout: 15_000 });
  await sampleAgentRow.click();
  await expect(page.getByText("Tenant Knowledge Search")).toBeVisible();

  await page.goto("/integrations");
  await expect(
    page.getByRole("heading", { name: "Integrations" }),
  ).toBeVisible();
  // Ensure the page is hydrated before opening the add form.
  await page.getByRole("button", { name: /Filesystem/ }).click();
  await expect(page.getByText("read_file", { exact: true })).toBeVisible();
  const addServerButton = page.getByRole("button", { name: "Add Server" });
  await addServerButton.click({ force: true });
  if (!(await page.locator("#server-name").isVisible())) {
    await addServerButton.click({ force: true });
  }
  await expect(page.locator("#server-name")).toBeVisible();
  await page.locator("#server-name").fill("Engineer E2E Stub");
  await page.getByRole("button", { name: "Test Connection" }).click();
  await expect(
    page.getByText("Connection test succeeded (mock)."),
  ).toBeVisible();
  await page.getByRole("button", { name: "Register Server" }).click();

  const newServerCard = page.locator("article").filter({
    hasText: "Engineer E2E Stub",
  });
  await expect(newServerCard).toBeVisible();
  await expect(newServerCard.getByText("ping", { exact: true })).toBeVisible();

  await newServerCard
    .getByRole("button", { name: "Bind to agent" })
    .first()
    .click();
  await expect(page.getByText('Bind "ping"')).toBeVisible();

  await page.goto("/agents");
  await expect(page.getByRole("row", { name: /Email Drafter/i })).toBeVisible({
    timeout: 15_000,
  });
  await page.getByRole("row", { name: /Email Drafter/i }).click();
  const actionsPanel = page
    .locator("section")
    .filter({ hasText: "Actions" })
    .first();
  await actionsPanel.locator("select").first().selectOption({ label: "ping" });
  await actionsPanel.getByRole("button", { name: "Bind to agent" }).click();
  await expect(page.getByRole("status")).toContainText("Bound ping");

  await page.getByRole("button", { name: "Run test invocation" }).click();
  const toast = page.getByRole("status");
  await expect(toast).toContainText("task-e2e-");
  const toastText = (await toast.textContent()) ?? "";
  const taskIdMatch = toastText.match(/task-e2e-[a-z0-9]+/);
  expect(taskIdMatch).not.toBeNull();
  const taskId = taskIdMatch?.[0];
  expect(taskId).toBeTruthy();

  await page.goto("/audit");
  await page.getByPlaceholder("task-a1b2…").fill(taskId!);
  await page.getByRole("button", { name: "Apply" }).click();

  await expect(
    page.locator(
      `[data-event-type="agent.execution_completed"][data-task-id="${taskId}"]`,
    ),
  ).toBeVisible();
});
