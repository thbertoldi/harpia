# ADR-012: Plan-Centric Task Model

> **⚠ Historical record — not the current source of truth.** The canonical description of the
> platform is the **[Platform Constitution](../architecture/harpia-platform.md)**, which
> supersedes ADR-001…017 as the reading order. See the
> **[ADR supersession map](README.md)** for how this ADR stands today. Where this ADR and the
> constitution disagree, the constitution wins.

**Status:** Accepted
**Date:** 2026-06-12
**Deciders:** thbertoldi

**References:**
- Schlup, M. (University of Liverpool) — atomic vs composite task taxonomy
- CommonKADS — atomic task as `(input, output)` contract
- ADR-002: Temporal + LangGraph Boundary
- ADR-006: Domain-Driven Design for Agentic Architecture
- ADR-007: Agentic Architecture Patterns for Harpia
- ADR-008: Tenant-Safe Boundaries for Generic Infrastructure
- ADR-009: LangGraph State Semantics
- ADR-010: Budget Policy Service
- ADR-011: MCP Capability Gating and Out-of-Process Workers
- ADR-013: ConnectRPC Python Adoption
- ADR-014: Agent Memory Boundary

## Context

Harpia's current domain model (ADR-006) treats a **Task** as the top-level work unit and **Subtasks** as dynamically decomposed pieces produced by the LangGraph planner at runtime. That model fits open-ended requests ("analyze my sales data") but does not cleanly express what customers actually buy and configure: **repeatable workflows** with assignable slots, mixed executors (deterministic integrations and autonomous agents), and schedulable runs.

A conceptual framework drawn from Schlup and CommonKADS offers a clearer foundation:

- An **atomic task** is an abstract contract: `(input, output)` — the executor is irrelevant to the contract.
- A **composite task** is a **plan**: a DAG of atomic tasks.
- There is **no 1:1 relationship** between an agent and a task; the orchestrator assigns each slot to either a deterministic integration or an agent graph.
- The chat interface configures a plan (template + slot assignments + overseer bindings); execution produces a **plan run** that walks the DAG.

This ADR formalizes that model inside Harpia's existing stack (Temporal, LangGraph, ConnectRPC, OpenFGA) and resolves terminology collisions with ADR-006.

## Decision

### 1. Ubiquitous Language

We adopt the following terms. Legacy `harpia.tasks.v1` is deprecated immediately; new work uses `harpia.plans.v1` for template-plan features. Adaptive free-form execution remains on the legacy path during migration and is not forced into the new model until template plans are stable.

| Term | Definition | Replaces (legacy) |
|---|---|---|
| **PlanTemplate** | Reusable DAG definition in the catalog. Contains ordered **PlanSteps** and dependency edges. Sold/configured as a product offering. | — (new) |
| **PlanStep** | One atomic slot in a PlanTemplate. Defined by `(input_schema, output_schema)` and human-readable metadata. | "atomic task" in the framework; partially maps to **Subtask** |
| **PlanConfiguration** | A tenant-specific binding: which **ExecutorInstallation** fills each slot, plus **OverseerBindings**, **PlanBehaviorPolicies**, seed inputs, and optional schedule. Created via chat UI or API. Snapshot when a run starts. | — (new) |
| **PlanConfigurationStatus** | Configuration lifecycle: `DRAFT`, `RUNNABLE`, `SCHEDULED`, `DISABLED`, `ARCHIVED`. Drafts may be incomplete; runnable/scheduled configurations must pass all invariants. | — (new) |
| **PlanBehaviorPolicies** | Runtime behavior selected by the tenant, including elicitation timeout behavior and publish approval mode. | — (new) |
| **PlanExecution** | One run of a configured plan. Walks the DAG, materializing **StepExecutions**. | Legacy **Task** (top-level run) |
| **StepExecution** | One firing of a PlanStep during a PlanExecution. Holds runtime input/output artifact refs and status. | Legacy **Subtask** (runtime instance) |
| **Executor** | Anything that satisfies a PlanStep contract. Two kinds: **AgentExecutor** (manifest-declared LangGraph graph) and **IntegrationExecutor** (deterministic activity). | Legacy `assigned_agent_id` only |
| **ExecutorSKU** | Global commercial product unit for an agent or integration, with pricing and compatibility metadata. | — (new) |
| **ExecutorEntitlement** | Tenant grant for an ExecutorSKU. Says the tenant may use the SKU, but not whether it is connected/configured. | — (new) |
| **ExecutorInstallation** | Tenant-scoped executable instance of an entitled SKU. For integrations, carries connection/config state; for agents, points to an agent manifest version and tenant policy. Slot bindings reference installations, not global SKUs. | — (new) |
| **ExecutorRequirement** | Optional PlanStep metadata describing required executor kind, capabilities, connection type, and contract constraints used to filter compatible installations. | — (new) |
| **Artifact** | Typed payload produced or consumed by a step. Stored in Garage; referenced by schema ID. | Unnamed blobs / inline JSON today |
| **ArtifactType** | Registered `(schema_ref, version)` describing a reusable I/O shape. | — (new) |
| **Overseer** | Human or delegated party who receives async elicitation from agent-backed steps. Required when an AgentExecutor is assigned. | Partially covered by Human Interaction BC |
| **OverseerBinding** | Per-step assignment of who the overseer is (user, team member, or future: delegated agent). | — (new) |
| **ElicitationRequest** | Agent-authored question for missing context during an agent-backed step. | FeedbackRequest |
| **ApprovalRequest** | Human gate for a risky transition, such as publishing a `LinkedInPostDraft`. Distinct from elicitation even if it reuses Human Interaction transport. | FeedbackRequest |
| **PlanSchedule** | Cron or event trigger that creates PlanExecutions from a PlanConfiguration. | — (new) |

