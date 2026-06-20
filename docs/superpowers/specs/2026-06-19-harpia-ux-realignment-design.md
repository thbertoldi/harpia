# Harpia UX Realignment — Design Spec

**Date:** 2026-06-19
**Status:** Approved (brainstorm), awaiting plan
**Originating conversation:** UX re-evaluation prompted by drift between implementation and PRD/ADR intent.
**Mockups:** `.superpowers/brainstorm/3640967-*/content/{home-layout,configuration-flow,expanded-canvas,unified-inbox}.html`

---

## 1. Why this exists

The current frontend (SvelteKit, see `frontend/src/routes/`) has drifted away from the design intent captured in:

- `docs/notes/prd-harpia-2026-06-12/prd.md` — single user persona (Ana), chat-assisted configuration, à la carte executor SKUs, behavior policies as configuration.
- `docs/adr/ADR-012` — Plan = composite task / DAG; ElicitationRequest and ApprovalRequest are subtypes of the same "Human Interaction" event; PlanConfiguration assistant is a chat surface.
- `docs/notes/prd-harpia-2026-06-12/prd-decision-log.md` — overseer per task, à la carte SKUs, timeout policies user-selectable.

Three concrete drifts the re-design corrects:

1. **Oversight fragmented across three sidebar routes** (`/oversee`, `/elicitations`, `/approvals`). ADR-012 §6 says they share the Human Interaction transport — they're subtypes of one notion ("AI is waiting on the Overseer"), not separate surfaces. Today: three lists, three badges, three load patterns, identical purpose.
2. **Configuration is a four-step pure-form wizard** (`/plans/[id]/configure/{slot-binding,overseer,policies,summary}`). ADR-012 §10 specifies a *PlanConfiguration assistant* — chat that walks intent → template → slots → overseer → policies → cost → save. The chat assistant does not exist in the current frontend; the wizard form is the *fallback*, not the primary surface that was designed.
3. **Persona inflation.** Dev login exposes `Leader / Overseer / Engineer`. The PRD defines one end-user (Ana, who *is* the Overseer) and an implicit Platform Engineer who owns admin surfaces. `Leader` and `Engineer` as end-user personas have no PRD anchor.

This spec realigns the frontend with the original intent, sharpened by additional decisions captured in the brainstorm.

---

## 2. Vocabulary (canonical)

These are the only nouns the product surfaces use. They map directly to backend bounded contexts.

| Noun | Definition |
|---|---|
| **Task** | Atomic unit. An (input, output) contract — a schema for what comes in and what comes out. Abstract; says nothing about who fulfills it. |
| **Plan** | Composite task. A DAG of Tasks. Branching is allowed. |
| **Agent** | A graph-shaped executor with autonomy. Can elicit clarifying questions during execution. Has a seniority tier (junior/senior). Internally a graph; externally a candidate to bind to a Task. |
| **Integration** | A deterministic executor bound to a fixed schema. No elicitation. |
| **Executor** | Umbrella term for Agent or Integration. A Task is fulfilled by one Executor. |
| **PlanConfiguration** | Ana's configured instance of a Plan: each Task bound to a chosen Executor, with Overseers and behavior policies set. Schedulable. |
| **Execution** | A running instance of a PlanConfiguration. |
| **Overseer** | The party that answers elicitations from a Task. Per-Task. v1: human only (defaults to Ana). v2: can be another Agent. |
| **Elicitation** | An agent's question to its Overseer, raised mid-execution. |
| **Approval** | A gate event raised before a publish step. Binary (approve/reject). |
| **Feedback** | A free-form asynchronous note the Overseer leaves on a step's output. |
| **Artifact** | Typed output produced by a Task. May be stored in our system or be a confirmation of an external side-effect. |

"Step", "slot", "agent role", "approval request", "elicitation request" — retire as user-facing terms. Internal types may retain them for protocol compatibility but UI copy uses the table above.

---

