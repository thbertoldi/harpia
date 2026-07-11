# Harpia

Multi-tenant, multi-agent AI task management platform.

## Architecture

```
Frontend     Svelte 5 + SvelteKit + shadcn-svelte + Tailwind + bun
API/Control  Go 1.24+ + ConnectRPC
Agents       Python 3.12 + LangGraph + ConnectRPC
Workflows    Temporal (Go + Python SDKs)
Database     PostgreSQL 16 + RLS + pgvector
Cache        Valkey
Object Store Garage (S3-compatible)
AuthN        Zitadel
AuthZ        Postgres RLS + role checks
Observability OTEL -> Tempo + Prometheus + Grafana
Infra        kind cluster (dev), k8s/Helm (prod)
```

## Domain-Driven Design

Harpia follows DDD principles from Vlad Khononov's framework:

**Subdomains:** Core (Agent Orchestration, Task Workflow) — Supporting (Human Interaction, Agent Registry, Tenant Mgmt) — Generic (Identity, AuthZ, Storage, Cache, Observability)

**Bounded contexts:** Task Management — Agent Orchestration — Human Interaction — Identity & Tenants — Workflow Engine

**Context map:** Partnership between core contexts; Conformist to Identity and AuthZ; Anticorruption Layer for Slack.

See [ADR-006](docs/adr/ADR-006-domain-driven-design.md).

## Architecture Decision Records

| ADR | Topic |
|---|---|
| [ADR-001](docs/adr/ADR-001-api-and-communication.md) | ConnectRPC for all communication |
| [ADR-002](docs/adr/ADR-002-workflow-and-agents.md) | Temporal + LangGraph boundary |
| [ADR-003](docs/adr/ADR-003-data-architecture.md) | PostgreSQL, Valkey, Garage |
| [ADR-004](docs/adr/ADR-004-identity-and-access.md) | Zitadel + OpenFGA |
| [ADR-005](docs/adr/ADR-005-development-and-delivery.md) | mise, Tilt, Buf, trunk-based dev |
| [ADR-006](docs/adr/ADR-006-domain-driven-design.md) | DDD for agentic architecture |
| [ADR-007](docs/adr/ADR-007-agentic-architecture-patterns.md) | Agentic patterns (Arsanjani & Bustos) |
| [ADR-008](docs/adr/ADR-008-tenant-safe-generic-infra-boundaries.md) | Tenant-safe generic infrastructure boundaries |
| [ADR-009](docs/adr/ADR-009-langgraph-state-semantics.md) | LangGraph state semantics |
| [ADR-010](docs/adr/ADR-010-budget-policy-service.md) | Budget Policy Service (Agent Orchestration capability) |
| [ADR-011](docs/adr/ADR-011-mcp-capability-gating.md) | MCP capability gating + out-of-process workers |
| [ADR-012](docs/adr/ADR-012-plan-centric-task-model.md) | Plan-centric task model (templates, steps, executors) |
| [ADR-013](docs/adr/ADR-013-connectrpc-python-adoption.md) | ConnectRPC Python adoption (`agent-runtime/`) |
| [ADR-014](docs/adr/ADR-014-agent-memory-boundary.md) | Agent memory boundary |
| [ADR-015](docs/adr/ADR-015-plan-template-authoring.md) | PlanTemplate authoring model (declarative seed → catalog service) |

Workflow guide: [docs/WORKFLOW.md](docs/WORKFLOW.md)

## Prerequisites

- [mise](https://mise.jdx.dev/) — runtime version manager
- [podman](https://podman.io/) — container runtime
- [kind](https://kind.sigs.k8s.io/) — local Kubernetes cluster
- [buf](https://buf.build/) — protobuf schema management

## Quick Start

```bash
# Install required tools
mise install

# Generate protobuf code
mise run buf-generate

# Start development environment
mise run dev
```

### Local dev login

The persona-based dev login is disabled by default. To expose it for the local
Tilt/Vite dev workflow only, set the public Vite flag when starting dev:

```bash
PUBLIC_DEV_LOGIN_ENABLED=true mise run dev
```

Do not set this flag for production artifacts. Dev login is only honored by the
Vite dev server, and the frontend build fails when
`PUBLIC_DEV_LOGIN_ENABLED=true` is present.

## Project Structure

```
docs/
  adr/              Architecture Decision Records
  WORKFLOW.md       Development workflow
proto/              Protobuf schemas (Buf-managed)
control-plane/      Go API server + k8s operator
agent-runtime/      Python agent runtime + LangGraph agents
frontend/           SvelteKit web application
deploy/             Helm charts + k8s manifests
database/           PostgreSQL migrations
```