**Composite task = PlanTemplate. Atomic task = PlanStep.** We stop using "task" for both levels in new documentation and APIs.

### 2. Core Entity Model

```
PlanTemplate
  ├── id, name, description, vertical (e.g. creator-economy)
  ├── steps: PlanStep[]
  └── edges: PlanStepDependency[]   // DAG

PlanStep
  ├── id, key (e.g. "fetch-news")
  ├── title, description
  ├── input_artifact_type_id
  ├── output_artifact_type_id
  ├── executor_requirement: ExecutorRequirement | null
  └── optional: default_executor_sku_key

PlanConfiguration
  ├── id, plan_template_id, plan_template_version, tenant_id, workspace_id
  ├── status: DRAFT | RUNNABLE | SCHEDULED | DISABLED | ARCHIVED
  ├── seed_artifacts: SeedArtifactBinding[]   // step_key/input_name → artifact or literal
  ├── slot_bindings: SlotBinding[]            // step_key → executor installation
  ├── overseer_bindings: OverseerBinding[]
  ├── behavior_policies: PlanBehaviorPolicies
  └── schedule: PlanSchedule | null

SlotBinding
  ├── step_key
  ├── executor_kind: AGENT | INTEGRATION
  ├── executor_sku_id
  └── executor_installation_id

PlanExecution
  ├── id, plan_configuration_id, plan_configuration_snapshot
  ├── status
  ├── step_executions: StepExecution[]
  └── triggered_at, completed_at

StepExecution
  ├── id, plan_step_key, status
  ├── input_artifact_id, output_artifact_id
  ├── executor_installation_snapshot
  ├── attempt
  ├── elicitation_thread_id (if agent-backed)
  └── approval_request_id (if publish approval or other gate applies)
```

**Invariants:**

- A `DRAFT` PlanConfiguration may be incomplete and user-saveable. A `RUNNABLE` or `SCHEDULED` PlanConfiguration is valid only if every PlanStep has a compatible SlotBinding, every agent-backed step has an OverseerBinding, required seed inputs exist, all executor entitlements are present, and required integration installations are connected.
- StepExecutions for steps with unsatisfied upstream dependencies cannot start until predecessor outputs exist.
- Output of step N must validate against `output_artifact_type` before step N+1 receives it as input.
- A PlanExecution uses a snapshot of the PlanConfiguration, PlanTemplate version, behavior policies, and executor installations so future configuration edits do not mutate runs in flight.
- SlotBinding compatibility is structural first: artifact contracts and executor requirements must match. Natural-language capability matching is only a suggestion mechanism.

### 3. Executor Abstraction (Ports & Adapters)

**Critical clarification:** Every PlanStep — deterministic or agent-backed — is orchestrated by **Temporal**. LangGraph does not replace Temporal. LangGraph is the **reasoning engine inside** agent-backed steps.

