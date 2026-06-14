# ADR-010: Budget Policy Service

**Status:** Accepted
**Date:** 2026-06-13
**Deciders:** thbertoldi

**References:**
- ADR-003: Data Architecture
- ADR-006: Domain-Driven Design for Agentic Architecture
- ADR-007: Agentic Architecture Patterns for Harpia
- ADR-012: Plan-Centric Task Model
- Issue #33: BYO API keys
- Issue #34: Cost tracking + per-tenant quotas
- Issue #35: LLM provider abstraction
- Issue #65: `common.v1` envelope types (`Tenant`, `AuditEvent`)

## Context

Three independent issues — #33 (BYO API keys), #34 (cost tracking + per-tenant quotas), and #35 (LLM provider abstraction) — all implement cost-related logic. The planner (#44), canary tester (#26), and trust scorer (#24/#25) must make cost-aware decisions before dispatching agents or selecting models. Without a single policy surface, every consumer will re-implement cost evaluation differently, quota state will be racy, and BYO key resolution will leak into LangGraph nodes.

Multi-perspective review during `bmad-generate-project-context` Cat 1 (2026-06-05) reached this synthesis:

- Mary argued for elevating Budget & Cost Control to a **6th bounded context** (Paperclip-style).
- Victor pushed back: bounded contexts are expensive; contracts are cheap. The need is a **policy surface**, not a new context.
- Synthesis: carve out a Budget Policy Service **inside** Agent Orchestration with its own proto package, enforcement middleware, and a single owner. ADR-006's five-context discipline is preserved.

ADR-007 already gates dispatch on cost: the `scorer` node validates that total estimated cost fits the task's `cost_budget`, and the Overseer sees cost estimates at Gate 1. This ADR defines the service that owns quota state, evaluates budgets, resolves provider credentials, and records usage — so those gates call one contract instead of ad-hoc logic.

## Decision

### 1. Capability, Not a Bounded Context

Budget Policy is a **capability inside Agent Orchestration**, not a sixth bounded context.

| Aspect | Bounded context | Capability (our choice) |
|---|---|---|
| Ubiquitous language | Own vocabulary, lifecycle, team | Reuses Agent Orchestration terms: `cost_budget`, `agent_type`, `provider`, `tenant` |
| Integration pattern | Partnership or Conformist with peers | Internal domain service called by orchestration nodes |
| Proto package | Standalone service boundary | `harpia.budget.v1` — owned by Agent Orchestration team, colocated contract |
| Operational cost | New on-call rotation, separate migrations team | Same owner as agent-runtime + control-plane budget package |

**Rationale vs Paperclip-style elevation:** Paperclip treats budget as a cross-cutting platform because agents are persistent and negotiate resources peer-to-peer. Harpia agents are **ephemeral executors** (ADR-007): they are instantiated per subtask, stateless, and gated by a human Overseer. Cost decisions happen at orchestration boundaries (plan, score, resolve model, record usage) — not as an autonomous agent-to-agent negotiation. A dedicated bounded context would add context-mapping ceremony (Partnership with Identity, Conformist to Tenants) without a distinct ubiquitous language or lifecycle. The policy surface is real; the context boundary is not.

**Ownership and boundaries:**

```
┌─────────────────────────────────────────────────────────────┐
│                  Agent Orchestration (BC)                    │
│  ┌─────────────┐  ┌──────────────────┐  ┌───────────────┐  │
│  │ LangGraph   │  │ Budget Policy    │  │ Agent Router  │  │
│  │ planner /   │──│ Service          │──│ (trust+cost)  │  │
│  │ scorer /    │  │ (capability)     │  │               │  │
│  │ worker      │  └────────┬─────────┘  └───────────────┘  │
│  └─────────────┘           │                               │
└────────────────────────────┼───────────────────────────────┘
                             │ Conformist (tenant_id, RLS)
                             ▼
                    ┌─────────────────┐
                    │ Identity &      │
                    │ Tenants         │
                    └─────────────────┘
```

