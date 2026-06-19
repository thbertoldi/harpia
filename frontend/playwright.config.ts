import { defineConfig, devices } from "@playwright/test";

const port = Number(process.env.E2E_PORT ?? "5173");
const host = "127.0.0.1";
const baseURL = `http://${host}:${port}`;

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL,
    trace: "retain-on-failure",
  },
  webServer: {
    // E2E runs without a live API; opt into dev mock fallback explicitly (issue #205).
    command: `PUBLIC_DEV_LOGIN_ENABLED=true PUBLIC_ZITADEL_CLIENT_ID=e2e-client VITE_ALLOW_MOCK_FALLBACK=true bun run dev --host ${host} --port ${port}`,
    url: baseURL,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