| Layer | Technology | Question it answers |
|---|---|---|
| Plan orchestration | **Temporal** (`PlanWorkflow`) | In what order do steps run? What if a process crashes? How do we retry, timeout, schedule, and pause for humans? |
| Deterministic step body | **Temporal activity** (Go) | Call this API with this typed input; return this typed output. |
| Agent step body | **Temporal activity** (Go) → gRPC → **LangGraph** (Python) | Given this typed input, reason, use tools, maybe ask the overseer, produce typed output. |

```text
PlanWorkflow (Temporal)
  │
  ├── StepExecution: fetch-news
  │     └── Activity: RunIntegration(installation_id, input_artifact)
  │           └── Go worker → RSS feeds → NewsList
  │
  ├── StepExecution: adapt-for-linkedin
  │     └── Activity: RunAgent(installation_id, input_artifact)
  │           └── Go worker → ConnectRPC → agent-runtime → LangGraph graph
  │                 ├── returns LinkedInPostDraft, or
  │                 └── returns ElicitationRequested
  │                       PlanWorkflow awaits Temporal signal and launches a continuation activity
  │
  └── StepExecution: publish-linkedin
        └── Activity: RunIntegration(installation_id, input_artifact)
              └── Go worker → LinkedIn API → PublishConfirmation
```

The plan engine invokes steps through a single port:

```text
ExecuteStep(step_execution_id, input_artifact) →
  StepCompleted(output_artifact) |
  ElicitationRequested(thread_id, question) |
  StepFailed(error)
```

Both executor kinds are **Temporal activities** registered on the same `PlanWorkflow`. The difference is what happens *inside* the activity:

| Executor kind | Activity implementation | Reasoning? | Elicitation? |
|---|---|---|---|
| **IntegrationExecutor** | Go control-plane worker calls external API | No | No |
| **AgentExecutor** | Go activity calls agent-runtime via ConnectRPC; Python runs a LangGraph graph | Yes | Yes (activity returns elicitation state; workflow waits for signal) |

Both executors **must** return output conforming to the PlanStep's `output_artifact_type`. Temporal handles durability for both. LangGraph handles autonomy for agent steps only. Agent activities must not block for hours waiting on humans; they return an elicitation state to the workflow, the workflow awaits the overseer signal, and a new or continued activity resumes the agent with the response.

**Agent = manifest-declared graph + SKU + tenant installation.** An AgentExecutor is backed by the canonical agent manifest (`id`, `version`, `input_schema`, `output_schema`, `allowed_tool_ids`, `model_id`, `cost_estimate`, metadata) introduced on trunk. The SKU sells a versioned capability; the tenant installation determines whether that SKU is entitled and enabled for a tenant. Senior and junior variants are separate agent manifest IDs/SKUs with distinct graphs, models, pricing, and capability declarations — not a runtime tier flag on one type.

**Integration = deterministic contract satisfier.** Declares `input_artifact_types[]`, `output_artifact_types[]`, required connection kind, configuration schema, and failure classes. No LangGraph involvement. No elicitation; failures surface as StepExecution errors with Temporal retry policy.

### 4. Artifact Type Registry

Typed I/O is the linchpin. Natural-language capability matching (ADR-002) remains useful for **suggestions**, but execution requires schema validation.

**New bounded context: Artifact Types** — a first-class supporting context with its own proto package (`harpia.artifacts.v1`), storage conventions, and validation rules.

MVP artifact schemas use JSON Schema as the canonical validation format because agent manifests already declare JSON Schema input/output contracts. Protobuf remains the service-contract and codegen mechanism. ArtifactType records may later point at protobuf message FQNs for payloads that need strongly generated language types, but the first implementation must not mix validation semantics per artifact.

Initial artifact types for the creator-economy vertical:

| ArtifactType | Purpose | Example producer | Example consumer |
|---|---|---|---|
| `DateRange` | Temporal window for fetches | User input / config | Fetch News step |
| `NewsList` | Curated articles | Fetch News (integration or agent) | Write Newsletter |
| `TextDraft` | Platform-neutral prose (source material) | Write Newsletter (agent) | Adapt-for-platform steps |
| `LinkedInPostDraft` | LinkedIn-specific text (tone, length, hooks) | Adapt-for-LinkedIn (agent) | Publish LinkedIn |
| `BlogPostDraft` | Blog-specific text (SEO, headings, length) | Adapt-for-Blog (agent) | Publish Blog |
| `ImageAsset` | Generated or uploaded image | Design agent (optional branch) | Publish steps |
| `PublishConfirmation` | External post receipt | Publish integrations | — |

