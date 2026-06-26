import { expect, test } from "@playwright/test";
import { loginAsAna, loginAsPlatformEngineer } from "./fixtures/personas";

test("Ana can open the M6 inbox surface", async ({ page, baseURL }) => {
  await loginAsAna(page, baseURL);
  await page.goto("/inbox");

  await expect(page.getByRole("heading", { name: "Needs you" })).toBeVisible();
  await expect(
    page.getByText("All caught up — your plans are running smoothly."),
  ).toBeVisible();
});

test("Platform Engineer can open tenant settings", async ({
  page,
  baseURL,
}) => {
  await loginAsPlatformEngineer(page, baseURL);
  await page.goto("/admin/settings");

  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
});