## 3. Personas

Two personas. Not three.

### 3.1 Ana — Operator / Overseer
The PRD's primary persona. A solo micro-entrepreneur or consultant in the creator economy. Configures plans, watches executions, answers elicitations and approvals. Does not provision integrations or pick which agents her tenant owns.

**Sees in sidebar:** `Needs you`, `Your plans`, `Discover`, `+ New plan`. Nothing else.

### 3.2 Platform Engineer — Inventory keeper
Provisions integrations and agents that her tenant owns, configures MCP servers, manages tenant settings, audits activity. Does not configure or run plans in this role.

**Sees in sidebar:** `Integrations`, `Agents`, `MCP servers`, `Tenant settings`, `Audit log`.

### 3.3 How personas relate

Personas are **sidebar surfaces**, not RBAC roles. A single user (the v1 GTM target is single-seat tenants) wears both hats and toggles between surfaces via the user menu. In larger tenants, separate users get one role.

Dev login collapses to these two:
- `Ana` — `seat:operator` permission
- `Platform Engineer` — `seat:admin` + the existing `manage*` permissions

Delete: `Leader`, the existing `Engineer` persona (replaced by Platform Engineer with the same permission set), the existing `Overseer` persona (folded into Ana).

---

## 4. Architecture decisions

### 4.1 Chat is the primary surface

Plans are configured through conversation with a PlanConfiguration assistant. The assistant matches intent to a template, walks Ana through binding each Task to an Executor, sets Overseer and behavior policies inline, surfaces a confirmation summary with cost, and persists the PlanConfiguration.

**Out of scope (per PRD §4.2):** natural-language-only configuration without a confirmation summary. The assistant always shows a typed summary before save.

### 4.2 One thread per Plan

Each Plan owns one durable chat thread. The thread begins with the configuration conversation. Subsequent Executions appear as collapsible sections inline. Pending Elicitations/Approvals from the active Execution pin to the bottom of that Plan's thread.

Threading model rejected: per-Execution threads (would fragment continuity), one mega-thread (would force "which plan is this about?" tagging).

### 4.3 DAG is embedded in the thread (with expand-to-canvas)

The Plan's DAG renders as an inline card inside the chat thread. During configuration the card shows binding state; during execution it shows runtime state. A subtle `Expand ⤢` affordance opens the full-bleed canvas route.

Rejected: persistent right pane (desktop-only; loses mobile), canvas-as-primary with chat sidebar (overshoots v1, contradicts chat-as-primary).

### 4.4 Composability is baked into v1's grammar; the action unlocks in v2

v1 lets Ana fill the predefined Tasks of a Plan template with chosen Executors. v2 will let her compose a Plan from scratch by wiring Tasks together. The v1 UI commits to vocabulary that makes v2 a natural extension:

- Tasks are visible chips/cards with their input/output **contract types displayed** ("NewsList → Text").
- Edges in the DAG carry the type ("NewsList") as a label.
- The Executor candidate list is implicitly filtered to executors whose schema satisfies the Task's contract.
- The canvas uses a positioning model that supports drag-rearrange later (no greyed-out future affordances in v1 — no expectation debt).

When v2 ships, the canvas gains drag handles, the `+` between nodes appears, and the assistant gains a "compose from scratch" turn type. No new surface introduced.

### 4.5 Overseer is a per-Task field on the Task card

The dedicated `/plans/[id]/configure/overseer` page dies. Overseer is set inline on each Task card during configuration (default: "You"). v1 only allows human Overseers; v2 will allow agent Overseers from the same picker.

### 4.6 Cost is surfaced in the topbar pill, not on a dedicated summary page

`/plans/[id]/configure/summary` dies. The topbar of the plan thread (during configuration and during execution) shows a live cost pill that updates as bindings change.

### 4.7 Marketplace is deferred

