import { expect, test, type Page, type Route } from "@playwright/test";
import {
  ConnectionStatus,
  ExecutorKind,
} from "../src/lib/gen/harpia/executors/v1/executors_pb";
import { loginAsPlatformEngineer } from "./fixtures/personas";

const RSS_SKU = {
  $typeName: "harpia.executors.v1.ExecutorSKU",
  id: "sku-rss-news-feed",
  key: "rss-news-feed",
  displayName: "RSS News Feed",
  description: "Fetches configured RSS or Atom feeds.",
  kind: ExecutorKind.INTEGRATION,
  compatibility: {
    inputArtifactTypeKeys: ["harpia.artifacts.v1.DateRange"],
    outputArtifactTypeKeys: ["harpia.artifacts.v1.NewsList"],
    connectionType: "rss_feed",
  },
  createdAt: "2026-06-01T00:00:00Z",
  updatedAt: "2026-06-01T00:00:00Z",
};

const LINKEDIN_SKU = {
  $typeName: "harpia.executors.v1.ExecutorSKU",
  id: "sku-linkedin-publish",
  key: "linkedin-publish",
  displayName: "LinkedIn Publish",
  description: "Publishes approved LinkedIn content.",
  kind: ExecutorKind.INTEGRATION,
  compatibility: {
    inputArtifactTypeKeys: ["harpia.artifacts.v1.LinkedInPostDraft"],
    outputArtifactTypeKeys: ["harpia.artifacts.v1.PublishConfirmation"],
    connectionType: "oauth_linkedin",
  },
  createdAt: "2026-06-01T00:00:00Z",
  updatedAt: "2026-06-01T00:00:00Z",
};

function encodeEnvelope(payload: Uint8Array, flags = 0): Uint8Array {
  const out = new Uint8Array(5 + payload.length);
  out[0] = flags;
  const view = new DataView(out.buffer, out.byteOffset, out.byteLength);
  view.setUint32(1, payload.length, false);
  out.set(payload, 5);
  return out;
}

function endStreamEnvelope(): Uint8Array {
  const payload = new TextEncoder().encode(JSON.stringify({ metadata: {} }));
  return encodeEnvelope(payload, 0x02);
}

function toStreamBody(chunks: Uint8Array[]): Buffer {
  return Buffer.concat(chunks.map((chunk) => Buffer.from(chunk)));
}

function requestJSON(route: Route): Record<string, unknown> {
  const raw = route.request().postData();
  if (!raw) return {};
  return JSON.parse(raw) as Record<string, unknown>;
}

function integrationPayload(request: Record<string, unknown>) {
  const integration = request.integration;
  return integration && typeof integration === "object"
    ? (integration as Record<string, unknown>)
    : {};
}

async function installExecutorApiStub(page: Page) {
  const entitlements = [RSS_SKU, LINKEDIN_SKU].map((sku) => ({
    $typeName: "harpia.executors.v1.ExecutorEntitlement",
    id: `ent-${sku.key}`,
    tenantId: "dev",
    executorSkuId: sku.id,
    grantedAt: "2026-06-01T00:00:00Z",
    grantedBy: "e2e",
  }));
  const installations: Record<string, Record<string, unknown>> = {};

  const fulfillUnary = async (route: Route, jsonBody: object) => {
    await route.fulfill({
      status: 200,
      headers: { "content-type": "application/json" },
      body: JSON.stringify(jsonBody),
    });
  };

  const fulfillStream = async (route: Route, jsonBody: object) => {
    const payload = new TextEncoder().encode(JSON.stringify(jsonBody));
    await route.fulfill({
      status: 200,
      headers: { "content-type": "application/connect+json" },
      body: toStreamBody([encodeEnvelope(payload), endStreamEnvelope()]),
    });
  };

  await page.route(
    "**/harpia.executors.v1.ExecutorService/*",
    async (route) => {
      const url = new URL(route.request().url());
      const method = url.pathname.split("/").at(-1) ?? "";

      if (method === "ListExecutorSKUs") {
        await fulfillStream(route, {
          executorSkus: [RSS_SKU, LINKEDIN_SKU],
          nextPageToken: "",
        });
        return;
      }

      if (method === "ListExecutorEntitlements") {
        await fulfillStream(route, { entitlements, nextPageToken: "" });
        return;
      }

      if (method === "ListExecutorInstallations") {
        await fulfillStream(route, {
          installations: Object.values(installations),
          nextPageToken: "",
        });
        return;
      }

      if (method === "CreateExecutorInstallation") {
        const request = requestJSON(route);
        const sku =
          request.executorSkuId === RSS_SKU.id ? RSS_SKU : LINKEDIN_SKU;
        const integration = integrationPayload(request);
        const now = new Date().toISOString();
        const installation = {
          $typeName: "harpia.executors.v1.ExecutorInstallation",
          id: `inst-${sku.key}`,
          tenantId: "dev",
          executorSkuId: sku.id,
          kind: ExecutorKind.INTEGRATION,
          displayName:
            typeof request.displayName === "string"
              ? request.displayName
              : sku.displayName,
          enabled: request.enabled !== false,
          createdAt: now,
          updatedAt: now,
          integration: {
            connectionStatus: ConnectionStatus.CONNECTED,
            configJson:
              typeof integration.configJson === "string"
                ? integration.configJson
                : "{}",
          },
        };
        installations[sku.id] = installation;
        await fulfillUnary(route, { installation });
        return;
      }

      if (method === "UpdateExecutorInstallation") {
        const request = requestJSON(route);
        const integration = integrationPayload(request);
        const installation = Object.values(installations).find(
          (entry) => entry.id === request.installationId,
        );
        if (!installation) {
          await route.fulfill({
            status: 404,
            headers: { "content-type": "application/json" },
            body: JSON.stringify({ message: "installation not found" }),
          });
          return;
        }

        installation.displayName =
          typeof request.displayName === "string"
            ? request.displayName
            : installation.displayName;
        installation.enabled = request.enabled !== false;
        installation.updatedAt = new Date().toISOString();
        installation.integration = {
          connectionStatus:
            request.enabled === false
              ? ConnectionStatus.DISCONNECTED
              : ConnectionStatus.CONNECTED,
          configJson:
            typeof integration.configJson === "string"
              ? integration.configJson
              : "{}",
        };
        await fulfillUnary(route, { installation });
        return;
      }

      await route.fulfill({
        status: 404,
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          message: `Unhandled ExecutorService method: ${method}`,
        }),
      });
    },
  );

  return { installations };
}