PlanConfiguration also stores **seed input bindings**. For the Weekly Newsletter MVP, `DateRange` can be generated from the schedule/run trigger, while RSS feed URLs belong to the RSS integration installation or per-configuration integration settings, not to the `NewsList` artifact.

**Platform adaptation is a separate PlanStep, not a hidden concern of the publisher.**

A `TextDraft` is intentionally platform-neutral. LinkedIn posts and blog posts differ in tone, length, structure, and formatting. That transformation is its own atomic contract:

```text
adapt-for-linkedin:  TextDraft → LinkedInPostDraft
adapt-for-blog:      TextDraft → BlogPostDraft
publish-linkedin:    LinkedInPostDraft → PublishConfirmation
publish-blog:        BlogPostDraft → PublishConfirmation
```

Why not bundle adaptation into the publish step?

| Approach | Problem |
|---|---|
| Publish integration adapts internally | Violates single responsibility; integration should only publish typed input; hard to test adaptation separately |
| Publish agent adapts + posts | Couples reasoning to one platform; can't reuse the same writer across a branched multi-platform plan |
| Platform-specific writer agents skip adaptation | Duplicates research/synthesis logic across N writers instead of write-once, adapt-many |

The recommended pattern for multi-platform content:

```text
fetch-news → write-draft → TextDraft
                              ├→ adapt-for-linkedin → LinkedInPostDraft → publish-linkedin
                              └→ adapt-for-blog     → BlogPostDraft     → publish-blog
```

Each adaptation step is assignable to a platform-specialized agent SKU (e.g. `linkedin-voice-senior`, `blog-seo-junior`). Publish steps remain thin integrations that accept only the platform-specific artifact type — wiring errors become compile-time/schema-time failures, not runtime surprises.

Artifact payloads live in Garage behind the tenant-safe object-store wrapper from ADR-008. The control plane stores metadata + schema ref + content hash. Proto messages reference artifacts by ID; schema definitions live in `harpia.artifacts.v1` registry records and are represented as JSON Schema for MVP.

```protobuf
// Illustrative — not yet implemented
message ArtifactType {
  string id = 1;
  string key = 2;              // e.g. "harpia.artifacts.v1.NewsList"
  string schema_ref = 3;       // JSON Schema URI or registry key for MVP
  int32 version = 4;
}

message Artifact {
  string id = 1;
  string artifact_type_id = 2;
  string storage_uri = 3;      // Garage S3 URI
  string content_hash = 4;
  string created_at = 5;
}

message PlanStep {
  string key = 1;
  string input_artifact_type_id = 2;
  string output_artifact_type_id = 3;
}
```

### 5. Two Planning Modes

Harpia supports **both** template plans and adaptive plans. They must not be conflated.

| Mode | Trigger | Structure | Planner involved? |
|---|---|---|---|
| **Template plan** | User selects catalog PlanTemplate; chat/API configures slots | Fixed DAG from template | **No** LangGraph planner |
| **Adaptive plan** | User submits open-ended goal (legacy "Create Task") | LangGraph planner decomposes at runtime | **Yes** — ADR-002/007 flow |

**Template plan execution flow:**

```text
1. User selects PlanTemplate "Weekly Newsletter (LinkedIn)" in chat UI
2. User assigns executors: Fetch News → RSS News Feed integration,
   Write Draft → Senior Writer agent, Adapt for LinkedIn → LinkedIn Voice agent,
   Publish → LinkedIn integration
3. User assigns self as Overseer for the agent-backed steps
4. User selects PlanBehaviorPolicies:
   elicitation timeout handling and publish approval mode
5. PlanConfiguration saved; optional PlanSchedule (Mon 08:00)
6. PlanSchedule fires → PlanExecution created from a configuration snapshot
7. Temporal PlanWorkflow walks DAG:
   a. StepExecution: fetch-news → IntegrationExecutor
   b. StepExecution: write-draft → AgentExecutor (may elicit via Overseer)
   c. StepExecution: adapt-for-linkedin → AgentExecutor
   d. Publish approval gate if configured
   e. StepExecution: publish-linkedin → IntegrationExecutor
8. UI streams StepExecution status via Connect server-stream
```

