import {
  ConnectionStatus,
  type ExecutorInstallation,
} from "$lib/gen/harpia/executors/v1/executors_pb";
import { executorClient } from "$lib/rpc";

interface InstallationPage {
  installations: ExecutorInstallation[];
}

interface AggregateExecutorClient {
  listExecutorInstallations(input: {
    tenantId: string;
    pageSize: number;
    pageToken: string;
  }): AsyncIterable<InstallationPage>;
  createExecutorInstallation(input: {
    tenantId: string;
    executorSkuId: string;
    displayName: string;
    enabled: boolean;
    initialDetail: {
      case: "integration";
      value: { connectionStatus: ConnectionStatus; configJson: string };
    };
  }): Promise<{ installation?: ExecutorInstallation }>;
}

export interface EnsureAggregateRssInstallationInput {
  tenantId: string;
  selectedInstallationIds: string[];
  executorClient?: AggregateExecutorClient;
}

function rssFeeds(installation: ExecutorInstallation): string[] {
  if (installation.detail.case !== "integration") return [];
  try {
    const parsed = JSON.parse(installation.detail.value.configJson || "{}");
    return Array.isArray(parsed.feeds)
      ? parsed.feeds.filter(
          (feed: unknown): feed is string => typeof feed === "string",
        )
      : [];
  } catch {
    return [];
  }
}

// isSourceInstallation is true only for installations that produce information a
// plan can read from — RSS feed integrations. Action integrations (e.g. a
// LinkedIn publisher) are destinations, not sources, and must never appear in a
// "sources" picker even though they share the INTEGRATION kind.
function isSourceInstallation(installation: ExecutorInstallation): boolean {
  if (installation.detail.case !== "integration") return false;
  try {
    const parsed = JSON.parse(installation.detail.value.configJson || "{}");
    return Array.isArray(parsed.feeds);
  } catch {
    return false;
  }
}

// sourceGroupInstallations narrows a raw installation list to the ones eligible
// as plan sources, dropping publishers and other action integrations.
export function sourceGroupInstallations(
  installations: ExecutorInstallation[],
): ExecutorInstallation[] {
  return installations.filter(isSourceInstallation);
}

function sameFeedSet(left: string[], right: string[]): boolean {
  if (left.length !== right.length) return false;
  const sortedLeft = [...left].sort();
  const sortedRight = [...right].sort();
  return sortedLeft.every((feed, index) => feed === sortedRight[index]);
}

async function listInstallations(
  client: AggregateExecutorClient,
  tenantId: string,
): Promise<ExecutorInstallation[]> {
  const out: ExecutorInstallation[] = [];
  for await (const page of client.listExecutorInstallations({
    tenantId,
    pageSize: 100,
    pageToken: "",
  })) {
    out.push(...page.installations);
  }
  return out;
}

export async function ensureAggregateRssInstallation({
  tenantId,
  selectedInstallationIds,
  executorClient: client = executorClient,
}: EnsureAggregateRssInstallationInput): Promise<string> {
  const selectedIds = selectedInstallationIds.filter(Boolean);
  if (selectedIds.length === 0) return "";
  if (selectedIds.length === 1) return selectedIds[0];

  const installations = await listInstallations(client, tenantId);
  const selected = selectedIds
    .map((id) => installations.find((installation) => installation.id === id))
    .filter((installation): installation is ExecutorInstallation =>
      Boolean(installation),
    );
  if (selected.length === 0) return "";

  const executorSkuId = selected[0].executorSkuId;
  const feeds = Array.from(new Set(selected.flatMap(rssFeeds)));
  const existing = installations.find(
    (installation) =>
      installation.executorSkuId === executorSkuId &&
      installation.id &&
      !selectedIds.includes(installation.id) &&
      sameFeedSet(rssFeeds(installation), feeds),
  );
  if (existing) return existing.id;

  const displayName = `Aggregate RSS: ${selected
    .map((installation) => installation.displayName || installation.id)
    .join(" + ")}`;
  const response = await client.createExecutorInstallation({
    tenantId,
    executorSkuId,
    displayName,
    enabled: true,
    initialDetail: {
      case: "integration",
      value: {
        connectionStatus: ConnectionStatus.CONNECTED,
        configJson: JSON.stringify({ feeds }),
      },
    },
  });
  return response.installation?.id ?? "";
}
