# ADR-005: Development Environment and Delivery

**Status:** Accepted
**Date:** 2026-06-03
**Deciders:** thbertoldi

## Context

We need a development environment that:

1. **Starts fast** — developers should not wait minutes for `docker compose` to spin up.
2. **Hot reloads** — changes to Go, Python, or Svelte code appear immediately.
3. **Runs the full stack locally** — control plane, agent runtime, frontend, database, Temporal, Zitadel, OpenFGA, Valkey, Garage.
4. **Leverages Kubernetes where it helps, skips it where it doesn't.**
5. **Uses open-source tools only.**

## Decision

### Runtime Manager: mise

**mise** manages language runtime versions (Go, Python, Node, Bun, uv) and provides task automation via `[tasks]` in `mise.toml`. This replaces the need for a separate Taskfile.

```toml
[tools]
go = "1.24"
python = "3.12"
node = "24"
bun = "latest"
uv = "latest"

[tasks.dev]
description = "Start development environment"
run = "tilt up"
```

### Live Reload: Tilt

**Tilt** orchestrates the dev environment: builds containers, applies Kubernetes manifests, proxies ports, and hot-reloads code on change. Defined in `Tiltfile`.

### Temporal in Dev: Temporalite

**Temporalite** is a single-binary Temporal server. No need for Cassandra/MySQL + Temporal Server + Temporal UI in development. One binary, starts in seconds.

### Schema Management: Buf

Protobuf schemas live in `proto/` at the repo root. **Buf** handles:
- Linting and breaking change detection (`buf lint`, `buf breaking`)
- Code generation for Go, Python, and TypeScript (`buf generate`)
- Dependency management via BSR or local resolution (`buf.yaml`)

### Container Registry: GitHub Packages

All container images are published to `ghcr.io/harpia/`. Built via GitHub Actions, triggered by PR merge to `trunk`.

### Repository Workflow

| Aspect | Tool | Detail |
|---|---|---|
| Task tracking | GitHub Issues | Features, bugs, ADR-driven tasks |
| Branch naming | `{type}/{issue}-{slug}` | e.g., `feat/42-agent-capability-matching` |
| Commits | Conventional Commits | `feat(agents): add capability matching via pgvector` |
| PR management | GitHub CLI (`gh`) | `gh pr create`, `gh issue list`, `gh run watch` |
| CI/CD | GitHub Actions | Build, test, lint, publish containers |

### Branch Strategy

- **`trunk`** — main branch, always deployable.
- Feature branches from trunk, merged via squash-merge.
- No `develop`, no `release` branches. Trunk-based development.

## Rationale

### Alternatives Considered

| Alternative | Why not |
|---|---|
| Taskfile (go-task) | mise already has built-in task support. One less file. |
| Docker Compose only | No hot reload. Tilt provides `live_update` for each service. |
| Full Temporal server in dev | Requires MySQL/Cassandra + Temporal Server + UI. Temporalite is one binary. |
| Nix | Excellent but steep learning curve. mise is simpler and already adopted. |
| protoc + scripts | Error-prone, platform-dependent. Buf provides linting, breaking changes, and managed mode. |
| GitFlow | Overkill for a team this size. Trunk-based development is simpler and faster. |

## Consequences

- **Easier:** Single command to start everything (`mise run dev`). Fast feedback loop via Tilt live reload. Consistent proto generation across all services.
- **Harder:** Tilt/Temporalite need to be installed. Buf requires learning its configuration model.
- **Next:** Set up the Buf configuration (`buf.yaml`, `buf.gen.yaml`). Configure Tilt to use Temporalite instead of full Temporal. Create GitHub Actions workflows.
