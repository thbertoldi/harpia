# Chat-First UX Backlog (post-Milestone-D review)

**Date:** 2026-07-01
**Status:** Backlog — captured from a live review of the Milestone D chat plan-proposal flow.
**North star:** A smooth, simple, and beautiful conversational experience where the user
talks to Harpia, Harpia identifies a plan, helps configure it, runs or schedules it, and
then keeps control, audit, execution history, and edits available through the same mental
model.

**Context:** Milestone D shipped (chat-first home → thread → LLM plan proposal → attach →
existing binding-matrix flow). Design spec: `2026-07-01-milestone-d-chat-plan-proposal-design.md`.
Plan: `../plans/2026-07-01-milestone-d-chat-plan-proposal.md`.

## Already fixed (done)
- Duplicate-proposal race in the chat auto-propose effect.
- Classifier defaults model per provider (platform env-key path).
- `selectOptions` handles `{value,label}` option arrays (was crashing the form).
- Date-range control (`{startDate,endDate}`), integration selector (lists installations),
  and pt-BR/en labels for template input fields (`plans.inputs.<key>.label` flat i18n keys).

## Remaining review items

### North-star operating model — capture from 2026-07-01 review

The desired product feel is not "chat as a launcher for forms." The user should feel they
are conversing with the system, and that the system can:

1. infer relevant PlanTemplates from natural language;
2. recommend and refine configuration choices through chat;
3. create a PlanConfiguration without losing the conversational context;
4. transition naturally into run/schedule/control states;
5. let the user inspect executions, artifacts, audit trail, and configuration;
6. support edits either conversationally or via structured settings, with the same plan
   representation in both places.

This review explicitly rejected flows where a card behaves like a modal that replaces its
own content. The thread should accumulate turns: proposal, user choice, refinement,
confirmation, creation, setup, run/schedule, follow-up questions. If a user clicks
"Anything else?", they stay in the same assistant conversation instead of being sent to the
home screen.

### Transitional model after plan configuration

Open design question to resolve before implementation: once the plan is configured enough
to be RUNNABLE, the assistant should not dump the user into an unrelated surface. Preferred
direction:

- The thread remains the user's home base.
- The assistant emits a post-configuration turn with contextual actions:
  - Run now
  - Schedule
  - Review plan
  - Adjust configuration
  - Ask about another plan
- "Ask about another plan" keeps the user in the same conversation and opens a new
  conversational branch/turn, rather than navigating to `/`.
- The canvas/settings/executions/artifacts/audit surfaces are linked from the conversation
  as control surfaces, not treated as alternate products.

### Plan representation cohesion

Plans are currently represented differently across chat proposal, binding matrix,
configuration/canvas surfaces, lists, and execution detail. This creates a fractured mental
model. A later UX pass must define one shared plan-summary vocabulary and reuse it
everywhere:

- Plan identity: template name, configured title/intent, status.
- Configuration state: executors, overseers, behavior policies, schedule, source groups,
  seed inputs.
- Runtime state: latest execution, next scheduled run, cost/risk/readiness indicators.
- Controls: run, schedule, revise, inspect artifacts, inspect audit.

The same PlanConfiguration should look like the same object whether it was created from
chat, a settings surface, a list, or a canvas. Structured surfaces may be denser than chat,
but they should not introduce different concepts or unavailable controls.

### Conversational refinement before create

Selected direction: **C — conversation + recommendations**.

Before creating the PlanConfiguration, the assistant should ask more questions and propose
defaults:

- Highlight the highest-confidence plan candidate as the best match while keeping
  alternatives visible.
- Recommend themes as selectable chips and allow the user to add custom themes.
- Recommend an audience/persona from the user's request and let the user edit it.
- Recommend topics to avoid, represented as removable chips plus free text.
- Recommend source groups based on the selected themes and allow multiple source groups.
- Explain date range as "the publication window for source articles."
- Use a final confirmation sentence that reads as a complete, natural-language summary
  rather than stitched fragments.

Source groups must be implemented end-to-end, not only as UI state. Because SlotBinding is
one installation per step, multiple source groups should not become duplicate SlotBindings
for `fetch-news`. The implementation must either materialize an aggregate RSS installation
configuration or extend the RSS installation/config path to represent multiple selected
groups while preserving ADR-012's rule that feed URLs/config live in ExecutorInstallation
config, not templates or artifacts.

### Overseer flow follow-up

The user reported that `conversational-overseer` does not appear to work. Initial
investigation suggests the issue may be experiential rather than only state-machine logic:
proposal/configuration still switches route modes and surfaces, so the newly added
`OVERSEER_STEP` can be hidden behind a fragmented transition. Keep investigating with a
real thread before assuming the state machine is wrong.

Concrete debugging leads:

