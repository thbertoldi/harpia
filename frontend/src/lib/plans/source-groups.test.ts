import { create } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";
import {
  ConnectionStatus,
  ExecutorInstallationSchema,
  ExecutorKind,
  IntegrationInstallationSchema,
  type ExecutorInstallation,
} from "$lib/gen/harpia/executors/v1/executors_pb";
import {
  ensureAggregateRssInstallation,
  sourceGroupInstallations,
} from "./source-groups";

function rssInstallation(
  id: string,
  displayName: string,
  feeds: string[],
): ExecutorInstallation {
  return create(ExecutorInstallationSchema, {
    id,
    tenantId: "tenant-1",
    executorSkuId: "sku-rss",
    kind: ExecutorKind.INTEGRATION,
    displayName,
    enabled: true,
    detail: {
      case: "integration",
      value: create(IntegrationInstallationSchema, {
        connectionStatus: ConnectionStatus.CONNECTED,
        configJson: JSON.stringify({ feeds }),
      }),
    },
  });
}

describe("ensureAggregateRssInstallation", () => {
  it("reuses an existing aggregate installation with the same feed set", async () => {
    const client = {
      async *listExecutorInstallations() {
        yield {
          installations: [
            rssInstallation("rss-tech", "Tech", ["https://tech.example/rss"]),
            rssInstallation("rss-business", "Business", [
              "https://business.example/rss",
            ]),
            rssInstallation("rss-existing", "Aggregate RSS", [
              "https://business.example/rss",
              "https://tech.example/rss",
            ]),
          ],
        };
      },
      async createExecutorInstallation() {
        throw new Error("should not create");
      },
    };

    await expect(
      ensureAggregateRssInstallation({
        tenantId: "tenant-1",
        selectedInstallationIds: ["rss-tech", "rss-business"],
        executorClient: client,
      }),
    ).resolves.toBe("rss-existing");
  });

  it("creates one aggregate installation with unioned feeds", async () => {
    const created: unknown[] = [];
    const client = {
      async *listExecutorInstallations() {
        yield {
          installations: [
            rssInstallation("rss-tech", "Tech", ["https://tech.example/rss"]),
            rssInstallation("rss-business", "Business", [
              "https://business.example/rss",
              "https://tech.example/rss",
            ]),
          ],
        };
      },
      async createExecutorInstallation(input: unknown) {
        created.push(input);
        return {
          installation: rssInstallation("rss-new", "Aggregate RSS", [
            "https://tech.example/rss",
            "https://business.example/rss",
          ]),
        };
      },
    };

    await expect(
      ensureAggregateRssInstallation({
        tenantId: "tenant-1",
        selectedInstallationIds: ["rss-tech", "rss-business"],
        executorClient: client,
      }),
    ).resolves.toBe("rss-new");
    expect(created).toEqual([
      {
        tenantId: "tenant-1",
        executorSkuId: "sku-rss",
        displayName: "Aggregate RSS: Tech + Business",
        enabled: true,
        initialDetail: {
          case: "integration",
          value: {
            connectionStatus: ConnectionStatus.CONNECTED,
            configJson:
              '{"feeds":["https://tech.example/rss","https://business.example/rss"]}',
          },
        },
      },
    ]);
  });
});

describe("sourceGroupInstallations", () => {
  it("keeps RSS feed integrations and removes non-source integrations", () => {
    const linkedin = create(ExecutorInstallationSchema, {
      id: "linkedin",
      tenantId: "tenant-1",
      executorSkuId: "sku-linkedin",
      kind: ExecutorKind.INTEGRATION,
      displayName: "LinkedIn Publisher",
      enabled: true,
      detail: {
        case: "integration",
        value: create(IntegrationInstallationSchema, {
          connectionStatus: ConnectionStatus.CONNECTED,
          configJson: JSON.stringify({ mode: "approval_only" }),
        }),
      },
    });

    expect(
      sourceGroupInstallations([
        rssInstallation("rss-tech", "Tech", ["https://tech.example/rss"]),
        linkedin,
      ]).map((installation) => installation.id),
    ).toEqual(["rss-tech"]);
  });
});
