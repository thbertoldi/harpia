# ADR-013: ConnectRPC Python Adoption

> **⚠ Historical record — not the current source of truth.** The canonical description of the
> platform is the **[Platform Constitution](../architecture/harpia-platform.md)**, which
> supersedes ADR-001…017 as the reading order. See the
> **[ADR supersession map](README.md)** for how this ADR stands today. Where this ADR and the
> constitution disagree, the constitution wins.

**Status:** Accepted
**Date:** 2026-06-13
**Deciders:** thbertoldi

**References:**

- [ADR-001: API and Service Communication](ADR-001-api-and-communication.md) — platform-wide ConnectRPC decision
- [ADR-002: Workflow and Agent Orchestration](ADR-002-workflow-and-agents.md) — Temporal activities call agent-runtime over RPC
- [ADR-005: Development Environment and Delivery](ADR-005-development-and-delivery.md) — Buf codegen via `mise run buf-generate`
- Issue #69 — documents this ADR (renumbered from a stale ADR-009 claim)

## Context

[ADR-001](ADR-001-api-and-communication.md) commits Harpia to ConnectRPC for all service
communication. Go (`connectrpc.com/connect`) and TypeScript (`@connectrpc/connect`) were
adopted at project setup. Python lagged behind because early scaffolding assumed the
`connectrpc` package was unavailable on PyPI.

