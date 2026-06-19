import { create } from "@bufbuild/protobuf";
import {
  ConnectionStatus,
  ExecutorKind,
  IntegrationInstallationSchema,
  type ExecutorEntitlement,
  type ExecutorInstallation,
  type ExecutorSKU,
} from "$lib/gen/harpia/executors/v1/executors_pb";
import { executorClient } from "$lib/rpc";

export const DEMO_INTEGRATION_SKU_KEYS = [
  "rss-news-feed",
  "linkedin-publish",
] as const;

export type DemoIntegrationSkuKey = (typeof DEMO_INTEGRATION_SKU_KEYS)[number];
export type DemoIntegrationKind = "rss" | "linkedin";
export type IntegrationValidationCode =
  | "rssFeedsRequired"
  | "linkedinCredentialRequired";

export interface DemoIntegrationFormValues {
  displayName: string;
  enabled: boolean;
  feedsText: string;
  oauthCredentialId: string;
}

export interface DemoIntegrationCard {
  kind: DemoIntegrationKind;
  sku: ExecutorSKU;
  entitlement?: ExecutorEntitlement;
  installation?: ExecutorInstallation;
  connectionStatus: ConnectionStatus;
  configured: boolean;
}

export interface DemoIntegrationContext {
  cards: DemoIntegrationCard[];
}

export function isDemoIntegrationSkuKey(
  value: string,
): value is DemoIntegrationSkuKey {
  return DEMO_INTEGRATION_SKU_KEYS.includes(value as DemoIntegrationSkuKey);
}

export function integrationKindForSkuKey(
  skuKey: string,
): DemoIntegrationKind | null {
  switch (skuKey) {
    case "rss-news-feed":
      return "rss";
    case "linkedin-publish":
      return "linkedin";
    default:
      return null;
  }
}

export async function loadDemoIntegrationContext(
  tenantId: string,
): Promise<DemoIntegrationContext> {
  const [skus, entitlements, installations] = await Promise.all([
    listIntegrationSKUs(),
    listIntegrationEntitlements(tenantId),
    listIntegrationInstallations(tenantId),
  ]);

  const entitlementBySkuID = new Map(
    entitlements.map((entitlement) => [entitlement.executorSkuId, entitlement]),
  );
  const installationBySkuID = new Map<string, ExecutorInstallation>();

  for (const installation of installations) {
    if (!installationBySkuID.has(installation.executorSkuId)) {
      installationBySkuID.set(installation.executorSkuId, installation);
    }
  }

  const order = new Map(
    DEMO_INTEGRATION_SKU_KEYS.map((key, index) => [key, index]),
  );

  const cards = skus
    .filter((sku) => isDemoIntegrationSkuKey(sku.key))
    .sort(
      (left, right) =>
        (order.get(left.key as DemoIntegrationSkuKey) ?? 99) -
        (order.get(right.key as DemoIntegrationSkuKey) ?? 99),
    )
    .map((sku) => {
      const kind = integrationKindForSkuKey(sku.key);
      if (!kind) {
        throw new Error(`Unsupported integration SKU: ${sku.key}`);
      }

      const installation = installationBySkuID.get(sku.id);
      const connectionStatus = connectionStatusFromInstallation(installation);

      return {
        kind,
        sku,
        entitlement: entitlementBySkuID.get(sku.id),
        installation,
        connectionStatus,
        configured: isInstallationConfigured(installation),
      };
    });

  return { cards };
}

export function formValuesFromCard(
  card: DemoIntegrationCard,
): DemoIntegrationFormValues {
  const config = parseConfigJSON(
    card.installation?.detail.case === "integration"
      ? card.installation.detail.value.configJson
      : "",
  );

  return {
    displayName:
      card.installation?.displayName || card.sku.displayName || card.sku.key,
    enabled: card.installation?.enabled ?? true,
    feedsText: Array.isArray(config.feeds)
      ? config.feeds.filter(isString).join("\n")
      : "",
    oauthCredentialId: isString(config.oauth_credential_id)
      ? config.oauth_credential_id
      : "",
  };
}

export function validateIntegrationForm(
  kind: DemoIntegrationKind,
  values: DemoIntegrationFormValues,
): IntegrationValidationCode | null {
  if (kind === "rss" && parseFeedLines(values.feedsText).length === 0) {
    return "rssFeedsRequired";
  }
  if (kind === "linkedin" && values.oauthCredentialId.trim().length === 0) {
    return "linkedinCredentialRequired";
  }
  return null;
}

