## Context

The chat thread already carries everything needed to show live execution progress. The
`ThreadMessage` kinds relevant here (`chat.proto:155-180`):

- `RUN_STARTED` (4) / `RUN_COMPLETED` (5) / `RUN_FAILED` (6) — whole-execution lifecycle.
- `STEP_STARTED` (12) — payload `{ step_key, step_execution_id }` → a step is running.
- `STEP_BOUND` (7) — per the proto comment, *"StepExecution completed (output artifact
  produced)"* → a step is **done**. Despite the legacy name, the runtime emits this as the
  per-step completion signal.
- `ARTIFACT_CREATED` (22) / `ARTIFACT_UPDATED` (23) — produced by a completing step.

`buildThreadSections` (`frontend/src/lib/plans/thread.ts:21-67`) already groups messages into
plan-scope vs per-`executionId` sections, so the grouping primitive exists. Today these
events render only as `SystemEventCard` lines (`ThreadMessage.svelte:101-102`), and the only
"working" affordance is a CSS pulse labeled "Thinking about a plan…" (`+page.svelte:348-355`).

So the gap is purely a **presentation** gap: no component turns the grouped event stream into
a legible live tracker. No backend or proto change is required.

## Goals / Non-Goals

**Goals:**
- Render one `PlanExecutionCard` per execution section, showing each `PlanStep`'s live status
  (`pending → running → done`, or `failed`) with a progress bar and connector timeline.
- Surface a global "an execution is running" state in the chat header (status pill).
- Replace the ad-hoc thinking pulse with motion-safe typing dots + an energy generating
  skeleton.
- Keep the change **frontend-only**; reuse the existing event stream and `executionId`
  grouping.

**Non-Goals:**
- Artifact previewing (a separate change: `artifact-preview-panel`).
- Adding a new `STEP_COMPLETED` message kind — `STEP_BOUND` already serves as completion.
- Token streaming for assistant free text (`ASSISTANT_TEXT` stays reserved/unused).
- The proposal/binding/policy conversational cards (already shipped) — this card appears only
  once a run has started (`RUN_STARTED`).
- Per-step elapsed-time/duration display (no timing data on the events today).

## Decisions

1. **The card is a view model over the event stream, not a new message kind.**

   Add a pure function `buildExecutionViewModel(messages, executionId)` (near
   `frontend/src/lib/plans/thread.ts`) that returns, for the plan template steps in order,
   each step's status plus execution-level state (`idle | running | completed | failed`), the
   running step's detail, and `done/total`. The card is rendered in the configured-thread
   branch from the section's messages — it does not consume a slot in `ThreadMessage.svelte`'s
   dispatch.

   Alternative considered: emit a dedicated `EXECUTION_PROGRESS` message kind from the
   backend. Rejected: the data already exists in the stream; a new kind duplicates it and adds
   backend work for no information gain.

2. **Step-status folding rules (deterministic, no new events).**

   For a given `executionId`, process its events in `sequence_number` order:
   - start every step at `pending`;
   - `STEP_STARTED{step_key}` → that step `running`;
   - `STEP_BOUND{step_key}` (or the next step's `STEP_STARTED`, or `RUN_COMPLETED`) → prior
     running step `done`;
   - `RUN_FAILED` → the currently-running step `failed`, execution `failed`;
   - `RUN_COMPLETED` → any still-`running` step forced to `done`, execution `completed`.

   This needs the card to know the template's ordered step list (keys + titles + details). The
   step list is already available on the attached `PlanConfiguration`/template (the same source
   the DAG mini-map uses). Step detail/title copy resolves via existing
   `catalog.plan.<key>.step.<step_key>.*` content keys with template-label fallback.

   Alternative considered: show only started steps, not the full template. Rejected: a
   progress bar needs the denominator (`done/total`), which requires the full step list.

3. **Header status pill derives from "any section running".**

   A small `$derived` over the section view models: if any execution section is `running`, the
   pill shows the energy accent + "Executing…", otherwise a neutral "Ready". Wired into the
   existing chat header in `+page.svelte`.

   Alternative considered: a global MCP/agent-health badge. Out of scope; this is purely
   per-thread execution state.

4. **Typing + generating affordances are motion-safe and token-driven.**

   Replace the "Thinking about a plan…" pulse with a typing-dots indicator (animate
   `transform`/`opacity` only, behind the existing reduced-motion guard in `app.css`). The
   energy generating skeleton (from ADR-016) appears inside the card while a step is
   `running`. No color/animation.

   Alternative considered: keep the existing pulse. Rejected: it does not convey step-level
   progress and is inconsistent with the energy-accent system.

5. **Localization and copy.**

   All card labels (title fallback, "Executing…", "Ready", "Plan completed", "Step failed",
  "running" chip, `done/total` aria-labels) live in both `frontend/src/lib/i18n/en.json` and
   `pt-BR.json` under flat keys (e.g. `thread.execution.*`).

## Risks / Trade-offs

- **Stale "running" between `STEP_STARTED` and `STEP_BOUND`.** If events arrive out of order
  or a step emits no `STEP_BOUND`, a step could appear stuck `running`. Mitigation: fold in
  `sequence_number` order (authoritative), and force-settle on `RUN_COMPLETED`/`RUN_FAILED`;
  add tests for late/out-of-order arrival.
- **Multi-execution threads.** A thread can contain several executions over time. Mitigation:
  one card per `executionId` section (already grouped); only the latest is expanded by
  default, older ones collapsed to their final state.
- **Template step list availability.** If the step list cannot be resolved for an execution,
  the card degrades to a simple "Executing…/Completed/Failed" summary without per-step rows
  rather than failing.
- **Failure semantics.** `RUN_FAILED` marks only the running step `failed`; earlier steps stay
  `done`. This matches "the run failed at step N" intent; document it in the card's failure
  copy.

## Migration Plan

Pre-v1; forward-only, frontend-only:
1. Implement the view-model builder + unit tests.
2. Implement `PlanExecutionCard` + header pill + typing/generating affordances + i18n.
3. Wire into the configured-thread branch of `+page.svelte`.
4. Run `cd frontend && bun run lint`, `bun run check` (no new baseline errors), and
   `bunx vitest run` for the new tests.
