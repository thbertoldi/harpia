# ADR-007: Agentic Architecture Patterns for Harpia

**Status:** Accepted
**Date:** 2026-06-04
**Deciders:** thbertoldi

**References:**
- Arsanjani, A. & Bustos, J.P. (2026) *Agentic Architectural Patterns for Building Multi-Agent Systems*. Packt Publishing.
- Khononov, V. (2021) *Learning Domain-Driven Design*. O'Reilly Media.
- ADR-002: Temporal + LangGraph Boundary
- ADR-006: Domain-Driven Design for Agentic Architecture
- ADR-011: MCP Capability Gating and Out-of-Process Worker Discipline (refines §7 below)

## Context

Arsanjani & Bustos catalog 40+ architectural patterns for multi-agent systems across coordination (Ch.5), fault tolerance (Ch.7), human interaction (Ch.8), agent-level behavior (Ch.9), and production readiness (Ch.10). Their model assumes **persistent agent ecosystems** — agents that live forever, accumulate knowledge, negotiate with peers, and self-organize.

Harpia's architecture is fundamentally different. Agents are **ephemeral executors** instantiated per-subtask within a task's bounded lifecycle (beginning → middle → end). Context comes from the task definition, parent results, MCP tools, and human feedback — not from persistent agent memory. The human Overseer owns all escalation decisions. This ADR identifies which patterns apply and which are actively rejected.

## Decision

### 1. Harpia Agent Lifecycle Model

```
Task created (Leader)
  ↓
Supervisor decomposes → subtasks (LangGraph planner)
  ↓
For each subtask:
  → Router suggests best agent type (pgvector + trust score + cost)
  → Overseer approves or selects a different agent type
  → Agent instance created (ephemeral, stateless)
  → Agent executes using MCP tools for domain context
  → Overseer reviews output (approve / modify / reject)
  → If rejected: Overseer provides feedback, optionally switches agent type, retries
  → Agent instance completed
  ↓
Task completed
```

Agents are **stateless**. They do not remember previous tasks. Context comes from:
1. **Task context** — the supervisor passes down the task description, parent subtask results, and constraints
2. **MCP tools** — agents call external systems via the **capability-gated MCP adapter** ([ADR-011](ADR-011-mcp-capability-gating.md)): request by capability, never by server name; servers run out-of-process; bindings come from the execution snapshot
3. **Overseer feedback** — human corrections, guidance, and agent type overrides

This is a **function-as-a-service** model, not a persistent service mesh.

### 2. Patterns We Adopt

| # | Pattern | Source (Ch.) | Implementation |
|---|---|---|---|
| P1 | **Agent Router (human-gated)** | Ch.5 | pgvector matches subtask → best agent type. Suggestion is shown to Overseer with trust score, cost estimate, and rationale. Overseer approves or selects another. |
| P2 | **Agent Calls Human** | Ch.8 | Two human gates per subtask: (a) Overseer approves agent type selection before dispatch, (b) Overseer reviews output after execution. Both via `human_feedback` node. |
| P3 | **Hybrid Planner + Scorer** | Ch.11 | Add a `scorer` node after `planner`. Validates plan coherence: Are subtasks correctly scoped? Are dependencies respected? Is total cost within budget? Routes back to planner on failure. |
| P4 | **Adaptive Retry with Prompt Mutation** | Ch.7 | When a subtask retries (Temporal activity retry + human feedback), the prompt is mutated to incorporate Overseer feedback and previous failure context. Not a different model — the same agent, smarter prompt. |
| P5 | **Trust Decay and Scoring** | Ch.7 | Track `success_count / total_count` per agent type per task domain. Display trust score to Overseer during suggestion approval. Agent types below threshold are flagged for Platform Engineer review. New agent types start with a neutral score (Cold Start handled by canary testing). |
| P6 | **Canary Agent Testing** | Ch.7 | When a new agent type version is registered, route a configurable percentage of matching tasks to it alongside the stable version. Compare results. Auto-promote if canary outperforms stable over N trials. |

### 3. Patterns We Reject

| Pattern | Source | Reason |
|---|---|---|
| Fallback Model Invocation | Ch.7 | Harpia does not automate model escalation. The human Overseer decides when to switch agent types. Automation removes the human from the loop, which violates our core premise. |
| Blackboard Knowledge Hub | Ch.5 | Tasks are tenant-isolated and task-bound. Cross-task knowledge sharing, if needed, goes through MCP tools (tenant knowledge artifacts), not a shared agent memory space. |
| Agent Negotiation / Consensus | Ch.5 | Workers are isolated executors. The supervisor owns all decomposition decisions. The Overseer owns all approval decisions. No peer-to-peer agent communication exists. |
| Supervision Tree / Formation Control | Ch.5 | Flat supervisor → workers structure is correct for MVP. Hierarchical supervisors may emerge naturally when tasks contain nested subtasks, but that complexity is deferred. Drone swarm analogies do not apply. |
| Agent-Level Persistent Memory | Ch.9 | Agents are ephemeral per subtask. Memory lives in MCP tools, not in the agent instance. If an agent needs history, it queries the relevant tool. |
| Coevolved Agent Training | Ch.11 | Requires a feedback pipeline and model training infrastructure. Post-MVP. |

