# ADR-002: Workflow and Agent Orchestration

> **⚠ Historical record — not the current source of truth.** The canonical description of the
> platform is the **[Platform Constitution](../architecture/harpia-platform.md)**, which
> supersedes ADR-001…017 as the reading order. See the
> **[ADR supersession map](README.md)** for how this ADR stands today. Where this ADR and the
> constitution disagree, the constitution wins.

**Status:** Accepted
**Date:** 2026-06-03
**Deciders:** thbertoldi

## Context

Harpia agents are AI-driven: they receive requests, decompose them into subtasks, and dispatch to specialized sub-agents based on capability matching. Human approval is required at critical decision points (human-in-the-loop). This demands two kinds of orchestration:

1. **AI reasoning** — what should happen next? (planning, tool selection, dynamic decomposition)
2. **Durable execution** — make sure it happens, with retries, timeouts, and resumability across process restarts.

We considered three patterns:
- LangGraph for everything
- Temporal for everything
- LangGraph + Temporal working together

## Decision

**Use both: LangGraph for AI reasoning, Temporal for durable execution.** They operate at different layers with a clear contract:

| Layer | Technology | Responsibility |
|---|---|---|
| AI Reasoning | LangGraph (Python) | Task decomposition, agent dispatch, tool selection, LLM interactions, dynamic planning |
| Durable Execution | Temporal | Retries, timeouts, human-in-the-loop signals, exactly-once semantics, audit history |

**They do not overlap.** LangGraph decides *what to do*. Temporal ensures it *gets done*.

### Execution Flow

```
1. Temporal Workflow receives "Analyze sales data"
2. Workflow calls agent-runtime via gRPC: DecomposeTask(request)
3. LangGraph supervisor runs: planner → creates subtasks
4. LangGraph returns subtask list to Temporal
5. Temporal iterates subtasks, calls agent-runtime: ExecuteSubtask(subtask)
6. LangGraph worker runs, encounters "needs approval"
7. Temporal pauses workflow, sends signal to Slack bot
8. Human approves in Slack → Slack bot → Temporal signal
9. Workflow resumes, calls agent-runtime: ContinueExecution(subtask_id, approval)
10. Throughout: Connect server-stream pushes progress to Svelte UI
```

### LangGraph Supervisor Graph

The agent-runtime implements a supervisor pattern:
- `planner` node: decomposes task into subtasks
- `worker` node: executes individual subtasks
- `human_feedback` node: blocks and waits for human input
- Conditional edges based on task state

### LangGraph State Model and Merge Semantics

The supervisor state uses a Pydantic `TaskState` model in `agent-runtime/src/harpia_agents/graph.py`. Domain state carried inside LangGraph remains typed: subtasks are `Subtask` instances and worker outputs are `SubtaskResult` instances. Transport adapters may serialize these models at process boundaries, but graph nodes should not convert them to untyped dictionaries for intra-graph handoff.

LangGraph node updates for this BaseModel-backed state replace the returned fields. We keep that explicit replacement convention instead of adding `TypedDict` reducers for now because the current graph is linear and only one worker updates aggregate state at a time. Nodes that extend aggregate fields, such as `results`, must copy the prior value and return the merged replacement.

[ADR-009](ADR-009-langgraph-state-semantics.md) records this state-merge decision and the conditions that would force a future move to reducer-based state.

### Agent Capability Matching

Agent types declare capabilities as natural language. These are embedded (via pgvector) and stored. When dispatching, subtask descriptions are embedded and matched via cosine similarity against agent capabilities. Cached in Valkey to avoid repeated embedding queries.

## Rationale

### Alternatives Considered

| Alternative | Why not |
|---|---|
| LangGraph only | No durability. Process crash loses all state. No retry/ timeout semantics. |
| Temporal only (AI inside activities) | Temporal activities are not designed for long-running LLM chains with dynamic branching. LangGraph's graph-based reasoning is purpose-built for this. |
| Celery + LangGraph | Celery has no workflow durability (no replay, no long-running workflows). Temporal's replay-based model is superior for human-in-the-loop. |
| Prefect | No human-in-the-loop primitives. Temporal's signals/queries are purpose-built for this. |

## Consequences

- **Easier:** Clear separation of concerns. Each tool does what it's best at. Temporal provides audit history for free.
- **Harder:** Two technologies to learn and operate. The LangGraph ↔ Temporal boundary needs careful API design.
- **Next:** Define the gRPC contract between Temporal activities and the agent-runtime in `proto/harpia/agents/v1/`. Implement the supervisor graph in `agent-runtime/src/harpia_agents/graph.py`. Use **Temporalite** (single binary) for local development.
