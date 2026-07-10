# Harpia Platform Constitution

**Status:** Canonical source of truth. **Supersedes ADR-001…017 as the reading order.**
**Last updated:** 2026-07-07
**Maintainer:** thbertoldi

> **Read this first.** This document is the single, reconciled description of what
> Harpia is, the vocabulary we use, the architecture we commit to, and the scope of
> the MVP. The ADRs in [`docs/adr/`](../adr/) remain as **dated historical record** —
> they show *how* we got here — but where an ADR and this constitution disagree, **the
> constitution wins.** Each ADR is stamped with a banner pointing here, and
> [`docs/adr/README.md`](../adr/README.md) carries the full supersession map.
>
> Companion documents:
> - **[MVP Roadmap](mvp-roadmap.md)** — current reality, scope, and the sequenced build.
> - **[Cleanup Backlog](cleanup-backlog.md)** — contradictions, duplication, and dead code to retire.

---

## 1. Why this document exists

Harpia's design was captured across seventeen ADRs written over five weeks. Several
ADRs *supersede* or *amend* earlier ones (the plan-centric model replaced the
task/subtask model; thread-first navigation replaced the 1:1 chat-in-a-config model;
declarative templates replaced SQL-seeded templates). Reading them in order forces a
reader — human or agent — to hold contradictory intermediate states in their head and
guess which one is current. That "fuzzy context" is the exact failure this document
removes: **one place that states the current truth, with the ADRs demoted to history.**

This is not a new decision log. It is the consolidation of the decisions already made.
New *decisions* still start as an idea in [`docs/notes/product-ideas.md`](../notes/product-ideas.md)
and graduate to an ADR when they are architectural; when an ADR is accepted, its outcome
is **folded into this constitution** and the ADR is stamped historical.

---

## 2. Product vision & positioning

Harpia is a **chat-first, AI-enabled operations platform** — a *generic* platform, **not**
a vertical CRM/ERP and not a corporate chatbot. It models a company the way the company
actually works: **Áreas** (business areas), shared data, people, tasks, agents, and human
decisions. The unit of value is a **hybrid team**: people working with agents, under human
direction, to gain clarity, speed, and operational control.

The permanent engine, in the product's own words:

> **Área → Plano → Tarefas → Agentes → Entrega → Decisão humana.**
> (Area → Plan → Tasks → Agents → Delivery → Human decision.)

Two corollaries constrain every design decision:

