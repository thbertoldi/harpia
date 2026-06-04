# ADR-006: Domain-Driven Design for Agentic Architecture

**Status:** Accepted
**Date:** 2026-06-03
**Deciders:** thbertoldi

**References:**
- Khononov, V. (2021) *Learning Domain-Driven Design*. O'Reilly Media.
- Khononov, V. (2023) *Balancing Coupling in Software Design*. Addison-Wesley.
- Khononov, V. (2019) *What Is Domain-Driven Design?* O'Reilly Media.
- Albada, M. (2025) *Building Applications with AI Agents*. O'Reilly Media.

## Context

Harpia is an agentic system: AI agents reason, decompose tasks, dispatch to sub-agents, and coordinate with humans. This is a fundamentally different domain model than traditional CRUD systems. We need to apply DDD systematically to avoid the most common failure mode of agent platforms: **tangled orchestration logic spread across LLM prompts, Temporal activities, and API handlers with no clear domain boundaries.**

Khononov's DDD framework provides the tools: subdomain classification, bounded contexts, context mapping, business logic patterns, and architectural patterns. Albada's work provides the agent-specific taxonomy: agent types, multi-agent coordination patterns, and human-agent collaboration models.

## Decision

### 1. Subdomain Classification

We classify each area of the system by its competitive importance and complexity, per Khononov's framework:

| Subdomain | Type | Complexity | Differentiation | Build vs Adopt |
|---|---|---|---|---|
| **Agent Orchestration** (task decomposition, dispatch, multi-agent coordination) | **Core** | High | Yes — this IS Harpia | Build |
| **Task Workflow** (lifecycle, state machine, human-in-the-loop) | **Core** | High | Yes — how we manage tasks | Build |
| **Agent Capability Registry** (what agents can do, semantic matching) | **Supporting** | Medium | No — but must integrate tightly | Build |
| **Human Interaction** (feedback, approval, Slack bridge) | **Supporting** | Medium | Partial — Slack UX matters | Build |
| **Tenant Management** | **Supporting** | Low | No | Build |
| **Identity & Authentication** | **Generic** | Low | No | Adopt (Zitadel) |
| **Authorization (ReBAC)** | **Generic** | Low | No | Adopt (OpenFGA) |
| **Object Storage** | **Generic** | Low | No | Adopt (Garage) |
| **Caching** | **Generic** | Low | No | Adopt (Valkey) |
| **Observability** | **Generic** | Low | No | Adopt (OTEL stack) |

**Key insight from Khononov**: Generic subdomains should be adopted, not built. Core subdomains must be built in-house and receive the most design investment. This means our design energy goes into Agent Orchestration and Task Workflow, not into auth or storage.

### 2. Bounded Contexts

Per Khononov: subdomains are the problem space, bounded contexts are the solution space. Each bounded context has its own ubiquitous language, model, and implementation.

We define **five bounded contexts**:

| Bounded Context | Ubiquitous Language | Proto Package | Implementation |
|---|---|---|---|
| **Task Management** | task, subtask, status, assign, dispatch, complete, cancel | `harpia.tasks.v1` | `control-plane/internal/tasks/` |
| **Agent Orchestration** | agent type, capability, execute, stream, supervisor, planner, worker, tools | `harpia.agents.v1` | `agent-runtime/src/harpia_agents/` |
| **Human Interaction** | feedback, approve, reject, modify, escalate, signal, notify, channel | `harpia.feedback.v1` | `control-plane/internal/feedback/` |
| **Identity & Tenants** | user, tenant, workspace, role, membership | `harpia.identity.v1` | `control-plane/internal/identity/` |
| **Workflow Engine** | workflow, activity, signal, query, timer, retry, saga | Temporal SDK (not proto) | Temporal workers |

**Critical addition: Human Interaction as its own bounded context.** In the original schema, feedback was embedded in the tasks table. DDD tells us this is wrong. Human-in-the-loop has its own lifecycle (request → wait → receive → apply), its own ubiquitous language (approve, reject, modify with comment, escalate), and its own integration patterns (Slack, UI, email). Treating it as a sub-case of "task status" creates a model where all feedback logic leaks into every other context.

### 3. Context Map (Integration Patterns)

How the bounded contexts interact, using Khononov's patterns:

```
┌──────────────┐    Partnership    ┌──────────────────┐
│    Task      │◄────────────────►│     Agent        │
│ Management   │                   │  Orchestration   │
└──────┬───────┘                   └────────┬─────────┘
       │                                    │
       │ Conformist                         │
       ▼                                    ▼
┌──────────────┐                   ┌──────────────────┐
│  Identity &  │                   │   Workflow       │
│   Tenants    │                   │   Engine         │
└──────┬───────┘                   │  (Temporal)      │
       │                           └────────┬─────────┘
       │                                    │
       │ Conformist                  Open-Host Service
       ▼                                    │
┌──────────────┐                   ┌────────▼─────────┐
│    AuthZ     │                   │     Human        │
│  (OpenFGA)   │                   │  Interaction     │
└──────────────┘                   └──────────────────┘
                                           │
                                Anticorruption Layer
                                           │
                                  ┌────────▼─────────┐
                                  │    Slack Bot     │
                                  │   (External)     │
                                  └──────────────────┘
```

**Relationship explanations:**