**Adaptive plan execution flow** (unchanged from ADR-002):

```text
1. User submits free-form goal
2. Temporal AdaptiveTaskWorkflow → LangGraph planner → dynamic subtasks
3. Overseer gates per ADR-007 (agent selection + output review)
4. Results may optionally be "saved as PlanTemplate" (future — not MVP)
```

Migration rule: template plans use `harpia.plans.v1` from the start. Adaptive free-form requests remain on the legacy task/subtask API until the template path is stable. A future ADR may expose adaptive runs through `PlanExecution` with `plan_template_id = null`, but this ADR does not require that migration for MVP.

### 6. Overseer and Async Elicitation

When an AgentExecutor lacks context, it emits **ElicitationRequests** on the step's elicitation thread. The configured Overseer receives them asynchronously (MVP: in-app; post-MVP: Slack/email via Human Interaction BC).

Publish approval is modeled separately as an **ApprovalRequest**. It may reuse the same Human Interaction transport and Temporal signal plumbing, but it is not an agent question. The event meaning is different: elicitation adds missing context; approval authorizes a risky transition such as publishing externally.

**MVP constraints** (per product alignment):

- One OverseerBinding per agent-backed step
- Overseer must be a human user in the tenant (no re-delegation to another agent in MVP)
- `RUNNABLE`/`SCHEDULED` PlanConfiguration validation **fails** if an agent-backed step lacks an overseer
- Publish approval mode is per PlanConfiguration in MVP and applies to all publish-type steps

**Post-MVP:** Overseer may delegate to another user or agent; elicitation becomes a nested assignment graph.

Elicitation and approval integrate with Temporal via signals on the PlanWorkflow (same pattern as ADR-007 Gate 2, scoped per StepExecution or gate request).

### 7. Temporal Mapping

| Domain concept | Temporal concept |
|---|---|
| PlanTemplate DAG | Workflow type definition (`PlanWorkflow`) + static step graph |
| PlanConfiguration | Workflow input payload (slot bindings, artifact seed data) |
| PlanExecution | Workflow run |
| StepExecution | Activity invocation (+ optional child workflow for long agent runs) |
| PlanSchedule | Temporal Schedule or cron-triggered starter |
| Elicitation pause | Agent activity returns `ElicitationRequested`; workflow await + signal (`ElicitationResponse`); continuation activity resumes |
| Publish approval pause | Workflow await + signal (`ApprovalDecision`) before publish activity |
| Adaptive plan | Existing `AdaptiveTaskWorkflow` (rename in migration) |

Template plans use a **single Temporal workflow** that iterates the DAG in topological order. Parallel steps with no dependency edge may run as concurrent activities. Agent-backed steps that elicit return control to the workflow; the workflow waits using durable Temporal state and resumes through a continuation activity after the overseer signal.

### 8. Commercial Packaging

Marketplace SKUs align to this model:

| SKU unit | Maps to |
|---|---|
| PlanTemplate | Catalog workflow (may be free or premium) |
| ExecutorSKU | Commercial product unit with list price and compatibility metadata |
| ExecutorEntitlement | Tenant grant to use a SKU |
| ExecutorInstallation | Tenant-scoped configured executable instance |
| IntegrationExecutor | Deterministic installation with connection/config state (OAuth, feed URLs, rate limits) |
| AgentExecutor | Manifest-backed installation (e.g. `newsletter-writer-senior`, `linkedin-voice-senior`) |
| PlanExecution | Metered run (optional billing dimension) |

A tenant's **PlanConfiguration** is the concrete bill of materials: N integration installations + M distinct agent installations, each backed by an entitled SKU. Cost estimates use SKU price metadata; execution uses installation snapshots.

### 9. Context Map Update

```
┌─────────────────┐     Partnership      ┌──────────────────┐
│  Plan Management │◄──────────────────►│     Agent        │
│  (renamed from   │                     │  Orchestration   │
│  Task Mgmt)      │                     └────────┬─────────┘
└────────┬─────────┘                              │
         │                                        │
         │ Conformist                      Open-Host Service
         ▼                                        ▼
┌─────────────────┐                     ┌──────────────────┐
│  Artifact Types │                     │   Workflow       │
│  (new)          │                     │   Engine         │
└─────────────────┘                     └────────┬─────────┘
         ▲                                       │
         │                              Partnership
         │                                       ▼
         │                              ┌──────────────────┐
         └──────────────────────────────│     Human        │
                                        │  Interaction     │
                                        └──────────────────┘
```

