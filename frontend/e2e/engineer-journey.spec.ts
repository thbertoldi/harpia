import { expect, test, type Page, type Route } from "@playwright/test";
import {
  ConnectionStatus,
  ExecutorKind as CatalogExecutorKind,
} from "../src/lib/gen/harpia/executors/v1/executors_pb";
import {
  ApprovalRequestStatus,
  ElicitationTimeoutBehavior,
  ExecutorKind as PlanExecutorKind,
  PlanConfigurationStatus,
  PublishApprovalMode,
} from "../src/lib/gen/harpia/plans/v1/plans_pb";
import {
  ThreadMessageKind,
  ThreadMessageRole,
} from "../src/lib/gen/harpia/chat/v1/chat_pb";
import { ManagedBy } from "../src/lib/gen/harpia/llm_config/v1/llm_config_pb";
import { loginAsPlatformEngineer } from "./fixtures/personas";

const RSS_SKU = {
  $typeName: "harpia.executors.v1.ExecutorSKU",
  id: "sku-rss-news-feed",
  key: "rss-news-feed",
  displayName: "RSS News Feed",
  description: "Fetches configured RSS or Atom feeds.",
  kind: CatalogExecutorKind.INTEGRATION,
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
  kind: CatalogExecutorKind.INTEGRATION,
  compatibility: {
    inputArtifactTypeKeys: ["harpia.artifacts.v1.LinkedInPostDraft"],
    outputArtifactTypeKeys: ["harpia.artifacts.v1.PublishConfirmation"],
    connectionType: "oauth_linkedin",
  },
  createdAt: "2026-06-01T00:00:00Z",
  updatedAt: "2026-06-01T00:00:00Z",
};

const LINKEDIN_PLAN_TEMPLATE = {
  $typeName: "harpia.plans.v1.PlanTemplate",
  id: "template-news-to-social-post",
  key: "news-to-social-post",
  name: "News to Social Post",
  description: "Fetch news, write a draft, adapt for LinkedIn, and publish.",
  vertical: "creator-economy",
  version: 1,
  steps: [
    {
      $typeName: "harpia.plans.v1.PlanStep",
      id: "step-fetch-news",
      key: "fetch-news",
      title: "Fetch News",
      description: "Collect curated articles for the configured date range.",
      inputArtifactTypeId: "harpia.artifacts.v1.DateRange",
      outputArtifactTypeId: "harpia.artifacts.v1.NewsList",
      defaultExecutorSkuKey: "rss-news-feed",
      executorRequirement: {
        executorKind: PlanExecutorKind.INTEGRATION,
        requiredCapabilities: [],
        connectionType: "rss_feed",
      },
    },
    {
      $typeName: "harpia.plans.v1.PlanStep",
      id: "step-write-draft",
      key: "write-draft",
      title: "Write Draft",
      description: "Synthesize a platform-neutral newsletter draft.",
      inputArtifactTypeId: "harpia.artifacts.v1.NewsList",
      outputArtifactTypeId: "harpia.artifacts.v1.TextDraft",
      defaultExecutorSkuKey: "newsletter-writer-senior",
      executorRequirement: {
        executorKind: PlanExecutorKind.AGENT,
        requiredCapabilities: [],
        connectionType: "",
      },
    },
    {
      $typeName: "harpia.plans.v1.PlanStep",
      id: "step-adapt-linkedin",
      key: "adapt-for-linkedin",
      title: "Adapt for LinkedIn",
      description: "Transform the draft into a LinkedIn-specific post.",
      inputArtifactTypeId: "harpia.artifacts.v1.TextDraft",
      outputArtifactTypeId: "harpia.artifacts.v1.LinkedInPostDraft",
      defaultExecutorSkuKey: "linkedin-voice-senior",
      executorRequirement: {
        executorKind: PlanExecutorKind.AGENT,
        requiredCapabilities: [],
        connectionType: "",
      },
    },
    {
      $typeName: "harpia.plans.v1.PlanStep",
      id: "step-publish-linkedin",
      key: "publish-linkedin",
      title: "Publish LinkedIn",
      description: "Publish the adapted post to LinkedIn.",
      inputArtifactTypeId: "harpia.artifacts.v1.LinkedInPostDraft",
      outputArtifactTypeId: "harpia.artifacts.v1.PublishConfirmation",
      defaultExecutorSkuKey: "linkedin-publish",
      executorRequirement: {
        executorKind: PlanExecutorKind.INTEGRATION,
        requiredCapabilities: [],
        connectionType: "oauth_linkedin",
      },
    },
  ],
  edges: [
    {
      $typeName: "harpia.plans.v1.PlanStepDependency",
      fromStepKey: "fetch-news",
      toStepKey: "write-draft",
    },
    {
      $typeName: "harpia.plans.v1.PlanStepDependency",
      fromStepKey: "write-draft",
      toStepKey: "adapt-for-linkedin",
    },
    {
      $typeName: "harpia.plans.v1.PlanStepDependency",
      fromStepKey: "adapt-for-linkedin",
      toStepKey: "publish-linkedin",
    },
  ],
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
  const body = route.request().postDataBuffer();
  if (!body || body.length === 0) return {};

  const raw = body.toString("utf8");
  if (raw.trimStart().startsWith("{")) {
    return JSON.parse(raw) as Record<string, unknown>;
  }

  if (body.length >= 5) {
    const payloadLength = body.readUInt32BE(1);
    if (payloadLength > 0 && body.length >= 5 + payloadLength) {
      const payload = body.subarray(5, 5 + payloadLength).toString("utf8");
      return JSON.parse(payload) as Record<string, unknown>;
    }
  }

  return {};
}

