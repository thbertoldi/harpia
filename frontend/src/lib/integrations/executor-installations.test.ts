import { create } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";
import {
  ConnectionStatus,
  ExecutorInstallationSchema,
  ExecutorKind,
  ExecutorSKUSchema,
  IntegrationInstallationSchema,
} from "$lib/gen/harpia/executors/v1/executors_pb";
import {
  buildIntegrationConfigJSON,
  connectionStatusLabelKey,
  formKeyForCard,
  formKeyForNewCard,
  formValuesFromCard,
  integrationKindForSkuKey,
  validateIntegrationForm,
  type DemoIntegrationCard,
  type DemoIntegrationFormValues,
} from "$lib/integrations/executor-installations";

const BASE_VALUES: DemoIntegrationFormValues = {
  displayName: "Demo integration",
  enabled: true,
  feedsText: "",
  linkedinMode: "oauth",
  oauthCredentialId: "",
};

describe("integration executor installations", () => {
  it("maps demo executor SKU keys to integration kinds", () => {
    expect(integrationKindForSkuKey("rss-news-feed")).toBe("rss");
    expect(integrationKindForSkuKey("linkedin-publish")).toBe("linkedin");
    expect(integrationKindForSkuKey("generic-server")).toBeNull();
  });

  it("builds RSS config JSON from trimmed feed lines", () => {
    const config = buildIntegrationConfigJSON("rss", {
      ...BASE_VALUES,
      feedsText: " https://example.com/feed.xml \n\nhttps://news.example/rss ",
    });

    expect(JSON.parse(config)).toEqual({
      feeds: ["https://example.com/feed.xml", "https://news.example/rss"],
    });
  });

  it("builds LinkedIn config JSON from the OAuth credential id", () => {
    const config = buildIntegrationConfigJSON("linkedin", {
      ...BASE_VALUES,
      oauthCredentialId: " linkedin-prod ",
    });

    expect(JSON.parse(config)).toEqual({
      mode: "oauth",
      oauth_credential_id: "linkedin-prod",
    });
  });

  it("builds LinkedIn approval-only config without an OAuth credential", () => {
    const config = buildIntegrationConfigJSON("linkedin", {
      ...BASE_VALUES,
      linkedinMode: "approval_only",
      oauthCredentialId: "",
    });

    expect(JSON.parse(config)).toEqual({
      mode: "approval_only",
    });
    expect(
      validateIntegrationForm("linkedin", {
        ...BASE_VALUES,
        linkedinMode: "approval_only",
      }),
    ).toBeNull();
  });

  it("validates required RSS and LinkedIn fields", () => {
    expect(validateIntegrationForm("rss", BASE_VALUES)).toBe(
      "rssFeedsRequired",
    );
    expect(validateIntegrationForm("linkedin", BASE_VALUES)).toBe(
      "linkedinCredentialRequired",
    );
    expect(
      validateIntegrationForm("rss", {
        ...BASE_VALUES,
        feedsText: "https://example.com/feed.xml",
      }),
    ).toBeNull();
  });

  it("hydrates form fields from a persisted RSS installation", () => {
    const card: DemoIntegrationCard = {
      kind: "rss",
      sku: create(ExecutorSKUSchema, {
        id: "sku-rss",
        key: "rss-news-feed",
        displayName: "RSS News Feed",
        kind: ExecutorKind.INTEGRATION,
      }),
      installation: create(ExecutorInstallationSchema, {
        id: "inst-rss",
        tenantId: "tenant-1",
        executorSkuId: "sku-rss",
        kind: ExecutorKind.INTEGRATION,
        displayName: "Morning feeds",
        enabled: true,
        detail: {
          case: "integration",
          value: create(IntegrationInstallationSchema, {
            connectionStatus: ConnectionStatus.CONNECTED,
            configJson:
              '{"feeds":["https://example.com/feed.xml","https://news.example/rss"]}',
          }),
        },
      }),
      connectionStatus: ConnectionStatus.CONNECTED,
      configured: true,
    };

    expect(formValuesFromCard(card)).toEqual({
      displayName: "Morning feeds",
      enabled: true,
      feedsText: "https://example.com/feed.xml\nhttps://news.example/rss",
      linkedinMode: "oauth",
      oauthCredentialId: "",
    });
  });

  it("uses installation id based form keys so many RSS groups do not collide", () => {
    const first = {
      kind: "rss",
      sku: create(ExecutorSKUSchema, { id: "sku-rss", key: "rss-news-feed" }),
      installation: create(ExecutorInstallationSchema, { id: "inst-rss-1" }),
      connectionStatus: ConnectionStatus.CONNECTED,
      configured: true,
    } as DemoIntegrationCard;
    const second = {
      ...first,
      installation: create(ExecutorInstallationSchema, { id: "inst-rss-2" }),
    } as DemoIntegrationCard;

    expect(formKeyForCard(first)).toBe("inst-rss-1");
    expect(formKeyForCard(second)).toBe("inst-rss-2");
    expect(formKeyForNewCard("rss-news-feed", 3)).toBe("new:rss-news-feed:3");
  });

  it("maps connection status values to translation keys", () => {
    expect(connectionStatusLabelKey(ConnectionStatus.CONNECTED)).toBe(
      "integrations.status.connected",
    );
    expect(connectionStatusLabelKey(ConnectionStatus.UNSPECIFIED)).toBe(
      "integrations.status.unspecified",
    );
  });
});
