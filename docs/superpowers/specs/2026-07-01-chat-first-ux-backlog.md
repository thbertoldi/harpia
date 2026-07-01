# Chat-First UX Backlog (post-Milestone-D review)

**Date:** 2026-07-01
**Status:** Backlog — captured from a live review of the Milestone D chat plan-proposal flow.
**North star:** A smooth, simple, and beautiful conversational experience for turning a
natural-language request into a plan.

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
- Integration selector lists ALL installations; filter to the param's required integration
  type once more integrations exist.
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
