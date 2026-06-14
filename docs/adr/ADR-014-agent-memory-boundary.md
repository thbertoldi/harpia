# ADR-014: Agent Memory Boundary

**Status:** Accepted
**Date:** 2026-06-14
**Deciders:** thbertoldi

**References:**
- ADR-007: Agentic Architecture Patterns for Harpia
- ADR-009: LangGraph State Semantics
- `proto/harpia/plans/v1/plans.proto`
- `proto/harpia/executors/v1/executors.proto`
- `docs/agents/MANIFEST.md`

## Context

ADR-007 established Harpia agents as ephemeral, stateless executors. That
decision remains correct, but the platform is moving from ad hoc tasks toward
configured plans with `PlanExecution`, `StepExecution`, `ExecutorInstallation`,
typed artifacts, and long-running elicitation.

That evolution raises a product and architecture question: should an agent that
exists during plan execution gain persistent memory, or should it continue to
depend on MCP tools and explicit execution context?

The product need is real. Customers will expect the platform to remember brand
voice, audience, prior approved content, product facts, CRM context, user
preferences, and past plan outcomes. The risk is also real. If that memory lives
inside an opaque agent instance, plan runs become harder to reproduce, audit,
retry, snapshot, price, and isolate by tenant.

The boundary we need is:

> Agents are ephemeral runners. Memory belongs to the platform, the tenant, and
> the execution record.

## Decision

### 1. Agent Instances Stay Ephemeral

An `AgentExecutor` is re-instantiated for a `StepExecution` or step attempt. It
does not carry private cross-run memory. A senior or junior agent SKU can differ
by graph, model, prompt, tool set, pricing, and policy, but not by hidden
autobiographical state accumulated across tenants or plan runs.

Persistent agent-level memory remains rejected for MVP and should not be added
as a convenience feature inside `agent-runtime`.

### 2. Memory Is Explicit and Layered

Harpia uses four memory layers:

| Layer | Scope | Owner | Purpose |
|---|---|---|---|
| `AgentCheckpoint` | One `StepExecution` attempt or elicitation thread | Workflow/runtime | Resume the same in-flight attempt after worker crashes or overseer responses |
| `StepContextPack` | One `StepExecution` | Plan execution | Snapshot the exact inputs, upstream artifacts, executor installation, selected memory resources, and elicitation history used by a step |
| `PlanExecution` memory | One plan run | Plan workflow | Preserve artifacts, approvals, elicitation answers, outputs, and step status for the run |
| `MemoryResource` | Tenant/workspace/brand/user scope | Tenant/platform | Durable reusable knowledge such as brand voice, style guides, examples, catalogs, audience notes, CRM summaries, and approved content |

Only `MemoryResource` crosses plan-run boundaries. It is tenant-owned,
versioned, permissioned, and inspectable. Checkpoints and context packs are
execution state, not product memory.

A retry that creates a new attempt must start from the `StepContextPack`,
upstream artifacts, failure metadata, and overseer feedback. It must not inherit
the previous attempt's checkpoint unless a retry policy explicitly copies a
small, audited subset into the new attempt context.

### 3. Memory Access Goes Through Tools

Agents may read or write memory only through declared tools: MCP servers,
control-plane services, or a future memory adapter that follows the same
capability and audit rules. Agent code must not connect directly to a private
vector store or hidden local cache.

Memory-backed tools must be represented in the agent manifest's
`allowed_tool_ids` contract. `MemoryBinding` narrows which tenant memory
resources those tools may access for a configured plan or executor
installation. An agent with a memory-capable tool but no resolved binding for a
step receives no memory resources for that step.

For MVP, memory resources may be backed by artifacts and Garage object storage.
The control plane owns metadata, permissions, version IDs, provenance, and
schema validation. Indexes or embeddings are derived views of those resources,
not the canonical memory.

### 4. Memory Use Is Snapshotted and Audited

Every agent-backed step must be explainable after the run. A `StepExecution`
that uses memory records:

- which `MemoryResource` IDs and versions were made available;
- which resources were actually read or written;
- which tool/capability performed the access;
- what executor installation snapshot and agent manifest version were active;
- which elicitation answers or approvals changed the context.