async function installAgentCatalogFallbackStub(page: Page) {
  await page.route("**/harpia.agents.v1.AgentService/*", async (route) => {
    const url = new URL(route.request().url());
    const method = url.pathname.split("/").at(-1) ?? "";

    if (method === "ListAgentTypes") {
      const payload = new TextEncoder().encode(
        JSON.stringify({ agentTypes: [], nextPageToken: "" }),
      );
      await route.fulfill({
        status: 200,
        headers: { "content-type": "application/connect+json" },
        body: toStreamBody([encodeEnvelope(payload), endStreamEnvelope()]),
      });
      return;
    }

    await route.fulfill({
      status: 404,
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        message: `Unhandled AgentService method: ${method}`,
      }),
    });
  });
}

test("Platform Engineer can complete agent integration journey", async ({
  page,
  baseURL,
}) => {
  await loginAsPlatformEngineer(page, baseURL);
  await installAgentCatalogFallbackStub(page);
  const executorApi = await installExecutorApiStub(page);

  await page.goto("/admin/agents");
  await expect(
    page.getByRole("heading", { name: "Agent Catalog" }),
  ).toBeVisible();
  const sampleAgentRow = page.getByRole("row", { name: /Email Drafter/i });
  await expect(sampleAgentRow).toBeVisible({ timeout: 15_000 });
  await sampleAgentRow.click();
  await expect(page.getByText("Tenant Knowledge Search")).toBeVisible();

  await page.goto("/admin/integrations");
  await expect(
    page.getByRole("heading", { name: "Integrations" }),
  ).toBeVisible();

  const rssCard = page.getByTestId("integration-card-rss-news-feed");
  await expect(rssCard).toBeVisible();
  await rssCard
    .locator("#feeds-rss-news-feed")
    .fill("https://example.com/feed.xml");
  await rssCard.getByRole("button", { name: "Save configuration" }).click();
  await expect(
    page.getByTestId("integration-saved-rss-news-feed"),
  ).toBeVisible();
  expect(executorApi.installations[RSS_SKU.id]?.integration).toMatchObject({
    connectionStatus: ConnectionStatus.CONNECTED,
    configJson: JSON.stringify({ feeds: ["https://example.com/feed.xml"] }),
  });

  const linkedinCard = page.getByTestId("integration-card-linkedin-publish");
  await expect(linkedinCard).toBeVisible();
  await linkedinCard
    .locator("#credential-linkedin-publish")
    .fill("linkedin-e2e-credential");
  await linkedinCard
    .getByRole("button", { name: "Save configuration" })
    .click();
  await expect(
    page.getByTestId("integration-saved-linkedin-publish"),
  ).toBeVisible();
  expect(executorApi.installations[LINKEDIN_SKU.id]?.integration).toMatchObject(
    {
      connectionStatus: ConnectionStatus.CONNECTED,
      configJson: JSON.stringify({
        oauth_credential_id: "linkedin-e2e-credential",
      }),
    },
  );

  await page.goto("/admin/agents");
  await expect(page.getByRole("row", { name: /Email Drafter/i })).toBeVisible({
    timeout: 15_000,
  });
  await page.getByRole("row", { name: /Email Drafter/i }).click();
  const actionsPanel = page
    .locator("section")
    .filter({ hasText: "Actions" })
    .first();
  await actionsPanel
    .locator("select")
    .first()
    .selectOption({ label: "read_file" });
  await actionsPanel.getByRole("button", { name: "Bind to agent" }).click();
  await expect(page.getByRole("status")).toContainText("Bound read_file");

  await page.getByRole("button", { name: "Run test invocation" }).click();
  const toast = page.getByRole("status");
  await expect(toast).toContainText("task-e2e-");
  const toastText = (await toast.textContent()) ?? "";
  const taskIdMatch = toastText.match(/task-e2e-[a-z0-9]+/);
  expect(taskIdMatch).not.toBeNull();
  const taskId = taskIdMatch?.[0];
  expect(taskId).toBeTruthy();

  await page.goto("/admin/audit");
  await page.getByPlaceholder("task-a1b2…").fill(taskId!);
  await page.getByRole("button", { name: "Apply" }).click();

  await expect(
    page.locator(
      `[data-event-type="agent.execution_completed"][data-task-id="${taskId}"]`,
    ),
  ).toBeVisible();
});
