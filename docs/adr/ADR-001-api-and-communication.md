# ADR-001: API and Service Communication

> **⚠ Historical record — not the current source of truth.** The canonical description of the
> platform is the **[Platform Constitution](../architecture/harpia-platform.md)**, which
> supersedes ADR-001…017 as the reading order. See the
> **[ADR supersession map](README.md)** for how this ADR stands today. Where this ADR and the
> constitution disagree, the constitution wins.

**Status:** Accepted
**Date:** 2026-06-03
**Deciders:** thbertoldi

## Context

Harpia is a multi-service platform with a Go control plane, Python agent runtime, and Svelte frontend. Services need to communicate with each other and the browser. We need:

1. **Service-to-service RPC** — control plane ↔ agent runtime ↔ Temporal workers
2. **Browser-to-server real-time** — the Svelte UI must show live agent progress
3. **Type safety** across Go, Python, and TypeScript
4. **Lean infrastructure** — no additional message broker beyond what we already need

We originally considered NATS JetStream for async messaging, but decided against it to keep the stack lean.

## Decision

**Use ConnectRPC (by Buf) for all communication.** One protocol, three modes:

| Mode | Use case |
|---|---|
| gRPC (HTTP/2) | Service-to-service: control plane ↔ agent runtime |
| Connect protocol (HTTP/1.1 + JSON) | Browser ↔ server: SvelteKit frontend to control plane |
| Server-streaming | Real-time agent progress to UI (replaces WebSocket) |

**Remove NATS JetStream from the stack.** Temporal handles durable messaging and workflow state. Connect handles synchronous RPC and real-time streaming.

**Use Buf for schema management.** A shared `proto/` directory at repo root, managed by `buf.yaml`. Code generated for Go, Python, and TypeScript from the same `.proto` files.

**Python adoption details:** see [ADR-013: ConnectRPC Python Adoption](ADR-013-connectrpc-python-adoption.md) for library pinning, ASGI server layout, transport defaults, and adapter isolation in `agent-runtime/`.

### Proto Organization

```
proto/
  buf.yaml              # Buf module configuration
  buf.gen.yaml          # Code generation configuration
  buf.lock              # Dependency lockfile
  harpia/
    tasks/v1/tasks.proto
    agents/v1/agents.proto
    identity/v1/identity.proto
```

## Rationale

ConnectRPC unifies patterns we'd otherwise need three tools for (gRPC, REST, WebSocket). It generates type-safe clients in all our languages, handles streaming natively in browsers, and keeps the infrastructure surface small.

### Alternatives Considered

| Alternative | Why not |
|---|---|
| gRPC + WebSocket | gRPC-Web requires a proxy (Envoy). WebSocket needs its own protocol design. Two different patterns to maintain. |
| REST + WebSocket | No type generation, manual serialization, API drift between services. |
| NATS JetStream | Overlaps with Temporal for durable messaging and Connect for real-time. Adds ops burden. |
| REST + SSE | SSE is viable but less rich than Connect server-streaming (only text, no multiplexing). |

## Consequences

- **Easier:** Single schema source of truth. Type-safe clients generated automatically. Real-time UI without WebSocket infrastructure.
- **Harder:** Need to manage proto breaking changes carefully. Buf tooling (buf breaking) addresses this.
- **Next:** Keep proto and generated clients in sync across Go, Python, and TypeScript per [ADR-013](ADR-013-connectrpc-python-adoption.md).
