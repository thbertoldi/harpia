---
name: harpia
description: Use when working on the Harpia multi-tenant AI agent platform. Covers Go + ConnectRPC, Python + LangGraph, Svelte 5 frontend, Temporal workflows, PostgreSQL + RLS + pgvector, Buf proto management, and mise task runner. Provides architecture decisions, dev workflow, and code generation steps.
---

# Harpia

Multi-tenant, multi-agent AI task management platform.

## Architecture

```
Frontend     Svelte 5 + SvelteKit + shadcn-svelte + Tailwind + bun
API/Control  Go 1.24+ + ConnectRPC
Agents       Python 3.12 + LangGraph + ConnectRPC
Workflows    Temporal (Go + Python SDKs)
Database     PostgreSQL 16 + RLS + pgvector
Cache        Valkey (BSD-3)
Object Store Garage (S3-compatible, AGPL-3.0)
AuthN        Zitadel (Apache 2.0)
AuthZ        OpenFGA (Apache 2.0)
Observability OTEL -> Tempo + Prometheus + Grafana
Infra        k3s/podman (dev), k8s/Helm (prod)
```

## Domain-Driven Design (ADR-006)

Based on Vlad Khononov's *Learning Domain-Driven Design* (2021) framework:

**Subdomain classification:**
- **Core:** Agent Orchestration, Task Workflow (our differentiation — build in-house)
- **Supporting:** Human Interaction, Agent Capability Registry, Tenant Management
- **Generic:** Identity (Zitadel), AuthZ (OpenFGA), Storage (Garage), Cache (Valkey), Observability (OTEL) — adopt, don't build

**Five bounded contexts (not three):**
1. **Task Management** (`harpia.tasks.v1`) — task lifecycle, subtask decomposition
2. **Agent Orchestration** (`harpia.agents.v1`) — agent types, capability matching, execution
3. **Human Interaction** (`harpia.feedback.v1`) — approval/rejection lifecycle, Slack bridge (CRITICAL: this is its own context, not embedded in tasks)
4. **Identity & Tenants** (`harpia.identity.v1`) — users, tenants, workspaces
5. **Workflow Engine** — Temporal (infrastructure, not a proto service)

**Context map (Khononov's integration patterns):**
- Task Management ↔ Agent Orchestration: **Partnership** (co-evolve)
- Identity → All: **Conformist** (Zitadel defines the model)
- Human Interaction → Slack: **Anticorruption Layer** (translate Slack events to domain)
- Workflow → Agent: **Open-Host Service** (Temporal provides standardized API)

**Business logic patterns per context:**
- Agent Orchestration → Domain Model (LangGraph IS the domain model)
- Task Management → Domain Model (rich state machine)
- Human Interaction → Event-Sourced Domain Model (every human decision is an event)
- Identity → Transaction Script (simple CRUD)

## Key Architecture Decisions

All decisions are documented in `docs/adr/`:
- `ADR-001`: ConnectRPC for all service communication (no NATS, no REST, no WebSocket)
- `ADR-002`: Temporal + LangGraph — Temporal for durable execution, LangGraph for AI reasoning
- `ADR-003`: PostgreSQL + RLS + pgvector for multi-tenant data, Valkey for cache, Garage for files. No graph database.
- `ADR-004`: Zitadel for authN, OpenFGA for authZ (ReBAC). Replaced Authentik.
- `ADR-005`: mise + Tilt + Temporalite + Buf for dev. Trunk-based development. GitHub CLI for issues/PRs.
- `ADR-006`: DDD applied to agentic architecture. Khononov's subdomains, bounded contexts, context map.

## Development Commands

```bash
mise run dev              # Start everything via Tilt
mise run dev-api          # Go control plane only
mise run dev-agent        # Python agent runtime only
mise run dev-web          # Svelte frontend only

mise run test             # Run all tests
mise run lint             # Lint all code
mise run fmt              # Format all code
mise run build            # Build all services

mise run buf-generate     # Generate proto code for all languages
mise run buf-lint         # Lint proto schemas
mise run buf-breaking     # Check for breaking proto changes
```

## Proto Workflow

When `.proto` files change:
1. Run `mise run buf-generate` to regenerate Go, Python stubs
2. Run `mise run buf-lint` to verify schemas
3. Run `mise run buf-breaking` to check compatibility

Generated code destinations:
- Go: `control-plane/gen/` (gitignored)
- Python: `agent-runtime/src/harpia_agents/gen/` (gitignored)

Proto files: `proto/harpia/{tasks,agents,identity}/v1/`

## Service Boundaries (DDD)

| Bounded Context | Proto Package | Implementation |
|---|---|---|
| Task Management | `harpia.tasks.v1` | `control-plane/internal/tasks/` |
| Agent Orchestration | `harpia.agents.v1` | `agent-runtime/src/harpia_agents/` |
| Human Interaction | `harpia.feedback.v1` | `control-plane/internal/feedback/` |
| Identity & Tenants | `harpia.identity.v1` | `control-plane/internal/identity/` |
| Workflow Engine | Temporal SDK | Temporal activities in both Go and Python |

## Database

PostgreSQL 16 with Row-Level Security per tenant.
- Schema: `database/migrations/000001_initial_schema.sql`
- Migrations run via Atlas: `mise run db-migrate`
- Key tables: `tenants`, `users`, `agent_types`, `tasks`, `subtasks`, `agent_instances`, `human_feedback`

## LangGraph ↔ Temporal Boundary

- **Temporal**: durability, retries, timeouts, human-in-the-loop signals, audit history
- **LangGraph**: AI reasoning, task decomposition, tool selection, LLM interactions
- Contract: Temporal calls agent-runtime via gRPC (`ExecuteTask`). LangGraph handles the graph. Temporal signals for human feedback.

## Repository Conventions

- Branch naming: `{type}/{issue}-{slug}` — e.g. `feat/42-agent-capability-matching`
- Commits: Conventional Commits — `feat(api): add task creation endpoint`
- PRs: Squash-merge to `trunk`. See `docs/WORKFLOW.md` for full guide.
- Issues: GitHub Issues with labels `feature`, `bug`, `chore`, `adr`, `docs`

## When Making Changes

1. Read relevant ADRs first (`docs/adr/`)
2. Follow the proto-defined contracts — never hand-code serialization
3. Keep tenant isolation at the DB level (RLS), not application level
4. New architecture changes require a new ADR
5. Run `mise run test` and `mise run lint` before completing
