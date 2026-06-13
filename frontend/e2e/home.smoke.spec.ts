import { expect, test } from "@playwright/test";
import { loginAsLeader } from "./fixtures/personas";

test("Leader can open the task entry page", async ({ page, baseURL }) => {
  await loginAsLeader(page, baseURL);

  await page.goto("/");

  await expect(
    page.getByRole("heading", { name: "What do you want to get done?" }),
  ).toBeVisible();
  await expect(page.getByText("Lena Leader")).toBeVisible();
  await expect(
    page.getByRole("link", { name: /View Task Dashboard/i }),
  ).toBeVisible();
});
