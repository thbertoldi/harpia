## 1. Backend — LLM restatement `summary`

- [x] 1.1 In `control-plane/internal/copilot/`, change the classifier result to carry a request-level `Summary string` alongside candidates. Define a `ClassifyResult { Candidates []Candidate; Summary string }` and update the `PlanClassifier.Classify` interface in `classifier.go` to return `(ClassifyResult, error)`. (Pre-v1: change in place, no shim.)
- [x] 1.2 In `control-plane/internal/copilot/llm_classifier.go`, extend the LLM prompt + response schema so the model returns a one-line `summary` restating the user's request **in the same language as the user's message**, and parse it tolerantly (missing/empty → `""`). Update `llm_classifier_test.go` to assert the summary is parsed and that a response without `summary` yields `""`.
- [x] 1.3 In `control-plane/internal/chat` payload builder (`BuildPlanProposedPayload`), add a `summary` field to the `PLAN_PROPOSED` payload JSON. Keep candidates unchanged.
- [x] 1.4 In `control-plane/internal/threads/handler.go` `ProposePlan`, use the new `ClassifyResult` and pass `result.Summary` into `BuildPlanProposedPayload`. Update all `Classify` call sites (incl. tests/mocks) to the new signature.
- [x] 1.5 Add/extend a Go test asserting `ProposePlan` embeds a non-empty `summary` in the payload when the classifier returns one, and omits/empties it gracefully otherwise.
- [x] 1.6 Verify backend: `cd control-plane && go test ./...` passes; `cd proto && buf lint` passes (no proto change expected).

## 2. Frontend — proposal card staging (confirm → form → created)

- [x] 2.1 In `frontend/src/lib/components/thread/PlanProposalCard.svelte`, read `summary` from the parsed payload. Add local `stage = $state<'confirm' | 'form' | 'created'>('confirm')` and a saved-config `status` holder for the created stage.
- [x] 2.2 Implement the **confirm** stage: render `translate("thread.propose.confirm", {summary})` (fallback to the active candidate's `template_name` when `summary` is empty) with a primary "Yes, create it" (`thread.propose.confirmYes`) and secondary "Adjust" (`thread.propose.adjust`).
- [x] 2.3 Add a `requiredInputsSatisfied(params, values)` helper (in `frontend/src/lib/plans/template-inputs.ts`, unit-tested) that returns true when every `required` param has a non-empty value. On "Yes, create it": if satisfied → create immediately (skip form); else → set `stage = 'form'`.
- [x] 2.4 "Adjust" → `stage = 'form'`. When `candidates.length > 1`, render a compact alternative-plan chip row above the form (reuse the existing candidate list, restyled) that re-enters `confirm` for the picked candidate.
- [x] 2.5 Keep the multi-candidate picker and the zero-candidate fallback as the pre-confirm entry states, restyled as chips; picking a candidate enters `confirm`.
- [x] 2.6 Implement the **created** stage: after `savePlanConfigurationRecord`, obtain the authoritative status (re-read via `getPlanConfiguration` or use the returned record). Render a celebration (checkmark + `thread.created.title`) with contextual actions: RUNNABLE → Run now / Schedule / Anything else; otherwise → Finish setup / Schedule / Anything else.
- [x] 2.7 Wire actions: "Run now" → `planClient.createPlanExecution` then `goto` the configured thread; "Finish setup" → `goto` the configured thread; "Schedule" → open `ScheduleDialog` for the created config; "Anything else?" → return to home / focus composer.

## 3. Frontend — motion & polish (opacity/transform only, reduced-motion aware)

- [x] 3.1 In `chat/[threadId]/+page.svelte`, wrap the chat-first message list items (USER_TEXT, PlanProposalCard) with `chatEnter`, applying a 30–50ms per-item stagger delay on the initial batch (cap the delay).
- [x] 3.2 Animate `PlanProposalCard` stage transitions with fade + small `translateY` (no height animation). Add `chipFlash` on confirm/adjust/chip activation and `hoverCardLift` on candidate chips.
- [x] 3.3 Created stage: use `saveCelebration` + a checkmark stroke-draw (mirror the `LandingCard` pattern; extract a shared snippet only if trivial). On failed create, apply `errorShake` and show the error message.
- [x] 3.4 Replace the static "Thinking about a plan…" text with a subtle pulsing/typing indicator (opacity-only), gated by `prefers-reduced-motion`.

## 4. i18n — keys and cleanup

- [x] 4.1 Add flat keys to BOTH `frontend/src/lib/i18n/en.json` and `frontend/src/lib/i18n/pt-BR.json`: `thread.propose.confirm` (`… {summary} …`), `thread.propose.confirmYes`, `thread.propose.adjust`, `thread.propose.otherPlan`, `thread.propose.whichPlan`, `thread.propose.noMatch`, `thread.propose.thinking`, `thread.created.title`, `thread.created.runNow`, `thread.created.schedule`, `thread.created.finishSetup`, `thread.created.anythingElse`.
- [x] 4.2 Replace hardcoded English in `PlanProposalCard.svelte` with keys: "Create this plan", "Creating…", "Loading template…", "Failed to load template.", "Which plan did you mean?", the no-match line, and "Browse templates".
- [x] 4.3 Replace the hardcoded home chat-first hint ("Tell Aiuna what you want to create, then a plan will appear here.") in `chat/[threadId]/+page.svelte` with a keyed string (both locales).

## 5. Verification

- [x] 5.1 Add/extend Vitest for `PlanProposalCard`: stage transitions (confirm→form→created), immediate-create when required inputs satisfied vs. form reveal when missing, contextual action selection by status, summary rendering + fallback, adjust + alternative chips. Run `cd frontend && bunx vitest run src/lib/components/thread/PlanProposalCard.<test>.ts`.
- [x] 5.2 Add Vitest for `requiredInputsSatisfied` in `template-inputs.test.ts`.
- [x] 5.3 Run `cd frontend && bun run check` — no new errors beyond the 12 pre-existing baseline; run `cd control-plane && go test ./...` and `cd proto && buf lint`.
- [x] 5.4 Manual smoke via `mise run dev`: send a request on home → confirm sentence appears → confirm/adjust → create → celebration with contextual actions; verify pt-BR and en, and reduced-motion. (Verified manually by the maintainer.)
