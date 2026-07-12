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
  feeds: [],
  linkedinMode: "oauth",
  oauthCredentialId: "",
  imageProvider: "noop",
  imageModel: "dall-e-3",
  imageDefaultSize: "1024x1024",
  imageDefaultQuality: "standard",
};

describe("integration executor installations", () => {
  it("maps demo executor SKU keys to integration kinds", () => {
    expect(integrationKindForSkuKey("rss-news-feed")).toBe("rss");
    expect(integrationKindForSkuKey("linkedin-publish")).toBe("linkedin");
    expect(integrationKindForSkuKey("image-asset-generator")).toBe("image");
    expect(integrationKindForSkuKey("generic-server")).toBeNull();
  });

  it("builds RSS config JSON from trimmed feed entries", () => {
    const config = buildIntegrationConfigJSON("rss", {
      ...BASE_VALUES,
      feeds: [" https://example.com/feed.xml ", "", "https://news.example/rss"],
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

  it("builds image generator config without credential fields", () => {
    const config = buildIntegrationConfigJSON("image", {
      ...BASE_VALUES,
      imageProvider: "openai",
      imageModel: " dall-e-3 ",
      imageDefaultSize: "1792x1024",
      imageDefaultQuality: "hd",
    });

    expect(JSON.parse(config)).toEqual({
      provider: "openai",
      model: "dall-e-3",
      default_size: "1792x1024",
      default_quality: "hd",
    });
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
        feeds: ["https://example.com/feed.xml"],
      }),
    ).toBeNull();
  });

  it("rejects malformed feed URLs", () => {
    expect(
      validateIntegrationForm("rss", {
        ...BASE_VALUES,
        feeds: ["not-a-url"],
      }),
    ).toBe("rssFeedInvalidUrl");
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
      feeds: ["https://example.com/feed.xml", "https://news.example/rss"],
      linkedinMode: "oauth",
      oauthCredentialId: "",
      imageProvider: "noop",
      imageModel: "dall-e-3",
      imageDefaultSize: "1024x1024",
      imageDefaultQuality: "standard",
    });
  });

  it("hydrates the credential-free image generator form from a persisted installation", () => {
    const card: DemoIntegrationCard = {
      kind: "image",
      sku: create(ExecutorSKUSchema, {
        id: "sku-image",
        key: "image-asset-generator",
        displayName: "Image Asset Generator",
        kind: ExecutorKind.INTEGRATION,
      }),
      installation: create(ExecutorInstallationSchema, {
        id: "inst-image",
        tenantId: "tenant-1",
        executorSkuId: "sku-image",
        kind: ExecutorKind.INTEGRATION,
        displayName: "OpenAI image generator",
        enabled: true,
        detail: {
          case: "integration",
          value: create(IntegrationInstallationSchema, {
            connectionStatus: ConnectionStatus.CONNECTED,
            configJson:
              '{"provider":"openai","model":"dall-e-3","default_size":"1792x1024","default_quality":"hd"}',
          }),
        },
      }),
      connectionStatus: ConnectionStatus.CONNECTED,
      configured: true,
    };

    expect(formValuesFromCard(card)).toMatchObject({
      displayName: "OpenAI image generator",
      imageProvider: "openai",
      imageModel: "dall-e-3",
      imageDefaultSize: "1792x1024",
      imageDefaultQuality: "hd",
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
    expect(
      formKeyForCard({
        kind: "rss",
        sku: create(ExecutorSKUSchema, {
          id: "sku-rss",
          key: "rss-news-feed",
        }),
        localFormKey: "new:rss-news-feed:4",
        connectionStatus: ConnectionStatus.UNSPECIFIED,
        configured: false,
      }),
    ).toBe("new:rss-news-feed:4");
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
