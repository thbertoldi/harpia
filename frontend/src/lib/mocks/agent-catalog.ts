import { create } from "@bufbuild/protobuf";
import {
  AgentTypeSchema,
  type AgentType,
} from "$lib/gen/harpia/agents/v1/agents_pb";

export type TenantVisibility = "global" | "restricted";

export interface BoundTool {
  id: string;
  name: string;
  description: string;
}

export interface AgentVersionEntry {
  version: string;
  deployedAt: string;
  status: "active" | "canary" | "deprecated" | "archived";
}

export interface AgentInvocation {
  id: string;
  taskId: string;
  status: "completed" | "failed" | "awaiting_feedback";
  startedAt: string;
  durationMs?: number;
}

/** Catalog fields not yet exposed by ListAgentTypes — keyed by agent id for API merge. */
export interface AgentCatalogMeta {
  activeVersion: string;
  canaryVersion?: string;
  trustScore: number;
  tenantVisibility: TenantVisibility;
  visibleTenantIds?: string[];
  lastUpdated: string;
  yamlSourceUrl: string;
  boundTools: BoundTool[];
  versionHistory: AgentVersionEntry[];
  recentInvocations: AgentInvocation[];
}

export interface AgentCatalogEntry {
  agentType: AgentType;
  activeVersion: string;
  canaryVersion?: string;
  trustScore: number;
  tenantVisibility: TenantVisibility;
  visibleTenantIds?: string[];
  lastUpdated: string;
  yamlSourceUrl: string;
  boundTools: BoundTool[];
  versionHistory: AgentVersionEntry[];
  recentInvocations: AgentInvocation[];
}

const EMAIL_DRAFTER_MANIFEST: AgentType = create(AgentTypeSchema, {
  id: "email-drafter",
  name: "email-drafter",
  displayName: "Email Drafter",
  description:
    "Drafts concise, context-aware email responses for overseer review.",
  capabilitiesText: "email, writing, drafting",
  capabilities: ["email", "writing", "drafting"],
  version: "1.0.0",
  modelId: "openai-gpt-4o-mini",
  systemPrompt:
    "Draft an email to {{recipient_name}} about {{purpose}}.\nKeep the tone {{tone}} and return only the proposed message body.",
  allowedToolIds: ["tenant-knowledge-search"],
  inputSchema: {
    type: "object",
    required: ["recipient_name", "purpose", "tone"],
    properties: {
      recipient_name: { type: "string" },
      purpose: { type: "string" },
      tone: { type: "string", enum: ["concise", "friendly", "formal"] },
    },
  },
  outputSchema: {
    type: "object",
    required: ["body"],
    properties: { body: { type: "string" } },
  },
  costEstimate: 0.01,
  metadata: { owner: "platform-engineering", maturity: "example" },
  createdAt: "2026-05-12T09:15:00Z",
});

