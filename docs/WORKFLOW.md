# Workflow

How we build Harpia.

## Principles

- **Open-source first, self-hosted whenever possible.** No vendor lock-in.
- **Lean stack.** Every dependency must earn its place. One tool per job.
- **Fast feedback.** The dev loop (`mise run dev`) should feel instant.
- **DDD-informed.** Organize code around the domain, not the framework.
- **ADR-driven.** Record architecture decisions. See `docs/adr/`.

## Repository

```
harpia/
├── docs/
│   ├── adr/              # Architecture Decision Records
│   └── WORKFLOW.md       # This file
├── proto/                # Protobuf schemas (Buf-managed)
│   ├── buf.yaml
│   ├── buf.gen.yaml
│   └── harpia/
│       ├── tasks/v1/
│       ├── agents/v1/
│       └── identity/v1/
├── control-plane/        # Go API server + k8s operator (ConnectRPC)
├── agent-runtime/        # Python agent runtime (LangGraph + ConnectRPC)
├── frontend/             # SvelteKit web application
├── database/             # PostgreSQL migrations (Atlas)
├── deploy/               # Helm charts + k8s manifests + compose
├── mise.toml             # Runtime versions + tasks
├── Tiltfile              # Dev environment orchestration
└── README.md
```

## Task Tracking

All work is tracked through **GitHub Issues** managed via the GitHub CLI (`gh`).

### Issue Types

| Label | Purpose | Example |
|---|---|---|
| `feature` | New capability | "Add agent capability matching via pgvector" |
| `bug` | Defect | "Task status not updating on reconnect" |
| `chore` | Maintenance | "Upgrade Python to 3.13" |
| `adr` | Architecture decision | "ADR-006: Observability stack" |
| `docs` | Documentation | "Document Temporal activity patterns" |

### Creating Issues

```bash
gh issue create \
  --title "feat(agents): add capability matching via pgvector" \
  --body-file docs/adr/ADR-002-workflow-and-agents.md \
  --label "feature" \
  --assignee @me
```

### Linking Issues

- Reference ADRs: `See ADR-002 in docs/adr/`
- Reference commits: `Implemented in abc1234`
- Reference PRs: `Closes #42`

## Branching

**Trunk-based development.** One main branch: `trunk`.

```
trunk
  └── feat/42-agent-capability-matching
  └── fix/43-task-status-reconnect
  └── chore/44-upgrade-python
```

Branch naming: `{type}/{issue-number}-{short-slug}`

Types: `feat`, `fix`, `chore`, `docs`, `refactor`

## Commits

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(agents): add pgvector-based capability matching
fix(tasks): handle task status on reconnect
chore(deps): upgrade Python runtime to 3.13
docs(adr): document Temporal-LangGraph boundary
refactor(api): extract task repository from handler
```

## Pull Requests

1. Branch from `trunk`
2. Implement + test (see testing section)
3. Open PR via CLI:
   ```bash
   gh pr create \
     --title "feat(agents): add capability matching via pgvector" \
     --body "Closes #42" \
     --base trunk
   ```
4. Squash-merge to `trunk` after review
5. Delete the feature branch

### PR Requirements

- [ ] Tests pass (`mise run test`)
- [ ] Lint clean (`mise run lint`)
- [ ] No breaking proto changes (verified by `buf breaking`)
- [ ] ADR if this changes architecture

## Development Loop

```bash
# Start everything
mise run dev                # Tilt: builds, deploys, hot-reloads

# Or run individual services
mise run dev-api            # Go control plane
mise run dev-agent          # Python agent runtime
mise run dev-web            # Svelte frontend

# Before committing
mise run lint               # Lint all services
mise run test               # Test all services
mise run fmt                # Format all services

# Build for production
mise run build
```

### Hot Reload

Tilt watches for changes and rebuilds only what's needed:

| Service | Watch path | Action |
|---|---|---|
| control-plane | `cmd/`, `internal/` | Rebuild Go binary, restart |
| agent-runtime | `src/` | Sync files, restart uvicorn |
| frontend | `src/` | HMR via Vite (instant) |
| proto | `proto/` | Must run `buf generate` manually |

## Proto Workflow

When you change a `.proto` file:

```bash
# Generate code for all languages
buf generate proto/

# Verify no breaking changes against trunk
buf breaking proto/ --against '.git#branch=trunk'

# Lint your protos
buf lint proto/
```

Generated code destinations:
- Go: `control-plane/gen/`
- Python: `agent-runtime/src/harpia_agents/gen/`

## Testing

### Philosophy

We do **not** practice strict TDD. We model domains first (DDD), implement, and then write tests that verify behavior. Tests should validate domain invariants and edge cases, not line coverage.

### Commands

```bash
mise run test
```

### Test Structure

| Service | Framework | Location |
|---|---|---|
| control-plane | `go test` | `control-plane/internal/**/` |
| agent-runtime | pytest + pytest-asyncio | `agent-runtime/tests/` |
| frontend | vitest | `frontend/src/**/*.test.ts` |

## CI/CD (Planned)

GitHub Actions will handle:

1. **On PR:** lint, test, `buf breaking`, build containers
2. **On merge to trunk:** build and push containers to `ghcr.io/harpia/`
3. **On release tag:** deploy to staging/production via Helm

Container images: `ghcr.io/harpia/{control-plane,agent-runtime,frontend}:{tag}`

## Communication

- **Architecture discussions:** Open an issue with label `adr`, draft an ADR, discuss in PR.
- **Bugs:** Open with minimal reproduction steps.
- **Features:** Start with an issue describing the problem. Implementations follow.

## Tools Reference

| Tool | Purpose | Command |
|---|---|---|
| `mise` | Runtime versions + task runner | `mise run {task}` |
| `tilt` | Dev environment | `tilt up` |
| `buf` | Proto schema management | `buf generate/lint/breaking` |
| `gh` | GitHub CLI (issues, PRs) | `gh issue/pr {...}` |
| `uv` | Python package manager | `uv sync/run` |
| `bun` | JS runtime + package manager | `bun run/dev/test` |
| `atlas` | Database migrations | `atlas migrate apply` |
