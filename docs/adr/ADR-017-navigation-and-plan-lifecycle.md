# ADR-017: Navigation & Plan-Lifecycle Model (thread-spawned plans)

**Status:** Accepted
**Date:** 2026-07-03
**Deciders:** thbertoldi

**References:**
- ADR-012: Plan-Centric Task Model (amended here — the 1:1 thread↔configuration assumption)
- ADR-015: PlanTemplate Authoring Model (the catalog = content)
- AGENTS.md: "the chat interface is a PlanConfiguration assistant"

## Context

The product was feeling incoherent. A plan was represented across **three inconsistent
surfaces** — the chat thread, a standalone plan-configuration route, and a full-screen
canvas — none of which clearly owned the lifecycle. The thread↔configuration link was
hard-coded **1:1** (`Thread.active_plan_configuration_id`), so a conversation could not
produce more than one plan. There was no first-class notion of *what kind* of plan it is
(one-shot vs. recurring), no single place to find running/scheduled/past executions, and
no shared vocabulary to even discuss it. Configuration was card-only with raw IDs, and the
composer was a dead "leave a note" box. The result: buttons that navigated to the wrong
place, an invisible Run affordance, and a journey that felt broken despite working plumbing.

This ADR captures the model the team converged on to make the journey coherent.

## Decision

### 1. Four navigation destinations, each with one job

| Surface | Role |
|---|---|
| **Home / Conversations** | Start a chat (prompt) and resume recent threads. |
| **Gallery** (`/plans`) | Discover `PlanTemplate`s (the catalog). Configuration does **not** happen here. |
| **Runs** (new top-level destination) | Recurring plans + **all** executions (in-progress, scheduled, past), grouped by plan, each linking back to its origin conversation. |
| **Conversation** (`/chat/[threadId]`) | The single work surface: configure, execute, review, and preview artifacts for the plan(s) it spawns. |

### 2. A conversation can spawn one or more plans (1:N)

`Thread 1—N PlanConfiguration`. The current single `Thread.active_plan_configuration_id`
is superseded by a collection. Within a thread, the plan(s) it spawned are shown as
**chips/tabs at the top**; selecting one focuses that plan (its configuration + executions)
while the conversation stays continuous below.

### 3. A plan has a type that drives the execution flow

`PlanConfiguration.kind ∈ { ONE_SHOT, RECURRING }` (first-class field):
- **ONE_SHOT** → "run now?" (no schedule).
- **RECURRING** → "run a test execution before scheduling?" then schedule.

### 4. Every plan traces to its origin conversation

`PlanConfiguration.origin_thread_id` is always set. **Every executed plan came from some
chat.** The Runs panel links each plan/execution back to its origin thread; resuming the
thread is always one click away.

### 5. Configuration is hybrid conversational, ending in a structured approval card

The composer is a real conversational input (not a notes box). The assistant fills smart
defaults from the user's free text, asks at most 1–2 clarifying questions where it is
unsure, and then presents a **structured approval card** with everything pre-filled
(recommended sources, schedule, tone/audience/themes — the user's to adjust; executor SKU
and overseer auto-decided, overridable). The plan only runs after explicit approval.

### 6. The canvas is a read-only, in-thread toggle — not a destination

The DAG graph remains useful for understanding flow, so it stays as an optional read-only
toggle **inside** the conversation. The standalone canvas route and the standalone
plan-configuration route are removed; no button navigates away from the thread to "see" or
"adjust" a plan.

### 7. Execution surfaces the **final** artifact, not every intermediate one

When a plan runs, progress appears as a message in the conversation. The **final output**
opens in a side preview (split, closable) so the user can continue the conversation.
Intermediate artifacts are not promoted into the UI — they remain queryable but do not
clutter the experience.

### 8. Recurring plans are edited in the Runs panel

Light edits and cancellation of a recurring plan (sources, schedule, tone) happen inline
in the Runs panel. Structural changes reopen the origin conversation.

## Rationale

- **One mental model, not three.** Collapsing the thread/plan/canvas trichotomy into
  "the conversation owns the plan" removes the navigation whiplash that made the journey
  feel broken.
- **Kind drives flow.** Making ONE_SHOT vs. RECURRING a first-class property lets the flow
  ask the right question ("run now?" vs. "test then schedule?") instead of always treating
  schedule as an afterthought.
- **Traceability.** `origin_thread_id` guarantees every executed plan is grounded in the
  conversation that produced it — essential for an audit-forward ops platform.
- **Final-artifact focus.** Most intermediate artifacts are plumbing; what users care about
  is the output, so that is what the UI promotes.

### Alternatives Considered

| Alternative | Why not |
|---|---|
| Keep 1:1 thread↔plan (one thread, one plan) | Simpler, but forces users to start a new chat for every plan and loses the "iterate on several ideas in one conversation" feel. |
| Keep the canvas as a primary editor | The conversational model + approval card already covers editing; a separate visual editor reintroduces the surface fragmentation this ADR removes. |
| Flat execution timeline in Runs | Rejected in favor of grouped-by-plan: a recurring plan's history is meaningless without its plan context. |
| Pure free-text config (no approval card) | Too unreliable pre-v1; the structured approval card keeps the user in control and the materialization auditable. |

## Consequences

### What becomes easier
- A coherent journey: Home → Gallery (discover) → Conversation (configure/run/review) →
  Runs (manage recurring + history).
- Clear vocabulary: Conversation, Plan (ONE_SHOT/RECURRING), Execution, Gallery, Runs.
- A defensible answer to "where do I find/run/edit my plan" for every plan kind.

### What becomes harder / what we must change
- **Data model:** `Thread.active_plan_configuration_id` (1:1) → a 1:N relationship; add
  `PlanConfiguration.kind` and `origin_thread_id`. Migration of the existing single
  template onto the new shape (pre-v1, no compat burden).
- **ADR-012 is amended:** the 1:1 assumption and the "chat is a PlanConfiguration
  assistant (singular)" framing become "a conversation is a PlanConfiguration assistant
  that may produce several plans."
- **New Runs surface** must be built (frontend route + grouping by plan + execution states
  + edit-recurring-inline + origin-thread links).
- **Conversational configuration (§5) is the largest and most uncertain build** — it needs
  an LLM materialization path with the approval card as the reliability backstop.

### Implementation status (phased)
1. **Thread coherence** — remove navigate-away buttons; in-thread canvas toggle; friendly
   names on cards (no raw IDs); visible Run. (Fast, visible.)
2. **Plan kind + lifecycle** — `ONE_SHOT`/`RECURRING`, origin_thread_id, the two run flows.
3. **Multi-plan thread** — 1:N, chips/tabs at top.
4. **Hybrid conversational configuration** — free-text → defaults + ≤2 clarifying Qs →
   approval card. (Largest rock.)
5. **Runs panel** — grouped by plan, execution states, edit-recurring inline, origin links.
6. **Final-artifact side preview** — suppress intermediate clutter.
7. **Navigation shuffle + surface cuts** — Home/Gallery/Runs/Conversation; remove the
   standalone plan-config route and the standalone canvas route.

### Out of scope (for now)
- Cross-thread plan merging or moving a plan between conversations.
- Marketplace / partner-authored recurring plans.
- Real-time collaborative editing of a conversation.