**Plan Management** absorbs template catalog, configuration, execution, and scheduling. **Artifact Types** is a supporting context consumed by Plan Management, Agent Orchestration, and Integration executors.

Marketplace concerns are not folded into Plan Management. ExecutorSKU, entitlement, and installation APIs form a commercial/access-control supporting context that Plan Management queries when validating slot bindings.

### 10. Chat UI Role

The chat interface is a **PlanConfiguration assistant**, not a generic task creator:

1. Discover intent → suggest PlanTemplate
2. Walk slot assignment ("Who should fetch the news?")
3. Confirm overseer for agent slots
4. Configure behavior policies, schedule, and review cost estimate
5. Save PlanConfiguration → trigger or schedule PlanExecution

Adaptive (free-form) requests remain available but are a separate entry point.

## Rationale

### Why contract-first steps?

Separating **what** (PlanStep contract) from **who** (ExecutorInstallation) lets the same newsletter-writing step run on a junior agent, senior agent, or — if we ever support it — a human freelancer portal, without changing the DAG or downstream steps. This is the CommonKADS insight applied to a multi-tenant agent marketplace.

### Why break early on proto naming?

The plan-centric model is a fundamental domain shift, not a cosmetic rename. Introducing `harpia.plans.v1` and `harpia.artifacts.v1` now prevents half-migrated mental models in proto, UI, and docs. `harpia.tasks.v1` is deprecated for new template-plan work; adaptive mode remains on its compatibility path until the template path is proven.

### Alternatives Considered

| Alternative | Why not |
|---|---|
| Keep Task/Subtask only; templates as metadata | Collapses catalog product and runtime instance; executor assignment stays agent-only |
| LangGraph planner for all plans including templates | Wastes tokens; non-deterministic structure for workflows customers expect to be stable |
| Protobuf-only artifact schemas | Fights the existing agent manifest contract, which already uses JSON Schema for input/output validation |
| One overseer per plan (not per step) | Fails agency model where different steps need different domain experts |
| Tier parameter on one agent type | Hides pricing/capability differences; separate SKUs are clearer for marketplace and billing |
| Bundle platform adaptation into publish step | Couples concerns; prevents branching one draft to multiple platforms |
| SlotBinding points directly at global SKU | Cannot represent OAuth/feed configuration, per-tenant enablement, or version snapshots cleanly |

## Consequences

### What Becomes Easier

- **Product clarity** — customers buy plans + executors, not abstract "tasks"
- **Mixed execution** — integrations and agents interoperate through artifact contracts
- **Scheduling** — PlanSchedule maps naturally to Temporal Schedules
- **Validation** — invalid configurations caught at save time, not mid-run
- **Vertical packaging** — creator-economy plans ship as catalog items with typed I/O
- **Commercial clarity** — SKU, entitlement, and installation each answer one question: what is sold, who owns it, and what can actually run

### What Becomes Harder

- **Schema governance** — artifact types must be versioned carefully; breaking changes ripple across templates
- **Two planning modes** — engineering must maintain template DAG engine and adaptive LangGraph planner
- **Migration** — legacy Task/Subtask API consumers need a deprecation window
- **Configuration UX** — slot assignment + overseer binding adds steps before first run
- **Snapshot discipline** — plan executions must capture configuration, installation, and policy versions for audit/retry correctness

### Implementation Status

