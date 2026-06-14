import { toUserMessage } from "$lib/connect-errors";
import type { AgentType } from "$lib/gen/harpia/agents/v1/agents_pb";
import {
  AGENT_CATALOG_META,
  mergeCatalogEntry,
  mockAgentCatalog,
  type AgentCatalogEntry,
} from "$lib/mocks/agent-catalog";
import { agentClient } from "$lib/rpc";

export type { AgentCatalogEntry } from "$lib/mocks/agent-catalog";

export type AgentCatalogSource = "api" | "mock";

export interface AgentCatalogResult {
  entries: AgentCatalogEntry[];
  source: AgentCatalogSource;
  error?: string;
}

function resolveAgentKey(agentType: AgentType): string {
  return agentType.id || agentType.name;
}

async function fetchAgentTypesFromApi(): Promise<AgentType[]> {
  const agentTypes: AgentType[] = [];
  for await (const page of agentClient.listAgentTypes({
    pageSize: 100,
    pageToken: "",
  })) {
    agentTypes.push(...page.agentTypes);
  }
  return agentTypes;
}

/** Loads catalog entries; merges ListAgentTypes with catalog metadata when API succeeds. */
export async function loadAgentCatalog(): Promise<AgentCatalogResult> {
  try {
    const agentTypes = await fetchAgentTypesFromApi();
    if (agentTypes.length === 0) {
      return { entries: mockAgentCatalog(), source: "mock" };
    }

    const entries = agentTypes.map((agentType) => {
      const key = resolveAgentKey(agentType);
      return mergeCatalogEntry(agentType, AGENT_CATALOG_META[key]);
    });

    return { entries, source: "api" };
  } catch (error) {
    return {
      entries: mockAgentCatalog(),
      source: "mock",
      error: toUserMessage(error),
    };
  }
}
