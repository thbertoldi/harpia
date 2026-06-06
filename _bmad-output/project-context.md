---
project_name: 'harpia'
user_name: 'Thiago'
date: '2026-06-06'
sections_completed: ['technology_stack', 'language_specific', 'framework_specific', 'testing', 'code_quality', 'development_workflow', 'critical_dont_miss']
status: 'complete'
optimized_for_llm: true
---

# Project Context for AI Agents

_This file contains critical rules and patterns that AI agents must follow when implementing code in this project. Focus on unobvious details that agents might otherwise miss._

---

## Technology Stack & Versions

### Languages & Runtimes (`mise.toml`)

| Runtime | Version |
|---|---|
| Go | 1.25 (control-plane `go.mod` declares `1.25.0`) |
| Python | 3.12 (`requires-python = ">=3.12"`; mypy `python_version = "3.12"`; ruff `target-version = "py312"`) |
| Node | 24 |
| Bun | latest (frontend uses `bun` as runtime + package manager) |
| uv | latest (Python deps) |
| Buf | latest (proto schema mgmt) |
| Tilt | latest (dev orchestration) |
| golangci-lint | latest |
| Atlas | latest (DB migrations) |
| Temporal CLI | latest (`temporal server start-dev`) |

### Control Plane — Go (`control-plane/`)

- `connectrpc.com/connect v1.20.0` — **all RPC is ConnectRPC**, not gRPC stdlib
- `github.com/jackc/pgx/v5 v5.10.0` — Postgres driver (not `database/sql`)
- `github.com/pgvector/pgvector-go v0.4.0` — vector embeddings
- `github.com/redis/go-redis/v9 v9.20.0` — used against Valkey (Redis wire-compatible)
- `go.temporal.io/sdk v1.44.1` — Temporal worker SDK
- `google.golang.org/protobuf v1.36.11`
- Layout: `cmd/api`, `internal/{agents,cache,config,database,feedback,identity,server,tasks,tenants,workflow}`, `gen/harpia/...` (generated proto)

### Agent Runtime — Python (`agent-runtime/`)

