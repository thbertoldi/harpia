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
AuthZ        OpenFGA
Observability OTEL -> Tempo + Prometheus + Grafana
Infra        k3s/podman (dev), k8s/Helm (prod)
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

Workflow guide: [docs/WORKFLOW.md](docs/WORKFLOW.md)

## Prerequisites

- [mise](https://mise.jdx.dev/) — runtime version manager
- [podman](https://podman.io/) — container runtime
- [k3s](https://k3s.io/) — lightweight Kubernetes
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