This produces a `MemoryUsageRecord` per step. The record may omit secret payloads
and large content bodies, but it must keep enough metadata for audit, replay
analysis, and user-facing "why did the agent do that?" inspection.

Memory bindings may be authored on plan configuration or executor installation,
but the resolved binding and resource versions must be frozen into the
`PlanExecution`/`StepContextPack`. Mutable configuration alone is not sufficient
for replay or audit.

### 5. Learning Is Productized, Not Private

Metrics such as trust score, cost, success rate, rejected outputs, and canary
performance belong to platform services keyed by agent type/SKU, task domain,
tenant, or workspace. They do not become private memory on the agent instance.

If the platform learns a durable preference from feedback, it must write that
preference to an explicit memory resource or configuration object with user
visibility and tenant controls.

Promotion of plan outputs into durable memory is a controlled workflow. Approved
artifacts, elicitation answers, and rejection feedback do not silently become
memory. A human-approved or policy-approved promotion step writes a new
`MemoryResource` version with provenance back to the originating
`PlanExecution`, `StepExecution`, artifact, and approver.

## Rationale

Ephemeral agents keep the execution model simple: a failed step can be retried,
a plan run can be inspected, and a tenant can swap an executor without inheriting
opaque state. This also preserves the contract that a `PlanStep` is satisfied by
an executor through typed input and output artifacts, not by whichever hidden
history happens to exist in an agent process.

Explicit memory still gives the product the behavior customers want. A LinkedIn
writer can "remember" a brand voice because the plan binds it to a versioned
brand memory resource. A newsletter writer can learn from prior approved drafts
because it queries an approved-content memory resource. The difference is that
the memory is visible, permissioned, versioned, and auditable.

### Alternatives Considered

| Alternative | Why not |
|---|---|
| Persistent agent instances with private memory | Breaks replayability and auditability; complicates tenant isolation, retries, and executor interchangeability |
| MCP-only with no first-class memory model | Too vague for product UX, versioning, provenance, and execution snapshots |
| Store memory only as artifacts | Artifacts provide payload storage, but memory also needs bindings, permissions, provenance, indexing, and usage records |
| Global shared memory across agents | Violates tenant isolation and recreates the blackboard pattern rejected by ADR-007 |

## Consequences

### What Becomes Easier

- **Auditable runs** -- execution details can show exactly which memory shaped a step
- **Tenant control** -- customers can inspect, edit, delete, or disable memory resources
- **Replay and retry discipline** -- in-flight state is scoped to `StepExecution`
- **Executor marketplace clarity** -- senior/junior agents differ by declared capability, not hidden accumulated state
- **Safer MCP integration** -- memory follows the same tool declaration and audit discipline as external context

### What Becomes Harder

- The platform needs memory-specific metadata, versioning, and UI instead of a
  simple agent-local vector store.
- `StepExecution` records need context-pack and usage-record fields.
- Agent-runtime needs checkpoint discipline that distinguishes resumable
  execution state from durable product memory.
- The configuration UI must expose memory bindings so users understand what an
  agent can use.

### Actions Required

1. Add a `MemoryResource` registry and API in the control plane. It should store
   tenant/workspace/brand scope, type, schema, version, provenance, permissions,
   and backing artifact references.
2. Add `MemoryBinding` support to plan configuration and/or executor
   installation configuration, then freeze the resolved binding into
   `PlanExecution` and `StepContextPack`.
3. Add `StepContextPack` and `MemoryUsageRecord` fields to plan execution
   storage and APIs.
4. Add agent-runtime checkpointing scoped to `StepExecution` attempts and
   elicitation threads. Checkpoints must never be treated as reusable memory.
5. Ensure memory-backed tools are declared through `allowed_tool_ids` and
   constrained by resolved `MemoryBinding` records.
6. Add at least one memory-backed tool/MCP adapter for tenant knowledge lookup.
7. Add a controlled promotion workflow for approved outputs, feedback, and
   preferences into new `MemoryResource` versions.
8. Add user-facing workspace or brand memory management after the backend
   resource model exists.