That assumption is **stale**. The official Python implementation
([connectrpc/connect-python](https://github.com/connectrpc/connect-python)) ships on PyPI as
`connectrpc` (currently `0.10.x`, pre-1.0). The repo already generates Python Connect stubs
via Buf and runs a Connect ASGI server in `agent-runtime/`, but the dependency was never
declared in `pyproject.toml`, leaving contributors without a documented pinning or isolation
strategy.

Without this ADR, Python work tends toward `httpx` + hand-rolled JSON or deferred gRPC
clients — both drift from the single-protocol model in ADR-001.

## Decision

### 1. Adopt `connectrpc` as the Python ConnectRPC library

- Pin `connectrpc` in `agent-runtime/pyproject.toml` (exact minor until v1.0).
- Treat generated code under `agent-runtime/src/harpia_agents/gen/` as the wire contract;
  regenerate with `mise run buf-generate` after proto changes.
- Do **not** add parallel HTTP/JSON shims for RPC paths that already exist in `proto/`.

### 2. Code generation (resolved)

| Language   | Buf plugin                         | Output |
| ---------- | ---------------------------------- | ------ |
| Go         | `buf.build/connectrpc/go`          | `control-plane/gen/` |
| Python     | `buf.build/connectrpc/python`      | `agent-runtime/src/harpia_agents/gen/` |
| TypeScript | `buf.build/bufbuild/es` + Connect  | `frontend/src/lib/gen/` |

Configuration lives in `proto/buf.gen.yaml`. Python generation produces `*_pb2.py` message
types and `*_connect.py` service protocols, clients, and ASGI/WSGI application wrappers.

**Rule:** Never hand-edit `gen/`; fix schemas or plugins instead.

### 3. Service-to-service transport (resolved)

**Default: Connect protocol with protobuf codec over HTTP/1.1** between control plane and
agent-runtime.

| Hop | Transport | Rationale |
| --- | --------- | --------- |
| Browser → control plane | Connect protocol, JSON codec | ADR-001; works in browsers without gRPC-Web proxy |
| Control plane → agent-runtime | Connect protocol, protobuf codec | Matches generated Go/Python clients; no Envoy sidecar |
| Optional future | gRPC over HTTP/2 (h2c) | Available in connect-go/connect-python when cluster ingress supports h2c and we need multiplexing; not required for MVP |

Go callers use the generated `*connect` clients from `control-plane/gen/`. Python exposes
`*ASGIApplication` wrappers from generated `*_connect.py` modules. Both sides speak the same
Connect paths (e.g. `/harpia.agents.v1.AgentService/ExecuteTask`).

**Rejected for Harpia:** raw gRPC-only Python servers (`grpcio` without Connect), REST
fallbacks, and `httpx` POST shims to agent-runtime.

### 4. Python server framework (resolved)

**ASGI via uvicorn.** Generated servers expose `ConnectASGIApplication`; the runtime entry
point composes middleware and runs uvicorn (`agent-runtime/src/harpia_agents/main.py`).

WSGI (`ConnectWSGIApplication`) is not used — server-streaming handlers are async iterators
and the LangGraph execution path is async-native.

### 5. Streaming (resolved)

Server-streaming RPCs (`ExecuteTask`, `ListAgentTypes`, etc.) are implemented as
`AsyncIterator` yields on the service class. This matches Connect server-stream semantics and
mirrors Go handler patterns using `connect.ServerStream`.

**Test requirement:** contract tests must cover at least one server-streaming RPC end-to-end
(client stream read until completion) before agent-runtime RPC paths are considered production-ready.

### 6. Layer isolation (adapter boundary)

`connectrpc` imports stay in transport adapters, not domain logic:

| Layer | Paths | May import `connectrpc`? |
| ----- | ----- | ------------------------ |
| Generated wire | `agent-runtime/src/harpia_agents/gen/` | Yes (generated) |
| RPC adapters | `main.py`, `services.py` | Yes — implement generated `*Service` protocols |
| Auth transport | `identity.py` (`TenantResolverMiddleware`) | No — pure ASGI; reads `RequestContext` from Connect scope |
| Domain | `graph.py`, `temporal_worker.py`, LangGraph nodes | **No** |
| Tests | `tests/` | Yes — clients/fixtures only |

Domain code depends on protobuf message types from `gen/` when needed for payloads, but not
on `connectrpc.client` or `connectrpc.server` directly.

### 7. Versioning and v1.0 migration

`connectrpc` is pre-1.0. Mitigations:

- Pin exact version in `pyproject.toml`; bump only via PR with release notes review.
- Regenerate Python stubs after plugin bumps (`buf generate`).
- **Migration spike (trigger):** when connect-python releases v1.0, run a focused spike:
  regenerate stubs, run agent-runtime tests, verify server-streaming contract test, update pin.

## Rationale

Adopting the official Buf-generated Python stack keeps all three languages on one proto
source, one error model (`ConnectError` / `Code`), and one streaming abstraction. ASGI +
Connect protocol avoids operating a separate gRPC server process while still allowing a
future move to gRPC-over-HTTP/2 through the same generated stubs.

### Alternatives Considered

| Alternative | Why not |
| ----------- | ------- |
| Defer Python RPC; use Temporal payloads only | Breaks synchronous control-plane → agent-runtime calls in ADR-002 flow |
| `grpcio` + `grpcio-tools` | Different codegen pipeline; no Connect browser parity; duplicates Buf investment |
| `httpx` + manual JSON | No type generation, drifts from proto, reimplements framing |
| Wait for connect-python v1.0 | Blocks agent-runtime integration; 0.10.x is usable with pinning |

## Consequences

- **Easier:** Typed Python clients and servers aligned with Go/TS; single `buf generate` workflow; clear adapter boundary for reviews.
- **Harder:** Pre-1.0 upstream API may change; server-streaming contract tests add CI surface.
- **Next:** Pin `connectrpc` in `pyproject.toml`; wire Go control-plane client to agent-runtime; add server-streaming contract test for `ExecuteTask`.

## Cross-reference (ADR-001)

ADR-001 establishes ConnectRPC for all communication. This ADR is the **Python implementation
appendix**: library choice, transport defaults, server framework, codegen layout, and
isolation rules for `agent-runtime/`. ADR-001's three-mode table remains authoritative;
Python service-to-service hops use Connect protocol with protobuf unless and until we
document an h2c gRPC migration in a future ADR.
