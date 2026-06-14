export type McpServerKind = "stdio" | "streamable_http";
export type McpServerStatus =
  | "connected"
  | "auth_required"
  | "error"
  | "disconnected";

export interface McpTool {
  name: string;
  description: string;
}

export interface StdioConfig {
  command: string;
  args: string[];
  env?: Record<string, string>;
}

export interface StreamableHttpConfig {
  url: string;
  headers?: Record<string, string>;
}

export type McpServerConfig = StdioConfig | StreamableHttpConfig;

export interface McpServer {
  id: string;
  name: string;
  kind: McpServerKind;
  status: McpServerStatus;
  toolCount: number;
  tools: McpTool[];
  config: McpServerConfig;
  lastError?: string;
}

export interface AddMcpServerInput {
  name: string;
  kind: McpServerKind;
  config: McpServerConfig;
}

export interface TestConnectionResult {
  ok: boolean;
  message: string;
  discoveredTools?: McpTool[];
}

const INITIAL_SERVERS: McpServer[] = [
  {
    id: "mcp-filesystem",
    name: "Filesystem",
    kind: "stdio",
    status: "connected",
    toolCount: 4,
    tools: [
      {
        name: "read_file",
        description: "Read the contents of a file at the given path.",
      },
      {
        name: "write_file",
        description: "Write content to a file at the given path.",
      },
      {
        name: "list_directory",
        description: "List files and directories at the given path.",
      },
      {
        name: "search_files",
        description: "Search for files matching a glob pattern.",
      },
    ],
    config: {
      command: "npx",
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/workspace"],
    },
  },
  {
    id: "mcp-github",
    name: "GitHub",
    kind: "streamable_http",
    status: "auth_required",
    toolCount: 0,
    tools: [],
    config: {
      url: "https://mcp.github.example/v1",
    },
  },
  {
    id: "mcp-postgres",
    name: "Postgres",
    kind: "stdio",
    status: "error",
    toolCount: 0,
    tools: [],
    lastError: "Connection refused: could not spawn postgres MCP process",
    config: {
      command: "npx",
      args: ["-y", "@modelcontextprotocol/server-postgres"],
      env: { DATABASE_URL: "postgres://localhost:5432/harpia" },
    },
  },
];

let servers = structuredClone(INITIAL_SERVERS);

export function resetMockMcpServers(): void {
  servers = structuredClone(INITIAL_SERVERS);
}

export function getMockMcpServers(): McpServer[] {
  return structuredClone(servers);
}

export function getMcpServerById(id: string): McpServer | undefined {
  return structuredClone(servers.find((server) => server.id === id));
}

export function countToolsForStatus(
  status: McpServerStatus,
  tools: McpTool[],
): number {
  if (status === "connected") {
    return tools.length;
  }
  return 0;
}

export function statusLabel(status: McpServerStatus): string {
  switch (status) {
    case "connected":
      return "Connected";
    case "auth_required":
      return "Auth Required";
    case "error":
      return "Error";
    case "disconnected":
      return "Disconnected";
  }
}

function slugify(name: string): string {
  return name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
}

function mockToolsForKind(kind: McpServerKind): McpTool[] {
  if (kind === "stdio") {
    return [
      {
        name: "ping",
        description: "Verify the MCP server responds to health checks.",
      },
      {
        name: "describe",
        description: "Return metadata about the connected MCP server.",
      },
    ];
  }

  return [
    {
      name: "list_resources",
      description: "List resources exposed by the remote MCP endpoint.",
    },
  ];
}

export async function testMockConnection(
  input: AddMcpServerInput,
): Promise<TestConnectionResult> {
  await delay(600);

  if (input.kind === "stdio") {
    const config = input.config as StdioConfig;
    if (!config.command.trim()) {
      return { ok: false, message: "Command is required for stdio servers." };
    }
  } else {
    const config = input.config as StreamableHttpConfig;
    if (!config.url.trim()) {
      return {
        ok: false,
        message: "URL is required for streamable HTTP servers.",
      };
    }
    if (!/^https?:\/\//.test(config.url)) {
      return { ok: false, message: "URL must start with http:// or https://." };
    }
  }

  const discoveredTools = mockToolsForKind(input.kind);
  return {
    ok: true,
    message: "Connection test succeeded (mock).",
    discoveredTools,
  };
}

export async function addMockMcpServer(
  input: AddMcpServerInput,
): Promise<McpServer> {
  await delay(300);

  const discoveredTools = mockToolsForKind(input.kind);
  const server: McpServer = {
    id: `mcp-${slugify(input.name)}-${Date.now()}`,
    name: input.name.trim(),
    kind: input.kind,
    status: "connected",
    toolCount: discoveredTools.length,
    tools: discoveredTools,
    config: structuredClone(input.config),
  };

  servers = [...servers, server];
  return structuredClone(server);
}

export async function connectMockOAuthServer(
  serverId: string,
): Promise<McpServer> {
  await delay(800);

  const index = servers.findIndex((server) => server.id === serverId);
  if (index === -1) {
    throw new Error("MCP server not found");
  }

  const current = servers[index];
  if (current.status !== "auth_required") {
    throw new Error("Server does not require OAuth connection");
  }

  const tools: McpTool[] = [
    {
      name: "list_repositories",
      description: "List repositories accessible to the authenticated user.",
    },
    {
      name: "create_issue",
      description: "Create a new issue in a repository.",
    },
    {
      name: "search_code",
      description: "Search code across connected repositories.",
    },
  ];

  const updated: McpServer = {
    ...current,
    status: "connected",
    toolCount: tools.length,
    tools,
    lastError: undefined,
  };

  servers = servers.map((server) =>
    server.id === serverId ? updated : server,
  );
  return structuredClone(updated);
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
