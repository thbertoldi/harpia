## Why

The chat-first plan-proposal flow works but feels abrupt: it drops a bare inputs form on the user with no conversational framing, gives no confirmation after a plan is created, and has none of the motion polish the configured-plan side already enjoys. The north star is a smooth, simple, beautiful conversation that turns a natural-language request into a plan.

## What Changes

- **Conversational confirmation.** Before showing the inputs form, the proposal leads with an assistant restatement — "Sounds like you want to create **X** — is that right?" — with **Yes, create it** / **Adjust** affordances. On confirm, if all required inputs are pre-filled the plan is created immediately; otherwise the form expands to just what's missing. **Adjust** opens the full form and, when multiple candidates exist, a compact alternative-plan chip row.
- **Post-create feedback.** On successful creation the card celebrates in place (checkmark + "Plan created!") and offers **contextual** next actions read from the saved config's status: runnable → Run now / Schedule / Anything else; needs binding → Finish setup / Schedule / Anything else.
- **Motion & polish.** Message entrance, stage transitions, chip/button activation, celebration, and error feedback use the existing tokenized, reduced-motion-aware primitives in `$lib/motion/transitions` (opacity/transform only — no palette, type, or height animation).
- **LLM restatement (backend).** The DeepSeek classifier's existing single call additionally returns a one-line natural-language `summary` (in the user's language), embedded in the free-form `PLAN_PROPOSED` payload JSON. **BREAKING** (pre-v1): `copilot.Classify` returns `{Candidates, Summary}` instead of `[]Candidate`.
- **i18n cleanup.** New `translate()` keys (en + pt-BR) for all new copy, plus keying several strings that are currently hardcoded English (`Create this plan`, `Creating…`, `Loading template…`, `Failed to load template.`, `Which plan did you mean?`, the no-match line, and the home chat-first hint).

## Capabilities

### New Capabilities
- `conversational-plan-proposal`: The chat-first experience of turning a user's natural-language request into a created plan — the LLM restatement + confirm/adjust step, the inputs form reveal, the post-create celebration with contextual next actions, and the motion binding it together.

### Modified Capabilities
<!-- None: openspec/specs/ is empty; no existing spec-level behavior changes. -->

## Impact

- **Frontend:** `PlanProposalCard.svelte` (staging), `TemplateInputsForm.svelte` (required-field awareness for immediate-create), chat page `chat/[threadId]/+page.svelte` and home `+page.svelte` (motion + keyed copy), `$lib/motion/transitions` (reuse), `$lib/i18n/{en,pt-BR}.json` (new keys). Reuses `ScheduleDialog`, `createPlanExecution`, and `LandingCard`'s celebration pattern.
- **Backend:** `internal/copilot/` (classifier summary), `internal/threads/handler.go` `ProposePlan` + `internal/chat` payload builder (embed summary). No proto/`buf` change — `PLAN_PROPOSED.payloadJson` is free-form JSON.
- **Verification:** Vitest (proposal-card stages, contextual actions, summary render), Go tests (classifier summary, ProposePlan payload), `bun run check`, `go test ./...`, `buf lint`.
