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
| `MCPServerConfig` | Per-tenant server registration: transport, endpoint, declared capabilities, server alias (admin-only), `is_default`, `priority` |
| `MCPServerRegistryService` | CRUD for tenant MCP server configs (#43) |
| `ResolveCapabilityRequest/Response` | Maps `(tenant, capability, binding_ref)` → concrete server connection params — **control-plane / adapter only** |

**Manifest fields are not interchangeable:**

| Field | Purpose | Authorization? |
|---|---|---|
| `capabilities_text` / routing hints | Natural-language description embedded for **pgvector agent routing** (ADR-002) | No |
| `required_mcp_capabilities` | MCP capability strings the agent may request at runtime (e.g. `read_files`) | Yes — adapter gate |
| `allowed_tool_ids` | Concrete tool bindings from the tool manifest (#41) | Yes — dispatch + adapter gate |

Agent type manifests (#42) declare **`required_mcp_capabilities`** and **`allowed_tool_ids`**. The router validates that every required capability is satisfied by the tenant's configured servers **before** dispatch (AND semantics — see §5). Semantic routing text MUST NOT be treated as an authorization contract.

### 2. Capability-Gated Client Adapter

All agent-runtime MCP access flows through a **capability-gated client adapter** in `agent-runtime/src/harpia_agents/mcp/`.

```text
Agent code
  → mcp.request(capability="read_files", tool="read", args={...})
  → CapabilityGatedClient
      1. Read execution snapshot MCP bindings (ADR-012) — not live registry
      2. Validate capability ∈ manifest.required_mcp_capabilities
      3. Validate tool ∈ manifest.allowed_tool_ids (when tool-level binding exists)
      4. Resolve capability → snapshotted MCPServerConfig via binding_ref
      5. Fetch credentials via GetMCPCredentials (if OAuth — §7)
      6. Emit mcp.tool.attempt audit event (§8) — fail-closed if not durable
      7. Connect via MCP transport (stdio / HTTP / SSE)
      8. Emit mcp.tool.completed audit event (best-effort, retried)
      9. Route tool call to resolved server
```

**Hard rule:** agent code MUST NOT call `mcp.connect("filesystem-server")`, pass `server_alias`, or reference server names directly. Server names and aliases are infrastructure identifiers for platform engineers and tenant admins — not agent logic. Disambiguation when multiple servers provide the same capability is resolved from **tenant config defaults, executor installation, or plan slot binding** captured in the execution snapshot (§4.1).

The adapter wraps `langchain-mcp-adapters` (already in `agent-runtime/pyproject.toml`) behind this resolution layer.

### 3. Request by Capability, Not Server Name

| Layer | Requests by | Example |
|---|---|---|
| Agent logic / LangGraph tools | **Capability only** | `capability="read_files"` |
| Capability-gated client | Capability + **snapshotted binding_ref** | Resolved from execution context |
| Platform engineer / tenant admin | **Server name / alias** | `filesystem-local`, `gdrive-prod` |
| Plan slot / ExecutorInstallation binding | **Pinned server config ID** | Frozen in PlanExecution snapshot |
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
- Agent-runtime receives server endpoints from the **execution snapshot** (§4.1) — not hardcoded in the chart and not from live registry reads mid-run.
- Network policies restrict agent-runtime pods to egress only to configured MCP server services (see §6).

The existing `agent-runtime/src/harpia_agents/mcp_servers/` package may contain server **implementations**, but those implementations are executed as separate processes — never imported by the LangGraph worker at runtime.

#### 4.1 Execution Snapshot (ADR-012 Alignment)

Per ADR-012, a `PlanExecution` (or legacy task run) materializes a **frozen snapshot** at start: plan configuration, executor installations, and MCP bindings. MCP resolution during execution MUST read from this snapshot, not the live tenant registry.

Snapshot fields (conceptual):

```text
McpBindingSnapshot {
  server_config_id
  server_config_version
  server_alias          // admin label; not passed to agent code
  capabilities[]        // declared at registration time
  transport, endpoint_ref
  oauth_secret_ref
  is_default, priority  // disambiguation metadata copied at snapshot time
}
```

Live CRUD on tenant MCP configs affects **future** runs only. Audit events (§8) include `server_config_id` + version from the snapshot for tamper-evident replay.

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

1. Exactly one registration SHOULD set `is_default = true` per `(tenant, capability)`; the adapter uses that server when no slot binding overrides exist.
2. `priority` (lower = preferred) breaks ties when importing configs or when no default is set — **stable ordering must not depend on CRUD/import sequence alone**.
3. **Plan slot bindings** and **ExecutorInstallation** records MAY pin a specific `server_config_id` for a step; the snapshot carries that pin for the run.
4. Agent code never passes disambiguation hints; platform engineers document expected pins in manifest metadata and plan templates.

If multiple servers match and no default, pin, or priority winner exists, resolution fails with `FailedPrecondition` (configuration error surfaced to the Overseer).

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

`required_mcp_capabilities` and `allowed_tool_ids` are the authorization contracts. `capabilities_text` remains a routing hint only (pgvector). The router rejects dispatch when the tenant's MCP registry does not satisfy the required capability set **at snapshot time**. Dynamic capability discovery at runtime is out of scope for MVP.

#### 5.6 Audit Granularity

**Decision:** log **every** MCP tool call. The audit log is the trust substrate for Overseer visibility and incident response. Sampling is deferred to post-MVP optimization only with explicit policy approval.

### 6. Production Network Policy (Coarse Defense-in-Depth)

Static Kubernetes `NetworkPolicy` objects **cannot** track per-tenant MCP server registrations at runtime. MVP network controls are **coarse defense-in-depth** by deployment, not dynamic authorization.

**MVP (Helm chart):**

1. **Agent-runtime egress:** allow ConnectRPC to control-plane; allow MCP transport to the **known MCP server Services** declared in the chart (one Service per bundled server type).
2. **MCP server egress:** allow only to the external provider endpoints that server type requires (e.g. `api.github.com`, `slack.com`).
3. **Default deny** inside the MCP namespace for lateral movement between unrelated server pods.

**Explicit non-goal:** generating per-tenant NetworkPolicies from live registry state. That requires a controller or policy generator (open follow-up: dynamic egress policy workstream). Until then, tenant isolation for MCP is enforced in software by the capability-gated adapter (§2) and execution snapshots (§4.1), not by kube-proxy rules alone.

Document baseline policies in `deploy/` README when MCP workloads land.

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

Every MCP tool invocation produces **two audit phases** via `common.v1.AuditEvent` (#65):

| Phase | Action | When | Required? |
|---|---|---|---|
| Attempt | `mcp.tool.attempt` | Before MCP connect / tool invoke | **Yes — fail-closed** |
| Completed | `mcp.tool.completed` | After success or error | Best-effort with retry |
| Denied | `mcp.tool.denied` | Capability or tool not authorized | **Yes — fail-closed** |

**Delivery semantics:**

1. Adapter writes attempt/denied events to a **durable outbox** in control-plane (Postgres) synchronously before invoking the tool.
2. If the outbox write fails or times out, the tool call **does not proceed** (fail-closed).
3. Completion events enqueue asynchronously; a background worker retries delivery. Missing completions are flagged for reconciliation (Overseer UI may show "attempt without completion").
4. Payloads exclude secrets and raw tool arguments; include snapshot `server_config_id`, capability, tool name, latency, status.

| Field | Value |
|---|---|
| `tenant_id` | From `identity.RequestContext` |
| `actor` | Agent instance ID + agent type ID |
| `action` | `mcp.tool.attempt` / `mcp.tool.completed` / `mcp.tool.denied` |
| `resource` | `{capability}/{tool_name}` |
| `metadata` | Snapshotted server config ID + version, latency, success/failure (no secrets) |

Cross-capability access attempts (capability not in `required_mcp_capabilities` or tool not in `allowed_tool_ids`) emit `mcp.tool.denied` and return `PermissionDenied` without connecting.

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
8. Extend agent type manifest validation: `required_mcp_capabilities` + `allowed_tool_ids` cross-checked against tenant registry (#41, #42)
9. Add `is_default` + `priority` fields to `MCPServerConfig`; document disambiguation rules in #43
10. Update [ADR-007](ADR-007-agentic-architecture-patterns.md) §7 with MCP capability-gating cross-reference

## Related ADRs and Issues

| Link | Relationship |
|---|---|
| [ADR-006](ADR-006-domain-driven-design.md) | Capability registry lives in Agent Orchestration bounded context |
| [ADR-007](ADR-007-agentic-architecture-patterns.md) | Refines — MCP is the canonical context source for ephemeral agents; see §7 cross-update |
| [ADR-012](ADR-012-plan-centric-task-model.md) | Aligns — MCP bindings snapshotted on PlanExecution start |
| [ADR-008](ADR-008-tenant-safe-generic-infra-boundaries.md) | Aligns — no platform-default servers; tenant isolation at MCP boundary |
| #41 | Tool manifest + agent↔tool binding (absorbs registry client scope) |
| #42 | Agent type manifest — `required_mcp_capabilities` + `allowed_tool_ids` are authorization contracts; routing text is not |
| #43 | Per-tenant MCP server config |
| #45 | MCP client infra in agent-runtime |
| #48 | OAuth helper — `GetMCPCredentials` |
| #49 | UI for MCP server and OAuth configuration |
| #65 | `common.v1.AuditEvent` envelope for audit trail |
