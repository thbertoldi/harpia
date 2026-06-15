import { describe, expect, it, beforeEach } from "vitest";
import {
  canManageIntegrations,
  getUserRole,
  isEngineer,
} from "$lib/auth-roles";
import {
  addMockMcpServer,
  connectMockOAuthServer,
  countToolsForStatus,
  getMockMcpServers,
  resetMockMcpServers,
  statusLabel,
  testMockConnection,
} from "$lib/mocks/mcp-servers";

describe("auth role gating", () => {
  it("grants integration access to leaders and engineers", () => {
    expect(canManageIntegrations({ role: "Engineer" })).toBe(true);
    expect(canManageIntegrations({ role: "Leader" })).toBe(true);
    expect(canManageIntegrations({ role: "Overseer" })).toBe(false);
    expect(canManageIntegrations(null)).toBe(false);
  });

  it("treats only Engineer as engineer role", () => {
    expect(isEngineer({ role: "Engineer" })).toBe(true);
    expect(isEngineer({ role: "Leader" })).toBe(false);
    expect(isEngineer({ role: "Overseer" })).toBe(false);
    expect(isEngineer(null)).toBe(false);
  });

  it("defaults unknown roles to Leader label without permissions", () => {
    expect(getUserRole({ role: "Admin" })).toBe("Leader");
    expect(canManageIntegrations({ role: "Admin" })).toBe(false);
    expect(getUserRole(undefined)).toBe("Leader");
  });
});

describe("mcp server mocks", () => {
  beforeEach(() => {
    resetMockMcpServers();
  });

  it("returns seeded servers with mixed statuses", () => {
    const servers = getMockMcpServers();
    expect(servers.length).toBeGreaterThanOrEqual(3);
    expect(servers.some((server) => server.status === "auth_required")).toBe(
      true,
    );
  });

  it("labels statuses for display", () => {
    expect(statusLabel("connected")).toBe("Connected");
    expect(statusLabel("auth_required")).toBe("Auth Required");
  });

  it("counts tools only for connected servers", () => {
    expect(
      countToolsForStatus("connected", [{ name: "a", description: "A" }]),
    ).toBe(1);
    expect(
      countToolsForStatus("auth_required", [{ name: "a", description: "A" }]),
    ).toBe(0);
  });

  it("validates stdio test connection input", async () => {
    const result = await testMockConnection({
      name: "Broken",
      kind: "stdio",
      config: { command: "", args: [] },
    });
    expect(result.ok).toBe(false);
  });

  it("adds a server and transitions OAuth servers to connected", async () => {
    const created = await addMockMcpServer({
      name: "Local Tools",
      kind: "stdio",
      config: { command: "npx", args: ["-y", "mcp-server"] },
    });
    expect(created.status).toBe("connected");
    expect(getMockMcpServers().some((server) => server.id === created.id)).toBe(
      true,
    );

    const oauthServer = getMockMcpServers().find(
      (server) => server.status === "auth_required",
    );
    expect(oauthServer).toBeDefined();

    const connected = await connectMockOAuthServer(oauthServer!.id);
    expect(connected.status).toBe("connected");
    expect(connected.toolCount).toBeGreaterThan(0);
  });
});
