# ADR-011: MCP Capability Gating and Out-of-Process Worker Discipline

**Status:** Accepted
**Date:** 2026-06-13
**Deciders:** thbertoldi

**References:**
- ADR-006: Domain-Driven Design for Agentic Architecture
- ADR-007: Agentic Architecture Patterns for Harpia
- ADR-008: Tenant-Safe Boundaries for Generic Infrastructure
- Model Context Protocol (MCP) specification — out-of-process server model
- Issues: #41 (tool manifest + binding), #42 (agent type manifest), #43 (per-tenant MCP config), #45 (MCP client infra), #48 (OAuth helper), #65 (`common.v1` envelope types)

## Context

Harpia agents need external context — files, knowledge bases, CRM records, Slack channels, GitHub repos — to execute subtasks. The Model Context Protocol (MCP) is the canonical integration mechanism (see ADR-007 §1: agents are ephemeral; context comes from MCP tools, not persistent agent memory).

Four implementation issues converge on MCP infrastructure (#41, #43, #45, #48) without a shared architectural contract. Without explicit discipline, each PR risks inventing its own integration shape:

- Agents connect to MCP servers **by name**, bypassing tenant policy and capability checks.
- MCP server logic gets **embedded in agent-runtime**, violating process isolation and complicating upgrades.
- OAuth tokens for stateful providers (Slack, GitHub, Linear) scatter across ad-hoc secrets.
- Tool access lacks an **audit trail**, undermining Overseer trust and incident response.

Paperclip's capability-gated client pattern and MCP's out-of-process server model provide the right primitives. This ADR pins the shape so downstream work (#41, #43, #45, #48, #49) shares one contract.

## Decision

### 1. Capability Registry

Introduce a **capability registry** as the single source of truth for what MCP servers provide and what agent types require.

**Proto location:** new package `proto/harpia/mcp/v1/` (preferred over extending `harpia.agents.v1` — MCP server configuration is a supporting subdomain with its own lifecycle; see ADR-006 §2 Agent Capability Registry).

Core messages:

| Message | Purpose |
|---|---|
| `Capability` | Identifies a logical tool domain (e.g. `read_files`, `search_knowledge`, `post_slack`) |
| `MCPServerConfig` | Per-tenant server registration: transport, endpoint, declared capabilities, optional server alias |
| `MCPServerRegistryService` | CRUD for tenant MCP server configs (#43) |
| `ResolveCapabilityRequest/Response` | Maps `(tenant, capability[, server_alias])` → concrete server connection params |

Agent type manifests (#42) declare **required capabilities** via the existing `AgentType.capabilities` field. The router validates that every required capability is satisfied by the tenant's configured servers **before** dispatch (AND semantics — see §5).

### 2. Capability-Gated Client Adapter

All agent-runtime MCP access flows through a **capability-gated client adapter** in `agent-runtime/src/harpia_agents/mcp/`.

```text
Agent code
  → mcp.request(capability="read_files", tool="read", args={...})
  → CapabilityGatedClient
      1. Validate capability ∈ agent_type.required_capabilities
      2. Resolve capability → tenant MCPServerConfig (control-plane)
      3. Fetch credentials via GetMCPCredentials (if OAuth — §7)
      4. Connect via MCP transport (stdio / HTTP / SSE)
      5. Emit AuditEvent (§8)
      6. Route tool call to resolved server
```

**Hard rule:** agent code MUST NOT call `mcp.connect("filesystem-server")` or reference server names directly. Server names are infrastructure identifiers visible only to platform engineers and tenant admins — not to agent logic.

The adapter wraps `langchain-mcp-adapters` (already in `agent-runtime/pyproject.toml`) behind this resolution layer.

### 3. Request by Capability, Not Server Name

| Layer | Requests by | Example |
|---|---|---|
| Agent logic / LangGraph tools | **Capability** | `capability="read_files"` |
| Capability-gated client | Capability → server resolution | Looks up tenant config |
| Platform engineer / tenant admin | **Server name / alias** | `filesystem-local`, `gdrive-prod` |
| MCP transport | Server endpoint | `stdio://...`, `https://mcp.example.com` |

This separation ensures a compromised or misconfigured third-party MCP server cannot expand its reach: the adapter only routes calls for capabilities the server declared **and** the agent type was authorized to use.

### 4. Out-of-Process Discipline

MCP servers **always** run as separate processes. Agent-runtime connects via MCP transport; it **never** imports MCP server logic into its own process space.

```text
┌─────────────────────┐     MCP transport      ┌─────────────────────┐
│   agent-runtime     │ ◄──────────────────► │  MCP server process │
│  (LangGraph worker) │   stdio / HTTP / SSE │  (filesystem, slack) │
└─────────────────────┘                       └─────────────────────┘
         │                                              │
         │ ConnectRPC                                   │ OAuth / API
         ▼                                              ▼
┌─────────────────────┐                       ┌─────────────────────┐
│   control-plane     │                       │  External provider  │
│  (config + creds)   │                       │  (GitHub, Slack…)   │
└─────────────────────┘                       └─────────────────────┘
```

**Rationale:** aligns with ADR-007 — agents are ephemeral, stateless executors. MCP servers are long-lived infrastructure workers with their own resource limits, credentials, and upgrade cadence. Co-locating server logic in agent-runtime would couple agent scaling to server lifecycle and violate MCP's process model.

**Dev environment:**
- MCP server manifests live in `deploy/dev/mcp/` (one directory per server).
- Tilt brings each server up as a **separate workload** alongside agent-runtime (see ADR-005 Tilt orchestration).
- Optional: `mise run dev-mcp` for standalone server debugging without the full stack.

**Production:**
- Each MCP server is a **sibling Deployment** in the Helm chart (same release, separate pods).
- Agent-runtime receives server endpoints via tenant config resolved at runtime — not hardcoded in the chart.
- Network policies restrict agent-runtime pods to egress only to configured MCP server services (see §6).

The existing `agent-runtime/src/harpia_agents/mcp_servers/` package may contain server **implementations**, but those implementations are executed as separate processes — never imported by the LangGraph worker at runtime.

### 5. Resolved Open Questions

#### 5.1 Capability Namespace

**Decision:** free-form strings with lint enforcement; formalize to proto enum when the vocabulary stabilizes (post-MVP).

Initial registered set (maintained in repo lint config, not yet a breaking proto enum):

```text
read_files, write_files, search_knowledge, post_slack,
read_github, write_github, read_calendar, query_database
```

Naming convention: `snake_case`, verb-noun pattern, registered in a `capabilities.yaml` reference file checked by CI.

#### 5.2 Multiple Servers per Capability

When a tenant configures two servers for the same capability (e.g. local filesystem + cloud storage):

1. Agent code may pass an optional `server_alias` to disambiguate.
2. If omitted, the adapter uses the **first-configured** server for that capability (stable ordering from registry).
3. Platform engineers SHOULD document alias choices in agent type manifests when ambiguity is expected.

#### 5.3 Capability Composition

**Decision:** AND semantics for the agent type's **required capability set**.

- An agent type declaring `[read_files, search_knowledge]` requires **both** capabilities to be configured for the tenant before dispatch.
- Each individual MCP call requests **one capability** at a time. The agent composes multi-capability workflows by making sequential calls — no single call spans multiple capabilities.
- The adapter validates the requested capability is in the agent's allowed set (derived from manifest + dispatch-time authorization).

#### 5.4 Missing Capability — Failure Mode

If the requested capability is not configured for the tenant:

- Return a typed Connect error (`FailedPrecondition`: capability not configured).
- Surface to the Overseer as a **configuration gap** — never silently substitute another server or capability.
- Do not fall back to a platform-default server; tenant isolation is absolute (consistent with ADR-008).

#### 5.5 Per-Agent-Type Capability Declaration

**Decision:** formalize in the agent type manifest (#42).

`AgentType.capabilities` is the contract. The router rejects dispatch when the tenant's MCP registry does not satisfy the agent's required set. Dynamic capability discovery at runtime is out of scope for MVP.

#### 5.6 Audit Granularity

**Decision:** log **every** MCP tool call. The audit log is the trust substrate for Overseer visibility and incident response. Sampling is deferred to post-MVP optimization only with explicit policy approval.

### 6. Production Network Policy (Recommendation)

In production Kubernetes, apply NetworkPolicy resources that:

1. **Agent-runtime egress:** allow ConnectRPC to control-plane; allow MCP transport only to MCP server Services registered for the tenant's namespace.
2. **MCP server egress:** allow only to the external provider endpoints that server requires (e.g. `api.github.com`, `slack.com`). Deny lateral movement to other MCP servers or databases.
3. **Default deny:** namespaces housing MCP servers use default-deny ingress/egress with explicit allowlists.

This is a **recommendation** for Helm chart authors (#43, deploy work) — not enforced by agent-runtime code. Document in `deploy/` README when MCP workloads land.

### 7. OAuth Helper (Issue #48)

Stateful MCP providers (Slack, GitHub, Linear, etc.) require OAuth token management. This belongs in **control-plane**, not agent-runtime.

**RPC:** `GetMCPCredentials(tenant_id, server_id) → credentials` on a new or extended MCP service in control-plane.

Flow:

```text
1. Tenant admin completes OAuth redirect flow (UI — #49)
2. control-plane stores encrypted tokens per (tenant, server) in secrets store
3. Capability-gated client calls GetMCPCredentials before MCP connect
4. Tokens never enter agent prompts, LangGraph state, or audit log payloads
```

Agent-runtime receives short-lived connection credentials; it does not store or refresh OAuth tokens.

### 8. Audit Trail (Issue #65)

Every MCP tool invocation emits a `common.v1.AuditEvent` (#65) via control-plane:

| Field | Value |
|---|---|
| `tenant_id` | From `identity.RequestContext` |
| `actor` | Agent instance ID + agent type ID |
| `action` | `mcp.tool.invoke` |
| `resource` | `{capability}/{tool_name}` |
| `metadata` | Server alias, latency, success/failure (no secrets, no payload contents) |

Audit events enable Overseer-side visibility into which tools agents reach for during subtask execution. Cross-capability access attempts (capability not in agent's allowed set) are logged as `mcp.tool.denied`.

## Rationale

### Why capability gating over direct server connection

Direct server-name coupling lets any connected MCP server expose tools beyond its declared scope once the transport is established. Capability gating enforces authorization at the call site — the adapter is the only component that knows server topology.

### Why a new `harpia.mcp.v1` proto package

MCP server configuration is tenant-scoped infrastructure with its own CRUD lifecycle. Keeping it out of `harpia.agents.v1` preserves bounded context boundaries (ADR-006 §2) and avoids bloating the agent manifest proto with deployment concerns.

### Why out-of-process always

MCP spec assumes separate server processes. Embedding breaks: independent scaling, credential isolation, server upgrades without agent-runtime redeploy, and crash containment (a runaway filesystem server must not take down the LangGraph worker).

### Alternatives Considered

| Alternative | Why not |
|---|---|
| Server-name-based client (status quo in many MCP integrations) | No authorization gate; tenant config leaks into agent code |
| Embed MCP servers in agent-runtime Python process | Violates MCP process model; couples scaling and crash domains |
| Proto enum for capabilities from day one | Slows iteration; free strings with lint gives vocabulary flexibility |
| Platform-default MCP servers when tenant unconfigured | Breaks tenant isolation; hides configuration gaps from Overseer |
| Sampled audit logging | Insufficient for trust substrate and incident forensics at MVP scale |

## Consequences

### What Becomes Easier

- **Consistent MCP integration** across #41, #43, #45, #48 — one adapter, one registry, one audit shape.
- **Tenant-safe tool access** — agents cannot reach undeclared capabilities; missing config surfaces clearly to the Overseer.
- **Independent server lifecycle** — upgrade filesystem MCP server without redeploying agent-runtime.
- **Overseer trust** — full audit trail of tool invocations per subtask execution.
- **ADR-007 alignment** — MCP remains the canonical context-from-tools mechanism for ephemeral agents.

### What Becomes Harder

- Extra resolution hop on every MCP call (capability → config → connect → audit).
- New proto package, control-plane service, and Python adapter to build before agents can use MCP in production.
- Tenant admins must configure MCP servers and OAuth before agents with external tool dependencies can run.
- Network policy authoring adds Helm complexity for each MCP server type.

### Actions Required

1. Create `proto/harpia/mcp/v1/` with capability registry and `MCPServerRegistryService` (#43)
2. Implement capability-gated client in `agent-runtime/src/harpia_agents/mcp/` (#45)
3. Add `GetMCPCredentials` RPC to control-plane (#48)
4. Wire `deploy/dev/mcp/` manifests and Tilt resources for out-of-process MCP servers
5. Add sibling MCP server Deployments to Helm chart
6. Integrate `common.v1.AuditEvent` emission on every MCP tool call (#65)
7. Add `capabilities.yaml` reference file and CI lint for capability namespace
8. Extend agent type manifest validation to cross-check capabilities against tenant registry (#41, #42)

## Related ADRs and Issues

| Link | Relationship |
|---|---|
| [ADR-006](ADR-006-domain-driven-design.md) | Capability registry lives in Agent Orchestration bounded context |
| [ADR-007](ADR-007-agentic-architecture-patterns.md) | Refines — MCP is the canonical context source for ephemeral agents |
| [ADR-008](ADR-008-tenant-safe-generic-infra-boundaries.md) | Aligns — no platform-default servers; tenant isolation at MCP boundary |
| #41 | Tool manifest + agent↔tool binding (absorbs registry client scope) |
| #42 | Agent type manifest — `capabilities` field is the required-capability contract |
| #43 | Per-tenant MCP server config |
| #45 | MCP client infra in agent-runtime |
| #48 | OAuth helper — `GetMCPCredentials` |
| #49 | UI for MCP server and OAuth configuration |
| #65 | `common.v1.AuditEvent` envelope for audit trail |
