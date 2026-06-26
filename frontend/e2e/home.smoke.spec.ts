import { expect, test } from "@playwright/test";
import { loginAsAna } from "./fixtures/personas";

test("Ana can open the task entry page", async ({ page, baseURL }) => {
  await loginAsAna(page, baseURL);

  await page.goto("/");

  await expect(
    page.getByRole("heading", { name: "What do you want to get done?" }),
  ).toBeVisible();
  await expect(
    page.getByRole("banner").getByText("Ana", { exact: true }),
  ).toBeVisible();
  await expect(page.getByRole("textbox")).toBeVisible();
});