- **Owns:** quota ledger, `cost_budget` evaluation, BYO API key resolution (encrypted retrieval), provider/model selection gated on budget + tenant policy, usage event append.
- **Does not own:** task lifecycle state machine (Plan Management / legacy Task Management), human approval UX (Human Interaction), agent capability matching (Agent Registry), workflow durability (Temporal).
- **Implementation home:** `control-plane/internal/budget/` (Go). Stateful, tenant-isolated, RLS-aware — fits the control-plane's repository pattern (ADR-003).
- **Consumers:** LangGraph `planner` and `scorer` nodes (Python, via Connect client), LangGraph **model factory / LLM callbacks** (primary enforcement), optional ConnectRPC entry guards, settings UI (#54), canary tester (#26), reflection loop (#46).

### 2.1 Dependency on ADR-012 Plan Snapshots

Budget evaluation operates on **frozen execution context**, not live tenant configuration.

| Path | Budget source | ID fields |
|---|---|---|
| **Plan-centric** (ADR-012) | `PlanExecution` snapshot: aggregated step `cost_estimate` values + optional run-level cap in `PlanBehaviorPolicies` | `plan_execution_id`, `step_execution_id`, `plan_configuration_snapshot_id` |
| **Adaptive legacy** (pre-migration) | Task-level `cost_budget` on the active run | `plan_execution_id` maps to legacy task ID; `step_execution_id` maps to subtask ID |

When a `PlanExecution` starts, Temporal materializes a snapshot of `PlanConfiguration`, executor installations, and per-step cost estimates. `EvaluatePlan` and `ReserveBudget` MUST read `cost_budget` and per-step ceilings from that snapshot so mid-run configuration edits cannot inflate spend. `ReleaseBudget` runs on step completion to return unused reservation headroom to the run ledger.

This ADR does **not** define plan template pricing UX (#120); it defines the enforcement contract once a run snapshot exists.

### 2. Proto Contract Sketch

New package: `proto/harpia/budget/v1/budget.proto` — `BudgetPolicyService`.

Depends on `common.v1` types from issue #65 (`Tenant`, `AuditEvent`) for tenant scoping and event-sourced usage recording.

```protobuf
syntax = "proto3";

package harpia.budget.v1;

import "google/protobuf/timestamp.proto";
import "harpia/common/v1/common.proto";

service BudgetPolicyService {
  // Called by planner (#44) and scorer (#24) before dispatch.
  rpc EvaluatePlan(EvaluatePlanRequest) returns (EvaluatePlanResponse);

  // Holds estimated spend against the run budget before LLM work begins.
  rpc ReserveBudget(ReserveBudgetRequest) returns (ReserveBudgetResponse);

  // Called at each LLM invocation boundary (model factory / callback).
  rpc ResolveModel(ResolveModelRequest) returns (ResolveModelResponse);

  // Called after each LLM invocation; appends to usage ledger (idempotent).
  rpc RecordUsage(RecordUsageRequest) returns (RecordUsageResponse);

  // Releases unused reservation on step completion or cancellation.
  rpc ReleaseBudget(ReleaseBudgetRequest) returns (ReleaseBudgetResponse);

  // Read-only quota snapshot for UI (#54) and pre-flight checks.
  rpc GetQuota(GetQuotaRequest) returns (GetQuotaResponse);
}

message EvaluatePlanRequest {
  harpia.common.v1.Tenant tenant = 1;
  string plan_execution_id = 2;          // PlanExecution or legacy adaptive task run ID
  string plan_configuration_snapshot_id = 3; // frozen config ref (ADR-012)
  repeated SubtaskCostEstimate subtasks = 4;
  Money cost_budget = 5;                 // ceiling copied from execution snapshot
}

message SubtaskCostEstimate {
  string subtask_key = 1;
  string agent_type_id = 2;
  string provider = 3;                   // e.g. "openai", "anthropic"
  Money estimated_cost = 4;
  int64 estimated_tokens = 5;
}

message EvaluatePlanResponse {
  bool allowed = 1;
  string reason = 2;                     // human-readable when allowed=false
  Money remaining_budget = 3;
  repeated SubtaskCostEstimate over_budget_subtasks = 4;
}

message ResolveModelRequest {
  harpia.common.v1.Tenant tenant = 1;
  string plan_execution_id = 2;
  string step_execution_id = 3;          // StepExecution / subtask instance
  string agent_type_id = 4;
  string subtask_key = 5;
  string provider_preference = 6;        // optional; tenant default if empty
  string reservation_id = 7;             // required; from ReserveBudget
  string idempotency_key = 8;            // unique per LLM invocation; dedupes RecordUsage
  Money max_spend = 9;                   // per-invocation ceiling from step snapshot
}

message ReserveBudgetRequest {
  harpia.common.v1.Tenant tenant = 1;
  string plan_execution_id = 2;
  string step_execution_id = 3;
  Money amount = 4;                      // estimated spend to hold for this step
  string idempotency_key = 5;
}

message ReserveBudgetResponse {
  string reservation_id = 1;
  Money remaining_budget = 2;
  google.protobuf.Timestamp expires_at = 3;
}

message ReleaseBudgetRequest {
  harpia.common.v1.Tenant tenant = 1;
  string reservation_id = 2;
  Money actual_spend = 3;                // optional; reconciles hold vs usage
}

message ReleaseBudgetResponse {
  Money remaining_budget = 1;
}

message ResolveModelResponse {
  string provider = 1;
  string model_id = 2;
  ChatModelHandle handle = 3;            // opaque ref: resolved credentials + model config
  Money estimated_unit_cost = 4;
}

message ChatModelHandle {
  string handle_id = 1;                // short-lived token; not the raw API key
  google.protobuf.Timestamp expires_at = 2;
}

message RecordUsageRequest {
  harpia.common.v1.Tenant tenant = 1;
  harpia.common.v1.AuditEvent audit = 2;
  string reservation_id = 3;
  string idempotency_key = 4;            // same key passed to ResolveModel for this call
  string provider = 5;
  string model_id = 6;
  int64 input_tokens = 7;
  int64 output_tokens = 8;
  Money usd_cost = 9;
  string plan_execution_id = 10;
  string step_execution_id = 11;
}

message RecordUsageResponse {
  Money remaining_budget = 1;
  bool quota_warning = 2;                // true when within 10% of cap
}

message GetQuotaRequest {
  harpia.common.v1.Tenant tenant = 1;
  string provider = 2;                   // optional filter
}

message GetQuotaResponse {
  repeated ProviderQuota quotas = 1;
}

message ProviderQuota {
  string provider = 1;
  int64 tokens_used_month = 2;
  int64 tokens_limit_month = 3;
  Money usd_used_month = 4;
  Money usd_limit_month = 5;
  google.protobuf.Timestamp period_reset_at = 6;
}

message Money {
  string currency = 1;                   // ISO 4217, default "USD"
  int64 amount_micros = 2;               // 1 USD = 1_000_000 micros
}
```

### 3. Enforcement: Per-Invocation Model Boundary (Not Handler-Only)

A single ConnectRPC handler (e.g. `ExecuteTask`) may run a LangGraph subgraph with **multiple** LLM calls. Handler-level middleware alone cannot enforce budget limits or record usage accurately. Enforcement MUST sit at the **model factory / LLM callback / per-invocation** layer, with optional RPC entry guards for defense in depth.

**Primary path (authoritative):**

```text
LangGraph worker / LLM node
  → BudgetModelFactory.get_model(ctx)
      1. ReserveBudget(plan_execution_id, step_execution_id, estimated_amount)
         — skipped if an unexpired reservation_id is already in step context
      2. ResolveModel(..., reservation_id, idempotency_key, max_spend)
         — hard gate; reject if reservation missing or exhausted
      3. Return BaseChatModel wrapped with UsageRecordingCallback
  → each LLM completion:
      UsageRecordingCallback → RecordUsage(idempotency_key, tokens, usd, reservation_id)
  → step terminal state:
      ReleaseBudget(reservation_id, actual_spend)
```

**Reservation flow:** `EvaluatePlan` validates the **whole plan** against the snapshot budget. `ReserveBudget` holds estimated spend per step before the first `ResolveModel`. Actual usage via `RecordUsage` debits the reservation; overages fail closed unless the Overseer extends budget (§4.4). `idempotency_key` (one per LLM HTTP request) prevents double-charging on Temporal activity retries.

**Secondary path (optional):** a ConnectRPC interceptor on agent-runtime handlers may verify that step context carries a valid `reservation_id` before entering LangGraph, but it MUST NOT be the only enforcement point.

Implementation homes:

| Component | Location |
|---|---|
| Budget model factory + callback | `agent-runtime/src/harpia_agents/budget/model_factory.py` |
| Connect client | `agent-runtime/src/harpia_agents/budget/client.py` |
| Optional RPC guard | `agent-runtime/src/harpia_agents/budget/interceptor.py` |
| Policy service | `control-plane/internal/budget/` + Connect registration |

**Call sites:**

| Caller | RPC | When |
|---|---|---|
| LangGraph `planner` | `EvaluatePlan` | After decomposition, before scorer |
| LangGraph `scorer` | `EvaluatePlan` | Validates total cost vs snapshot `cost_budget` (ADR-007 P3) |
| LangGraph worker (step start) | `ReserveBudget` | Before first LLM call in step |
| LangGraph model factory / callback | `ResolveModel` + `RecordUsage` | Immediately before/after each LLM invocation |
| Step completion / cancel | `ReleaseBudget` | Return unused reservation headroom |
| Optional Connect handler guard | validates `reservation_id` in context | RPC entry only |
| Settings UI (#54) | `GetQuota` | Tenant admin dashboard |
| Canary tester (#26) | `EvaluatePlan` | Pre-flight cost check for canary route |

The Budget Policy Service server runs in the control-plane Connect stack (`control-plane/internal/server/` registers `BudgetPolicyService`).

### 4. Resolved Open Questions

#### 4.1 Implementation language

**Decision:** Go in `control-plane/internal/budget/`; Python consumers via Connect client.

| Alternative | Why not |
|---|---|
| Python in agent-runtime | Quota ledger and BYO key storage need RLS-aware Postgres repos already established in control-plane; duplicating tenant isolation in Python increases leak risk |
| Split Go + Python writers | Two writers to quota state guarantees races; single source of truth requires single implementation |

#### 4.2 State store

**Decision:** PostgreSQL source of truth; Valkey TTL cache for `GetQuota` hot reads.

| Store | Role |
|---|---|
| Postgres (`quota`, `usage_event`, `tenant_provider_config` tables) | Authoritative ledger, RLS per tenant (ADR-003) |
| Valkey (`budget:quota:{tenant_id}:{provider}` keys, 30s TTL) | `GetQuota` read path; invalidated on `RecordUsage` write |

Aligns with ADR-003 cache patterns: warming on `RecordUsage`, TTL expiry as safety net. `EvaluatePlan` and `ResolveModel` always read Postgres (consistency over latency).

#### 4.3 Quota granularity

**Decision:** Per-tenant + per-provider initially.

| Dimension | MVP | Deferred |
|---|---|---|
| Tenant | Yes | — |
| Provider (`openai`, `anthropic`, …) | Yes | — |
| Agent type | No | Add when #25 trust scoring enables quotas-by-trust |
| Agent type + provider | No | Composite dimension post-MVP |

Starting coarse avoids combinatorial quota matrix explosion. BYO keys are naturally per-provider. Agent-type quotas become meaningful once trust scores (#24/#25) differentiate risk tiers.

#### 4.4 Failure modes

**Decision:** `cost_budget_exceeded` state-machine path with Overseer override; no silent fallback.

When `EvaluatePlan` returns `allowed=false`:

```text
planner/scorer calls EvaluatePlan
  → allowed=false
  → task/plan status → COST_BUDGET_EXCEEDED
  → Temporal workflow signals Human Interaction (Overseer Gate)
  → Overseer options:
      (a) OVERRIDE_BUDGET — increase cost_budget, resume planning
      (b) MODIFY_PLAN — edit subtasks/constraints, re-enter planner
      (c) CANCEL — terminal failure
```

When `ResolveModel` fails (quota exhausted mid-run or reservation invalid):

```text
model factory calls ResolveModel
  → RESOURCE_EXHAUSTED / reservation invalid
  → step status → QUOTA_EXCEEDED (distinct from plan-level budget)
  → no automatic model downgrade (rejects ADR-007 "Fallback Model Invocation")
  → ReleaseBudget if reservation partially consumed
  → Overseer notified; may override quota or cancel subtask
```

**Invariant:** Budget Policy never auto-escalates to a more expensive model or bypasses quota. The Overseer is the only override authority (consistent with ADR-007 human gates).

## Rationale

### Why a capability inside Agent Orchestration?

Cost policy is inseparable from orchestration decisions: planning, scoring, routing, and execution all need the same numbers. Extracting it as a peer bounded context would force Partnership negotiations with Agent Orchestration on every schema change, while the ubiquitous language would remain orchestration vocabulary (`cost_budget`, `agent_type`, `provider`). A capability with a dedicated proto package gives a stable contract without context-mapping overhead.

### Alternatives Considered

| Alternative | Why not |
|---|---|
| 6th bounded context "Budget & Cost Control" | No distinct lifecycle or team; language overlaps Agent Orchestration; Paperclip model assumes persistent negotiating agents — not Harpia's ephemeral FaaS agents |
| Embed budget logic in each LangGraph node | Duplicated evaluation, racy quota updates, inconsistent BYO key handling (#33/#34/#35) |
| Middleware only, no service | No single ledger; `RecordUsage` and `GetQuota` need authoritative store |
| Handler-only Connect interceptor | Misses multi-call LangGraph handlers; cannot enforce per-invocation budget or idempotent usage |
| Per-request budget check without `EvaluatePlan` | Planner and scorer cannot validate whole-plan cost before spending resources (ADR-007 scorer requirement) |

## Consequences

### What Becomes Easier

- **Single policy surface** — planner, scorer, router, canary tester, and UI share one contract
- **Quota consistency** — one Postgres writer, Valkey cache for reads; no races across Python nodes
- **BYO key isolation** — encrypted key retrieval centralized; never passed through LangGraph state
- **ADR-006 discipline preserved** — five bounded contexts unchanged; budget is a documented capability
- **Downstream issues unblocked** — #33/#34/#35 implement against this contract; #24/#26/#44/#46 gate on it

### What Becomes Harder

- **Cross-process enforcement** — model factory and callbacks must call control-plane on every LLM invocation (latency; mitigated by `ChatModelHandle` short-lived tokens and reservation batching per step)
- **Cache invalidation** — Valkey TTL + write-through invalidation on `RecordUsage` must be correct
- **Overseer workflow** — new `COST_BUDGET_EXCEEDED` status and override signal path in Temporal + Human Interaction
- **Dependency on #65** — `common.v1` types must land before `budget.proto` is finalized

### Cross-References

| Relationship | ADR / Issue |
|---|---|
| Refines | ADR-006 (capability inside Agent Orchestration, not 6th context) |
| Refines | ADR-007 (scorer cost gate, Overseer override, rejects automated model fallback) |
| Aligns | ADR-003 (Postgres + Valkey), ADR-012 (plan execution snapshots supply `cost_budget` and step ceilings) |
| Enables | #33 (BYO keys), #34 (cost tracking + quotas), #35 (LLM provider abstraction), #54 (settings UI) |
| Gates | #24 (scorer), #26 (canary tester), #44 (planner), #46 (reflection loop) |
| Depends on | #65 (`common.v1` — `Tenant`, `AuditEvent`) |

### Actions Required

1. Create `proto/harpia/budget/v1/budget.proto` after #65 lands (`common.v1` types available)
2. Implement `control-plane/internal/budget/` — domain model, Postgres repositories (RLS), Valkey cache adapter
3. Atlas migration: `quota`, `usage_event`, `tenant_provider_config` tables with tenant RLS policies
4. Register `BudgetPolicyService` handler in `control-plane/internal/server/`
5. Implement `BudgetModelFactory` + `UsageRecordingCallback` in `agent-runtime/src/harpia_agents/budget/model_factory.py`
6. Add optional Connect RPC guard in `agent-runtime/src/harpia_agents/budget/interceptor.py`
7. Add Python Connect client adapter in `agent-runtime/src/harpia_agents/budget/client.py`
8. Wire `EvaluatePlan` into LangGraph `planner` and `scorer` nodes (#44, #24)
9. Wire `ReserveBudget` / `ReleaseBudget` into step lifecycle in Temporal + LangGraph worker
10. Add `COST_BUDGET_EXCEEDED` task/plan status + Overseer override signal in Temporal workflow
11. Relabel issues #33, #34, #35 with `capability:budget-policy` label
12. Expose `GetQuota` in settings UI (#54)
