import { expect, test } from "@playwright/test";
import {
  ThreadMessageKind,
  ThreadMessageRole,
} from "../src/lib/gen/harpia/chat/v1/chat_pb";
import { loginAsAna } from "./fixtures/personas";

test("Ana submits a prompt and lands in the chat workspace", async ({
  page,
  baseURL,
}) => {
  let createCallCount = 0;
  const threadId = "thread-leader-e2e";
  const tenantId = "dev";
  const messageText = "Leader journey stub task";

  await page.route("**/harpia.chat.v1.ThreadService/*", async (route) => {
    const url = new URL(route.request().url());
    const method = url.pathname.split("/").at(-1) ?? "";

    if (method === "CreateThread") {
      createCallCount += 1;
      await route.fulfill({
        status: 200,
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          thread: {
            $typeName: "harpia.chat.v1.Thread",
            id: threadId,
            tenantId,
            title: messageText,
            activePlanConfigurationId: "",
            status: 1,
            createdAt: "2026-06-26T15:00:00Z",
            updatedAt: "2026-06-26T15:00:00Z",
          },
        }),
      });
      return;
    }

    if (method === "GetThread") {
      await route.fulfill({
        status: 200,
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          thread: {
            $typeName: "harpia.chat.v1.Thread",
            id: threadId,
            tenantId,
            title: messageText,
            activePlanConfigurationId: "",
            status: 1,
            createdAt: "2026-06-26T15:00:00Z",
            updatedAt: "2026-06-26T15:00:00Z",
          },
        }),
      });
      return;
    }

    if (method === "ListThreadMessages") {
      await route.fulfill({
        status: 200,
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          messages: [
            {
              $typeName: "harpia.chat.v1.ThreadMessage",
              id: "message-leader-e2e",
              tenantId,
              threadId,
              executionId: "",
              role: ThreadMessageRole.OVERSEER,
              kind: ThreadMessageKind.USER_TEXT,
              text: messageText,
              payloadJson: "{}",
              authorUserId: "",
              sequenceNumber: "1",
              createdAt: "2026-06-26T15:00:00Z",
            },
          ],
          nextPageToken: "",
        }),
      });
      return;
    }

    if (method === "WatchThreadMessages") {
      await route.fulfill({
        status: 200,
        headers: { "content-type": "application/connect+json" },
        body: Buffer.from([
          0x02,
          0,
          0,
          0,
          15,
          ...new TextEncoder().encode('{"metadata":{}}'),
        ]),
      });
      return;
    }

    if (method === "ProposePlan") {
      await route.fulfill({
        status: 200,
        headers: { "content-type": "application/json" },
        body: JSON.stringify({}),
      });
      return;
    }

    await route.fulfill({
      status: 404,
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        message: `Unhandled ThreadService method: ${method}`,
      }),
    });
  });

  await loginAsAna(page, baseURL);
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "What do you want to get done?" }),
  ).toBeVisible();

  const taskInput = page.getByRole("textbox", {
    name: /what do you want to get done/i,
  });
  await taskInput.fill(messageText);
  const submitButton = page.locator("form button[type='submit']").first();
  await expect(submitButton).toBeEnabled();
  await submitButton.click();

  expect(createCallCount).toBeGreaterThan(0);

  await expect(page).toHaveURL(new RegExp(`/chat/${threadId}$`));
  await expect(page.getByText(messageText)).toBeVisible();
});