1. ~~Accept ADR~~ — Accepted 2026-06-12
2. ~~Publish ADR and README index entry~~
3. ~~Create `proto/harpia/plans/v1/plans.proto`~~ — PlanTemplate, PlanStep, PlanConfiguration, PlanBehaviorPolicies, PlanExecution, StepExecution
4. ~~Create `proto/harpia/artifacts/v1/artifacts.proto`~~ — ArtifactType, Artifact, preview payloads, and MVP schema metadata
5. ~~Create `proto/harpia/executors/v1/executors.proto`~~ — ExecutorSKU, ExecutorEntitlement, ExecutorInstallation, compatibility/price metadata
6. Decide whether `proto/harpia/integrations/v1/integrations.proto` is needed after integration-specific connection/config APIs outgrow the executor package
7. Continue deprecating `proto/harpia/tasks/v1/tasks.proto` for new template-plan UI/workflow code paths; keep adaptive compatibility
8. ~~Add database migrations~~ — `plan_templates`, `plan_configurations`, `plan_executions`, `step_executions`, `artifact_types`, `artifacts`, `executor_skus`, `executor_entitlements`, `executor_installations`
9. Implement `PlanWorkflow` Temporal worker — all steps as activities; agent steps delegate to agent-runtime and return elicitation state instead of blocking on humans inside the activity
10. Extend chat UI for PlanConfiguration flow (template pick → slot assign → overseer → behavior policies → schedule)
11. Seed first vertical template: **Weekly Newsletter (LinkedIn)** with RSS fetch and adaptation step (see Appendix A)
12. Update ADR-006 ubiquitous language table with migration notes

### Out of Scope (Post-MVP)

- Save adaptive decomposition as new PlanTemplate
- Overseer re-delegation to agents
- Cross-tenant plan template marketplace
- Automatic executor suggestion without overseer approval (conflicts with ADR-007 human gates for adaptive mode; template mode uses pre-assigned executors)
- Per-step publish approval policy; MVP publish mode applies to all publish-type steps in the configuration

## Appendix A: Example — Weekly Newsletter (LinkedIn MVP)

```text
PlanTemplate: weekly-newsletter-linkedin
  Step fetch-news:        DateRange         → NewsList
  Step write-draft:       NewsList          → TextDraft
  Step adapt-for-linkedin: TextDraft        → LinkedInPostDraft
  Step publish-linkedin:  LinkedInPostDraft  → PublishConfirmation

DAG edges:
  fetch-news → write-draft
  write-draft → adapt-for-linkedin → publish-linkedin

PlanConfiguration (tenant acme):
  status             → RUNNABLE
  behavior_policies  → elicitation=PAUSE_UNTIL_ANSWERED/48h,
                        publish=REQUIRE_APPROVAL
  seed input         → DateRange derived from schedule window
  fetch-news         → installation:rss-news-feed
                        config: feed_urls=[...]
  write-draft        → agent:newsletter-writer-senior, overseer=alice@acme
  adapt-for-linkedin → agent:linkedin-voice-senior, overseer=alice@acme
  publish-linkedin   → integration:linkedin

PlanSchedule: cron "0 8 * * 1" (Mondays 08:00 UTC)
```

Post-MVP multi-platform variant branches after `write-draft`:

```text
write-draft → adapt-for-linkedin → publish-linkedin
            └→ adapt-for-blog     → publish-blog
```

## Appendix B: Legacy Mapping

| Legacy (ADR-006) | New model |
|---|---|
| `CreateTask` (free-form) | Compatibility path for adaptive task execution; future migration may create `PlanExecution` without template |
| `Task` | `PlanExecution` for template-plan runs |
| `Subtask` | `StepExecution` for template-plan runs |
| `assigned_agent_id` | `SlotBinding.executor_installation_id` (agent or integration) |
| LangGraph `planner` node | Adaptive mode only |
| `TaskStatus.PLANNING` | `PlanExecutionStatus.CONFIGURING` or adaptive planning phase |

## Appendix C: Executor Stack (FAQ)

**Q: Does LangGraph replace Temporal for agent steps?**
No. Temporal orchestrates every step. LangGraph runs inside the agent-runtime when a Temporal activity invokes `RunAgent`.

**Q: Why involve Temporal at all for agent steps?**
Because agents can run for minutes, crash mid-run, need retries, and may request overseer input. Temporal provides durable state and signal handling. LangGraph provides reasoning within one bounded activity invocation or continuation.

**Q: Can an integration ever call LangGraph?**
No. Integrations are deterministic activities. If reasoning is needed, the PlanStep contract should be assigned to an AgentExecutor.

**Q: Can an agent publish directly to LinkedIn?**
Technically yes (agent with LinkedIn MCP tool), but for template plans we prefer: agent produces `LinkedInPostDraft`, integration publishes it. Keeps publish logic testable, retryable, and OAuth-managed in one place.

**Q: Why not let a Temporal activity wait for the user's answer?**
Human response time is unbounded. The activity should return an explicit elicitation state, and the workflow should wait durably for a signal. This keeps worker slots free, keeps retries clear, and makes timeout policy enforceable in one place.