function integrationPayload(request: Record<string, unknown>) {
  const integration = request.integration;
  if (integration && typeof integration === "object") {
    return integration as Record<string, unknown>;
  }

  const initialDetail = request.initialDetail;
  if (initialDetail && typeof initialDetail === "object") {
    const detail = initialDetail as Record<string, unknown>;
    if (detail.value && typeof detail.value === "object") {
      return detail.value as Record<string, unknown>;
    }
  }

  const detail = request.detail;
  if (detail && typeof detail === "object") {
    const detailRecord = detail as Record<string, unknown>;
    if (detailRecord.value && typeof detailRecord.value === "object") {
      return detailRecord.value as Record<string, unknown>;
    }
  }

  return {};
}

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === "object"
    ? (value as Record<string, unknown>)
    : {};
}

function parseParameterValues(raw: unknown): Record<string, unknown> {
  if (typeof raw !== "string" || raw.trim() === "") return {};
  try {
    const parsed = JSON.parse(raw);
    return asRecord(parsed);
  } catch {
    return {};
  }
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
  const createCounters = new Map<string, number>();

  const nextInstallationId = (skuKey: string): string => {
    const next = (createCounters.get(skuKey) ?? 0) + 1;
    createCounters.set(skuKey, next);
    return `inst-${skuKey}-${next}`;
  };

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
        const id = nextInstallationId(sku.key);
        const installation = {
          $typeName: "harpia.executors.v1.ExecutorInstallation",
          id,
          tenantId: "dev",
          executorSkuId: sku.id,
          kind: CatalogExecutorKind.INTEGRATION,
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
        installations[id] = installation;
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

async function installLLMConfigStub(page: Page) {
  const configs: Record<string, Record<string, unknown>> = {};

  const configFor = (
    provider: string,
    request: Record<string, unknown> = {},
  ) => ({
    $typeName: "harpia.llm_config.v1.LLMProviderConfigMetadata",
    id: `llm-${provider}`,
    tenantId: "dev",
    provider,
    defaultModel:
      typeof request.defaultModel === "string" && request.defaultModel
        ? request.defaultModel
        : provider === "deepseek"
          ? "deepseek-v4-flash"
          : "",
    allowedModels: Array.isArray(request.allowedModels)
      ? request.allowedModels
      : [],
    hasKey: true,
    managedBy: ManagedBy.TENANT_SELF,
    kekVersion: "e2e",
    lastRotatedAt: "2026-06-26T15:00:00Z",
    createdAt: "2026-06-26T15:00:00Z",
    updatedAt: "2026-06-26T15:00:00Z",
  });

  await page.route(
    "**/harpia.llm_config.v1.LLMConfigService/*",
    async (route) => {
      const url = new URL(route.request().url());
      const method = url.pathname.split("/").at(-1) ?? "";
      const request = requestJSON(route);

      if (method === "GetLLMProviderConfigs") {
        await route.fulfill({
          status: 200,
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ configs: Object.values(configs) }),
        });
        return;
      }

      if (method === "SetLLMProviderConfig") {
        const provider =
          typeof request.provider === "string" ? request.provider : "deepseek";
        const config = configFor(provider, request);
        configs[provider] = config;
        await route.fulfill({
          status: 200,
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ config }),
        });
        return;
      }

      if (method === "RotateLLMProviderConfigKey") {
        const provider =
          typeof request.provider === "string" ? request.provider : "deepseek";
        const config = configFor(provider, request);
        configs[provider] = config;
        await route.fulfill({
          status: 200,
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ config }),
        });
        return;
      }

      if (method === "DeleteLLMProviderConfig") {
        const provider =
          typeof request.provider === "string" ? request.provider : "";
        delete configs[provider];
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
          message: `Unhandled LLMConfigService method: ${method}`,
        }),
      });
    },
  );

  return { configs };
}

