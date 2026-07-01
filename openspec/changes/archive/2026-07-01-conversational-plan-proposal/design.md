## Context

Milestone D shipped the chat-first flow: home → thread → `ProposePlan` (DeepSeek classifier) → `PLAN_PROPOSED` → `PlanProposalCard` → inputs form → "Create this plan" → config attached → binding-matrix / configured-plan flow. The configured-plan side already has rich conversational infrastructure: a `planassistant` backend state machine driving `ASSISTANT_PROMPT` messages, a `LandingCard` (checkmark + Run now / Schedule / walk-away, wired to `createPlanExecution`), a `BindingMatrixCard`, and a tokenized, reduced-motion-aware motion library (`$lib/motion/transitions`: `chatEnter`, `chipFlash`, `saveCelebration`, `errorShake`, `hoverCardLift`).

The chat-first *proposal* side is the flat part: raw `USER_TEXT` divs plus a `PlanProposalCard` that drops straight into a bare form, no assistant framing, no motion, and no creation confirmation (after create it navigates and the binding matrix simply appears).

Constraints: design tokens (colors/fonts) are locked; all copy needs en + pt-BR flat `translate()` keys; no pre-v1 compat burden; keep the expensive model off the implementation hot path (delegate to a cheaper model / Cursor); verify with vitest + `bun run check` + `go test ./...` + `buf lint`.

## Goals / Non-Goals

**Goals:**
- Lead the proposal with a warm LLM restatement + confirm/adjust, revealing the form only when needed (progressive disclosure).
- Confirm creation in place with contextual next actions read from real config status.
- Bring the chat-first surfaces up to the motion polish the configured side already has.
- One narrow backend touch (a `summary` string), no proto change.

**Non-Goals:**
- Persisting the confirmation/celebration as backend chat messages or folding the chat-first flow into the `planassistant` state machine (rejected as Approach B — too large for the value).
- Auto-binding or promoting DRAFT plans to RUNNABLE (out of scope; the Run-now branch simply lights up when a plan is created already runnable).
- New plan templates and RSS presets (separate content-track backlog items).

## Decisions

- **Client-side staging over backend messages.** `PlanProposalCard` holds a `stage` (`confirm | form | created`) in local state. Chosen over new message kinds / a state machine because the acknowledgement is transient and durable state already lives in the config + configured-side thread. Smallest change, frontend-heavy (delegatable), reversible.
- **`summary` in free-form payload, not proto.** `PLAN_PROPOSED.payloadJson` is parsed with `JSON.parse`, so `summary` is added to the JSON built by `chat.BuildPlanProposedPayload` — no `.proto`/`buf` change. `copilot.Classify` returns `{Candidates, Summary}` (pre-v1 breaking) so the summary rides the existing single LLM call rather than adding a second call.
- **Summary language = user's message language.** The summary is dynamic content and cannot be a `translate()` key; the classifier is instructed to write it in the user's language. The scaffolding sentence is a `translate()` key with a `{summary}` param. Fallback to template display name when summary is absent.
- **Immediate-create when complete.** On confirm, if all `required` input params already have values (seeded from `input_values_json`), create directly; otherwise reveal the form to collect the gaps. `TemplateInputsForm` exposes / the card computes a "required satisfied" check.
- **Contextual actions from authoritative status.** After `savePlanConfigurationRecord`, re-read the config (or use its returned status) and branch: RUNNABLE → Run now/Schedule/Anything else; else → Finish setup/Schedule/Anything else. Reuses `createPlanExecution`, `ScheduleDialog`, and `LandingCard`'s celebration visuals (extract the checkmark/celebration into a shared piece if cheap, else mirror the pattern).
- **Motion via existing primitives only.** `chatEnter` (message entrance, staggered 30–50ms on the initial batch), fade + small `translateY` for stage transitions (no height animation → no CLS), `chipFlash`/`hoverCardLift` on chips/buttons, `saveCelebration` + checkmark stroke-draw for the created stage, `errorShake` on failure. All primitives already honor `prefers-reduced-motion`.

## Risks / Trade-offs

- **Run-now branch rarely shows today** → Accepted and documented: the only current template saves as DRAFT needing binding, so users usually see Finish setup. The branch is built correctly and lights up automatically as templates/auto-binding evolve. Better than a Run-now button that errors.
- **Transient celebration lost on reload** → Acceptable: reloading resolves the thread's attached config and hands off to the durable configured-side view; the ephemeral card is only a момentary acknowledgement.
- **Classifier response schema change could break parsing** → Mitigate with a tolerant parser (missing/empty `summary` → fallback) and a Go test asserting both shapes.
- **Locale drift for the summary** → If the model returns the wrong language, the sentence still reads naturally around the `{summary}`; not correctness-critical.
- **Height/layout shift during stage transitions** → Mitigate by animating opacity/transform only and letting content reflow instantly, per the locked-token rule.

## Migration Plan

No data migration. Pre-v1, so `copilot.Classify`'s signature changes in place with callers updated in the same change; no shim. Rollback is reverting the change (frontend staging + the classifier field are self-contained).

## Open Questions

- None blocking. "Adjust" with a single template currently just opens the form; the alternative-plan chip row becomes meaningful once the content-track adds more templates.