export const AGENT_CATALOG_META: Record<string, AgentCatalogMeta> = {
  "email-drafter": {
    activeVersion: "1.0.0",
    canaryVersion: "1.1.0-rc1",
    trustScore: 92,
    tenantVisibility: "global",
    lastUpdated: "2026-06-10T14:22:00Z",
    yamlSourceUrl:
      "https://github.com/harpia/harpia/blob/trunk/agents/email-drafter/v1.yaml",
    boundTools: [
      {
        id: "tenant-knowledge-search",
        name: "Tenant Knowledge Search",
        description: "Semantic search over tenant-scoped knowledge bases.",
      },
    ],
    versionHistory: [
      {
        version: "1.1.0-rc1",
        deployedAt: "2026-06-08T11:00:00Z",
        status: "canary",
      },
      {
        version: "1.0.0",
        deployedAt: "2026-05-12T09:15:00Z",
        status: "active",
      },
      {
        version: "0.9.0",
        deployedAt: "2026-04-01T08:00:00Z",
        status: "archived",
      },
    ],
    recentInvocations: [
      {
        id: "inv-7f3a",
        taskId: "task-a1b2c3d4",
        status: "completed",
        startedAt: "2026-06-12T16:40:00Z",
        durationMs: 4200,
      },
      {
        id: "inv-2e91",
        taskId: "task-e5f6g7h8",
        status: "awaiting_feedback",
        startedAt: "2026-06-12T15:05:00Z",
      },
      {
        id: "inv-9c44",
        taskId: "task-i9j0k1l2",
        status: "failed",
        startedAt: "2026-06-11T09:30:00Z",
        durationMs: 1800,
      },
    ],
  },
  "research-analyst": {
    activeVersion: "2.3.1",
    trustScore: 87,
    tenantVisibility: "restricted",
    visibleTenantIds: ["dev", "acme-corp"],
    lastUpdated: "2026-06-09T08:45:00Z",
    yamlSourceUrl:
      "https://github.com/harpia/harpia/blob/trunk/docs/agents/MANIFEST.md",
    boundTools: [
      {
        id: "web-fetch",
        name: "Web Fetch",
        description: "Retrieve and summarize public web pages.",
      },
      {
        id: "tenant-knowledge-search",
        name: "Tenant Knowledge Search",
        description: "Semantic search over tenant-scoped knowledge bases.",
      },
    ],
    versionHistory: [
      {
        version: "2.3.1",
        deployedAt: "2026-06-09T08:45:00Z",
        status: "active",
      },
      {
        version: "2.3.0",
        deployedAt: "2026-05-20T12:00:00Z",
        status: "archived",
      },
    ],
    recentInvocations: [
      {
        id: "inv-aa01",
        taskId: "task-m3n4o5p6",
        status: "completed",
        startedAt: "2026-06-12T10:15:00Z",
        durationMs: 12500,
      },
    ],
  },
  "calendar-coordinator": {
    activeVersion: "1.2.0",
    canaryVersion: "1.3.0-beta",
    trustScore: 78,
    tenantVisibility: "global",
    lastUpdated: "2026-06-11T19:30:00Z",
    yamlSourceUrl:
      "https://github.com/harpia/harpia/blob/trunk/docs/agents/MANIFEST.md",
    boundTools: [
      {
        id: "calendar-read",
        name: "Calendar Read",
        description: "Read availability and events from connected calendars.",
      },
      {
        id: "calendar-write",
        name: "Calendar Write",
        description: "Propose and confirm calendar events.",
      },
    ],
    versionHistory: [
      {
        version: "1.3.0-beta",
        deployedAt: "2026-06-11T19:30:00Z",
        status: "canary",
      },
      {
        version: "1.2.0",
        deployedAt: "2026-05-28T07:00:00Z",
        status: "active",
      },
    ],
    recentInvocations: [
      {
        id: "inv-bb22",
        taskId: "task-q7r8s9t0",
        status: "completed",
        startedAt: "2026-06-12T08:00:00Z",
        durationMs: 3100,
      },
      {
        id: "inv-cc33",
        taskId: "task-u1v2w3x4",
        status: "completed",
        startedAt: "2026-06-11T14:20:00Z",
        durationMs: 2900,
      },
    ],
  },
};

let runtimeAgentCatalogMeta: Record<string, AgentCatalogMeta> =
  structuredClone(AGENT_CATALOG_META);

const RESEARCH_ANALYST: AgentType = create(AgentTypeSchema, {
  id: "research-analyst",
  name: "research-analyst",
  displayName: "Research Analyst",
  description:
    "Synthesizes findings from web and tenant knowledge sources into briefs.",
  capabilitiesText: "research, analysis, summarization",
  capabilities: ["research", "analysis", "summarization"],
  version: "2.3.1",
  modelId: "openai-gpt-4o",
  systemPrompt:
    "Research {{topic}} and produce a structured brief with citations.",
  allowedToolIds: ["web-fetch", "tenant-knowledge-search"],
  inputSchema: {
    type: "object",
    required: ["topic"],
    properties: { topic: { type: "string" } },
  },
  outputSchema: {
    type: "object",
    required: ["brief"],
    properties: { brief: { type: "string" } },
  },
  costEstimate: 0.08,
  metadata: { owner: "platform-engineering" },
  createdAt: "2026-04-15T10:00:00Z",
});

const CALENDAR_COORDINATOR: AgentType = create(AgentTypeSchema, {
  id: "calendar-coordinator",
  name: "calendar-coordinator",
  displayName: "Calendar Coordinator",
  description: "Schedules meetings and resolves calendar conflicts.",
  capabilitiesText: "calendar, scheduling, coordination",
  capabilities: ["calendar", "scheduling", "coordination"],
  version: "1.2.0",
  modelId: "openai-gpt-4o-mini",
  systemPrompt:
    "Find a meeting time for {{participants}} about {{subject}} within {{window}}.",
  allowedToolIds: ["calendar-read", "calendar-write"],
  inputSchema: {
    type: "object",
    required: ["participants", "subject", "window"],
    properties: {
      participants: { type: "array", items: { type: "string" } },
      subject: { type: "string" },
      window: { type: "string" },
    },
  },
  outputSchema: {
    type: "object",
    required: ["proposed_slot"],
    properties: { proposed_slot: { type: "string" } },
  },
  costEstimate: 0.02,
  metadata: { owner: "platform-engineering" },
  createdAt: "2026-05-01T12:00:00Z",
});