- **Task Management ↔ Agent Orchestration: Partnership.** These two are the core domain. They evolve together. Changes in one will likely affect the other. They share the concept of "task status" and "execution state."

- **Identity → All others: Conformist.** All bounded contexts must conform to Zitadel's identity model. We don't control the upstream, so we conform.

- **Workflow Engine → Agent Orchestration: Open-Host Service.** Temporal exposes a well-defined API (activities, signals, queries). The agent runtime consumes this API. Temporal defines the protocol; we conform to it.

- **Human Interaction → Slack: Anticorruption Layer.** Slack's event model (interactive messages, slash commands, events API) must not leak into our domain. We translate between Slack concepts and our domain concepts (e.g., "button click on interactive message" → "FeedbackDecision.APPROVE").

- **Workflow Engine ↔ Human Interaction: Partnership.** Human interaction drives workflow state changes. The workflow blocks on human input and resumes when it arrives. These contexts are tightly coupled by design.

### 4. Business Logic Patterns (per Bounded Context)

Khononov provides a decision tree (Ch. 10) for choosing the right pattern:

| Bounded Context | Pattern | Why |
|---|---|---|
| **Agent Orchestration** | **Domain Model** | Complex behavior with many rules: agent selection, capability matching, execution state machine, tool routing. The LangGraph supervisor graph IS the domain model. |
| **Task Management** | **Domain Model** | Rich state machine with invariants: a task can't be "in progress" if no agent is assigned. Subtask completion gates parent task status. |
| **Human Interaction** | **Event-Sourced Domain Model** | Every human decision is an event worth keeping. Audit trail of approvals/rejections. Temporal events give us this for free via workflow history. |
| **Identity & Tenants** | **Transaction Script** | Simple CRUD operations. No complex invariant checks beyond what Zitadel already handles. |
| **Workflow Engine** | **Ports & Adapters** (architectural, not pattern) | Temporal is infrastructure. LangGraph is the domain. The boundary between them uses the Ports & Adapters pattern: Temporal activities are the adapter that calls into the agent domain model. |

### 5. Mapping Agent Types to DDD

Albada's agent types map to Khononov's implementation patterns:

| Agent Type (Albada) | DDD Role | Implementation |
|---|---|---|
| **Planner-Executor** | Domain Service | LangGraph supervisor with `planner` and `worker` nodes |
| **ReAct** | Application Service | Individual tool-using agents that reason and act in a loop |
| **Reflection** | Domain Event Handler | Post-execution critique that feeds back into planning |
| **Deep Research** | Subdomain-specific agent | A worker with extended tool access (search, browse, summarize) |

### 6. Multi-Agent Coordination = Context Mapping

Albada's coordination patterns are structurally identical to Khononov's context mapping:

| Albada Pattern | Khononov Equivalent | When to Use |
|---|---|---|
| Manager | **Customer-Supplier** | Orchestrator defines the plan; worker agents conform to it. |
| Hierarchical | **Partnership + Conformist** | Parent agents partner with children; leaf agents conform. |
| Democratic | **Partnership** | Peer agents jointly decide. Rare in our domain. |
| Actor-Critic | **Open-Host Service** | Evaluator exposes a standardized feedback protocol. |

For Harpia, we primarily use **Manager** (supervisor dispatches to workers) with **Customer-Supplier** dynamics (supervisor is upstream, workers conform).

### 7. EventStorming the Core Domain

Using Khononov's 10-step EventStorming process, the core domain events are:

```text
TaskCreated → TaskReceived → PlanningStarted → TaskDecomposed →
SubtaskCreated → AgentMatched → ExecutionStarted →
[ExecutionCompleted | ExecutionFailed] →
[FeedbackRequested → FeedbackReceived →
  (ExecutionResumed | ExecutionRejected)] →
TaskCompleted | TaskFailed
```

These domain events should drive our API contracts, not CRUD mental models.

## Consequences

### What Becomes Easier
- **Clean service boundaries** — each bounded context has a clear proto contract and implementation directory.
- **Human feedback is first-class** — its own context with its own lifecycle, not a status flag on a task.
- **Agent behavior is testable** — LangGraph graphs can be unit-tested as domain models without Temporal or Slack.
- **Evolution** — per Khononov's Ch. 11, when the domain evolves (e.g., new agent types added), we know exactly which bounded context changes and how it affects its integration partners.

### What Becomes Harder
- **Five contexts instead of three** — adds a new proto package and service. The `feedback.v1` proto must be defined and generated alongside the existing three.
- **Event-sourced human interaction** — requires Temporal workflow history to serve as the event store, which means workflow history queries are part of our domain API.
- **Ubiquitous language discipline** — every proto message name, every Go struct, every Python class must use the agreed-upon terms. No "just call it status" — is it `TaskStatus`, `ExecutionState`, or `FeedbackDecision`?

### Actions Required
1. Create `proto/harpia/feedback/v1/feedback.proto` with FeedbackService
2. Move `SubmitFeedback` RPC from `tasks.proto` to `feedback.proto`
3. Define the Anticorruption Layer between Human Interaction and Slack as a separate Go package
4. Refactor `graph.py` to explicitly model the LangGraph nodes as a Domain Model (with aggregates, value objects, and domain services)
5. Create database migration adding a `feedback_events` table (for the event-sourced human interaction context)
6. Write ADR-007: Ports & Adapters for Temporal ↔ LangGraph boundary
