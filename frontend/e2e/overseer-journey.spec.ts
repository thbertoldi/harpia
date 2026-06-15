import { expect, test } from "@playwright/test";
import { TaskStatus } from "../src/lib/gen/harpia/tasks/v1/tasks_pb";
import { loginAsOverseer } from "./fixtures/personas";
import { installOverseerJourneyMocks } from "./fixtures/tasks";

test("Overseer completes Gate 1 + Gate 2 iteration journey", async ({
  page,
  baseURL,
}) => {
  const harness = await installOverseerJourneyMocks(page, {
    scenario: "gate1_gate2_loop",
  });

  try {
    await loginAsOverseer(page, baseURL);
    await page.goto("/oversee");

    await page.getByTestId("feedback-item-fb-gate1-attempt1").click();

    await expect(page.getByTestId("feedback-question")).toContainText("Gate 1");
    await expect(page.getByTestId("feedback-option-0")).toContainText(
      "subtask_description=Draft outreach email sequence",
    );
    await expect(page.getByTestId("feedback-option-1")).toContainText(
      `suggested_agent_type=${harness.initialGate1AgentType}`,
    );
    await expect(page.getByTestId("feedback-option-2")).toContainText(
      "trust_score=0.91",
    );
    await expect(page.getByTestId("feedback-option-3")).toContainText(
      "estimated_cost_usd=0.42",
    );

    await page.getByTestId("feedback-approve").click();
    await expect(page.getByTestId("feedback-submitted")).toBeVisible();
    await expect
      .poll(async () => (await harness.getDispatchLog()).length, {
        timeout: 5_000,
      })
      .toBe(1);

    await page.getByTestId("feedback-item-fb-gate2-attempt1").click();
    await expect(page.getByTestId("feedback-question")).toContainText("Gate 2");
    await page
      .locator("#feedback-comment")
      .fill("Please tighten the CTA copy.");
    await page.getByTestId("feedback-reject-retry").click();
    await expect(page.getByTestId("feedback-submitted")).toBeVisible();

    await page.getByTestId("feedback-item-fb-gate2-attempt2").click();
    await expect(page.getByTestId("feedback-option-0")).toContainText(
      "iteration_loop=true",
    );
    await expect(page.getByTestId("feedback-option-1")).toContainText(
      "mutated_prompt_version=2",
    );
    await expect(page.getByTestId("feedback-option-2")).toContainText(
      "feedback_comment=Please tighten the CTA copy.",
    );

    await page.getByTestId("feedback-approve").click();
    await harness.waitForTaskStatus(TaskStatus.COMPLETED, 5_000);
  } finally {
    await harness.dispose();
  }
});

test("Overseer can switch agent type at Gate 1", async ({ page, baseURL }) => {
  const harness = await installOverseerJourneyMocks(page, {
    scenario: "switch_gate1_agent_type",
  });

  try {
    await loginAsOverseer(page, baseURL);
    await page.goto("/oversee");

    await page.getByTestId("feedback-item-fb-gate1-attempt1").click();
    await expect(page.getByTestId("feedback-option-1")).toContainText(
      `suggested_agent_type=${harness.initialGate1AgentType}`,
    );

    await page
      .locator("#feedback-comment")
      .fill("Switch to code agent for deterministic output.");
    await page.getByTestId("feedback-modify").click();
    await expect(page.getByTestId("feedback-submitted")).toBeVisible();

    await page.getByTestId("feedback-item-fb-gate1-attempt2").click();
    await expect(page.getByTestId("feedback-question")).toContainText(
      "Updated plan",
    );
    await expect(page.getByTestId("feedback-option-1")).toContainText(
      `suggested_agent_type=${harness.switchedGate1AgentType}`,
    );
    await expect(page.getByTestId("feedback-option-4")).toContainText(
      "feedback_comment=Switch to code agent for deterministic output.",
    );

    await page.getByTestId("feedback-approve").click();
    await expect
      .poll(async () => (await harness.getDispatchLog()).length, {
        timeout: 5_000,
      })
      .toBe(1);
    await expect(
      page.getByTestId("feedback-item-fb-gate2-attempt2"),
    ).toBeVisible();
  } finally {
    await harness.dispose();
  }
});
