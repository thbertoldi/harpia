import { create } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";
import { AgentTypeSchema } from "$lib/gen/harpia/agents/v1/agents_pb";
import {
  mergeCatalogEntry,
  mockAgentCatalog,
  MOCK_AGENT_TYPES,
} from "$lib/mocks/agent-catalog";

describe("agent catalog mocks", () => {
  it("returns three seeded catalog entries", () => {
    const entries = mockAgentCatalog();
    expect(entries).toHaveLength(3);
    expect(entries.map((e) => e.agentType.id)).toEqual([
      "email-drafter",
      "research-analyst",
      "calendar-coordinator",
    ]);
  });

  it("localizes mock agent catalog content for pt-BR", () => {
    const entries = mockAgentCatalog("pt-BR");
    expect(entries[0]?.agentType.displayName).toBe("Redator de E-mails");
    expect(entries[0]?.agentType.systemPrompt).toContain("Redija um e-mail");
  });

  it("merges API agent types with catalog metadata by id", () => {
    const apiAgent = {
      ...MOCK_AGENT_TYPES[0],
      description: "Updated from API",
    };
    const entry = mergeCatalogEntry(apiAgent);
    expect(entry.agentType.description).toBe("Updated from API");
    expect(entry.activeVersion).toBe("1.0.0");
    expect(entry.canaryVersion).toBe("1.1.0-rc1");
    expect(entry.trustScore).toBe(92);
  });

  it("falls back to defaults for unknown agent types", () => {
    const entry = mergeCatalogEntry(
      create(AgentTypeSchema, {
        id: "custom-agent",
        name: "custom-agent",
        displayName: "Custom",
        description: "Test agent",
        capabilitiesText: "",
        capabilities: [],
        version: "0.1.0",
        modelId: "test-model",
        systemPrompt: "",
        allowedToolIds: ["tool-a"],
        costEstimate: 0,
        createdAt: "2026-01-01T00:00:00Z",
      }),
    );

    expect(entry.activeVersion).toBe("0.1.0");
    expect(entry.trustScore).toBe(75);
    expect(entry.boundTools).toHaveLength(1);
    expect(entry.boundTools[0].id).toBe("tool-a");
  });
});