- `langgraph ~=0.2.0`, `langchain ~=0.3.0`, `langchain-core ~=0.3.0`, `langchain-openai ~=0.3.0`, `langchain-mcp-adapters ~=0.1.0` — pre-1.0 APIs drift across minors; tighten to compatible-release operator until the planner (#44), scorer (#24), and reflection loop (#46) stabilize
- `temporalio ~=1.8.0` (Python SDK) MUST match `Temporal SDK v1.44.1` (Go) at the workflow-versioning layer and the `temporal server` version pinned in `Tiltfile`
- `redis >= 5.0` — used against Valkey. **MUST set `protocol=2`** in client config — RESP3 push handler + `CLIENT TRACKING` semantics diverge between redis-py 5.x and Valkey 7.2/8.x
- `pydantic ~=2.0`, `httpx ~=0.28.0`, `structlog ~=24.0`, `uvicorn ~=0.32.0`
- `protobuf` exact pin required (`==5.28.x` OR `==6.30.x` — pick one): 5.x→6.x flipped the C++/upb default backend; descriptor pool collisions surface as `TypeError: Couldn't build proto file` at import time. The current `>=7.35.0` floor is suspect — confirm intent.
- `connectrpc ~=0.10.0` — **available on PyPI** (beta, Apache 2.0; https://github.com/connectrpc/connect-python). Codegen: `protoc-gen-connectrpc` or `buf.build/connectrpc/python` remote plugin. Supports Connect + gRPC transports (gRPC-Web not yet). Beta → pin exact; isolate the import to ONE adapter file (e.g., `agent-runtime/src/harpia_agents/transport/connect_client.py`).
- `pydantic-settings` is a SEPARATE package — `BaseSettings` import path moved out of `pydantic` core
- Dev: `pytest ~=8.0`, `pytest-asyncio ~=0.24.0` (asyncio_mode = "auto"), `ruff ~=0.8.0`, `mypy ~=1.13.0` (strict)
- Layout: `src/harpia_agents/{agents,graph.py,main.py,mcp_servers,services.py,temporal,temporal_worker.py,tools}`, `gen/harpia/...` (generated proto — SIBLING of `src/`, NOT inside), `tests/`

### Frontend — SvelteKit (`frontend/`)

- Svelte `^5.0.0` + SvelteKit `^2.16.0` + `@sveltejs/adapter-node ^5.0.0`
- Vite `^6.0.0` with `@sveltejs/vite-plugin-svelte ^4.0.0` — **requires Node ≥18.19 / 20.6 / 22 / 24**. `mise.toml` now pins `node = "24"` (was 22 before #76); consider tightening to a specific 24.x minor
- **Tailwind v4** via `@tailwindcss/vite ^4.0.0` — pin exact minor (Oxide engine + config-in-CSS is a real shift). `bun why postcss` from `frontend/` MUST return nothing — if any dep drags PostCSS back in, the Oxide engine silently double-processes
- `bits-ui ~1.0.0` (shadcn-svelte primitives) — **tighten from `^1` to `~1.x`** until Slack bot UI (#19), audit log viewer (#50), and agent catalog (#52) ship; pre-stable primitives can still shift API
- `lucide-svelte ^0.460.0`, `mode-watcher ^0.5.1`
- TypeScript `^5.7.0` (strict, `moduleResolution: bundler`)
- Vitest `^2.0.0`, Prettier `^3.4.0` + `prettier-plugin-svelte`, `prettier-plugin-tailwindcss`
- `@opentelemetry/api ^1.9.0` for browser tracing
- Package manager: **Bun** (`bun.lock` is the lockfile — NOT npm/pnpm). `engines.node: "24.x"` is now committed in `package.json` matching `mise.toml`.
- Layout: `src/{app.css,app.d.ts,app.html,lib/{auth.ts,client.ts,components,gen,types.ts},routes/{auth,login,oversee,tasks,+layout.{server.ts,svelte},+page.svelte}}` — generated Connect TS client lives in `src/lib/gen/harpia/...` and is imported via SvelteKit's `$lib/gen/...` alias

### Infrastructure

- **kind** cluster for dev; **k8s + Helm** for prod (`deploy/harpia/Chart.yaml`)
- **Podman** as container runtime (`mise.toml` builds via `podman build`, dev infra via `podman compose -f deploy/dev/compose.yaml`)
- **Tilt** orchestrates dev (`Tiltfile` at repo root) — builds, deploys, hot-reloads
- **Atlas** for Postgres migrations (`database/`)

### Schema & Codegen

- Protos in `proto/harpia/{tasks,agents,identity}/v1/...` plus planned `feedback/v1` (per ADR-006) and planned `budget/v1` (Budget Policy Service capability inside Agent Orchestration — see "Budget Policy Service" rule below)
- `proto/` holds **only** source `.proto` files — NEVER generated output
- **Generated code paths (canonical):**
  - **Go:** `control-plane/gen/harpia/<context>/v1/` — imports as `github.com/harpia/control-plane/gen/harpia/<context>/v1`
  - **Python:** `agent-runtime/gen/harpia/<context>/v1/` — **SIBLING of `src/`, NOT inside `src/harpia_agents/`** (avoids editable-install double-registration). Add to `[tool.hatch.build.targets.wheel] packages = ["src/harpia_agents", "gen/harpia"]`
  - **TypeScript:** `frontend/src/lib/gen/harpia/<context>/v1/` — natural import via `$lib/gen/...` alias
- **`buf generate` is manual** — Tilt does NOT auto-regenerate on `.proto` edits. Run `mise run buf-generate` after editing protos.
- **`buf breaking` against `trunk` MUST pass in CI** before merge (currently not enforced — backlog gap)
- **`buf lint` must pass** before generation

### Version-Pinning Discipline

- All Python deps in `agent-runtime/pyproject.toml` MUST use compatible-release (`~=`) or exact pins — NOT open `>=`. Pre-1.0 libraries (`langgraph`, `langchain-*`, `langchain-mcp-adapters`, `connectrpc`) get exact or `~=0.x.y` pins; transitive 0.x bumps WILL break the agent graph.
- `uv lock` MUST be committed. CI runs `uv sync --frozen`. Renovate or Dependabot drives bumps as deliberate PRs — never silent.
- Go module `go.sum` committed; `go mod tidy` clean on every PR.
- Frontend lockfile is `bun.lock` (Bun is the package manager). Lockfile committed; CI runs `bun install --frozen-lockfile`.

### Runtime Version Source of Truth

- **`mise.toml` is canonical** for Go, Python, Node, Bun, uv, Buf, Tilt, golangci-lint, Atlas, Temporal CLI.
- Downstream files MUST agree:
  - `control-plane/go.mod` `go` directive matches `mise.toml`'s Go
  - `agent-runtime/pyproject.toml` `requires-python` matches `mise.toml`'s Python
  - `frontend/package.json` `engines.node` matches `mise.toml`'s Node
- NO `.tool-versions` file — mise reads `mise.toml`; a parallel file invites drift.
- Recommended CI check: `scripts/check-version-drift.sh` — grep+compare across the four locations; fail on mismatch.

### Service & Provider Versions

- **Valkey:** clients (Go `go-redis/v9`, Python `redis`) MUST set `Protocol: 2` / `protocol=2`. RESP3 behavior diverges from upstream Redis.
- **Temporal:** Python SDK (`temporalio`), Go SDK (`go.temporal.io/sdk`), AND the `temporal server` version in `Tiltfile` all pinned together. `workflow.patched()` semantics surface only at version boundaries.
- **Postgres 16 + pgvector:** Atlas migrations are the ONLY source of truth — no out-of-band `psql` schema edits. Every migration touching tenant-scoped tables MUST include `pg_policies` assertions in its `_test.sql`.

### Budget Policy Service (capability inside Agent Orchestration)

- Cost is a **policy surface**, NOT a 6th bounded context. Lives inside Agent Orchestration as the Budget Policy Service with its own proto package: `harpia.budget.v1`.
- One enforcement middleware; one source of truth for quota state and `cost_budget` evaluation.
- Planner (#44), canary tester (#26), trust scorer (#24/#25), and BYO key resolution (#33), cost tracking (#34), LLM provider abstraction (#35) all call into it.
- Ship the proto + middleware BEFORE the planner lands, so planner is born talking to a real contract.
- **Re-evaluate as a full context only when** a non-engineering persona (Leader/Overseer) starts asking "what's my budget showing this week?" as a primary workflow.

---

## Critical Implementation Rules

### Foundational Rules (cross-cutting)

- **All RPC = ConnectRPC.** Three modes: gRPC HTTP/2 (svc↔svc), Connect protocol over HTTP/1.1+JSON (browser↔server), server-streaming (real-time UI). No WebSocket, no NATS, no REST handcrafted. (ADR-001)
- **LangGraph for reasoning, Temporal for durability — they do not overlap.** LangGraph nodes decide what; Temporal activities ensure it gets done. Graph contract: `planner → scorer → worker → router → {worker | human_feedback | END}`. (ADR-002, ADR-007)
- **Agents are ephemeral and stateless.** No persistent agent memory; context comes from task definition, parent results, MCP tools, Overseer feedback. (ADR-007). Overseer feedback that *persists across runs* (trust scores, audit) is **Overseer-owned memory** — lives in Identity/Agent Orchestration stores, NOT inside the agent process.
- **Five bounded contexts, not three:** Task Management, Agent Orchestration, Human Interaction, Identity & Tenants, Workflow Engine. `feedback.v1` proto is its own package (not embedded in `tasks.v1`). (ADR-006). Cost = **capability** inside Agent Orchestration with its own `harpia.budget.v1` proto — NOT a 6th context.
- **No `database/sql`.** Use `pgx/v5` with RLS-aware queries; tenant isolation enforced at the DB layer via `current_tenant_id()`. (ADR-003)
- **Domain logic is protected from dependencies.** Hexagonal/ports-and-adapters discipline. Proto types stop at handlers (Go) and at the transport boundary (Python/TS). Provider SDKs (langchain_openai, @connectrpc/connect, pgx) are imported in ONE adapter file per concern, never in domain code.

### Language-Specific Rules

#### Go (`control-plane/`)

- `pgx/v5` directly via `pgxpool.Pool`; never `database/sql`. RLS context per-transaction via `SET LOCAL app.tenant_id = $1` (NOT pool-wide, NOT session GUC).
- Error wrapping: `fmt.Errorf("context: %w", err)`. Sentinels as `var ErrFoo = errors.New(...)`. Check via `errors.Is` / `errors.As`. Convert to `connect.NewError(connect.CodeX, err)` at the handler boundary — never return bare errors through ConnectRPC.
- ConnectRPC handler signature: `func (s *Service) Method(ctx context.Context, req *connect.Request[pb.MethodRequest]) (*connect.Response[pb.MethodResponse], error)`. Always wrap returns with `connect.NewResponse(...)`.
- Internal types ≠ proto types. Repository/domain layers use plain Go structs; convert at the handler boundary. Generated proto types stay behind the handler — never leak.
- Temporal workflows: pure & deterministic. No goroutines, no `time.Now()`, no `math/rand`, no IO. Use `workflow.Now(ctx)`, `workflow.NewTimer(ctx, d)`, `workflow.Go(ctx, f)`. Side effects belong in activities.
- `internal/<context>/` mirrors bounded contexts (`tasks`, `agents`, `feedback`, `identity`, `tenants`, `workflow`). Cross-context coupling allowed ONLY through proto interfaces in `gen/`.
- pgvector: `pgvector.RegisterTypes(ctx, conn)` in the pool's after-connect hook, else binary encoding fails silently.
- NEVER read `X-Tenant-ID` directly. Tenant context flows through the resolver middleware (issue #31). The `"anonymous"` fallback in `internal/server/middleware.go:RateLimit` is a placeholder, NOT production discipline (issue #31 annotated).

#### Python (`agent-runtime/`)

- `mypy --strict` mandatory. **`Any` is forbidden in domain code.** Permitted ONLY at MCP / proto / langchain boundaries, with `# type: ignore[<rule>]` annotated with the boundary reason. Domain logic that returns `Any` is a refactor target.
- `pydantic v2`, not v1. `model_config = ConfigDict(...)`, NOT inner `class Meta:`. Use `model_validate` / `model_dump` (NOT `parse_obj` / `.dict()`). Mutable defaults via `Field(default_factory=list)`.
- `pydantic-settings` is a SEPARATE package — add to deps when env-loading configs.
- **LangChain provider isolation**: import `BaseChatModel` from `langchain_core.language_models` everywhere EXCEPT one adapter file per provider (`harpia_agents/llm/openai_provider.py`, `..._anthropic_provider.py`, etc.). Issue #35 covers the refactor; `graph.py:12` is the current violation.
- LangGraph state: pydantic `BaseModel` OR `TypedDict` with `Annotated[..., reducer]` for merge semantics. Returned fields **fully replace** by default — explicit-merge convention (`{**state.field, **new}`) required for accumulating fields. Decision (BaseModel + convention vs TypedDict + reducers) recorded in ADR-008 (per issue #64 acceptance).
- NEVER use `dict[str, Any]` inside graph state. Carry typed instances (`list[Subtask]`, `dict[str, SubtaskResult]`). Issue #64 covers the refactor.
- Async-by-default tests: `pytest-asyncio asyncio_mode = "auto"` — do NOT add `@pytest.mark.asyncio` to `async def test_*` functions.
- `structlog` over stdlib `logging`. Configure once in `main.py`; `logger = structlog.get_logger()` everywhere. Bind `tenant_id` / `task_id` at the request boundary.
- Temporal Python workflows: same purity rules as Go. No `datetime.now()`, no `random`, no naked `asyncio.sleep`. Use `workflow.now()`, `workflow.uuid4()`, `workflow.sleep()`.
- Hardcoded model strings forbidden outside the provider adapter. `graph.py:51 ChatOpenAI(model="gpt-4o-mini")` is the current violation (issue #35 annotated).
- `ruff` selects `E F I N W UP`, ignores `E501`. Do NOT add `D` (docstrings) or `S` (bandit) without team discussion.

#### TypeScript / Svelte 5 (`frontend/`)

- **Svelte 5 runes ONLY for new code.** State: `let count = $state(0)`. Derived: `let doubled = $derived(count * 2)`. Effects: `$effect(() => { ... })`. Props: `let { foo, bar } = $props()`. Legacy `writable()` / `derived()` stores are tolerated ONLY in `+layout.server.ts` / `+page.server.ts` for SSR data flow until rune-native equivalents land — never in components.
- TS `moduleResolution: "bundler"` (correct for Vite + Bun). NEVER switch to `"node"` or `"node16"`.
- ESM-only: `"type": "module"` in `package.json`. Use `import.meta.url`, never `__dirname`. Top-level `await` allowed.
- SvelteKit `$lib` resolves to `src/lib/` automatically — NEVER add `@/` or `~/` aliases. Generated Connect TS clients import from `$lib/gen/harpia/...`.
- Tailwind v4 design tokens in `src/app.css` via `@theme { --color-* / --font-* }`. NOT in `tailwind.config.js`. **Tokens are LOCKED** per project policy. Reference as `bg-obsidian`, `text-cream`, `font-heading`, etc.
- bits-ui composition: `import { Dialog } from "bits-ui"` → `<Dialog.Root><Dialog.Trigger /><Dialog.Content /></Dialog.Root>`. Compose via the namespace; never reach for sub-components individually.
- **ConnectRPC client = official `@connectrpc/connect` + `@connectrpc/connect-web`**. The current hand-roll in `frontend/src/lib/client.ts` is being replaced (issue #63). All new RPC calls use generated typed clients from `$lib/gen/...` via `createPromiseClient`.
- Streaming = Connect server-streaming via async iterators (`for await (const msg of stream)`). NOT WebSocket, NOT SSE, NOT custom protocols.
- Type-only imports: `import type { Foo } from '...'` for type-only references.
- `svelte-check --tsconfig ./tsconfig.json` (`bun run check`) for full project check — `tsc --noEmit` MISSES `.svelte` files; never substitute.
- `prettier-plugin-tailwindcss` owns class ordering — do not hand-order.
- OpenTelemetry browser: `@opentelemetry/api` is the abstraction; full provider/exporter config belongs in `src/lib/otel.ts`. `@opentelemetry/sdk-*` imports stay in that file.

### Framework-Specific Rules

#### ConnectRPC

- **Three transport modes, never more:** gRPC HTTP/2 (svc↔svc) · Connect HTTP/1.1+JSON (browser↔server) · Connect server-streaming (real-time UI). No WebSocket, no SSE. **No bidi streaming** — server-streaming covers current needs; a bidi requirement requires an ADR amendment, not a default-open door.
- **Interceptor chain (canonical order):** tenant resolver → auth → OTel tracing → retry → logging. Adding/reordering requires code review.
- **Error mapping:** every `connect.Code` has a documented domain meaning. `Unauthenticated`/`PermissionDenied` from resolver+AuthZ only. Handlers use `Internal` for bugs, `InvalidArgument` for validation, `NotFound` for missing entities, `FailedPrecondition` for state-machine violations.
- **Server-streaming envelopes are Connect-framed** — generated clients handle this. Hand-rolled framing forbidden (issue #63).
- **Proto deprecation:** `[deprecated = true]` fields stay one minor release before removal; `buf breaking` enforces.

#### Buf

- `proto/buf.yaml` = module config; `proto/buf.gen.yaml` = codegen; `proto/buf.lock` committed.
- `mise run buf-generate` is canonical; CI runs it then `git diff --exit-code` to detect uncommitted gen.
- `buf lint` + `buf breaking --against '.git#branch=trunk'` MUST pass in CI before merge (currently not enforced — backlog gap).
- Plugins: `buf.build/connectrpc/{go,python,es}` for Connect stubs; `buf.build/bufbuild/{go,es}` + `protoc-gen-python` for messages.

#### Cross-Language Contracts (`proto/harpia/common/v1`)

- Shared envelope package consumed by every service proto. Not a bounded context — a contracts package.
- Contents: `Tenant` (id, slug), `TraceContext` (W3C traceparent), `Pagination` (page_token, page_size), `Error` (code, message, details — for in-stream error reporting), `AuditEvent` (shared envelope for `feedback.v1` and the audit log #56).
- Forces Go/Python/TS onto identical wire shapes — eliminates per-service drift.

#### LangGraph

- **Node purity:** `(state) -> dict` or `(state) -> str` (router). IO injected via context, never module-scope imports.
- **Conditional edges** declare return-string → node mapping: `graph.add_conditional_edges("worker", router_node, {"worker": "worker", "human_feedback": "human_feedback", END: END})`.
- **State merge:** default = full-replace per field. Accumulating fields use explicit-merge convention `{**state.field, **new}` OR `TypedDict` + `Annotated[..., reducer]` — decision recorded in ADR-008 (per issue #64 acceptance).
- **Checkpointer pinned:** `MemorySaver` for tests, `PostgresSaver` (`langgraph-checkpoint-postgres`) for prod. Version-pinned alongside `langgraph` — replay breaks on drift.
- **`interrupt_before` / `interrupt_after`** for human-in-the-loop. Never ad-hoc `if state.human_feedback is None: return` in domain nodes. Temporal signals resume.
- **Graph contract** (ADR-002 + ADR-007): `planner → scorer → worker → router → {worker | human_feedback | END}`. Extending via ADR amendment, never ad-hoc.

#### Temporal

- **Workflow code is deterministic & pure:** no `time.Now()` / `datetime.now()`, no `math/rand` / `random`, no IO, no native goroutines/threads/sleeps. Replay produces identical results.
- **Activities are where IO lives**, retried per the workflow's policy.
- **Retry policy** declared on workflow/activity options, NOT in business logic. Default: exponential backoff, max 5 attempts, max 10 min/activity. Override when justified.
- **Signals & queries** carry strongly-typed payloads from `harpia.tasks.v1` / `harpia.feedback.v1`. Signals = events IN (Overseer approval); queries = state OUT (UI status read).
- **`workflow.patched()`** for breaking changes — new versions get a patch-id branch or new task queue.
- **Worker registration:** control-plane → Go workers on `harpia-tasks`; agent-runtime → Python workers on `harpia-agents`. Never mix.
- **Tiltfile dev server version pinned** to match SDK versions (Cat 1 service-versions rule).

#### SvelteKit

- **`+page.server.ts` vs `+page.ts`**: server-only for tenant-sensitive data, auth tokens, server-side Connect calls. Universal pages are rare in this app.
- **`+layout.server.ts`** owns session resolution + tenant context. Children read via `data` prop, never store imports.
- **Hooks (`src/hooks.server.ts`)**: `handle` for per-request interceptors (auth, tenant, OTel root span); `handleFetch` for SSR outbound rewrites. Keep handlers slim — heavy logic in `$lib/`.
- **Form actions** for page-bound state mutations; Connect clients for everything else (real-time, cross-page, complex).
- **Server-only modules**: `*.server.ts` + `$env/.../private` are SSR-only. Vite enforces; do not bypass via dynamic imports.
- **`adapter-node`** for prod (matches Helm chart). `vite preview` is a smoke tool, not a runtime.
- **Path aliases**: `$lib` + `$env` only — no project-specific aliases.
- **Route groups**: `(group)/+page.svelte` for layout isolation (currently `auth`, `oversee`, `tasks`).

#### Tailwind v4

- Single CSS entry: `frontend/src/app.css` with `@import "tailwindcss";` + `@theme { ... }`.
- **Design tokens LOCKED** (auto-memory): `obsidian`, `obsidian-light`, `plumage`, `talon-gold`, `talon-gold-bright`, `cream`, `crown-ash`, `crown-ash-dark`; fonts Manrope/DM Sans/JetBrains Mono/Bodoni Moda. No modification; no new color tokens without team review.
- **`@reference "tailwindcss"`** at the top of any `.css` file outside `app.css` that uses Tailwind utilities — without it, classes don't resolve.
- **Dark mode**: `mode-watcher` drives `[data-theme="dark"]` on `<html>`. Use `dark:` variant; theme-swap tokens via override block in `app.css`.
- **No `tailwind.config.js`** — v4 is config-in-CSS by design.
- **`bun why postcss` returns nothing** — Oxide engine runs without PostCSS.

#### Atlas (database migrations)

- **Versioned migrations only.** `atlas migrate diff` → `atlas migrate apply`. NEVER `atlas schema apply` (declarative bypasses the log).
- Migrations in `database/migrations/` with sequential SHA-stamped names.
- **`atlas migrate lint --latest 1` in CI** blocks destructive ops without explicit override.
- **RLS policies live in migrations**, not bootstrap code. Every migration touching tenant-scoped tables: (a) table change, (b) `ENABLE ROW LEVEL SECURITY`, (c) policies, (d) sibling `_test.sql` asserting via `pg_policies`.
- `atlas schema diff` vs freshly-applied dev DB MUST be empty on every PR.
- No out-of-band `psql` schema edits. Ever. Hot-fixes are migrations.

#### Helm

- Chart at `deploy/harpia/`. Templates per-service; values per-environment.
- `Chart.yaml` version bumps semver on every templated change.
- **No inline secrets in values.** Kubernetes Secrets or external-secrets operator only (issue #30).
- `templates/_helpers.tpl` for shared label/name/selector helpers.
- `helm lint deploy/harpia/` MUST pass in CI (issue #23 is the current violation).

#### Tilt

- `Tiltfile` at repo root: Podman image builds, k8s manifests (kind), port-forwards, `local_resource` for host services.
- **Hot-reload**: control-plane via `live_update` (sync `cmd/` + `internal/`, rebuild Go, restart); agent-runtime via `live_update` (sync `src/`, restart uvicorn); frontend NOT in cluster — host Vite (auto-memory; issue #62).
- **`local_resource`** for host processes (Zitadel port-forward sync per issue #60).
- Tilt does NOT auto-run `buf generate` — proto edits need manual `mise run buf-generate`.
- Tilt UI at `http://localhost:10350`.

#### Zitadel (AuthN)

- Bootstrap pipeline (auto-memory): init Job → PAT issuance → registration Job that creates the harpia OIDC application via Zitadel API. **Masterkey is for encryption, NOT API auth** — API auth uses the PAT.
- Dev access: `localhost:8085` via service port-forward (auto-memory).
- Dev login fallback: 3 personas via `devLogin()` (auto-memory). Issue #59 gates to dev only.
- OIDC client registration uses `appType: OIDC_APP_TYPE_USER_AGENT` for the SPA (per issue #61 resolution).

#### OpenFGA (AuthZ — ReBAC)

- Zanzibar model lands with issue #37: `user → tenant.member → workspace.{editor,viewer} → task.{view,edit}`.
- **Separation of concerns** (ADR-004): Zitadel = who, OpenFGA = what, Postgres RLS = data-layer enforcement. All three must agree.
- AuthZ check middleware on every Connect handler touching tenant-scoped resources (issue #38) — `Check(user, relation, object)` before proceeding.
- OpenFGA tuple writes transactional with the corresponding Postgres write.
- Frontend AuthZ guards hide actions the user can't perform — handler check is canonical, never trust the hide.

#### MCP (Model Context Protocol)

- Agents reach external systems via MCP servers — never direct API integrations in agent code.
- **Capability-gated client**: client advertises a capability per server; agent code requests by capability, not by server name. Issue #45 covers infra.
- **Out-of-process workers**: MCP servers are separate processes per the spec. Agent-runtime connects via MCP transport; never embeds MCP server logic.
- Per-tenant MCP server config (issue #43): tenants declare which servers their agents may use.
- OAuth helper for stateful providers (issue #48): Slack, GitHub, etc. — handles the redirect dance, stores tokens encrypted in tenant secrets.

#### OpenTelemetry

- **Trace propagation via Connect interceptors** (Cat 2 + ConnectRPC rule). Cross-language traces work because interceptors share the proto envelope.
- Each service initializes the OTel SDK ONCE in `main` / `main.py` / `src/lib/otel.ts`. Never per-request, never in domain code.
- Exporters: Tempo for traces (collector at `tempo:4317` in dev), Prometheus scrape for metrics (issues #20, #21).
- Span naming: `<service>.<operation>` (e.g., `task.create`, `agent.execute`). RPC spans inherit names from the proto method — don't rename.

### Testing Rules

#### Philosophy

- **DDD-first, not strict TDD** (per `docs/WORKFLOW.md`). Model domain invariants from ADRs → implement → test behavior. Tests verify contracts, not implementation — refactors must leave them green.
- **Coverage is NOT a target metric.** Behavior coverage of ADR-defined invariants is the goal: bounded-context boundaries, state-machine transitions, RLS isolation, agent statelessness, feedback round-trips.
- **`mise run test` is canonical** — runs Go (`go test ./... -race`), Python (`uv run pytest`), TS (`bun test`). CI runs the same.
- **Tests live next to source:** Go `*_test.go` in package, Python `tests/` directory, TS `*.test.ts` colocated, E2E in `frontend/e2e/`.

#### Mocking & test doubles (ports-and-adapters discipline)

- **Define ports (interfaces) at the domain edge.** Examples: `TaskRepository`, `AgentRegistry`, `ChatModel`, `MCPClient`, `EventBus`, `BudgetPolicy`. Domain code depends on these — never on `pgxpool.Pool`, `*connect.Client[X]`, `ChatOpenAI`, or `redis.Client` directly.
- **Domain tests use fakes that satisfy the port.** Hand-rolled `fakeFoo` structs (Go) / `FakeFoo` classes (Python) / fixture objects (TS). Encouraged — fast, deterministic, no infra. This IS interface-switching, exactly the freedom the ports give you.
- **Adapter tests use real services.** `pgxTaskRepository_test.go` against real Postgres (shared dev DB with transaction rollback per test, or testcontainers when isolation matters). The langchain adapter MAY use `FakeListChatModel` (a LangChain-provided fake of `BaseChatModel`) — that's still using a fake of the **port**, not a mock of the OpenAI HTTP client.
- **Hard prohibitions** (the only "never"):
  - **Never mock concrete driver types**: `pgx.Conn`, `connect.Client`, `redis.Client`, raw `temporalio.client.Client`. Couples tests to driver internals; prevents driver swaps.
  - **Never mock RLS.** Real Postgres only — policy behavior is too subtle.
  - **Never mock the Connect wire framing.** Use `createRouterTransport` (TS) / call handlers directly (Go/Python) — never hand-roll envelope mocks.
- **E2E tests use no test doubles.** Real Postgres, real Temporal, real Zitadel, real OpenFGA against a seeded dev environment.

#### Go (`control-plane/`)

- `go test -race ./...` — race detector ON in CI, always.
- **`t.Cleanup(...)` over `defer`** for teardown — runs in correct LIFO order even with subtests.
- **Subtests via `t.Run(name, ...)`** for table-driven cases.
- **ConnectRPC handler tests call handlers directly**: `req := connect.NewRequest(&pb.CreateTaskRequest{...}); resp, err := handler.CreateTask(ctx, req)`. No HTTP transport needed.
- **pgx adapter tests** use a real Postgres via `BEGIN; ... ROLLBACK;` per test, OR testcontainers when isolation matters. pgvector tests need `pgvector.RegisterTypes(ctx, conn)` in setup (Cat 2 rule).
- **Temporal workflow tests**: `testsuite.WorkflowTestSuite{}` → `s.NewTestWorkflowEnvironment()` for workflows, `s.NewTestActivityEnvironment()` for activities. Signals via `env.RegisterDelayedCallback`; assert via `env.GetWorkflowResult(...)`.
- **Test fixtures**: factory functions per test, NOT shared package-level globals.

#### Python (`agent-runtime/`)

- `uv run pytest`. `pytest-asyncio asyncio_mode = "auto"` — NO `@pytest.mark.asyncio` decorator (Cat 2 rule).
- **`mypy --strict` is part of the test gate** — failing mypy fails the test pipeline.
- **LangGraph node tests** = call the node function directly with a constructed `TaskState`, assert the returned dict. No graph compilation needed.
- **LangGraph integration tests**: compile a test graph with `MemorySaver` checkpointer, drive via `graph.invoke` / `graph.stream`, assert state evolution. Use `interrupt_before` to test human-in-the-loop semantics.
- **LLMs in tests use `FakeListChatModel` / `FakeMessagesListChatModel`** from `langchain_core.language_models.fake`. Never call real OpenAI/Anthropic/Ollama in tests.
- **Temporal Python tests**: `temporalio.testing.WorkflowEnvironment.from_local()` for time-skip; `from_client()` for real-server end-to-end tests.
- **Pydantic round-trip tests**: `model_validate_json(model.model_dump_json())` then assert equality — catches schema regressions cheaply. Required for any state model touched by issue #64.
- **Property-based testing via `hypothesis`** for state machines (task status transitions, feedback decisions) and combinatorial logic (subtask scheduling, capability matching). Not required everywhere — required where the state space is combinatorial.
- **`structlog` in tests**: `structlog.testing.LogCapture` to assert on logged events. Don't grep stdout.
- **Test fixtures**: pytest `@fixture` functions, function-scoped by default. `session` scope only for genuinely expensive setup.

#### TypeScript / Svelte (`frontend/`)

- **Unit: `vitest run`** (NOT `vitest watch` in CI). Run via `bun run test`.
- **Component tests**: `@testing-library/svelte` with Svelte 5 runes — `render(Component, { props })`, assert via `screen.getByRole` / `screen.getByText`. **No snapshot tests** — they rot, hide intent, break on whitespace.
- **Type check is part of the test gate**: `bun run check` (svelte-check) — fails CI on type errors. Never substitute `tsc --noEmit` alone.
- **ConnectRPC clients in tests**: use the official `createRouterTransport(routes)` from `@connectrpc/connect` to fake the wire without HTTP. NEVER mock `fetch` directly — Connect framing is opaque to `fetch`.
- **Test fixtures**: factory functions in `src/lib/test/factories.ts`. NEVER share mutable state across tests.

#### E2E (Playwright)

- Issues #53 (infra), #55/#57/#58 (Leader / Overseer / Platform Engineer persona journeys) cover the rollout.
- Tests live in `frontend/e2e/`. Separate runner, separate lifecycle from unit tests.
- Run against the dev environment: `mise run dev` then Playwright against `http://localhost:5173`.
- **Per-persona fixtures** seeded via Atlas migrations + `e2e/fixtures/*.sql`. Three personas + Platform Engineer pre-provisioned in a known tenant.
- **NEVER run against prod.** Dev/staging only.
- **CI runs Playwright headless** with HTML report uploaded as artifact (trace + video on failure).
- E2E coverage: critical-path golden flows only (Leader creates task → Overseer reviews → agent runs → result). Not exhaustive — that's the unit + integration tier's job.

#### Migration tests (`database/migrations/*_test.sql`)

- Every migration touching tenant-scoped tables ships a sibling `_test.sql`.
- Asserts via `pg_policies` that RLS is enabled and the expected policies exist on every tenant-scoped table.
- Asserts via `pg_tables` / `pg_class` that no orphaned tables or sequences linger.
- Runs after `atlas migrate apply` in CI; failure blocks merge.

#### Cross-cutting test discipline

- **Tests assert ADR invariants explicitly:**
  - **ADR-002 boundary**: a test asserts that no Temporal activity calls `llm.invoke` directly.
  - **ADR-006 boundaries**: cross-context calls go through proto contracts — a test fails if a non-handler Go file imports from another context's `gen/` package.
  - **ADR-007 statelessness**: an agent integration test re-instantiates the agent class with no prior state and asserts it can execute given only task + parent results + tools + feedback.
  - **Feedback round-trip**: `RequestFeedback` → `SubmitFeedback` → read-back asserts content + status invariants from `feedback.v1`.
- **Contract tests for the wire**: round-trip every proto message type through Connect framing (serialize → deserialize → assert equality). Catches generator drift.

### Code Quality & Style Rules

#### Linters & Formatters

**Go (`control-plane/`)**
- `golangci-lint run` via `mise run lint`. Baseline `.golangci.yml` lands per issue #66 — pinned linter version in `mise.toml`, fixed enabled set (`errcheck`, `govet`, `staticcheck`, `gosimple`, `ineffassign`, `unused`, `gocyclo`, `goconst`, `gosec`, `revive`, `gocritic`, `prealloc`, `unparam`), excluded `gochecknoglobals`/`wsl`/`nlreturn`, generated code never linted.
- `go fmt ./...` via `mise run fmt` (gofumpt-compatible). `goimports` runs as part of fmt — organizes imports into stdlib / external / internal groups.
- `go mod tidy` clean on every PR.

**Python (`agent-runtime/`)**
- `ruff check .` via `mise run lint`. Selects: `E F I N W UP`. Ignores `E501` (formatter owns line length).
- `ruff format .` via `mise run fmt` — replaces black + isort.
- `mypy --strict src/` is part of the test gate (Cat 4).
- Do NOT enable `D` (pydocstyle) or `S` (bandit) without team discussion — noisy / acceptable patterns flagged.

**Frontend (`frontend/`)**
- `prettier --check .` via `bun run lint`. `prettier --write .` via `bun run format`.
- Required plugins: `prettier-plugin-svelte`, `prettier-plugin-tailwindcss` (class ordering — Cat 2/3).
- `svelte-check` runs via `bun run check` (test gate per Cat 4).
- ESLint config decision per issue #67 — recommended path: enable with `typescript-eslint` + `eslint-plugin-svelte` + `eslint-config-prettier` boundary (Prettier owns formatting; ESLint owns correctness/style).

#### Naming Conventions

**Go**
- Exported: `PascalCase`. Unexported: `camelCase`. Files: `snake_case.go`. Tests: `*_test.go` colocated.
- Interfaces: descriptive nouns (`TaskRepository`, `BudgetPolicy`). `-er` suffix ONLY for single-method interfaces (`Stringer`).
- Per-context: `internal/<context>/{domain,ports,handler,<technology>_<port>}.go`.

**Python**
- `snake_case` for modules, functions, variables.
- `PascalCase` for classes (pydantic models, dataclasses, TypeAliases, NamedTuples).
- `UPPER_SNAKE` for module-level constants.
- Private with leading `_`. Files: `snake_case.py`. Tests: `tests/test_<module>.py` mirroring source.

**TypeScript / Svelte**
- Components: `PascalCase.svelte` (`TaskList.svelte`).
- TS modules (non-component): `kebab-case.ts` for new modules. Current flat `client.ts`/`auth.ts` grandfathered.
- Functions / variables: `camelCase`. Types / interfaces / classes: `PascalCase`. Constants: `UPPER_SNAKE` (module-level immutable) or `camelCase`.
- SvelteKit route files follow framework convention EXACTLY: `+page.svelte`, `+page.server.ts`, `+layout.server.ts`, `+server.ts`. Never rename.
- Generated Connect TS clients: `$lib/gen/harpia/<context>/v1` (Cat 1 path rule).

#### Code Organization — Hexagonal Layout

Universal per-service layout enforcing ports-and-adapters:

| Layer | Purpose | Go | Python | TS |
|---|---|---|---|---|
| Domain | Pure types + logic | `internal/<context>/domain.go` | `src/harpia_agents/<context>/domain.py` | `src/lib/<feature>/types.ts` |
| Ports | Interfaces | `internal/<context>/ports.go` | `src/harpia_agents/<context>/ports.py` | `src/lib/<feature>/ports.ts` |
| Adapters | Concrete impls | `internal/<context>/<technology>_<port>.go` | `src/harpia_agents/<context>/adapters/<technology>.py` | `src/lib/<feature>/adapters/<technology>.ts` |
| Entry | Thin RPC / route | `internal/<context>/handler.go` | `src/harpia_agents/services.py` | `+page.server.ts` |

- **No `utils` / `helpers` / `common` catch-all packages.** Functions belong with their domain. (`proto/harpia/common/v1` is a contracts package, not a code dump.)
- **`internal/` in Go enforces module boundaries** — use it.
- **One bounded context = one Go internal subdir = one Python module subdir = one proto package.** Mirror is exact (ADR-006).
- **Test files colocated with source** (Go `*_test.go`, TS `*.test.ts`); Python tests in `tests/` mirroring source tree.

#### Documentation

**Comments**
- **Default = no comments.** Comments explain WHY when non-obvious; self-evident WHAT is conveyed by names.
- **Forbidden**: comments describing what well-named code already does; comments referencing the current task/fix/callers (`// added for issue #42`); multi-paragraph docstrings for self-explanatory functions.
- **Required**: comments at non-obvious invariants; doc comments above public proto messages/fields (drives Connect client doc generation); doc comments above public Go types when name+signature don't capture purpose (Go-doc opener `<TypeName>`).

**ADRs**
- One ADR per architectural decision. Location: `docs/adr/ADR-NNN-<slug>.md`. Template: `docs/adr/ADR-TEMPLATE.md`.
- Sequentially numbered. Next available is **ADR-012**.
- **Pending ADRs filed as issues:** ADR-008 (#68 — LangGraph state-merge semantics), ADR-009 (#69 — ConnectRPC Python adoption), ADR-010 (#70 — Budget Policy Service), ADR-011 (#71 — MCP capability gating).
- ADR status: `Proposed` → `Accepted` → `Deprecated` / `Superseded`. Deprecated ADRs stay in the repo (never deleted).

**READMEs**
- Project root `README.md` + `docs/` are canonical. `deploy/dev/README.md` is justified (self-contained dev infra docs).
- **README files in arbitrary subdirectories are a smell.** Exception: a directory that ships as a standalone component (Helm chart, MCP server, Containerfile-only subdir).

### Development Workflow Rules

#### Branching (trunk-based per `docs/WORKFLOW.md`)

- One main branch: `trunk`. No `develop`, no `release`.
- Branch naming: `{type}/{issue-number}-{short-slug}` — `feat/42-agent-capability-matching`, `fix/61-zitadel-redirect`, `chore/66-golangci-baseline`, `docs/68-adr-008`, `refactor/63-connect-client`.
- Types: `feat`, `fix`, `chore`, `refactor`, `docs`, `test`.
- Force-push allowed on feature branches before merge. NEVER on `trunk`.
- Branch deleted after squash-merge: `gh pr merge --squash --delete-branch`.

#### Commits (Conventional Commits)

- Format: `<type>(<scope>): <subject>` — `feat(agents): add pgvector capability matching`.
- Subject lowercase, imperative, ~72 chars max.
- Scopes: bounded context or technical area (`agents`, `tasks`, `feedback`, `identity`, `infra`, `ui`, `api`, `agent`, `obs`, `proto`, `deps`).
- Body explains WHY when non-obvious; references issues via `Closes #N` / `Refs #N`.
- Never `--no-verify` / `--no-gpg-sign`. Never amend published commits.

#### Pull Requests

- Created via `gh pr create --base trunk`.
- Body template (`.github/PULL_REQUEST_TEMPLATE.md`):
  - `## Summary` (1–3 bullets, WHY-focused)
  - `## Changes` (scoped file/area list)
  - `## Test plan` (markdown checklist)
  - `## Related` (`Closes #N`, ADR reference if applicable)
- Pre-merge required CI checks:
  - `mise run lint` (all services)
  - `mise run test` (all services)
  - `buf breaking` against `trunk` (when protos changed)
  - `helm lint` (when chart changed)
  - `atlas migrate lint --latest 1` (when migrations added)
  - CVE/SBOM scans (see Supply-chain section below)
  - Container BUILD verification (`buf generate`, `docker build`) — never `docker push` on PR
- Squash-merge only.

#### Issues

- Created via `gh issue create`.
- Required body structure: `## Bounded Context` · `## Problem` · `## Scope` · `## Acceptance`.
- Standard labels: `bug`, `feature`, `chore`, `infra`, `adr`, `docs`.
- **Every issue carries a `complexity:*` label** — the signal used to pick which coding agent picks up the issue. The four labels are live in the repo:
  - `complexity:trivial` (light green `#C2E0C6`) — mechanical, locally-scoped, no design choices (typo fixes, formatting, dep bumps, config edits)
  - `complexity:routine` (green `#5DBE3F`) — standard pattern applied to a new instance inside an existing bounded context
  - `complexity:substantive` (amber `#F0A500`) — genuine design choices within a bounded context; multiple valid solutions; trade-offs to weigh
  - `complexity:architectural` (red `#B60205`) — cross-cutting concerns, interface contracts, security boundaries, migrations, ADRs
- **`requires-human-approval` label** (purple `#5319E7`) — cannot merge without an explicit human review. Applied automatically when the PR diff touches sensitive paths (security/identity, ADRs, proto contracts, migrations, CI workflows, Helm chart) OR when an agent disables a lint rule / bumps a runtime version / inflates scope (per the per-agent CI guards). May also be applied manually for refactor PRs that need extra scrutiny regardless of paths touched. See "Coding-agent dispatch" + Cat 7 for the path-based rule set.
- **Coding-agent dispatch** is the user's choice — either manual (`gh issue view` + invoke the right CLI) OR via the dispatch script (per backlog issue — see "Dispatch script" below).
- **Parallel `gh issue create` races** (auto-memory): when issue bodies cross-reference each other, file sequentially — never `&`-fan-out. Numbering races produce wrong references.

#### Dispatch script (`cmd/dispatch/`)

- Small Go binary that uses the `complexity:*` label + a `.dispatch.toml` agent roster to suggest or run the right coding-agent CLI per issue.
- Three commands: `dispatch next --agent <name>`, `dispatch review --pr <N> --agent <name>` (refuses if that agent authored the PR — enables cross-agent review), `dispatch quota`.
- No service / daemon / database / web UI. Stdout + `gh` CLI + agent CLI shell-outs.
- Reversible: one directory, one binary; trivially deletable.

#### `mise` is the canonical task runner

- `mise run dev` / `dev-api` / `dev-agent` / `dev-web` / `dev-worker` / `dev-temporal` / `dev-down`
- `mise run lint` / `test` / `fmt` / `build` / `build-containers`
- `mise run buf-generate` / `buf-lint` / `buf-breaking`
- `mise run db-migrate`
- Don't bypass `mise` — it carries env vars (`HARPIA_ENV = "development"`).

#### Codegen workflow

- Edit `.proto` in `proto/harpia/<context>/v1/`.
- Run `mise run buf-generate` → updates `control-plane/gen/`, `agent-runtime/gen/`, `frontend/src/lib/gen/`.
- Commit generated files in the same PR as the proto change.
- CI runs `buf generate` + `git diff --exit-code` — uncommitted gen fails the build.
- `buf breaking --against '.git#branch=trunk'` in CI — proto regressions block merge.

#### Migration workflow

- Edit schema; run `atlas migrate diff --dir database/migrations`.
- Write the sibling `_test.sql` (asserts `pg_policies` / `pg_tables` invariants per Cat 3 + Cat 4).
- Locally apply: `mise run db-migrate`.
- Verify `atlas schema diff` empty against freshly-applied DB.
- Commit migration + `_test.sql` together.
- CI runs `atlas migrate lint --latest 1` — destructive ops blocked unless explicit override.

#### CI / Container build cadence

**Three triggers, three behaviors:**

| Trigger | Behavior |
|---|---|
| `pull_request` (any branch → `trunk`) | Lint, test, `buf breaking`, `helm lint`, `atlas migrate lint`, CVE/SBOM scans. **Build containers** (verify they CAN build) but never push. |
| `push: branches: [trunk]` | Same as PR (no push). Optional: post-merge regression checks. |
| `push: tags: ['v*']` OR `release: types: [published]` | Build + **push** containers to `ghcr.io/harpia/{api,agent,web}:{semver}` AND `:latest`. Generate CycloneDX SBOMs via `syft`. `cosign attest` the SBOMs into the OCI artifact. Trigger Helm chart release (chart version matches the tag). |

- Two workflows in `.github/workflows/`: `ci.yml` (PR + push to trunk) and `release.yml` (tags / releases).
- Containers are NEVER pushed per PR or per trunk-push — only when a release tag is cut.
- Helm chart version (`Chart.yaml`) bumps in lockstep with the tag.

#### CVE / SBOM / supply-chain

Three gates, fast enough to keep:

**Pre-merge** (target <90s, all parallel):
- `osv-scanner` on lockfiles (`go.sum`, `agent-runtime/uv.lock`, `frontend/bun.lock`) — cross-language CVE coverage
- `govulncheck ./control-plane/...` — Go reachability analysis
- `trivy config` on `deploy/` + `.github/workflows/` — IaC misconfig
- `gitleaks` — secret scan

**Release (tag-triggered, container-build job):**
- `trivy image` on each pushed image — block push on Critical/High
- `syft` generates a CycloneDX SBOM per image
- `cosign attest` signs the SBOM into the OCI artifact in `ghcr.io` — pulling the image pulls the SBOM

**Nightly:**
- Re-scan deployed images against fresh CVE DB
- Open a `security` + severity-labeled issue automatically on findings

**Suppressions / VEX:**
- `.trivyignore` / `osv-scanner.toml` empty by default
- Every suppression entry: CVE ID, justification, 90-day expiry, GitHub issue link, CODEOWNERS-gated

#### Dev environment loop

- First-time: `mise install` → `mise run buf-generate` → `mise run dev`
- Tilt UI at `localhost:10350`. Vite (host) at `localhost:5173`. Zitadel at `localhost:8085` via service port-forward (auto-memory)
- Dev-login fallback (auto-memory): 3 personas (Leader / Overseer / Engineer) via `devLogin()` — bypasses Zitadel. Gated to dev environments per issue #59.

#### Sequencing — identity hardening first

The identity-hardening cluster (#29, #30, #31, #32, #37, #38, #59) is load-bearing for every tenant-scoped feature. Should swimlane ahead of the agent-runtime build-out (#24, #44, #45, #46, etc.) — every agent-runtime PR assumes a working tenant context. Concurrency fine within either swimlane.

#### Renovate / Dependabot

- Renovate drives all dep bumps as deliberate PRs. Pinned floors per Cat 1 (`~=` for pre-1.0 Python deps; tight semver ranges elsewhere). `mise.toml` runtime versions tracked by Renovate's `customManagers`.

### Critical Don't-Miss Rules

The consolidated "never do this" list. Each rule is enforced elsewhere in the document; this is the curated index.

#### Security & tenant isolation

- **NEVER read `X-Tenant-ID` directly from a request.** Always go through the tenant-resolver middleware. The `"anonymous"` fallback in `internal/server/middleware.go:RateLimit` is a placeholder; treating it as production discipline leaks tenants. (Issue #31 annotated.)
- **NEVER mock RLS in tests.** Real Postgres only — policy behavior is too subtle to fake.
- **NEVER edit Postgres schema out-of-band** (`psql` direct, manual `ALTER`). Every change is an Atlas migration with a sibling `_test.sql` asserting `pg_policies` invariants.
- **NEVER commit secrets.** `gitleaks` runs pre-merge; suppressions in `.gitleaks.toml` need CVE-ID-style justification + 90-day expiry.
- **NEVER use the masterkey for API auth in Zitadel** — it's for encryption. API calls use the PAT issued by the init Job. (Auto-memory.)
- **NEVER modify the locked design tokens** in `frontend/src/app.css` (`obsidian`, `obsidian-light`, `plumage`, `talon-gold`, `talon-gold-bright`, `cream`, `crown-ash`, `crown-ash-dark`; fonts Manrope / DM Sans / JetBrains Mono / Bodoni Moda). (Auto-memory.)

#### Stack discipline

- **NEVER bypass ConnectRPC.** No WebSocket, no SSE, no hand-rolled REST. Three modes only: gRPC HTTP/2 (svc↔svc) · Connect HTTP/1.1+JSON (browser↔server) · Connect server-streaming. No bidi streaming without an ADR amendment.
- **NEVER use `database/sql`.** `pgx/v5` directly via `pgxpool.Pool`, with RLS context per-transaction (`SET LOCAL app.tenant_id = $1`), not pool-wide.
- **NEVER hand-roll Connect framing on the wire.** Use generated clients (`@connectrpc/connect-web`, Go ConnectRPC SDK, `connectrpc ~=0.10` Python). The current `frontend/src/lib/client.ts` is being replaced (issue #63).
- **NEVER collapse the LangGraph ↔ Temporal boundary.** LangGraph reasons; Temporal durabilizes. Activities never call `llm.invoke` directly. Workflow code is deterministic — no `time.Now()` / `datetime.now()`, no `random`, no IO, no naked sleeps.
- **NEVER let proto types leak past the handler boundary.** Domain code uses plain Go structs / pydantic models. Proto types stay behind handlers.
- **NEVER import `langchain_openai`** (or any provider package) outside its one adapter file per provider. Current violation: `agent-runtime/src/harpia_agents/graph.py:12,51` (issue #35 annotated).
- **NEVER use `dict[str, Any]` inside LangGraph state.** Carry typed instances (`list[Subtask]`, `dict[str, SubtaskResult]`). Current violation: `graph.py:38` (issue #64).
- **NEVER mock concrete driver types** (`pgx.Conn`, `connect.Client`, `redis.Client`, raw `temporalio.client.Client`). Define ports at the domain edge; fake the ports. Adapter tests use real services.

#### Tooling traps

- **NEVER assume Tilt regenerates proto code.** It does not. After editing `.proto`, run `mise run buf-generate` manually.
- **NEVER put generated proto code inside an importable Python package.** `agent-runtime/gen/` is a sibling of `src/`, not under `src/harpia_agents/` — prevents editable-install double-registration.
- **NEVER push containers on PR or `trunk` push.** Only on tag (`v*`) / release. PR + trunk = verify build only.
- **NEVER use the CDN Tailwind script.** Tailwind v4 wires via `@tailwindcss/vite` + `import "../app.css"`. CDN breaks theme tokens. (Auto-memory.)
- **NEVER run Vite in the cluster.** Vite runs on host (`mise run dev-web`); the k8s frontend pod is dead (issue #62). (Auto-memory.)
- **NEVER bypass `mise`.** Run tasks via `mise run <task>` — env vars (`HARPIA_ENV = "development"`) are part of the contract.
- **NEVER `atlas schema apply`.** Declarative mode bypasses the migration log. Use `atlas migrate diff` → `atlas migrate apply`.

#### Workflow discipline

- **NEVER `--no-verify` / `--no-gpg-sign`** unless explicitly authorized. Fix the hook failure instead.
- **NEVER force-push to `trunk`** (or `main`).
- **NEVER `&`-fan-out `gh issue create`** when issue bodies cross-reference each other — numbering races produce wrong references. (Auto-memory.) File sequentially.
- **NEVER call real LLMs in tests.** Use `FakeListChatModel` / `FakeMessagesListChatModel` from `langchain_core.language_models.fake`.
- **NEVER run E2E tests against prod.** Dev/staging only — Playwright tests assume seeded fixtures.
- **NEVER use snapshot tests** in the frontend. They rot, hide intent, break on whitespace.

#### Codegen + agent dispatch

- **NEVER ship a PR without the matching generated proto code.** CI runs `buf generate` + `git diff --exit-code` — uncommitted gen fails the build.
- **NEVER ask a coding agent to review its own PR.** The dispatch script (`cmd/dispatch review`) refuses this; the rule applies in manual dispatch too.
- **NEVER assign `complexity:trivial` to a PR that modifies CODEOWNERS-protected paths.** Path wins over complexity label; human review is the gate.

---

## Usage Guidelines

**For AI agents:**
- Read this file before implementing any code for harpia.
- Follow the rules exactly. When in doubt, prefer the more restrictive option.
- The cross-references to issues (#NN) and ADRs are live links to GitHub; check issue status before assuming the rule applies as written.
- If a rule conflicts with a freshly-merged change, the freshly-merged code wins — flag the conflict and propose a doc update PR.

**For Thiago:**
- Re-read after every architectural shift (new ADR accepted, new bounded context, runtime change).
- Quarterly review: remove rules that have become obvious; promote new patterns surfacing in PR review.
- Update the labels list when GitHub labels are added/removed.
- Bumped runtime versions in `mise.toml` should trigger a Cat 1 check.

Last Updated: 2026-06-06
