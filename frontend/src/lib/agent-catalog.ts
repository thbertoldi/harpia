import { toUserMessage } from "$lib/connect-errors";
import type { AgentType } from "$lib/gen/harpia/agents/v1/agents_pb";
import {
  AGENT_CATALOG_META,
  bindMockToolToAgent,
  mergeCatalogEntry,
  mockAgentCatalog,
  runMockAgentInvocation,
  type AgentCatalogEntry,
  type BoundTool,
} from "$lib/mocks/agent-catalog";
import type { AuditEvent } from "$lib/mocks/audit-events";
import { appendMockAuditEvent } from "$lib/audit/audit-store";
import { agentClient } from "$lib/rpc";

export type { AgentCatalogEntry } from "$lib/mocks/agent-catalog";

export type AgentCatalogSource = "api" | "mock";

export interface AgentCatalogResult {
  entries: AgentCatalogEntry[];
  source: AgentCatalogSource;
  error?: string;
}

const AGENT_CATALOG_API_TIMEOUT_MS = 2000;

type BindToolResult = {
  entry: AgentCatalogEntry;
  source: AgentCatalogSource;
};

type InvocationResult = {
  entry: AgentCatalogEntry;
  source: AgentCatalogSource;
  taskId: string;
};

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

async function withTimeout<T>(
  promise: Promise<T>,
  timeoutMs: number,
): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const timer = setTimeout(() => {
      reject(new Error("Agent catalog request timed out"));
    }, timeoutMs);

    promise.then(
      (value) => {
        clearTimeout(timer);
        resolve(value);
      },
      (error) => {
        clearTimeout(timer);
        reject(error);
      },
    );
  });
}

/** Loads catalog entries; merges ListAgentTypes with catalog metadata when API succeeds. */
export async function loadAgentCatalog(): Promise<AgentCatalogResult> {
  try {
    const agentTypes = await withTimeout(
      fetchAgentTypesFromApi(),
      AGENT_CATALOG_API_TIMEOUT_MS,
    );
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

export async function bindToolToAgentCatalogEntry(
  source: AgentCatalogSource,
  agentId: string,
  tool: BoundTool,
): Promise<BindToolResult> {
  if (source !== "mock") {
    throw new Error("Binding tools in live catalog is not available yet.");
  }

  const entry = bindMockToolToAgent(agentId, tool);
  if (!entry) {
    throw new Error("Agent catalog entry not found.");
  }

  return { entry, source };
}

export async function runTestInvocationForAgent(
  source: AgentCatalogSource,
  agentId: string,
): Promise<InvocationResult> {
  if (source !== "mock") {
    throw new Error("Test invocations in live catalog are not available yet.");
  }

  const taskId = `task-e2e-${Math.random().toString(36).slice(2, 8)}`;
  const entry = runMockAgentInvocation(agentId, taskId);
  if (!entry) {
    throw new Error("Agent catalog entry not found.");
  }

  const event: AuditEvent = {
    eventId: `evt-${Math.random().toString(36).slice(2, 10)}`,
    tenantId: "dev",
    eventType: "agent.execution_completed",
    actor: {
      kind: "agent",
      id: `agent-inst-${agentId}`,
      displayName: entry.agentType.displayName || entry.agentType.name,
      agentType: entry.agentType.name || agentId,
    },
    boundedContext: "agent_orchestration",
    taskId,
    payloadDiff: [
      { field: "status", before: "running", after: "completed" },
      {
        field: "agent_type",
        before: null,
        after: entry.agentType.name || agentId,
      },
    ],
    timestamp: new Date().toISOString(),
    traceId: `trace-${Math.random().toString(16).slice(2, 18)}`,
  };
  appendMockAuditEvent(event);

  return { entry, source, taskId };
}