async function installPlanServiceStub(page: Page) {
  const tenantId = "dev";
  const planConfigurationId = "config-linkedin-demo";
  const originThreadId = "thread-linkedin-demo";
  let sequenceNumber = 1n;
  let configuration: Record<string, unknown> = {
    $typeName: "harpia.plans.v1.PlanConfiguration",
    id: planConfigurationId,
    tenantId,
    workspaceId: "",
    planTemplateId: LINKEDIN_PLAN_TEMPLATE.id,
    planTemplateVersion: LINKEDIN_PLAN_TEMPLATE.version,
    status: PlanConfigurationStatus.DRAFT,
    originThreadId,
    seedArtifacts: [],
    slotBindings: [],
    overseerBindings: [],
    createdAt: "2026-06-26T15:00:00Z",
    updatedAt: "2026-06-26T15:00:00Z",
  };
  const approvalRequest: Record<string, unknown> = {
    $typeName: "harpia.plans.v1.ApprovalRequest",
    id: "approval-linkedin-demo",
    tenantId,
    stepExecutionId: "step-exec-publish-linkedin",
    planExecutionId: "plan-exec-linkedin-demo",
    planStepKey: "publish-linkedin",
    inputArtifactId: "artifact-linkedin-draft",
    status: ApprovalRequestStatus.PENDING,
    decisionReason: "",
    requestedAt: "2026-06-26T16:00:00Z",
    planConfigurationId,
    threadId: originThreadId,
  };

  function materializeConfiguration(
    request: Record<string, unknown>,
  ): Record<string, unknown> {
    const params = parseParameterValues(request.parameterValuesJson);
    const sourceGroup =
      typeof params.source_group === "string" ? params.source_group : "";
    const approvalMode =
      params.approval_mode === "auto_publish"
        ? PublishApprovalMode.AUTO_PUBLISH
        : PublishApprovalMode.REQUIRE_APPROVAL;
    const slotBindings = [
      {
        $typeName: "harpia.plans.v1.SlotBinding",
        stepKey: "fetch-news",
        executorKind: PlanExecutorKind.INTEGRATION,
        executorSkuId: "sku-rss-news-feed",
        executorInstallationId: sourceGroup || "inst-rss-news-feed-1",
      },
      {
        $typeName: "harpia.plans.v1.SlotBinding",
        stepKey: "write-draft",
        executorKind: PlanExecutorKind.AGENT,
        executorSkuId: "sku-newsletter-writer-senior",
        executorInstallationId: "inst-newsletter-writer",
      },
      {
        $typeName: "harpia.plans.v1.SlotBinding",
        stepKey: "adapt-for-linkedin",
        executorKind: PlanExecutorKind.AGENT,
        executorSkuId: "sku-linkedin-voice-senior",
        executorInstallationId: "inst-linkedin-voice",
      },
      {
        $typeName: "harpia.plans.v1.SlotBinding",
        stepKey: "publish-linkedin",
        executorKind: PlanExecutorKind.INTEGRATION,
        executorSkuId: "sku-linkedin-publish",
        executorInstallationId: "inst-linkedin-publish-1",
      },
    ];

    return {
      parameterValuesJson:
        typeof request.parameterValuesJson === "string"
          ? request.parameterValuesJson
          : "",
      slotBindings,
      behaviorPolicies: {
        $typeName: "harpia.plans.v1.PlanBehaviorPolicies",
        elicitationTimeoutBehavior:
          ElicitationTimeoutBehavior.PAUSE_UNTIL_ANSWERED,
        elicitationTimeoutHours: 48,
        publishApprovalMode: approvalMode,
      },
    };
  }

  const threadMessages: Record<string, unknown>[] = [
    threadMessage({
      id: "message-binding-matrix",
      tenantId,
      threadId: originThreadId,
      role: ThreadMessageRole.SYSTEM,
      kind: ThreadMessageKind.ASSISTANT_PROMPT,
      text: "Choose the executors for this LinkedIn newsletter plan.",
      payloadJson: JSON.stringify({
        state: "BINDING_MATRIX",
        policies_set: true,
        rows: [
          matrixRow({
            stepKey: "fetch-news",
            title: "Fetch news",
            input: "DateRange",
            output: "NewsList",
            currentExecutorId: "inst-rss-news-feed-1",
            options: [
              matrixOption({
                id: "inst-rss-news-feed-1",
                label: "Sports RSS headlines",
                sublabel: "Folha Esporte and Hacker News front page configured",
                value: "sports-rss",
                priceBrl: 0.5,
              }),
            ],
          }),
          matrixRow({
            stepKey: "write-draft",
            title: "Write newsletter draft",
            input: "NewsList",
            output: "TextDraft",
            currentExecutorId: "inst-newsletter-writer",
            options: [
              matrixOption({
                id: "inst-newsletter-writer",
                label: "Newsletter Writer",
                sublabel: "DeepSeek tenant key",
                value: "newsletter-writer",
                priceBrl: 2,
              }),
            ],
          }),
          matrixRow({
            stepKey: "adapt-for-linkedin",
            title: "Adapt for LinkedIn",
            input: "TextDraft",
            output: "LinkedInPostDraft",
            currentExecutorId: "inst-linkedin-voice",
            options: [
              matrixOption({
                id: "inst-linkedin-voice",
                label: "LinkedIn Voice",
                sublabel: "Platform Engineer voice",
                value: "linkedin-voice",
                priceBrl: 1.5,
              }),
            ],
          }),
          matrixRow({
            stepKey: "publish-linkedin",
            title: "Publish LinkedIn",
            input: "LinkedInPostDraft",
            output: "PublishConfirmation",
            currentExecutorId: "inst-linkedin-publish-1",
            options: [
              matrixOption({
                id: "inst-linkedin-publish-1",
                label: "LinkedIn approval-only",
                sublabel: "Creates approval request before any publish",
                value: "linkedin-approval-only",
                priceBrl: 0,
              }),
            ],
          }),
        ],
      }),
    }),
  ];

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

  await page.route("**/harpia.plans.v1.PlanService/*", async (route) => {
    const url = new URL(route.request().url());
    const method = url.pathname.split("/").at(-1) ?? "";
    const request = requestJSON(route);

    if (method === "ListPlanTemplates") {
      await fulfillStream(route, {
        planTemplates: [LINKEDIN_PLAN_TEMPLATE],
        nextPageToken: "",
      });
      return;
    }

    if (method === "GetPlanTemplate" || method === "GetPlanTemplateByKey") {
      await fulfillUnary(route, {
        planTemplate: LINKEDIN_PLAN_TEMPLATE,
      });
      return;
    }

    if (method === "CreatePlanConfiguration") {
      const materialized = materializeConfiguration(request);
      configuration = {
        ...configuration,
        planTemplateId:
          typeof request.planTemplateId === "string"
            ? request.planTemplateId
            : LINKEDIN_PLAN_TEMPLATE.id,
        status:
          typeof request.status === "number"
            ? request.status
            : PlanConfigurationStatus.DRAFT,
        seedArtifacts: [],
        slotBindings: materialized.slotBindings,
        overseerBindings: Array.isArray(request.overseerBindings)
          ? request.overseerBindings
          : [],
        behaviorPolicies: materialized.behaviorPolicies,
        schedule: asRecord(request).schedule,
        parameterValuesJson: materialized.parameterValuesJson,
        updatedAt: "2026-06-26T15:01:00Z",
      };
      await fulfillUnary(route, { planConfiguration: configuration });
      return;
    }

    if (method === "GetPlanConfiguration") {
      await fulfillUnary(route, { planConfiguration: configuration });
      return;
    }

    if (method === "UpdatePlanConfiguration") {
      const materialized = materializeConfiguration(request);
      configuration = {
        ...configuration,
        status:
          typeof request.status === "number"
            ? request.status
            : configuration.status,
        seedArtifacts: [],
        slotBindings: materialized.slotBindings,
        overseerBindings: Array.isArray(request.overseerBindings)
          ? request.overseerBindings
          : [],
        behaviorPolicies: materialized.behaviorPolicies,
        schedule: request.schedule,
        parameterValuesJson: materialized.parameterValuesJson,
        updatedAt: "2026-06-26T15:02:00Z",
      };
      await fulfillUnary(route, { planConfiguration: configuration });
      return;
    }

    if (method === "ListPlanConfigurations") {
      await fulfillStream(route, {
        planConfigurations: [configuration],
        nextPageToken: "",
      });
      return;
    }

    if (method === "ListApprovalRequests") {
      await fulfillUnary(route, {
        approvalRequests: [approvalRequest],
        nextPageToken: "",
      });
      return;
    }

    if (method === "WatchApprovalRequests") {
      await fulfillStream(route, { approvalRequests: [approvalRequest] });
      return;
    }

    if (method === "GetApprovalRequest") {
      await fulfillUnary(route, { approvalRequest });
      return;
    }

    if (method === "RespondToApprovalRequest") {
      const approved = request.approved === true;
      approvalRequest.status = approved
        ? ApprovalRequestStatus.APPROVED
        : ApprovalRequestStatus.REJECTED;
      approvalRequest.decisionReason =
        typeof request.reason === "string" ? request.reason : "";
      approvalRequest.decidedAt = "2026-06-26T16:05:00Z";
      await fulfillUnary(route, { approvalRequest });
      return;
    }

    if (method === "ListElicitations") {
      await fulfillUnary(route, { elicitations: [], nextPageToken: "" });
      return;
    }

    if (method === "WatchElicitations") {
      await fulfillStream(route, { elicitations: [] });
      return;
    }

    await route.fulfill({
      status: 404,
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        message: `Unhandled PlanService method: ${method}`,
      }),
    });
  });

  await page.route("**/harpia.chat.v1.ThreadService/*", async (route) => {
    const url = new URL(route.request().url());
    const method = url.pathname.split("/").at(-1) ?? "";
    const request = requestJSON(route);

    if (method === "CreateThread") {
      await fulfillUnary(route, {
        thread: {
          $typeName: "harpia.chat.v1.Thread",
          id: originThreadId,
          tenantId,
          title:
            typeof request.title === "string"
              ? request.title
              : "Weekly Newsletter (LinkedIn)",
          status: 1,
          createdAt: "2026-06-26T15:00:00Z",
          updatedAt: "2026-06-26T15:00:00Z",
        },
      });
      return;
    }

    if (method === "GetThread") {
      await fulfillUnary(route, {
        thread: {
          $typeName: "harpia.chat.v1.Thread",
          id: originThreadId,
          tenantId,
          title: "News to Social Post",
          status: 1,
          createdAt: "2026-06-26T15:00:00Z",
          updatedAt: "2026-06-26T15:00:00Z",
        },
      });
      return;
    }

    if (method === "ListThreadMessages") {
      await fulfillUnary(route, {
        messages: threadMessages,
        nextPageToken: "",
      });
      return;
    }

    if (method === "WatchThreadMessages") {
      await fulfillStream(route, { messages: [] });
      return;
    }

    if (method === "AppendThreadMessage") {
      const message = threadMessage({
        id: `message-${String(sequenceNumber + 1n)}`,
        tenantId,
        threadId:
          typeof request.threadId === "string"
            ? request.threadId
            : originThreadId,
        executionId:
          typeof request.executionId === "string" ? request.executionId : "",
        role: threadRoleFromRequest(request.role) ?? ThreadMessageRole.SYSTEM,
        kind:
          threadKindFromRequest(request.kind) ??
          ThreadMessageKind.ASSISTANT_TEXT,
        text: typeof request.text === "string" ? request.text : "",
        payloadJson:
          typeof request.payloadJson === "string" ? request.payloadJson : "{}",
      });
      threadMessages.push(message);
      await fulfillUnary(route, { message });
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

  return {
    get configuration() {
      return configuration;
    },
    approvalRequest,
  };

  function threadMessage(input: {
    id: string;
    tenantId: string;
    threadId: string;
    role: number;
    kind: number;
    text: string;
    payloadJson: string;
    executionId?: string;
  }): Record<string, unknown> {
    const message = {
      $typeName: "harpia.chat.v1.ThreadMessage",
      id: input.id,
      tenantId: input.tenantId,
      threadId: input.threadId,
      executionId: input.executionId ?? "",
      role: input.role,
      kind: input.kind,
      text: input.text,
      payloadJson: input.payloadJson,
      authorUserId: "",
      sequenceNumber: String(sequenceNumber),
      createdAt: "2026-06-26T15:00:00Z",
    };
    sequenceNumber += 1n;
    return message;
  }
}

function threadRoleFromRequest(value: unknown): number | null {
  if (typeof value === "number") return value;
  if (typeof value !== "string") return null;
  if (value.endsWith("_OVERSEER")) return ThreadMessageRole.OVERSEER;
  if (value.endsWith("_AGENT")) return ThreadMessageRole.AGENT;
  if (value.endsWith("_SYSTEM")) return ThreadMessageRole.SYSTEM;
  return null;
}

function threadKindFromRequest(value: unknown): number | null {
  if (typeof value === "number") return value;
  if (typeof value !== "string") return null;
  if (value.endsWith("_USER_TEXT")) return ThreadMessageKind.USER_TEXT;
  if (value.endsWith("_ASSISTANT_TEXT"))
    return ThreadMessageKind.ASSISTANT_TEXT;
  if (value.endsWith("_CONFIGURATION_SAVED"))
    return ThreadMessageKind.CONFIGURATION_SAVED;
  if (value.endsWith("_CONFIGURATION_STARTED"))
    return ThreadMessageKind.CONFIGURATION_STARTED;
  if (value.endsWith("_ASSISTANT_PROMPT"))
    return ThreadMessageKind.ASSISTANT_PROMPT;
  if (value.endsWith("_USER_SELECTION"))
    return ThreadMessageKind.USER_SELECTION;
  if (value.endsWith("_STEP_REBOUND")) return ThreadMessageKind.STEP_REBOUND;
  if (value.endsWith("_SCHEDULE_SET")) return ThreadMessageKind.SCHEDULE_SET;
  if (value.endsWith("_RUN_STARTED")) return ThreadMessageKind.RUN_STARTED;
  if (value.endsWith("_RUN_COMPLETED")) return ThreadMessageKind.RUN_COMPLETED;
  if (value.endsWith("_RUN_FAILED")) return ThreadMessageKind.RUN_FAILED;
  if (value.endsWith("_STEP_STARTED")) return ThreadMessageKind.STEP_STARTED;
  if (value.endsWith("_STEP_BOUND")) return ThreadMessageKind.STEP_BOUND;
  if (value.endsWith("_ELICITATION_RAISED"))
    return ThreadMessageKind.ELICITATION_RAISED;
  if (value.endsWith("_ELICITATION_ANSWERED"))
    return ThreadMessageKind.ELICITATION_ANSWERED;
  if (value.endsWith("_APPROVAL_RAISED"))
    return ThreadMessageKind.APPROVAL_RAISED;
  if (value.endsWith("_APPROVAL_DECIDED"))
    return ThreadMessageKind.APPROVAL_DECIDED;
  return null;
}

function matrixRow(input: {
  stepKey: string;
  title: string;
  input: string;
  output: string;
  currentExecutorId?: string;
  options: Record<string, unknown>[];
}) {
  return {
    step_key: input.stepKey,
    step_title: input.title,
    contracts: {
      input: input.input,
      output: input.output,
    },
    options: input.options,
    current_executor_id: input.currentExecutorId ?? "",
    current_overseer_id: "user-platform-engineer",
    current_overseer_label: "Platform Engineer",
  };
}

function matrixOption(input: {
  id: string;
  label: string;
  sublabel: string;
  value: string;
  priceBrl: number;
}) {
  return {
    id: input.id,
    label: input.label,
    sublabel: input.sublabel,
    value: input.value,
    price_brl: input.priceBrl,
  };
}

async function installAgentCatalogFallbackStub(page: Page) {
  await page.route("**/harpia.agents.v1.AgentService/*", async (route) => {
    const url = new URL(route.request().url());
    const method = url.pathname.split("/").at(-1) ?? "";

    if (method === "ListAgentTypes") {
      const payload = new TextEncoder().encode(
        JSON.stringify({
          agentTypes: [
            {
              id: "email-drafter",
              name: "email-drafter",
              displayName: "Email Drafter",
              description:
                "Drafts concise, context-aware email responses for overseer review.",
              capabilitiesText: "email, writing, drafting",
            },
          ],
          nextPageToken: "",
        }),
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
  test.setTimeout(120_000);

  await loginAsPlatformEngineer(page, baseURL);
  await installAgentCatalogFallbackStub(page);
  const executorApi = await installExecutorApiStub(page);
  const llmApi = await installLLMConfigStub(page);
  const planApi = await installPlanServiceStub(page);

  await page.goto("/admin/settings#llm-providers");
  await expect(page.getByTestId("provider-card-deepseek")).toBeVisible();
  await page.getByTestId("key-input-deepseek").fill("deepseek-e2e-key");
  await page.getByTestId("save-key-deepseek").click();
  await expect(page.getByTestId("key-masked-deepseek")).toBeVisible();
  expect(llmApi.configs.deepseek).toMatchObject({
    provider: "deepseek",
    defaultModel: "deepseek-v4-flash",
    hasKey: true,
    managedBy: ManagedBy.TENANT_SELF,
  });

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

  const rssCards = page.getByTestId(/integration-card-.*rss-news-feed/);
  await expect(rssCards).toHaveCount(1);
  await page.getByRole("button", { name: "Add feed group" }).click();
  await expect(rssCards).toHaveCount(2);
  const rssCard = page.getByTestId("integration-card-new:rss-news-feed:0");
  await expect(rssCard).toBeVisible();
  await rssCard.getByRole("button", { name: /Add feed/i }).click();
  await rssCard.getByRole("button", { name: /Add feed/i }).click();
  const feedInputs = rssCard.getByPlaceholder("https://example.com/feed.xml");
  await feedInputs
    .nth(0)
    .fill("https://feeds.folha.uol.com.br/esporte/rss091.xml");
  await feedInputs.nth(1).fill("https://hnrss.org/frontpage");
  await rssCard.getByRole("button", { name: "Save configuration" }).click();
  await expect(
    page.getByTestId("integration-saved-inst-rss-news-feed-1"),
  ).toBeVisible();
  const savedRss = Object.values(executorApi.installations).find(
    (entry) => entry.executorSkuId === RSS_SKU.id,
  );
  expect(savedRss?.integration).toMatchObject({
    connectionStatus: ConnectionStatus.CONNECTED,
    configJson: JSON.stringify({
      feeds: [
        "https://feeds.folha.uol.com.br/esporte/rss091.xml",
        "https://hnrss.org/frontpage",
      ],
    }),
  });

  const linkedinCard = page.getByTestId(
    "integration-card-new:linkedin-publish:0",
  );
  await expect(linkedinCard).toBeVisible();
  await linkedinCard.locator("select").selectOption("approval_only");
  await linkedinCard
    .getByRole("button", { name: "Save configuration" })
    .click();
  await expect(
    page.getByTestId("integration-saved-inst-linkedin-publish-1"),
  ).toBeVisible();
  const savedLinkedIn = Object.values(executorApi.installations).find(
    (entry) => entry.executorSkuId === LINKEDIN_SKU.id,
  );
  expect(savedLinkedIn?.integration).toMatchObject({
    connectionStatus: ConnectionStatus.CONNECTED,
    configJson: JSON.stringify({ mode: "approval_only" }),
  });

  await page.goto("/new");
  await page.getByRole("button", { name: /News to Social Post/i }).click();
  await expect(page).toHaveURL(/\/chat\/thread-linkedin-demo/);
  await expect(page.getByText(/4 of 4 bound/i)).toBeVisible();

  const slotBindings = planApi.configuration.slotBindings as Record<
    string,
    unknown
  >[];
  expect(
    slotBindings.map((binding) => [
      binding.stepKey,
      binding.executorInstallationId,
    ]),
  ).toEqual([
    ["fetch-news", "inst-rss-news-feed-1"],
    ["write-draft", "inst-newsletter-writer"],
    ["adapt-for-linkedin", "inst-linkedin-voice"],
    ["publish-linkedin", "inst-linkedin-publish-1"],
  ]);
  const fetchNewsBinding = slotBindings.find(
    (binding) => binding.stepKey === "fetch-news",
  );
  expect([PlanExecutorKind.INTEGRATION, "EXECUTOR_KIND_INTEGRATION"]).toContain(
    fetchNewsBinding?.executorKind,
  );
  const policies = planApi.configuration.behaviorPolicies as Record<
    string,
    unknown
  >;
  expect([
    ElicitationTimeoutBehavior.PAUSE_UNTIL_ANSWERED,
    "ELICITATION_TIMEOUT_BEHAVIOR_PAUSE_UNTIL_ANSWERED",
  ]).toContain(policies.elicitationTimeoutBehavior);
  expect([
    PublishApprovalMode.REQUIRE_APPROVAL,
    "PUBLISH_APPROVAL_MODE_REQUIRE_APPROVAL",
  ]).toContain(policies.publishApprovalMode);

  await expect(page.getByRole("link", { name: /Inbox 1/i })).toBeVisible();

  await page.goto("/admin/agents");
  await expect(page.getByRole("row", { name: /Email Drafter/i })).toBeVisible({
    timeout: 15_000,
  });
  await page.getByRole("row", { name: /Email Drafter/i }).click();
  await expect(
    page.getByRole("button", { name: "Bind to agent" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Run test invocation" }),
  ).toBeVisible();
});