export const MOCK_AGENT_TYPES: AgentType[] = [
  EMAIL_DRAFTER_MANIFEST,
  RESEARCH_ANALYST,
  CALENDAR_COORDINATOR,
];

export function mergeCatalogEntry(
  agentType: AgentType,
  meta?: AgentCatalogMeta,
): AgentCatalogEntry {
  const fallbackMeta: AgentCatalogMeta = {
    activeVersion: agentType.version || "0.0.0",
    trustScore: 75,
    tenantVisibility: "global",
    lastUpdated: agentType.createdAt || new Date().toISOString(),
    yamlSourceUrl:
      "https://github.com/harpia/harpia/blob/trunk/docs/agents/MANIFEST.md",
    boundTools: agentType.allowedToolIds.map((id) => ({
      id,
      name: id,
      description: "Registered tool binding.",
    })),
    versionHistory: [
      {
        version: agentType.version || "0.0.0",
        deployedAt: agentType.createdAt || new Date().toISOString(),
        status: "active",
      },
    ],
    recentInvocations: [],
  };

  const resolved = meta ?? AGENT_CATALOG_META[agentType.id] ?? fallbackMeta;

  return {
    agentType,
    activeVersion: resolved.activeVersion,
    canaryVersion: resolved.canaryVersion,
    trustScore: resolved.trustScore,
    tenantVisibility: resolved.tenantVisibility,
    visibleTenantIds: resolved.visibleTenantIds,
    lastUpdated: resolved.lastUpdated,
    yamlSourceUrl: resolved.yamlSourceUrl,
    boundTools: resolved.boundTools,
    versionHistory: resolved.versionHistory,
    recentInvocations: resolved.recentInvocations,
  };
}

export function mockAgentCatalog(): AgentCatalogEntry[] {
  return MOCK_AGENT_TYPES.map((agentType) =>
    mergeCatalogEntry(agentType, runtimeAgentCatalogMeta[agentType.id]),
  );
}

export function resetMockAgentCatalog(): void {
  runtimeAgentCatalogMeta = structuredClone(AGENT_CATALOG_META);
}

export function bindMockToolToAgent(
  agentId: string,
  tool: BoundTool,
): AgentCatalogEntry | null {
  const current = runtimeAgentCatalogMeta[agentId];
  if (!current) {
    return null;
  }

  const hasTool = current.boundTools.some(
    (boundTool) => boundTool.id === tool.id,
  );
  const nextBoundTools = hasTool
    ? current.boundTools
    : [...current.boundTools, tool];

  runtimeAgentCatalogMeta[agentId] = {
    ...current,
    boundTools: nextBoundTools,
    lastUpdated: new Date().toISOString(),
  };

  const agentType = MOCK_AGENT_TYPES.find(
    (candidate) => candidate.id === agentId,
  );
  if (!agentType) {
    return null;
  }

  return mergeCatalogEntry(agentType, runtimeAgentCatalogMeta[agentId]);
}

export function runMockAgentInvocation(
  agentId: string,
  taskId: string,
): AgentCatalogEntry | null {
  const current = runtimeAgentCatalogMeta[agentId];
  if (!current) {
    return null;
  }

  const invocation: AgentInvocation = {
    id: `inv-${Math.random().toString(36).slice(2, 8)}`,
    taskId,
    status: "completed",
    startedAt: new Date().toISOString(),
    durationMs: 2400,
  };

  runtimeAgentCatalogMeta[agentId] = {
    ...current,
    recentInvocations: [invocation, ...current.recentInvocations].slice(0, 10),
    lastUpdated: new Date().toISOString(),
  };

  const agentType = MOCK_AGENT_TYPES.find(
    (candidate) => candidate.id === agentId,
  );
  if (!agentType) {
    return null;
  }

  return mergeCatalogEntry(agentType, runtimeAgentCatalogMeta[agentId]);
}