v1 has no in-product purchase flow. Platform Engineers provision Integrations and Agents that the tenant owns; Ana picks among what her tenant owns. Per-Executor pricing is already visible on candidate cards (R$ X / run), so the grammar for billing is in place — when the store ships, it slots in without re-language.

---

## 5. Surfaces (the four screens)

Visual mockups for each are in `.superpowers/brainstorm/3640967-*/content/`.

### 5.1 Unified inbox — `/inbox`
**Replaces:** `/oversee`, `/elicitations`, `/approvals`.

Cross-plan aggregator of pending Human Interaction events. Rows show subtype pill (Elicit / Approve / Feedback), source breadcrumb (`PlanName · TaskName`), the ask, age, and a primary action sized to the subtype:
- **Approvals** get inline `Preview / Reject / Approve` buttons (binary; Ana doesn't leave the inbox).
- **Elicitations** and **Feedback** get an `Open thread` button (free-form answers belong in chat).

Filter chips at the top (`All / Elicitations / Approvals / Feedback`). A dimmed "Earlier today" section at the bottom shows recently-completed items for audit-by-glance.

Sidebar item: a single `Needs you` entry with the total badge.

### 5.2 Plan thread — `/plans/[id]`
**Replaces:** `/plans/[id]/configure`, `/plans/[id]/configure/overseer`, `/plans/[id]/configure/summary`, and serves as the home for both configuration and execution observation of a single Plan.

A chat thread. Configuration messages live at the top of history; Executions render as collapsible sections; pending Human Interaction cards pin to the bottom. Composer always at the foot. Quick-action chips under assistant messages let Ana advance without typing.

Inline embedded objects:
- **DAG card** — small graph of the Plan, with node-state styling (●bound / ⬡binding / ⊘unbound during config; ✓done / ● running / ★ waiting / ⊘ pending during execution). `Expand ⤢` opens the canvas.
- **Task card** — appears during configuration when the assistant walks through binding a Task. Shows name, contract, candidate list (Junior / Senior / specialized agents with prices and tier badges), overseer row.
- **Elicitation card** — Appears when an Agent asks a question. Form embedded inline; Ana answers without leaving the thread.
- **Approval card** — Binary; same inline pattern.
- **System note** — Run started / Run completed / Run failed markers, with timestamps.

Top bar: Plan name + schedule meta + live cost pill.

### 5.3 Expanded canvas — `/plans/[id]/canvas?run=X`
**Replaces:** `/plans/executions/[id]`.

Full-bleed graph view of one Execution (or of the unbound Plan template if no `run` query param). Each node carries: Task name, contract, bound Executor (icon + name + tier), Overseer, state badge. Edges are labeled with the type that rides them.

Right-side detail pane (340px). Selected node fills it: Executor details, runtime stats, inline Elicitation/Approval form, links to input/output Artifacts. The detail pane is **fixed-position**; nodes don't open popovers.

Top bar: `← Back to thread` (returns to the plan thread), Plan name + run timestamp, action buttons (`Run history`, `Schedule`, `Settings`, primary `Answer ★ N`).

Subtle dotted-grid background signals "canvas." No drag affordances in v1.

`Run history` opens a drawer listing past Executions of this Plan; clicking one navigates to its canvas. `Settings` opens a drawer with behavior policies, schedule, and a Plan-level default Overseer that pre-fills any per-Task Overseer field left unset (per-Task remains the source of truth, per §4.5). `Schedule` opens a focused dialog for cron/recurrence config.

### 5.4 Discover / Templates — `/discover` (repurpose of `/plans`)
A gallery of available Plan templates, browsable by category. Each template shows its Task chain visually (using the same Task-as-chip vocabulary as the rest of the product). "Use this" launches a new plan thread pre-loaded with the template — the assistant greets Ana already inside it. This surface answers "what can the assistant do for me" without forcing Ana to start with a blank composer.

### 5.5 New plan — `/new`
Empty-state chat. The assistant opens with "What would you like to automate?" plus 3-4 suggested template cards inline. As Ana types or picks one, the conversation transitions into the standard plan thread (and gets persisted as a new Plan).

---

## 6. Route mapping (full disposition table)

| Current route | Disposition | Notes |
|---|---|---|
| `/` | **Redirect** | Landing logic: see §7 |
| `/oversee` | **Delete** | Folded into `/inbox` |
| `/elicitations` | **Delete** | Folded into `/inbox` |
| `/approvals` | **Delete** | Folded into `/inbox` |
| `/plans` | **Repurpose** → `/discover` | Templates gallery |
| `/plans/[id]` | **Repurpose** | Becomes plan thread (chat home) |
| `/plans/[id]/configure` | **Delete** | Chat handles it |
| `/plans/[id]/configure/overseer` | **Delete** | Overseer is per-Task field on Task card |
| `/plans/[id]/configure/policies` | **Delete as route** | Becomes Settings drawer over canvas |
| `/plans/[id]/configure/summary` | **Delete** | Cost is in topbar pill |
| `/plans/executions` | **Delete** | Per-plan `Run history` lives in canvas topbar drawer |
| `/plans/executions/[id]` | **Repurpose** → `/plans/[id]/canvas?run=X` | Expanded canvas |
| `/plans/executions/[id]/elicitations/[id]` | **Keep as deep-link target only** | Notifications/emails open it; canvas detail pane is the primary surface |
| `/plans/executions/[id]/approvals/[id]` | **Keep as deep-link target only** | Same |
| `/tasks`, `/tasks/ongoing` | **Delete** | The "Tasks" route family was a placeholder; replaced by Plan threads |
| `/integrations` | **Move** → `/admin/integrations` | Platform Engineer surface |
| `/agents` | **Move** → `/admin/agents` | Platform Engineer surface |
| `/audit` | **Move** → `/admin/audit` | Platform Engineer surface |
| `/settings` | **Move** → `/admin/settings` | Platform Engineer surface |

**New routes:** `/inbox`, `/new`, `/plans/[id]/canvas`, `/discover`, `/admin/*`.

**Transitional redirects:** for one release cycle, deleted routes serve a 302 to their new home (e.g., `/oversee` → `/inbox`, `/plans/executions/[id]` → `/plans/[id]/canvas?run=[id]`). After one release, return 404.

---

## 7. Landing logic at `/`

Order of precedence:
1. If `Needs you` count > 0 → `/inbox`.
2. Else if Ana has any Plans → most-recently-active Plan thread (`/plans/[id]`).
3. Else → `/new`.

Platform Engineer mode bypasses this: lands at `/admin/integrations` if no items pending operator attention, else at the relevant admin view.

---

## 8. Backend implications (sketch — not implementation)

Backend changes the frontend implies. None require new bounded contexts; they refine existing ones.

- **PlanConfiguration assistant** is a new bounded context (or a service inside the existing Plan BC). Owns the chat protocol: intent matching, template suggestion, candidate enumeration, contract-aware filtering, cost computation, save. Communicates with the frontend over the existing Connect/Protobuf RPC layer.
- **Per-plan thread persistence.** A thread is identified by Plan ID. Thread messages include user/assistant/system text plus structured embeds (DAG-snapshot, task-binding-card, elicitation-card, approval-card). The protobuf message type is an oneof carrying the embed payload.
- **Inbox aggregator** is a new read-model that fans out across all Plans owned by the user and surfaces pending Human Interaction events. Likely a Temporal query or a materialized view; no new write path.
- **Hexagonal boundary preserved.** The chat surface is an adapter on top of the existing PlanConfiguration domain — it does not own configuration semantics. Behavior policies, executor binding, schedule, overseer assignment continue to live in the Plan/Execution BCs unchanged.

Domain rule unchanged: an Executor binding is valid iff its declared output schema satisfies the Task's contract. The chat assistant enforces this by filtering candidates pre-display; the backend re-validates on save. Schema mismatch is unrepresentable in the UI.

---

## 9. v1 scope vs v2 deferred

### v1 (in this spec)
- Two personas (Ana, Platform Engineer)
- Chat-driven configuration with template matching, slot-fill, candidate picker
- Per-plan thread with inline DAG / Task / Elicitation / Approval cards
- Unified inbox replacing the three oversight routes
- Expanded canvas replacing the executions detail
- Persona-aware sidebar
- Admin routes moved under `/admin/*`
- Deletion of seven legacy routes + transitional redirects

### v2 (deferred, but unblocked by v1's grammar)
- Drag-to-compose Plans from scratch on the canvas
- Agent-as-Overseer (Overseer picker shows agents)
- Marketplace / store for Agents and Integrations
- Multi-overseer chains and delegation
- Branching DAG editor with conditional edges
- Cross-tenant Plan template sharing

The v1 design must not foreclose any v2 item. Specifically: Task vocabulary on every surface, contract types on every node and edge, candidate-list interface for executor binding, canvas with a positioning model that supports drag-rearrange.

---

## 10. Implementation milestones

This spec is too large for a single implementation plan. The work decomposes into six milestones, each independently shippable behind a feature flag (`uxRealignment.{milestone}`). Each milestone produces its own implementation plan.

1. **M1 — Foundation cleanup.** New routes scaffolded (`/inbox`, `/plans/[id]`, `/plans/[id]/canvas`, `/admin/*`, `/new`, `/discover`) with stubs. Sidebar restructured behind a persona-mode flag. No legacy routes deleted yet.
2. **M2 — Unified inbox.** `/inbox` implemented with cross-plan aggregator. Inline approve/reject for binary approvals. "Open thread" links non-functional pending M3 (gracefully degrade to existing detail pages until then). Delete `/oversee`, `/elicitations` (list), `/approvals` (list); add transitional redirects.
3. **M3 — Plan thread.** Chat thread per Plan rendered. Existing PlanConfiguration data displayed (read-only at first). Elicitation/Approval cards inline. "Open thread" from inbox now works. DAG card embedded.
4. **M4 — Expanded canvas.** `/plans/[id]/canvas?run=X` route. Full DAG render with state. Detail pane with inline forms. Run history drawer. Settings drawer. Delete `/plans/executions` list.
5. **M5 — Chat as configuration surface.** Chat assistant goes from passive display to active configuration: intent → template match → task-by-task binding → policies → save. Delete `/plans/[id]/configure/*`.
6. **M6 — Persona/admin split & cleanup.** Move admin routes under `/admin/*`. Persona toggle in user menu. Dev-login persona collapse. Remove transitional redirects. Final IA in place.

M1-M2 are user-visible quickly (the inbox alone removes three sidebar items and a real source of cognitive load). M3-M5 are the bulk of new product. M6 is polish/IA cleanup.

Each milestone's spec lives at `docs/superpowers/specs/<date>-harpia-ux-realignment-m<N>-<topic>.md`.

---

## 11. Out of scope for this spec

- Backend domain model changes (the existing Plan / PlanConfiguration / Execution / HumanInteraction BCs are sufficient).
- Telemetry / analytics events.
- Internationalization beyond keeping the existing i18n prefix structure (`nav.*` → updated to new vocabulary).
- Mobile-specific layouts beyond ensuring the chosen designs are mobile-tractable (single-column reduction).
- Pricing/billing surfaces beyond the per-run cost pill.
- Onboarding/welcome flow polish — `/new`'s suggested templates are a starting point, not the final shape.

---

## 12. Open questions

None blocking. The brainstorm resolved every shaping decision. Items that will surface during implementation but don't need pre-design:

- Exact protobuf shape for the chat-message oneof.
- Whether Templates Discovery is searchable from day one or only browsable by category.
- Whether the Settings drawer is per-Plan or per-PlanConfiguration (likely per-Configuration; resolved during M4).