export function buildIntegrationConfigJSON(
  kind: DemoIntegrationKind,
  values: DemoIntegrationFormValues,
): string {
  if (kind === "rss") {
    return JSON.stringify({ feeds: parseFeedLines(values.feedsText) });
  }

  return JSON.stringify({
    oauth_credential_id: values.oauthCredentialId.trim(),
  });
}

export async function saveDemoIntegrationInstallation(input: {
  tenantId: string;
  card: DemoIntegrationCard;
  values: DemoIntegrationFormValues;
}): Promise<ExecutorInstallation> {
  const configJson = buildIntegrationConfigJSON(input.card.kind, input.values);
  const connectionStatus = input.values.enabled
    ? ConnectionStatus.CONNECTED
    : ConnectionStatus.DISCONNECTED;
  const integration = create(IntegrationInstallationSchema, {
    connectionStatus,
    configJson,
  });
  const displayName =
    input.values.displayName.trim() ||
    input.card.sku.displayName ||
    input.card.sku.key;

  if (input.card.installation) {
    const response = await executorClient.updateExecutorInstallation({
      tenantId: input.tenantId,
      installationId: input.card.installation.id,
      displayName,
      enabled: input.values.enabled,
      detail: {
        case: "integration",
        value: integration,
      },
    });
    if (!response.installation) {
      throw new Error("Update did not return an executor installation.");
    }
    return response.installation;
  }

  const response = await executorClient.createExecutorInstallation({
    tenantId: input.tenantId,
    executorSkuId: input.card.sku.id,
    displayName,
    enabled: input.values.enabled,
    initialDetail: {
      case: "integration",
      value: integration,
    },
  });
  if (!response.installation) {
    throw new Error("Create did not return an executor installation.");
  }
  return response.installation;
}

export function connectionStatusLabelKey(status: ConnectionStatus): string {
  switch (status) {
    case ConnectionStatus.CONNECTED:
      return "integrations.status.connected";
    case ConnectionStatus.DISCONNECTED:
      return "integrations.status.disconnected";
    case ConnectionStatus.ERROR:
      return "integrations.status.error";
    case ConnectionStatus.CONNECTING:
      return "integrations.status.connecting";
    default:
      return "integrations.status.unspecified";
  }
}

function connectionStatusFromInstallation(
  installation?: ExecutorInstallation,
): ConnectionStatus {
  if (installation?.detail.case !== "integration") {
    return ConnectionStatus.UNSPECIFIED;
  }
  return installation.detail.value.connectionStatus;
}

function isInstallationConfigured(
  installation?: ExecutorInstallation,
): boolean {
  if (installation?.detail.case !== "integration") {
    return false;
  }
  return (
    installation.enabled &&
    installation.detail.value.configJson.trim().length > 0
  );
}

function parseFeedLines(value: string): string[] {
  return value
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line.length > 0);
}

function parseConfigJSON(value: string | undefined): Record<string, unknown> {
  if (!value?.trim()) {
    return {};
  }

  try {
    const parsed = JSON.parse(value);
    return parsed && typeof parsed === "object" && !Array.isArray(parsed)
      ? parsed
      : {};
  } catch {
    return {};
  }
}

function isString(value: unknown): value is string {
  return typeof value === "string";
}

async function listIntegrationSKUs(): Promise<ExecutorSKU[]> {
  const skus: ExecutorSKU[] = [];
  for await (const response of executorClient.listExecutorSKUs({
    kindFilter: ExecutorKind.INTEGRATION,
    pageSize: 100,
  })) {
    skus.push(...response.executorSkus);
  }
  return skus;
}

async function listIntegrationEntitlements(
  tenantId: string,
): Promise<ExecutorEntitlement[]> {
  const entitlements: ExecutorEntitlement[] = [];
  for await (const response of executorClient.listExecutorEntitlements({
    tenantId,
    pageSize: 100,
  })) {
    entitlements.push(...response.entitlements);
  }
  return entitlements;
}

async function listIntegrationInstallations(
  tenantId: string,
): Promise<ExecutorInstallation[]> {
  const installations: ExecutorInstallation[] = [];
  for await (const response of executorClient.listExecutorInstallations({
    tenantId,
    kindFilter: ExecutorKind.INTEGRATION,
    pageSize: 100,
  })) {
    installations.push(...response.installations);
  }
  return installations;
}