- Verify a created thread receives `ASSISTANT_PROMPT` with `BINDING_STEP`, then
  `OVERSEER_STEP` after all SlotBindings are persisted.
- Verify the chat route reloads into `ConversationalWorkspace` after plan attach and shows
  all plan-scope assistant prompts.
- Verify template `executor_requirement.executor_kind` is present in the PlanTemplate proto
  returned by `GetPlanTemplate`.
- Verify `STEP_REBOUND` after overseer selection triggers `NextTurn`.

### UX redesign (the "smooth & beautiful" pass) — use the UI/UX skill
1. **Conversational confirmation (#5).** Lead the proposal with an assistant sentence —
   "Sounds like you want to plan **X**, is that right?" — with inline confirm/adjust
   affordances (autocomplete-style suggestions), instead of dropping a bare form.
2. **Post-create feedback (#6).** After the plan is attached, tell the user it was created
   and offer clear next actions: **Run now**, **Schedule**, and **"Anything else?"**.
   Today clicking "Create this plan" gives no confirmation beyond the matrix appearing.
3. **Motion / polish (#3).** The UI feels flat and abrupt. Add tasteful transitions and
   micro-animations (message entrance, card expand, state changes) for a smoother feel.
   NOTE: design tokens (colors, fonts) are locked — animate layout/opacity/transform, do
   not change the palette or type.

### Content track (unblocks richer proposals)
4. **More plan templates (#2, #7).** Only one template exists today
   (`weekly-newsletter-linkedin`), so the router can only ever propose that one. Author a
   small library so proposals/shortlists are meaningful.
5. **RSS feed presets (#9).** Add curated `rss-news-feed` source-group presets. Suggested
   starting set (verify each feed URL): Tech/startup (TechCrunch, The Verge, Ars Technica,
   Hacker News), Business (HBR, MIT Sloan, Reuters Business), Marketing/creator (Social
   Media Today, Content Marketing Institute), Brazil/pt-BR (InfoMoney, Exame, Tecnoblog,
   Canaltech, Startupi).

## Flagged follow-ups (from D review)
- `date_range` serialization shape (`{startDate,endDate}`) is assumed — confirm against the
  executor's expected `parameter_values_json` when execution (Milestone E) is tested.
- **Rolling `date_range` presets need execution-time resolution (Milestone E).** The chat
  form now stores a rolling preset (`{"preset":"last_7_days"}`) for the weekly newsletter
  instead of freezing concrete dates, so a scheduled plan covers the previous week on *every*
  run. Two backend gaps remain before execution works end-to-end:
  1. `parameter_values` → seed artifacts / slot bindings / behavior policies is **not**
     expanded anywhere yet (runtime mappings are validated in `template_seed.go` but never
     applied). The generic chat flow sends empty `seed_artifacts`, so `fetch-news` has no
     `DateRange` seed to run against.
  2. When that expansion is built, a `{"preset":"last_7_days"}` value must resolve to the
     previous 7 days **at each run's start**, not at config time. Mirror
     `resolveDateRangePreset` (frontend `template-inputs.ts`) on the backend so both agree.
- Integration selector filtered to RSS feed sources in chat (`sourceGroupInstallations`);
  generalize to the param's required integration type as more source kinds exist.
- Classifier bypasses tenant LLM budget controls — wire budget before heavy production use.
- Generalize `BindingMatrixCard`'s LinkedIn-specific input form to reuse `TemplateInputsForm`.
- `/inbox` should list only tasks (home is now the chat-first entry).

## Key files
- Proposal card: `frontend/src/lib/components/thread/PlanProposalCard.svelte`
- Generic inputs form: `frontend/src/lib/components/thread/TemplateInputsForm.svelte`
- Chat page: `frontend/src/routes/chat/[threadId]/+page.svelte`
- Home (chat-first entry): `frontend/src/routes/+page.svelte`
- Backend proposal RPC: `control-plane/internal/threads/handler.go` (`ProposePlan`)
- Classifier: `control-plane/internal/copilot/`
- i18n: `frontend/src/lib/i18n/{en,pt-BR}.json` (flat dotted keys)

## Working constraints
- Trunk-based; small commits; never add a `Co-Authored-By: Claude` trailer.
- Design tokens (colors/fonts) are locked — do not modify.
- All user-facing copy must have en + pt-BR (flat `translate()` keys).
- Keep Opus off the hot path: implement with a cheaper model (or Cursor from a plan), review on the strong model.
- Verification: `cd control-plane && go test ./...`; `cd proto && buf lint`;
  `cd frontend && bunx vitest run <file>` (NOT `bun test`); `cd frontend && bun run check`
  (12 pre-existing baseline errors in files this work does not touch — introduce no new ones).
- Env: `mise run dev` (single command); DeepSeek key in `deploy/dev/kind/secrets.local.env`.