### 4. LangGraph Graph — Revised Structure

```
           ┌─────────┐
           │ planner │  decompose task → subtasks
           └────┬─────┘
                │
           ┌────▼─────┐
    ┌──────│  scorer  │  validate: coherence, dependencies, cost
    │      └────┬─────┘
    │   [fail]  │  [pass]
    └───────────┘  │
                   ▼
           ┌──────────┐
           │  worker   │  execute current subtask
           └────┬──────┘
                │
           ┌────▼─────┐
           │  router   │  conditional edges
           └────┬──────┘
     ┌──────────┼──────────┐
     ▼          ▼          ▼
  worker    human_fb     END
  (next)    (await
           overseer)
```

**New node: scorer** — validates the plan before dispatch. Checks:
- Are all subtasks independently scoped (no overlap)?
- Are dependencies between subtasks satisfied?
- Is the total estimated cost within the task's `cost_budget`?
- Are suggested agent types compatible with the subtask descriptions?

If the scorer rejects the plan, it returns to `planner` with specific critique. The planner regenerates the decomposition with the scorer's feedback.

### 5. Human Gates (Overseer Approval Points)

```
Gate 1 — Agent Type Selection
  Overseer sees: subtask description, suggested agent type,
  trust score, estimated cost. Actions: approve, pick different
  agent type, or modify task parameters.

Gate 2 — Output Review
  Overseer sees: execution result, confidence score, execution
  trace. Actions: approve, reject with feedback, or reject
  and switch agent type for retry.
```

Both gates are implemented as the existing `human_feedback` LangGraph node. Temporal signals carry the Overseer's decision.

### 6. Maturity Level

Per Arsanjani & Bustos's GenAI Maturity Model (Ch.12):

| Level | Description | Harpia Status |
|---|---|---|
| 1–3 | Foundational → Capable but Brittle | Completed |
| **4** | **Resilient Collaborative Team** | **Current — multi-agent with Temporal durability** |
| 5 | Self-Correcting Ecosystem | Target — scorer node, trust tracking, canary testing |
| 6 | Self-Improving | Post-MVP — coevolved training |

### 7. MCP Integration Discipline (Refined by ADR-011)

This section records the agent-runtime boundary for MCP that ADR-011 formalizes. Ephemeral agents (§1) MUST obtain domain context through MCP, but with strict discipline:

| Rule | Rationale |
|---|---|
| Request by **capability**, not server name | Prevents agent logic from coupling to tenant infrastructure topology |
| MCP servers run **out-of-process** | Crash isolation, independent scaling, aligns with MCP spec and ADR-011 §4 |
| Authorization via manifest `required_mcp_capabilities` + `allowed_tool_ids` | Separates pgvector routing hints from tool authorization (ADR-011 §1) |
| Resolve bindings from **execution snapshot** | ADR-012 — live registry edits must not change in-flight runs |
| Audit every attempt/completion | Overseer trust substrate; fail-closed on attempt (ADR-011 §8) |

Agent code never embeds MCP server implementations or passes `server_alias`. The capability-gated client in `agent-runtime/src/harpia_agents/mcp/` is the sole MCP entry point for LangGraph workers.

Cost-aware model usage remains governed separately by [ADR-010](ADR-010-budget-policy-service.md) at the LLM invocation boundary.

## Consequences

### What Becomes Easier
- **Plan quality** improves via scorer validation before resources are spent
- **Overseer trust** increases because suggestions come with rationale and track record
- **Agent type reliability** is measurable over time (trust scores)
- **Platform Engineer** can safely deploy new agent types via canary testing

### What Becomes Harder
- The LangGraph graph gains a node and another conditional edge — more state to debug
- Two human gates per subtask increase latency; acceptable for async task model
- Trust scoring requires tracking execution outcomes, adding a write path to agent type metrics

### Actions Required
1. Add `scorer` node with conditional edge in `agent-runtime/src/harpia_agents/graph.py`
2. Add `model_priority`, `success_count`, `total_count`, `canary_pct` columns to `agent_types` table (migration `000003`)
3. Display trust score and estimated cost in the Overseer's Gate 1 UI (issue #18)
4. Implement prompt mutation decorator for Temporal activities in `agent-runtime/src/harpia_agents/temporal/worker.py`
5. Register Overseer approval gates in the Temporal workflow signal handler
6. Update ADR-002 with the revised graph structure