1. **The catalog is content, not code.** Plans are declarative catalog entries
   ([§7](#7-the-plan-centric-model), [§13](#13-authoring-the-catalog)), not hand-coded
   vertical modules. Adding a plan should not mean adding a module.
2. **Surfaces are browsers/editors over the artifact & resource model — not bespoke CRUD
   stores.** Business state and knowledge live as typed **Artifacts** (plan step I/O) and
   **Resources** (the shared layer, [§8](#8-the-resource-layer-the-shared-layer)), governed
   by that model — versioned, permissioned, Área-gated, auditable. UI surfaces are *views
   and governed editors* over those streams, never hand-coded CRUD modules that write
   business state the model doesn't know about. This is what keeps Harpia a *generic*
   platform instead of a vertical CRM/ERP. It does **not** mean everything must pass through
   a plan: **Resources have two legitimate creation paths — governed direct upload/authoring
   (e.g. a brand kit) and promotion from a plan output** — and plan *configuration* inputs
   and executor *connection config* (feed URLs, OAuth) are entered directly too. The
   invariant is *how* state is modeled and governed, not *that a plan produced it*.

The **first commercial front** (Aiuna "Growth Front": Marketing + Sales) is *content on
top of this platform*, not a rewrite of it. See [§12](#12-mvp-scope) and the
[roadmap](mvp-roadmap.md).

---

## 3. Principles

These are the non-negotiables. They come from the delivery constraints in `AGENTS.md`
and the accepted ADRs, reconciled.

1. **Chat-first.** The conversation is the single work surface. A user talks to Harpia;
   Harpia proposes, configures, runs, and reports on plans in that thread. Chat must never
   feel like a launcher for modal forms. ([§9](#9-navigation--lifecycle))
2. **Catalog-as-content.** PlanTemplates are declarative, versioned catalog entries, not
   code paths. ([§13](#13-authoring-the-catalog))
3. **Typed contracts between steps.** Every PlanStep is an `(input_artifact_type →
   output_artifact_type)` contract. Wiring errors are schema errors, caught at
   configuration/seed time — not runtime surprises. ([§7](#7-the-plan-centric-model))
4. **Artifacts are the substrate.** Work products are typed Artifacts in a tenant stream.
   Durable, reusable knowledge is promoted into **Resources** ([§8](#8-the-resource-layer-the-shared-layer)).
5. **Ephemeral agents, explicit memory.** Agents are stateless executors. All memory is
   platform-owned, versioned, permissioned, and auditable — never hidden agent state.
   ([§8](#8-the-resource-layer-the-shared-layer))
6. **Human-in-the-loop by contract.** Two distinct human gates: **Elicitation** (agent
   asks for missing context) and **Approval** (human authorizes a risky transition such as
   publishing). They are different events with different meaning. ([§7.4](#74-human-interaction))
7. **Tenant-safe by construction.** Tenant isolation is enforced at the boundary (RLS,
   tenant-prefixed cache/object keys) and callers cannot bypass the wrappers.
   ([§6](#6-architecture-layers))
8. **Protect the domain.** Hexagonal / ports-and-adapters. Infrastructure and third-party
   SDKs stay out of domain packages; they live in adapters.
9. **Pre-v1: break freely.** Until v1 is cut, break schemas/routes/protocols without
   migrations, deprecation shims, or transitional redirects. Prefer a clean model over a
   compatible one.
10. **Trunk-based, small logical commits.** Never add a `Co-Authored-By: Claude` trailer.
11. **Design tokens: fonts LOCKED, color/depth governed.** Fonts are fixed. Color/depth
    tokens follow the Harpy Eclipse system ([§10](#10-design-system--motion)); animate
    layout/opacity/transform only.
12. **Bilingual always.** All user-facing copy ships `en` + `pt-BR` via flat `translate()`
    keys; catalog/template content localizes via `catalog.plan.<key>.*` and
    `plans.inputs.<key>.label`.
13. **Highest quality bar.** Value decisions belong to the human and are not relitigated as
    "overengineering."
14. **Templates are outcome-shaped; genericity in the catalog, ease in the assistant.** A
    PlanTemplate's identity is the *job/outcome*, never a fixed cadence/channel/format tuple.
    Those are **configuration axes** ([§7.6](#76-template-granularity-identity-is-the-outcome)).
    The extra flexibility does not become user burden because the conversational assistant
    infers and pre-fills defaults ([§9](#9-navigation--lifecycle)).

---

## 4. Ubiquitous language (the reconciled glossary)

These terms are authoritative. Use them exactly, in proto, Go, Python, TypeScript, and
docs. Where a term drifted across ADRs, the **current** definition is given and the
superseded one noted.

### Plan-centric core

| Term | Definition |
|---|---|
| **PlanTemplate** | A reusable catalog DAG of **PlanSteps** with dependency edges. Declarative content, versioned. The *what*, independent of the *who*. |
| **PlanStep** | One atomic slot: an `(input_artifact_type → output_artifact_type)` contract plus human-readable metadata. |
| **PlanConfiguration** | A tenant binding of a template: **SlotBindings** + **OverseerBindings** + **PlanBehaviorPolicies** + seed inputs + optional **PlanSchedule**. Lifecycle: `DRAFT → RUNNABLE → SCHEDULED → DISABLED → ARCHIVED`. Carries `kind` and `origin_thread_id`. |
| **PlanConfiguration.kind** | `ONE_SHOT` or `RECURRING`. Drives the run flow ("run now?" vs "test then schedule"). |
| **PlanBehaviorPolicies** | Tenant-selected runtime behavior: approval mode + elicitation-timeout behavior (generalized beyond LinkedIn). |
| **PlanExecution** | One run of a configuration, walking the DAG. Uses an immutable **snapshot** of the configuration, template version, policies, and installations. |
| **StepExecution** | One firing of a PlanStep in a PlanExecution. Holds input/output artifact refs, status, attempt, and (agent-backed) elicitation/approval refs. |
| **PlanSchedule** | Cron/event trigger that creates PlanExecutions (backed by Temporal Schedules). |

> **Superseded:** "Task" (top-level) → **PlanExecution**; "Subtask" (runtime instance) →
> **StepExecution**; `assigned_agent_id` → `SlotBinding.executor_installation_id`. The
> nouns "task/subtask" survive only in the *adaptive* path (§7.5) and in some proto field
> names slated for rename (see [cleanup backlog](cleanup-backlog.md), C4).

### Executors (the *who*)

| Term | Definition |
|---|---|
| **Executor** | Anything that satisfies a PlanStep contract. Two kinds. |
| **AgentExecutor** | A manifest-declared LangGraph graph. Reasons; may elicit. **Multi-capable** — one agent carries several capabilities, so a small team covers many steps. |
| **IntegrationExecutor** | A deterministic Temporal activity calling an external API. No reasoning, no elicitation. |
| **Capability** | A discrete thing an agent can do (e.g. `linkedin-content-adaptation`, `carousel-authoring`). Plan steps declare the capabilities they *require*; binding matches agents by capability coverage. |
| **Seniority Tier** | An agent's quality/cost grade — Júnior / Pleno / Sênior — each with its own graph, tool access, and per-execution cost. The same role exists across tiers. |
| **ExecutorSKU** | The sold commercial unit (pricing, compatibility metadata). An agent SKU = role × tier, carrying its capability set. |
| **ExecutorEntitlement** | A tenant's grant to use a SKU. |
| **ExecutorInstallation** | A tenant-configured instance of an entitled SKU. Carries OAuth/feed/connection config (integrations) or manifest+policy (agents). **SlotBindings point at installations, never global SKUs.** |
| **Agent Team** | The set of agents (role × tier) assembled to run a plan — recommended by the platform to cover every step's required capabilities, then accepted/swapped by the user. |
| **ExecutorRequirement** | PlanStep metadata declaring the **required capabilities** (and executor kind) that drive team recommendation and compatible-installation filtering. |
| **SlotBinding** | Per-step assignment `step_key → ExecutorInstallation`, chosen so the bound agent's capabilities satisfy the step's `ExecutorRequirement`. |

### Artifacts & Resources

| Term | Definition |
|---|---|
| **Artifact** | A typed payload produced/consumed by a step. Metadata + schema ref + content hash in Postgres; payload in Garage. |
| **ArtifactType** | A registered `(schema_ref, version)` describing a reusable I/O shape. JSON Schema is canonical for MVP. |
| **Resource** | A **promoted, typed, versioned, permissioned** artifact living at tenant/workspace/brand/user scope — the shared layer. Bindable two ways: as agent-readable memory (**MemoryBinding**) and as a plan seed input (**SeedArtifactBinding**). Brand kit, design system, company/brand profile, knowledge base, and offer catalog are Resource *types*. Generalizes ADR-014's `MemoryResource`. ([§8](#8-the-resource-layer-the-shared-layer)) |
| **MemoryBinding** | Narrows which Resources an agent step's tools may read. |
| **SeedArtifactBinding** | Feeds an artifact/Resource/literal into a step as input. |
| **MemoryUsageRecord** | Per-step audit of which Resource IDs/versions were available, read, and by which tool. |

### Human interaction

| Term | Definition |
|---|---|
| **Overseer** | The human who receives elicitation/approval for agent-backed steps. **OverseerBinding** is per-step. MVP: a human tenant user; no re-delegation. |
| **ElicitationRequest** | An agent-authored question for missing context. |
| **ApprovalRequest** | A human gate authorizing a risky transition (e.g. publishing). Distinct from elicitation even if it reuses the same transport. |

### Organization & access

| Term | Definition |
|---|---|
| **Tenant / Workspace / User** | Tenancy hierarchy. Tenant = organization. |
| **Área** | A business area (Marketing, Sales, …). **Áreas are RBAC entities** — membership-based, enforced with Zitadel roles + app-level checks (not a ReBAC graph); see [§11](#11-áreas--rbac). Memberships gate Plans, Agents, Integrations, Artifacts, and Resources; fine-grained enforcement is light at MVP. |
| **Thread / Conversation** | The durable top-level work object the user opens, resumes, and searches. A Thread spawns **1:N** PlanConfigurations. |

### Commercial & metering

| Term | Definition |
|---|---|
| **Credit** | The end-user unit of account. Actions consume credits; a subscription grants credits per month. Credits are the *external* denomination shown to users — distinct from the *internal* money ledger. ([§14](#14-commercial-model--credits-subscriptions--budget)) |
| **Credit rate** | The credit↔money conversion used to price and reconcile. A pricing lever, not a fixed function of measured cost; snapshotted into each PlanExecution at charge time. |
| **CreditWallet** | A tenant's credit balance (granted allotment + top-ups − debits), the source of truth for "can this run be afforded." |
| **SubscriptionTier** | A commercial plan = **entitlements** (which ExecutorSKUs are unlocked) **+ a monthly credit allotment**. Two independent dimensions. |
| **PriceBook** | The mapping ExecutorSKU/step → **credit price** used to estimate and debit. Distinct from the internal `Money` cost. |
| **Budget Policy / cost ledger** | The *internal*, money-denominated accounting (per-provider, per-LLM-invocation): `Money(amount_micros)`, reserve/record/release, `cost_budget` ceiling. The credit layer sits on top of it; both are kept. |

### UI vocabulary (synonyms layered over the domain, not renames)

**Conversation** = Thread. **Gallery** = the PlanTemplate catalog browser. **Runs** = the
executions/recurring-plans browser. **Plan** (in UI) = a PlanConfiguration; **Execution**
(in UI) = a PlanExecution. Structured pages may be denser than chat but must reuse this
exact vocabulary.

---

## 5. Bounded contexts & context map

Canonical set — **six bounded contexts** (this resolves the ADR-010 vs ADR-012
"five vs six" contradiction in favor of six):

| Bounded context | Ubiquitous language | Proto package | Home |
|---|---|---|---|
| **Plan Management** *(was "Task Management")* | template, step, configuration, execution, schedule, slot binding | `harpia.plans.v1` | `control-plane/internal/plans/` |
| **Agent Orchestration** | agent type, capability, execute, stream, elicitation; **Budget Policy** is a *capability here* | `harpia.agents.v1`, `harpia.budget.v1`, `harpia.mcp.v1` | `agent-runtime/`, `control-plane/internal/{budget,mcp}/` |
| **Artifact Types & Resources** *(new)* | artifact, artifact type, resource, memory binding, promotion, usage record | `harpia.artifacts.v1` | `control-plane/internal/artifacts/` |
| **Human Interaction** | elicitation, approval, overseer, notify, signal | `harpia.feedback.v1` | `control-plane/internal/feedback/` |
| **Identity, Tenants & Áreas** | user, tenant, workspace, área, role, membership | `harpia.identity.v1` | `control-plane/internal/identity/` |
| **Workflow Engine** | workflow, activity, signal, timer, retry, schedule | Temporal SDK | Temporal workers |

Supporting commercial context: **Executor Catalog / Marketplace** (`harpia.executors.v1`)
— ExecutorSKU/Entitlement/Installation, plus the **commercial pricing model** (SubscriptionTier,
PriceBook, CreditWallet; [§14](#14-commercial-model--credits-subscriptions--budget)). Plan
Management *queries* it when validating slot bindings and estimating credit cost; it is not
folded into Plan Management. Full marketplace is post-MVP.

Key relationships: Plan Management ↔ Agent Orchestration is a **Partnership** (the core
domain). Workflow Engine → Agent Orchestration is an **Open-Host Service** (Temporal
defines the protocol). Human Interaction → Slack (post-MVP) is an **Anticorruption Layer**.
Everything conforms to Identity.

**Budget is a capability, not a context** (per ADR-010): it lives inside Agent
Orchestration with its own proto package, enforced at the per-LLM-invocation layer. It owns
the *internal money ledger*; the *external credit* pricing/wallet is a commercial concern
([§14](#14-commercial-model--credits-subscriptions--budget)) — enforcement mechanism and
pricing policy stay separate.

---

## 6. Architecture layers

- **Communication — ConnectRPC (Buf).** One protocol, three modes: gRPC/HTTP2
  service-to-service, Connect/HTTP1.1+JSON browser↔server, server-streaming for live UI.
  Schemas in repo-root `proto/`. Python uses the `connectrpc` PyPI library in transport
  adapters only.
- **Orchestration — Temporal + LangGraph.** **Temporal orchestrates every step**
  (order, retries, timeouts, schedules, durable human waits). **LangGraph is the reasoning
  engine *inside* agent-backed steps only.** Every step — integration or agent — is a
  Temporal activity on one `PlanWorkflow` that walks the DAG topologically. Agent steps
  that need a human return an elicitation/approval state; the workflow waits durably on a
  signal and resumes via a continuation activity. Agents never block a worker slot waiting
  on a human.
- **Adaptive-mode reasoning state.** The adaptive planner (§7.5) uses a Pydantic
  `TaskState` with an explicit copy-and-merge convention; no LangGraph reducers until there
  are multiple independent writers to one field.
- **Data — PostgreSQL 16.** Row-Level Security for tenant isolation
  (`current_tenant_id()`), pgvector for embeddings, recursive CTEs for trees. **Valkey**
  for cache, **Garage** for S3-compatible object storage.
- **Tenant-safe boundaries.** Raw Valkey access is confined to `internal/cache/valkey.go`
  behind `cache.TenantStore` (`t:{tenant_id}:{logical_key}`). Garage uses one shared
  bucket with `tenant/{tenant_id}/{logical_path}`. Wrappers derive tenant from
  `identity.RequestContext` and reject pre-scoped keys; a source-scan test fails the build
  if domain code bypasses them. Rate limiting is a Connect interceptor.
- **Identity & access — Zitadel + RLS + thin role checks.** *Who you are* = Zitadel AuthN;
  *data enforcement* = Postgres RLS (tenant isolation); *what you can do* = a small set of
  coarse roles (owner/admin/member) from Zitadel enforced by app-level checks. **OpenFGA/ReBAC
  is retired** ([§11](#11-áreas--rbac)) — it was complexity for permissions the product does
  not yet need. Authorization on **downstream systems of record (e.g. Odoo) is deferred to
  those systems** via the scoped credentials on their ExecutorInstallation.
- **Dev & delivery — mise + Tilt + Temporalite.** `mise run dev` brings the stack up on
  kind. Trunk-based, squash-merge, Conventional Commits. Registry `ghcr.io/harpia/`.

---

## 7. The plan-centric model

### 7.1 Shape

```text
PlanTemplate ──has──> PlanStep[] + edges (DAG)
     │
     └─configured-by─> PlanConfiguration ──runs──> PlanExecution ──walks──> StepExecution[]
                          (SlotBindings, OverseerBindings,
                           PlanBehaviorPolicies, seed inputs, schedule,
                           kind, origin_thread_id)
```

**Invariants.** A `DRAFT` configuration may be incomplete. A `RUNNABLE`/`SCHEDULED` one is
valid only if: every step **that will run** has a compatible SlotBinding (a step whose
capability the user did not opt into — e.g. image generation — is not part of the run and
needs no binding); every agent-backed step has an OverseerBinding; required seed inputs
exist; entitlements are present; required integration installations are connected. Step N+1
cannot start until step N's output validates against its `output_artifact_type`. A
PlanExecution runs against a frozen **snapshot** — later edits never mutate in-flight runs.
This **execution snapshot** is the most consistently reused idea in the platform (budget,
MCP bindings, memory bindings, template version all freeze into it).

### 7.2 Executors satisfy contracts

Separating the *what* (PlanStep contract) from the *who* (ExecutorInstallation) lets the
same step run on a junior agent, a senior agent, or an integration without touching the DAG
or downstream steps. Both executor kinds are Temporal activities; the difference is what
happens inside (deterministic API call vs LangGraph reasoning that may elicit).

**Capability-based teams (ADR-018).** Agents are **multi-capable** and **tiered**
(Júnior / Pleno / Sênior); a PlanStep declares the **capabilities it requires**, not a
specific SKU. Given a plan, the platform recommends a minimal **agent team** (role × tier)
whose combined capabilities cover every step; the user accepts or swaps members. A
multi-capable agent may serve several steps, keeping the team small. Adding a content
capability usually means adding it to an existing agent (or a new role), not minting a new
SKU per step. Capabilities the user does not opt into (e.g. image generation) are excluded
from the run and require no binding — so an optional capability never blocks configuration.

### 7.3 Platform adaptation is its own step

A neutral `TextDraft` is adapted to a platform shape by a dedicated step
(`adapt-for-linkedin: TextDraft → LinkedInPostDraft`), which a thin publish integration
then consumes. This keeps *write-once, adapt-many* and makes wiring errors schema-time
failures. Do not bundle adaptation into publishers or into writer agents.

### 7.4 Human interaction

Two gates, distinct by meaning: **ElicitationRequest** adds missing context; **ApprovalRequest**
authorizes a risky transition (publishing). Both ride Temporal signals scoped to the
StepExecution/gate. MVP: one human overseer per agent-backed step; publish-approval mode is
per configuration.

### 7.5 Two planning modes — do not conflate

| Mode | Trigger | Structure | LangGraph planner? |
|---|---|---|---|
| **Template plan** | User configures a catalog PlanTemplate | Fixed DAG | **No** — executors pre-assigned at config time |
| **Adaptive plan** | Open-ended free-form goal (legacy path) | Planner decomposes at runtime | **Yes** — 5-node supervisor graph (planner/worker/scorer/router/human_feedback) |

This resolves the apparent ADR-007-vs-ADR-012 conflict: the per-dispatch overseer gate
(Gate 1 agent selection) applies to **adaptive** mode; **template** mode pre-assigns
executors during configuration, so it has no per-dispatch selection gate — only elicitation
and approval at run time. Template plans are the product's spine; adaptive mode is a
retained secondary entry point.

### 7.6 Template granularity: identity is the outcome

A template's identity is the **job-to-be-done**, not a specific cadence/channel/format tuple.
The current example `weekly-newsletter-linkedin` is the anti-pattern: it bakes **three
configuration axes into its name** — cadence ("weekly"), channel ("linkedin"), and format
("newsletter", itself misleading since it publishes a post). Each belongs in configuration:

| Axis | Where it belongs | Notes |
|---|---|---|
| **Cadence** | `PlanSchedule` + `PlanConfiguration.kind` | Never in template identity. The `DateRange` seed should be *derived from the schedule window*, not hardcoded (`last_7_days`). |
| **Format** | A configuration choice (post / carousel / image-backed / later email) | The "rich content" work ([roadmap](mvp-roadmap.md) Phase 3). |
| **Channel** | `SlotBinding` (which channel executor fills the adapt/publish slots) | The structural one — see below. |
| **Tone / audience / theme** | Template `input_parameters` (already so) | Pre-filled by the assistant. |

**Channel as a configuration axis — preserving typed contracts.** Typed drafts
(`LinkedInPostDraft` vs `BlogPostDraft`) are the safety linchpin ([§7.3](#73-platform-adaptation-is-its-own-step))
but make channel *structurally fixed*. The resolution is neither a template-per-channel
(catalog explosion) nor an untyped blob (loses safety), but a **channel-discriminated
artifact family**: a neutral `TextDraft` feeds generic `adapt-to-channel (TextDraft →
ChannelPostDraft)` and `publish-to-channel (ChannelPostDraft → PublishConfirmation)` steps,
where the specific channel is chosen at configuration time by the **SlotBinding** (bind the
LinkedIn adapter+publisher, or Instagram, …) and an **ExecutorRequirement** keeps adapter and
publisher on the same channel. **Multi-channel is fan-out** from one `TextDraft` to several
channel slots, with the user *enabling* the channels they want — not editing the DAG. (This
needs new channel executors + artifact types, so it is post-MVP; the *shape* is what makes
plans generic.)

**Do not over-rotate to a god-template.** Too generic ("do content") loses both
intent-matching (the assistant can't suggest the right template) and typed-contract safety.
Outcome-shaped templates keep matching easy (match on outcome, extract the rest as
parameters) while making cadence/channel/format configurable.

---

## 8. The Resource layer (the shared layer)

**This is the keystone that makes the platform cohere.** The PRD's *camadas compartilhadas*
(shared layers: CRM, knowledge base, catalog, brand profile) and the requests for "upload a
brand kit that agents consume" and "one plan's output becomes another plan's input" are the
**same concept**. ADR-014 already defined the primitive — it just framed it narrowly as
*agent memory*. We generalize it.

**A Resource is a typed, versioned, permissioned knowledge object at tenant/workspace/
brand/user scope.** It is the only layer that crosses plan-run boundaries.

**Two creation paths** (both yield the same governed object; provenance records which):

1. **Governed direct upload/authoring** — the user uploads or authors a Resource directly
   (a brand kit, design system, company/brand profile). This is *not* raw CRUD: the payload
   is validated against the Resource type's schema, versioned, permissioned, Área-gated, and
   audited, with provenance `user-upload`. This is the primary path for branding the user
   feeds the platform.
2. **Promotion from a plan output** — an artifact produced by a plan run is promoted into a
   Resource (e.g. a design-system plan's output becomes a reusable `DesignSystem`), with
   provenance back to the originating PlanExecution/StepExecution/artifact/approver.

Once created (either way), a Resource can be bound two ways:

1. **As agent-readable memory** via `MemoryBinding` — the agent's declared tools may read
   it during a step (e.g. the LinkedIn writer reads the brand voice).
2. **As a plan seed input** via `SeedArtifactBinding` — it is fed into a step as typed
   input (e.g. a design-system Resource seeds a content step).

Both binding paths already exist in the taxonomy; the Resource layer connects them.

```text
Plan A ──produces──> Artifact ──[controlled promotion]──> Resource (versioned, scoped)
                                                              │
                              ┌──────────────MemoryBinding────┤ (agent reads it)
                              └──────────────SeedArtifactBinding┘ (plan consumes it)
                                                              │
                                                     Plan B, Plan C, …
```

**Resource types (all one concept, different schemas):** Brand Kit, Design System,
Company/Brand Profile (PRD **N01**), Knowledge Base (**N11**), Offer Catalog (**N04**),
approved-content examples, audience notes.

**Rules (inherited from ADR-014, now platform-wide):**

- Resources are **tenant-owned, versioned, permissioned, inspectable**. Not hidden agent
  state.
- Promotion is a **controlled workflow with provenance** back to the originating
  PlanExecution/StepExecution/artifact/approver. Approved outputs and elicitation answers do
  **not** silently become Resources.
- Agents access Resources **only through declared tools** (`allowed_tool_ids`) narrowed by a
  resolved `MemoryBinding`. Access is snapshotted and audited (`MemoryUsageRecord`).
- Payloads live in Garage behind the tenant-safe wrapper; Postgres owns metadata,
  permissions, versions, provenance. Indexes/embeddings are derived views, not canonical.
- Áreas gate Resource visibility ([§11](#11-áreas--rbac)).

**One browser, two governed entry points.** The Artifact Library ([§9](#9-navigation--lifecycle))
is the surface for browsing artifacts and managing Resources; **"Upload / Create Resource"**
(direct) and **"Save as Resource"** (promote a plan output) are the two entry points. Both
land in the same versioned, permissioned registry, so cross-plan reuse falls out for free.

---

## 9. Navigation & lifecycle

Four destinations, each with one job (thread-first):

| Surface | Role |
|---|---|
| **Home / Conversations** | Start a chat; resume recent threads. |
| **Gallery** (`/plans`) | Discover PlanTemplates (the catalog). No configuration here. |
| **Runs** | Recurring plans + all executions (in-progress/scheduled/past), grouped by plan, each linking back to its origin conversation. Recurring plans edited inline. |
| **Conversation** (`/chat/[threadId]`) | The single work surface: configure, run, review, preview artifacts for the plan(s) it spawns. |
| **Artifact Library** (`/artifacts`) | Browse/search the artifact stream; promote artifacts to Resources ([§8](#8-the-resource-layer-the-shared-layer)). |

Rules:
- **Thread is the durable top-level object.** A Thread spawns **1:N** PlanConfigurations,
  shown as chips/tabs; the conversation stays continuous below. (This supersedes the old
  1:1 `Thread.active_plan_configuration_id`.)
- **Every plan traces to its origin thread** (`origin_thread_id` always set).
- **Configuration is hybrid conversational**, ending in a **structured approval card**:
  the assistant fills smart defaults from free text, asks ≤2 clarifying questions, and
  presents everything pre-filled for the user to adjust; the plan runs only after explicit
  approval.
- **The canvas is a read-only in-thread toggle**, not a destination.
- **Execution surfaces the final artifact** in a side preview; intermediates stay
  queryable but do not clutter the thread.

> **Naming resolved (C2):** "the chat interface is a PlanConfiguration assistant" is now
> *"a **conversation** is a PlanConfiguration assistant that may produce **several** plans."*

---

## 10. Design system & motion

**Harpy Eclipse** dual-accent token system: gold (`--token-primary`) is brand *identity*;
electric teal (`--token-energy*`) is the *energy* accent for generative/active/running
state; the 5-layer obsidian surface scale is cooled toward a bluer near-black; semantic
status tokens are `--token-status-running/-done/-pending` and `--token-danger`.

**Locked vs governed:** **fonts are LOCKED.** Color/depth values are *governed by the token
system* (Harpy Eclipse unlocked them from the earlier blanket freeze) — change them only
through the token system, never ad hoc. **Motion:** animate opacity/transform/layout only;
gradients are static. Themes via `data-theme` (default/aiuna/tenant-base/brand).

---

## 11. Áreas & RBAC

**Áreas are RBAC entities** — the business areas (Marketing, Sales, …) a user belongs to —
but at MVP they are **membership-based, not a relationship graph**. The authorization model is
deliberately minimal:

- **Tenant isolation → Postgres RLS.** The real security boundary (`current_tenant_id()`); a
  user only ever sees their tenant's data. Non-negotiable, kept.
- **Coarse roles → Zitadel + thin app-level checks.** A small set of roles (owner/admin/member,
  plus who may approve) gates the few sensitive actions: approve a pending action (N07),
  configure/run plans, change autonomy rules (N08), manage integrations/credentials, manage
  billing/credits.
- **Áreas → light membership.** The concept stays first-class in the domain, but fine-grained
  área-scoped enforcement is **deferred** — small tenants (owner + a few people) see their
  tenant's work; área-gating activates when orgs are large enough to need it.
- **Downstream authorization → the system of record.** What Aiuna may do in Odoo (or any other
  downstream) is governed by the scoped credentials on that system's ExecutorInstallation and
  enforced by that system's own permission model — Aiuna does not mirror it.

**OpenFGA/ReBAC is retired** (reversing ADR-004's authorization layer). Zanzibar-style ReBAC
solves nested groups, per-resource sharing, and cross-resource inheritance — none of which the
MVP has — while taxing every feature with dual writes (Postgres + tuples) and an extra service.

**Two authorization axes — spend complexity on the second, not the first:**

- *User authorization* (which human can do what) — simple, handled above.
- *Agent autonomy* (what the AI may do without a human, and whether it can be undone) —
  `PlanBehaviorPolicies`/autonomy rules (N08) + approval (N07) + operation reversibility. This
  is the axis users actually experience and the platform's differentiator; governance
  investment belongs here, not in user RBAC.

**Tripwire to reintroduce ReBAC (enterprise tier):** per-resource sharing ("share this artifact
with these people"), cross-área/workspace inheritance, or an enterprise customer demanding
fine-grained auditable authz — modeled *then* against real patterns, not guessed now.

---

## 12. MVP scope

**Decision: content-depth first.** Of the PRD's 34 MVP plans, the platform today fully
addresses **zero** and partially addresses four (N07, N08, N12, M04) — it is a single
LinkedIn content chain. Building the full Growth Front breadth (CRM, pipeline, proposals,
reporting, multi-channel, image/video) is not feasible before the ~2026-08-22 deadline.

The MVP therefore goes **deep on the Marketing content vertical the platform already lives
in**, on top of the governance layers that already exist:

**In scope (MVP):**
- The **Resource layer** ([§8](#8-the-resource-layer-the-shared-layer)) — brand kit,
  company/brand profile (**N01**), consumed by content agents.
- The **browser surfaces** — recent Conversations + Artifact Library
  ([§9](#9-navigation--lifecycle)).
- **Rich content production** (**M04**, with **M02/M03** partials) — carousels, images,
  stronger elicitation, adversarial review inside the graph, single preview artifact.
- The **governance layers already present** — approval queue (**N07**), agent autonomy
  policies (**N08**), agent action log (**N12**).
- **Phase 0 cleanup** — the [backlog](cleanup-backlog.md), including the latent
  thread-resolver bug.

**Explicitly deferred (post-MVP):** all Sales plans (V02–V08), CRM/customer/history
(N02/N03), offer catalog (N04) as a full plan, opportunity pipeline (N05), leader
dashboard/indicators/reporting (N09/N10/M08/V08), multi-channel publishing beyond LinkedIn,
paid media (M07), strategic-diagnosis agents (M01), scheduling breadth, the full
catalog-authoring service + marketplace, and **the credit/subscription monetization layer**
([§14](#14-commercial-model--credits-subscriptions--budget)).

The sequenced build (Phase 0 → Resource layer → browsers → rich content) with per-phase
executors, artifact types, data models, and OpenSpec changes lives in the
**[MVP Roadmap](mvp-roadmap.md)**.

---

## 13. Authoring the catalog

MVP authoring is **declarative YAML seed files** under
`control-plane/internal/plans/templates/`, `go:embed`-compiled and upserted by key via an
idempotent `EnsurePlanTemplates` startup seeder (mirroring `EnsureCatalog` /
`EnsureAgentTypes`). Schema stays in migrations; template *data* does not. Load-time
validation fails the deploy on DAG cycles, dangling refs, or contract mismatches. Each file
carries an explicit integer `version`; snapshots protect in-flight runs. The end-state — a
full catalog service with CRUD/UI and Áreas RBAC — is deferred.

Adding a genuinely new plan generally requires new **ArtifactTypes** and possibly a new
agent **role** (or new capabilities on an existing role); it rarely requires a new SKU per
step, because agents are multi-capable and tiered (ADR-018) — a small team covers many
steps, and the catalog grows by adding capabilities to agents rather than one SKU per
capability. The roadmap tracks which roles/capabilities/artifact types each phase adds.

Author templates by the **outcome**, keeping cadence/channel/format out of their identity
([§7.6](#76-template-granularity-identity-is-the-outcome)) — including localization keys
(`catalog.plan.<key>.*`) that describe the job, not a one-off cadence/channel.

---

## 14. Commercial model — credits, subscriptions & budget

The platform meters and prices work through **two ledgers**, and this separation is the
whole point:

- **Internal money ledger** *(exists — ADR-010 Budget Policy)* — the truth about *our* cost.
  Money-denominated (`Money(amount_micros)`), per-provider, per-LLM-invocation, with the
  reserve → resolve → record → release flow and a `cost_budget` ceiling. Required for cost
  control, margin analysis, and provider reconciliation. Never removed.
- **External credit ledger** *(new)* — the denomination the **end user** sees. A
  subscription grants a monthly credit allotment; every action (typically per plan step)
  debits credits. Credits convert to money via a **credit rate** that is a *pricing lever*,
  not a pure function of measured cost.

**Why credits, not raw money, for the user.** Provider costs are volatile and USD-denominated
while the product is sold in BRL; credits **decouple the customer-facing price from provider
cost** (the platform absorbs FX and model-price volatility), let us price by **value** rather
than cost-plus (a senior writer step can be worth more credits than its raw tokens; an RSS
fetch can be near-free), and collapse heterogeneous costs (LLM tokens + image generation +
publish API calls + integration calls) into one legible unit.

### 14.1 How it maps onto the existing model

| Concept | Home | Note |
|---|---|---|
| **PriceBook** (SKU/step → credit price) | Executor Catalog (commercial context, §5) | Per-step-different-cost falls out of the SKU's listed credit price. |
| **SubscriptionTier** (entitlements + monthly credit allotment) | Executor Catalog | Two dimensions: *what is unlocked* (entitlements) and *how much you can run* (credits). |
| **CreditWallet** (balance) | Executor Catalog / billing | Source of truth for affordability. |
| **Credit estimate on the approval card** | Plan Management `EvaluatePlan` → chat (§9) | `EvaluatePlan` already produces a cost estimate; denominate it in credits ("~35 credits") — far better UX than "~\$0.42", and shown before approval. |
| **Reserve → debit → release** | Budget capability (§5) | Reuse the existing reservation flow: reserve credits on approval, debit on delivery, **release on failure/retry** so users pay only for delivered value. |
| **`cost_budget` money ceiling** | Budget capability | Kept *underneath* credit pricing as a safety net: a flat-priced step still cannot cost more real money than its ceiling (guards a runaway run). |

### 14.2 Decisions this model commits to

1. **Two ledgers, not one.** Credits are the skin; money is the skeleton. Keep both.
2. **Pricing policy is separate from enforcement mechanism.** PriceBook/SubscriptionTier/
   CreditWallet live in the commercial context; reserve/debit/ceiling live in the Budget
   capability.
3. **Flat per-action credit price** (a step's credit cost is *listed and known before the
   run*) is preferred over metered credit cost, because it makes the approval-card estimate
   trustworthy and shields users from per-run variance. The internal money ceiling absorbs
   the variance risk.
4. **Charge for delivered value.** Platform-error retries and unrun steps release their
   reserved credits.
5. **A tier = entitlements + credit allotment** — the clean commercial packaging.
6. **Snapshot the credit rate** into the PlanExecution at charge time, same discipline as
   every other execution input, so a run is auditable at the rate it was charged.

### 14.3 Open wrinkles (to settle when built)

- **BYO API keys** (ADR-010): if a tenant brings its own LLM key, credits should cover only
  platform value (orchestration, integrations, agents), not the LLM cost they pay directly.
- Free tier / top-ups / rollover of unused monthly credits — commercial policy, not
  architecture.

**Status:** post-MVP **monetization** — behind the content-depth cut ([§12](#12-mvp-scope)).
The hooks already exist (Budget capability, SKU price metadata, the approval-card cost
summary), so no MVP work is blocked. A future **ADR-018** may capture the full rationale;
this section is the canonical model in the meantime.

---

## 15. Superseded-ADR map

The ADRs are retained as dated history. This is the reading key; the full per-ADR
supersession detail is in [`docs/adr/README.md`](../adr/README.md).

| ADR | Subject | Status vs constitution |
|---|---|---|
| 001 | API & communication | **Live** — folded into §6 (+ADR-013 for Python). |
| 002 | Workflow & agents | **Amended** — 3-node graph → 5-node (ADR-007); "task" flow → adaptive mode (§7.5). |
| 003 | Data architecture | **Live** — §6. Task/subtask hierarchy renamed (§4). |
| 004 | Identity & access | **Amended** — Zitadel AuthN + Postgres RLS live; the **OpenFGA/ReBAC authz layer is retired**, replaced by RLS + Zitadel roles + thin app checks (§6, §11). |
| 005 | Dev & delivery | **Live** — §6. |
| 006 | Domain-driven design | **Amended** — "Task Management"→"Plan Management"; five contexts → six (§5). |
| 007 | Agentic patterns | **Amended** — MCP §7 refined by ADR-011; memory refined by ADR-014; gates scoped to adaptive mode (§7.5). |
| 008 | Tenant-safe boundaries | **Live** — §6. |
| 009 | LangGraph state semantics | **Live** — adaptive mode only (§6). |
| 010 | Budget policy service | **Live** — budget = capability in Agent Orchestration (§5); its "five contexts" claim is corrected to six. The **credit layer** (§14) sits on top of its money ledger. |
| 011 | MCP capability gating | **Live** — Agent Orchestration (§5). |
| 012 | Plan-centric task model | **Core, amended** — the spine (§7); 1:1 chat→config superseded by ADR-017 (§9); SQL templates → YAML (ADR-015). |
| 013 | ConnectRPC Python | **Live** — §6. |
| 014 | Agent memory boundary | **Live, generalized** — MemoryResource → **Resource** (§8). |
| 015 | PlanTemplate authoring | **Live** — §13; Áreas RBAC now membership-based, not OpenFGA (§11). |
| 016 | Design token refresh | **Live** — §10; "tokens LOCKED" clarified to fonts-locked/color-governed. |
| 017 | Navigation & lifecycle | **Live** — §9; amends ADR-012's 1:1 assumption. |
| 018 | Capability-based agent teams (roles × tiers) | **Accepted** — §4 (Capability, Seniority Tier, Agent Team; SKU = role × tier), §7.2 (capability-based teams), §13 (catalog grows by capability, not SKU-per-step). Supersedes the one-SKU-per-step model built under `rich-linkedin-content`. |

---

## 16. How to change this document

1. Capture the idea in [`docs/notes/product-ideas.md`](../notes/product-ideas.md).
2. If it is an architectural *decision*, write an ADR in [`docs/adr/`](../adr/).
3. When the ADR is accepted, **fold its outcome into this constitution** and stamp the ADR
   historical (banner + entry in [`docs/adr/README.md`](../adr/README.md)).
4. When ready to build, open an OpenSpec change (`openspec/`).

The constitution always reflects *current* truth. If you find it disagreeing with itself or
with the code, that is a bug in the document — fix it here first.
