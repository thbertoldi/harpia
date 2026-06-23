# Harpia UX Realignment — M6 Lapidação — Implementation Design Spec

**Date:** 2026-06-22
**Status:** Approved (brainstorm), awaiting plan
**Originating conversation:** Brainstorming session 2026-06-22 between Thiago and Claude.
**Extends:** `docs/superpowers/specs/2026-06-19-harpia-ux-realignment-design.md` (master design)
**Companion:** `docs/superpowers/specs/2026-06-22-aiuna-mvp-roadmap-revision.md` (post-M6 partition revision)

The master spec's M6 (§10) is a narrow polish/IA cleanup milestone. This spec **expands M6** based on observations Thiago accumulated while shipping M1-M5: motion is missing, the plan-configuration walk is redundant, the graph feels static, the chrome is monotone, the auth surface breaks the brand spell. M6 becomes *lapidação* — the finishing pass that turns the M1-M5 scaffolding into a product that feels real.

The complementary out-of-scope work surfaced during this brainstorm (integration previews, LLM provider configuration, the Aiuna PRD's camadas fixas, business Áreas as RBAC) is captured separately in the companion roadmap revision; M6 does not absorb it.

---

## 1. Scope

M6 ships, behind no flag (same posture as M2-M5 — universal):

1. **Plan configuration rebuild** — replace the per-step chip walk with a single binding matrix card; delete SET_OVERSEER from the assistant state machine; rebuild the end-of-conversation as a Landing card with three explicit next actions (`Run now` / `Schedule` / `Save and walk away`).
2. **Graph as a living surface** — inline DAG card at the top of the plan thread + expanded canvas both subscribe to live configuration state and runtime execution state; edges anchor properly to node bounds; nodes carry hover previews; canvas drops the literal grid for a vignette background.
3. **Motion + microinteractions** — five categories applied across every Ana-facing surface: chat as living conversation (stagger-in + optimistic send), cards lift on hover, click acknowledgement flash, content-shaped skeletons replace spinners, visible focus rings. Two anchor moments (Save celebration, inbox resolve) get extra treatment. Reduced-motion respected globally.
4. **Tonal richness** — extend the surface token system from two layers to five (`deep / surface / elevated / hover / pop`); enforce gold-accent discipline (signal, not decoration); reform muted-text usage; subtle vignette depth on canvas and sidebar.
5. **Sidebar count badges** — live counts on `Needs you` and `Your plans` sidebar items, sourced from existing M2 and M5 streams; hidden entirely when count is 0; counters tween instead of teleporting.
6. **Zitadel theming** — brand the login, register, password-reset, MFA, consent screens and the email templates so the auth surface matches Harpia. Uses the existing branding-configmap pipeline.
7. **IA cleanup residue** — delete the legacy `/agents`, `/integrations`, `/audit`, `/settings` route directories (admin equivalents are canonical), delete the M2/M5 transitional redirects, collapse dev-login from three personas (Leader / Overseer / Engineer) to two (Ana operator / Platform Engineer).

**Out of scope** (deferred to follow-on milestones — see companion roadmap revision):
- Integration previews (e.g., RSS pulling and rendering live content).
- LLM provider configuration with BYOK + dynamic model discovery.
- Áreas as RBAC entities, typed Artifact foundation, typed Artifact browsers, Plan template editor.
- Camadas fixas N01-N12 in any form (Plans-generate-Artifacts substrate not yet in place).
- Painel do líder (N09), Indicadores básicos (N10).
- Mobile-specific layouts beyond ensuring the new components reduce to single column gracefully.

---

## 2. Resolved decisions

### 2.1 Binding matrix replaces the chip walk

The M5 assistant walks `BINDING_STEP(step_1) → SET_OVERSEER(step_1) → BINDING_STEP(step_2) → SET_OVERSEER(step_2) → …`. For a 3-task plan: ~8 prompts, 3 of which (overseer per step) always default to "You" in v1. The shape is non-intuitive and adds time without adding value.

The matrix card collapses all bindings into a single visual: each task is a row showing its name + input/output contract, an executor picker, and an overseer cell (defaulted to Ana, overridable via a hover-revealed pencil). Policies fold into the card footer; cost pill anchors bottom-left; primary action enables once every row is bound.

**State-machine change:**
- `BINDING_STEP`, `SET_OVERSEER`, `SET_POLICIES`, and `CONFIRM` are all deleted from `planassistant`'s state list.
- A new `BINDING_MATRIX` state replaces them and is the only state between thread-seed and SAVED. Derivation: state is `BINDING_MATRIX` whenever the configuration has any unbound step OR unset policies OR status is still DRAFT. The single `ASSISTANT_PROMPT` carries the whole step list in `payload_json.options`; the matrix card renders the policy fields inline in its footer and writes them via `UpdatePlanConfiguration` directly (no separate prompt for policies).
- The matrix card's `Save & make runnable` button is the gate to `SAVED`. Clicking it promotes status DRAFT → RUNNABLE (or DRAFT → SCHEDULED if a cron is already set, per M5 §2.7). Once status leaves DRAFT, derivation lands at `SAVED` and `NextTurn` emits the LandingCard.
- An idempotency check on `NextTurn` (already in place for the M5 flow) prevents re-emitting the LandingCard if one has already been written for the current cursor.

Overseer-per-Task remains a domain field (per master §4.5) and remains overridable per row; it just stops being a separate assistant turn. The override path writes a `STEP_REBOUND` with an additive `overseer` field in its payload.

Rejected: keeping the chip walk and only deleting SET_OVERSEER. Cuts redundancy but leaves the per-step ceremony intact; user-side calibration confirmed the walk itself is the wrong shape for v1's linear templates.

**Matrix card write model and trigger chain:**
- **Row picker clicks** (executor selection, overseer override, policy field changes) call `UpdatePlanConfiguration` directly. They do **not** write per-pick `USER_SELECTION` messages — the matrix card itself is the visible record. Writing one chat row per pick would create thread noise without adding value.
- **Save button click** writes a single `USER_SELECTION` to the thread with `payload_json = { state: "binding-matrix", action: "save", bindings_summary: [...] }` — one row in the thread records the save event with its binding summary, preserving audit trail.
- **Server-side trigger.** M5 §2.6 wired `NextTurn` to `USER_SELECTION` writes. M6 also wires `NextTurn` to status promotion: `configuration_handler.go`'s `UpdatePlanConfiguration` calls `NextTurn(configId)` when the incoming update flips status out of `DRAFT`. This is what causes the LandingCard to fire after the Save button validates and promotes. The `USER_SELECTION` Save-event is the audit trail; the status-promotion call is the trigger. Both happen in the same RPC round trip; idempotency keeps a double-call harmless.

### 2.2 Landing card replaces the abrupt ending

The current end-of-flow narrative ("Saved. Open the canvas any time.") provides no agency to Ana — she's saved her plan and the conversation just ends. M6 replaces this with a **LandingCard**: a chat-embedded card that appears when the assistant transitions to `SAVED`, offering three explicit next actions:

- **Run now** — triggers an Execution immediately via existing RPC; the thread receives the resulting `RUN_STARTED` system event and Ana can scroll to the inline DAG to watch.
- **Schedule…** — opens the M5 ScheduleDialog.
- **Save and walk away** — dismisses the card; status stays RUNNABLE; the thread shows a quiet "Plan saved." system note.

Visual treatment per Section 3 of WS3 motion: subtle gold-accent breathe (2 cycles, ~1.4s), checkmark draws itself in via SVG stroke animation, three action chips fade in staggered. Premium, not gimmicky.

Protocol: reuses `ASSISTANT_PROMPT` with `payload_json.state = "landing"`. No new `ThreadMessageKind`. The M5-era `ConfirmCard.svelte` component is deleted; LandingCard replaces it.

### 2.3 Graph subscribes to live configuration + runtime state

The inline DAG card and the expanded canvas render the same component (`PlanCanvas.svelte`), sized for context. Both subscribe to the configuration store (same source the BindingMatrix reads) and the execution stream (same source the M3/M4 chat dispatcher reads). When Ana binds a row in the matrix, the corresponding node flips state from `unbound` to `bound` in the same frame.

**Node state vocabulary** (used by both surfaces):

| State | Treatment |
|---|---|
| `unbound` | Muted slate fill, dashed 1px border |
| `bound` | Gold inner ring, embedded executor icon, solid border |
| `running` | Pulse (opacity 1.0 ↔ 0.55, 1.4s cubic-in-out) |
| `waiting` | Static gold halo (3px outer ring) |
| `done` | Solid check glyph, full opacity |
| `failed` | Thin red border, error icon |

**Edge anchoring.** Edges currently float near boxes. Fix: compute anchor points from node bounds — right-edge centre of source → left-edge centre of target — with a small horizontal control offset for the bezier. The type label rides the midpoint in a small chip. Linear templates only in v1; no routing algorithm needed.

**Hover preview.** On node hover (150ms delay before show, 80ms before hide), a `NodeHoverCard` popover anchors above the node showing executor name + tier + price, overseer, input/output contract chips, current state. Click still opens the right-side detail pane on the expanded canvas; the popover is hover-only and dismisses on click.

**Canvas background.** Drop the dotted grid for a subtle radial vignette (canvas centre lighter than corners by ~3% brightness on `--token-surface-deep`). Combined with the WS4 tonal pass, the canvas reads as workspace.

**Empty + loading states.** No template: single dashed-outline placeholder with "Pick a template to see the plan." Loading: shimmer skeleton across all nodes (no spinner) consistent with WS3.

### 2.4 Motion as a library, not ad-hoc

`lib/motion/` becomes the single source of motion primitives:

- `transitions.ts` — named Svelte transitions: `chatEnter` (fade + 8px slide-up, 180ms), `cardLift` (translateY -1px + shadow on hover, 120ms), `chipFlash` (gold wash 50ms + decay 80ms), `errorShake` (60ms red shake on rejection), `saveCelebration` (gold breathe + stroke draw + staggered fade).
- `springs.ts` — preconfigured `spring` and `tweened` stores: `countTween` (for badge counters), `progressSpring` (for cost-pill amount changes), `nodeStateTween` (for graph node opacity pulses).
- `reducedMotion.ts` — single guard reading `prefers-reduced-motion: reduce`; every transition helper consults it and collapses to opacity-only 0ms when set.

Every component currently importing animation logic inline gets refactored to import from `lib/motion/`. Easings standardize on `cubic-out` for most enters and `cubic-in-out` for state pulses, 150-220ms range.

**Two anchor moments** get bespoke treatment because they bookend Ana's session:

- **Save celebration** on LandingCard appearance (see §2.2).
- **Inbox item resolve** — row gold-flashes once, then shrinks-and-fades (200ms), remaining rows slide up smoothly, badge counter tweens down.

### 2.5 Surface tokens — five layers, not two

`frontend/src/lib/themes/tokens.css` gains three new surface tokens **without modifying existing ones** (per the locked-design-tokens guardrail):

```css
--token-surface-deep:      #0a0b0e;   /* sidebar, canvas vignette outer */
--token-surface:           #121318;   /* page background (existing) */
--token-surface-elevated:  #1a1b24;   /* cards, top bars (existing) */
--token-surface-hover:     #20222c;   /* row hover, picker dropdown */
--token-surface-pop:       #2a2d3a;   /* floating popovers */
```

Theme variants (`default.css`, `aiuna.css`) supply tinted equivalents — Aiuna gets cooler greys, default keeps warm obsidian. Tailwind aliases registered under `@theme` so utility classes (`bg-surface-deep`, etc.) generate automatically.

**Gold-accent discipline.** Gold is signal, not decoration:

| Earns gold | Doesn't earn gold |
|---|---|
| Primary action button | Generic dividers |
| Active sidebar / nav item | Static section labels |
| Count pill on `Needs you · 3` | Informational text |
| Focus ring (every interactive element) | Body copy emphasis |
| Cost amount in the pill | Default chip borders |
| Bound-node accent in graph | Resolved-item states |
| Save celebration breathe | Empty states |

A short companion `docs/design/m6-tone-system.md` records the layer map + gold-discipline table so future component authors stay in line without re-discovering the rules.

**Muted text reform.** `--token-text-muted-dark` (`#6b7080`) is too dim against `#121318` in places — fails contrast for informational text. Reserve it for tertiary chrome only (placeholders, "or" between buttons). Promote anything that's actually information (timestamps, secondary labels, contracts) to `--token-text-muted` (`#9da1ab`).

**Vignette depth.** The canvas vignette idiom (per §2.3) also applies to the sidebar: very faint top-to-bottom gradient (`--token-surface-deep` at top → `#080a0d` at bottom). Almost imperceptible, but the eye registers depth.

### 2.6 Count badges — live, additive, quiet at zero

Sidebar entries gain an optional `count` prop. The badge renders as a small gold pill (`bg-primary text-on-primary`, 11px, 4px horizontal padding, 999px radius) when count > 0; **hides entirely when count is 0**. Per §2.4, count changes use the `countTween` store so 3 → 2 animates over 200ms instead of teleporting.

Data sources reuse existing streams:
- `Needs you` count subscribes to the M2 inbox aggregator's pending-count.
- `Your plans` count subscribes to `ListPlanConfigurations`, filtering client-side to RUNNABLE + SCHEDULED status (DRAFTs don't count — they're not yet plans Ana owns operationally).

No new RPCs.

### 2.7 Zitadel theming — pipeline, not redesign

The existing `deploy/dev/kind/zitadel-branding-configmap.yaml` + `zitadel-init.yaml` already wire branding into Zitadel via its private-label-policy API. M6 fills in real branding content.

**Assets:**
- Harpia wordmark in Bodoni Moda, exported as SVG (light variant for dark Zitadel surface; the auth UI uses a dark theme to match Harpia, not the inverse).
- Custom CSS overriding Zitadel's defaults: `#121318` background, `#1a1b24` cards, gold primary on buttons, DM Sans body type.
- Background image — subtle obsidian gradient PNG (1920x1080, ~30KB).
- Self-hosted WOFF2 fonts (Bodoni Moda + DM Sans + Manrope) under `deploy/dev/kind/assets/fonts/`. Self-hosted (not Google Fonts) for offline-clean install and predictable rendering.
- Email template overrides for password-reset, invite, MFA — branded HTML + plain-text variants.

**Pipeline:**
- Assets live under `deploy/dev/kind/assets/` (new directory) for the dev cluster.
- The `zitadel-init.yaml` Job gets new steps that POST the assets to Zitadel's branding endpoints during cluster bootstrap.
- The production helm chart's `deploy/harpia/templates/zitadel.yaml` extends with the same configmap pattern, so the customer-facing cluster install carries Harpia branding by default.

### 2.8 IA cleanup — delete, don't redirect

Per [[feedback-no-pre-v1-compat]], no grace period. The following are deleted from `frontend/src/routes/`:

- `/agents`, `/integrations`, `/audit`, `/settings` — entire directories. `/admin/agents`, `/admin/integrations`, `/admin/audit`, `/admin/settings` are canonical.
- `/oversee`, `/elicitations`, `/approvals`, `/tasks` — M2 transitional redirects. `/inbox` is canonical.
- `/plans/[templateId]/configure/`, `/configure/overseer/`, `/configure/policies/`, `/configure/summary/` — M5 transitional redirects. `/new?template=<id>` is canonical.

A short `grep` pass over `frontend/src/` confirms no internal links still reference the deleted paths before each deletion lands.

**Dev login persona collapse.** Per master §3.3, three personas (Leader / Overseer / Engineer) become two (Ana operator / Platform Engineer). The dev-login screen drops the Leader and Overseer buttons; Engineer is renamed Platform Engineer; the Ana persona button is added with `seat:operator` permission. The [[project-dev-login]] memory file's "3 personas" claim becomes stale after this work — update it in the same commit.

**Persona toggle in user menu.** The recent commit `fix(nav): persona toggle routes to new persona's home; drop /tasks CTA` indicates this already exists. The implementation plan verifies it works after the persona collapse + leaves it alone otherwise.

### 2.9 Status-field semantics unchanged

The DRAFT → RUNNABLE / SCHEDULED transitions from M5 §2.11 are unchanged. The `Run now` LandingCard action on a DRAFT configuration first promotes to RUNNABLE (server-side validation), then enqueues the Execution. The `Schedule…` action behaves identically to the M4-stubbed canvas-top-bar Schedule chip. The `Save and walk away` action is a no-op on status (it was already promoted by the matrix card's save flow).

### 2.10 AbortSignal continuity

M6 components subscribing to streams (sidebar count subscriptions, live graph state) reuse the M3-established `AbortController + $effect` pattern. Nothing new on the wire; just consistent usage.

---

## 3. Architecture sketch

### 3.1 Backend

```
control-plane/internal/planassistant/                      [EXTEND]
  state.go         Delete BINDING_STEP + SET_OVERSEER + SET_POLICIES
                   + CONFIRM states. Add BINDING_MATRIX as the sole
                   pre-SAVED state. DeriveState returns BINDING_MATRIX
                   if any step lacks a slot binding OR policies unset
                   OR status == DRAFT; returns SAVED once status
                   leaves DRAFT (NextTurn then emits LandingCard).
  prompts.go       New builder for BINDING_MATRIX (all-steps payload).
                   New builder for landing-state ASSISTANT_PROMPT.
                   Delete builders for the removed states.
  controller.go    NextTurn unchanged in shape; updated to emit the
                   new state values. SeedThread emits BINDING_MATRIX
                   as the first prompt (was BINDING_STEP).
  state_test.go,   Update fixtures for the collapsed walk.
  controller_test.go,
  prompts_test.go

control-plane/internal/plans/                              [LIGHT EXTEND]
  configuration_handler.go
                   No structural change. The STEP_REBOUND payload
                   may carry an additive `overseer` field; the
                   handler persists it via the existing slot-binding
                   update path. No proto change.

(no proto changes, no migrations)
```

### 3.2 Frontend

```
frontend/src/lib/motion/                                   [NEW DIRECTORY]
  transitions.ts   chatEnter, cardLift, chipFlash, errorShake,
                   saveCelebration named Svelte transitions.
  springs.ts       countTween, progressSpring, nodeStateTween.
  reducedMotion.ts Single guard reading prefers-reduced-motion.

frontend/src/lib/themes/tokens.css                         [EXTEND]
  Append three new surface tokens (surface-deep, surface-hover,
  surface-pop). Register @theme aliases for Tailwind.

frontend/src/lib/themes/default.css, aiuna.css             [EXTEND]
  Tinted variants of the new surface tokens.

frontend/src/lib/components/Skeleton.svelte                [NEW]
  Content-shaped skeleton component (rect/circle props, shimmer).

frontend/src/lib/components/thread/                        [EXTEND/NEW]
  BindingMatrixCard.svelte       [NEW] the matrix card.
  LandingCard.svelte             [NEW] post-save three-action card.
  AssistantPromptCard.svelte     [MODIFY] branch on state for
                                  BINDING_MATRIX / landing renderers.
  ConfirmCard.svelte             [DELETE] folded into LandingCard.

frontend/src/lib/components/canvas/                        [EXTEND/NEW]
  PlanCanvas.svelte              [MODIFY] live configuration +
                                  runtime subscription; node-state
                                  vocabulary; vignette background.
  NodeHoverCard.svelte           [NEW] hover popover.
  EdgePath.svelte (or equivalent) [MODIFY] anchor-point computation
                                            against node bounds.

frontend/src/lib/components/sidebar/                       [EXTEND]
  SidebarItem.svelte (or equivalent) — count prop + badge render.
  Sidebar.svelte                 [MODIFY] wire Needs you count from
                                  inbox aggregator stream; wire
                                  Your plans count from configuration
                                  stream (filtered RUNNABLE+SCHEDULED).

frontend/src/app.css                                       [EXTEND]
  Global :focus-visible rule (gold ring, 2px, 4px offset).
  @media (prefers-reduced-motion: reduce) overrides.

frontend/src/routes/                                       [DELETE]
  agents/, integrations/, audit/, settings/        (legacy duplicates)
  oversee/, elicitations/, approvals/, tasks/      (M2 redirects)
  plans/[templateId]/configure/, .../overseer/, .../policies/,
    .../summary/                                   (M5 redirects)

frontend/src/routes/login/+page.svelte                     [MODIFY]
  Persona collapse: drop Leader/Overseer; add Ana; rename Engineer
  → Platform Engineer.

frontend/src/lib/i18n/{en,pt-BR}.json                      [EXTEND]
  Lockstep additions:
    assistant.bindingMatrix.*  matrix card copy.
    assistant.landing.*        landing card copy.
    sidebar.needsYou.*         badge label + count interpolation.
    sidebar.yourPlans.*        same.
    cost.*                     extend (in-card vs in-topbar variants).
    auth.email.*               Zitadel email subjects/bodies.
    motion.reducedAnnounce     screen-reader-only motion notice.

docs/design/m6-tone-system.md                              [NEW]
  Surface-layer map + gold-accent discipline table.
  Reference for future component authors.

deploy/dev/kind/zitadel-branding-configmap.yaml            [EXTEND]
  Custom CSS + email templates.

deploy/dev/kind/zitadel-init.yaml                          [EXTEND]
  Branding-upload steps via Zitadel private-label-policy API.

deploy/dev/kind/assets/                                    [NEW DIRECTORY]
  logo.svg, background.png, fonts/{bodoni,dmsans,manrope}.woff2,
  emails/{password-reset,invite,mfa}.{html,txt}

deploy/harpia/templates/zitadel.yaml                       [EXTEND]
  Production-cluster branding configmap parity with dev/kind.
```

### 3.3 Hexagonal posture

No new bounded contexts. M6 is entirely an adapter-layer pass: the assistant state machine (an application service on top of the Plan BC) gets simpler, the frontend (an adapter for the Plan / Inbox / Execution BCs) gets richer, the auth surface (an adapter for the IAM BC via Zitadel) gets branded. Domain rules unchanged.

---

## 4. v1 → roadmap hinges (what M6 must not foreclose)

- **Typed Artifacts foundation (post-M6 M8).** The matrix card's row vocabulary (`task name + contract types + executor + overseer`) is the same vocabulary that will surface as columns in the Artifact browsers. No rename needed when Artifacts ship.
- **Áreas as RBAC (post-M6 M8).** The sidebar count subscriptions filter client-side today; when Áreas become permission entities, the subscriptions filter server-side and the frontend stays unchanged.
- **Plan template editor (post-M6 M9).** The matrix card's renderer is template-agnostic — it walks `template.steps` and renders rows. New templates authored via the editor render automatically without component changes.
- **Painel do líder (post-M6 M11).** The sidebar count subscriptions are the seam — the painel becomes a richer page over the same streams. The gold-accent discipline carries the eye to "what needs attention" from day one.

---

## 5. Implementation milestones (sketch — refined in the plan)

Approximate task decomposition. Order roughly mirrors dependency.

### Foundation
1. `lib/motion/`: transitions.ts, springs.ts, reducedMotion.ts — the motion primitives.
2. `lib/components/Skeleton.svelte` — the universal skeleton.
3. Extend `tokens.css` + `default.css` + `aiuna.css` with the three new surface tokens.
4. Extend `app.css` with the global focus-ring rule + reduced-motion overrides.
5. `docs/design/m6-tone-system.md` companion doc.

### Backend (planassistant)
6. `state.go`: delete BINDING_STEP / SET_OVERSEER / SET_POLICIES / CONFIRM; add BINDING_MATRIX as the sole pre-SAVED state; update DeriveState (returns BINDING_MATRIX while DRAFT, SAVED once status leaves DRAFT); update tests.
7. `prompts.go`: new BINDING_MATRIX + landing builders; delete obsolete builders; update tests.
8. `controller.go`: NextTurn emits new state values; SeedThread emits BINDING_MATRIX first; update tests.

### Plan thread + canvas (frontend)
9. `lib/components/thread/BindingMatrixCard.svelte` — the matrix card.
10. `lib/components/thread/LandingCard.svelte` — the post-save card with save-celebration motion.
11. `lib/components/thread/AssistantPromptCard.svelte` — branch on state for new renderers.
12. Delete `lib/components/thread/ConfirmCard.svelte`.
13. `lib/components/canvas/PlanCanvas.svelte` — live state subscription, node-state vocabulary, vignette background.
14. `lib/components/canvas/NodeHoverCard.svelte` — hover popover.
15. `lib/components/canvas/EdgePath.svelte` (or equivalent) — anchor-point computation.

### Sidebar + chrome
16. `lib/components/sidebar/SidebarItem.svelte` — count prop + badge render.
17. `lib/components/sidebar/Sidebar.svelte` — wire count subscriptions.
18. Mechanical sweep: every card/list component imports from `lib/motion/` and adopts new surface tokens (cards lift, hover surfaces, popovers). Spinners → skeletons everywhere.

### IA cleanup
19. Delete `/agents`, `/integrations`, `/audit`, `/settings` route directories after a grep confirms no live references.
20. Delete `/oversee`, `/elicitations`, `/approvals`, `/tasks` route directories.
21. Delete `/plans/[templateId]/configure/` and all four sub-routes.
22. `routes/login/+page.svelte`: persona collapse (drop Leader + Overseer; rename Engineer → Platform Engineer; add Ana).
23. Update `~/.claude/projects/-home-thbertoldi-harpia/memory/project-dev-login.md` to reflect the two-persona reality.

### Zitadel
24. Create `deploy/dev/kind/assets/` with logo SVG, background, fonts, email templates.
25. Extend `deploy/dev/kind/zitadel-branding-configmap.yaml` with custom CSS + email templates.
26. Extend `deploy/dev/kind/zitadel-init.yaml` with branding-upload Job steps.
27. Extend `deploy/harpia/templates/zitadel.yaml` for production parity.

### i18n + verification
28. Add new keys to `en.json` + `pt-BR.json` lockstep.
29. End-to-end verification log: empty state → /new → BindingMatrix → Save → LandingCard → Run now; sidebar badges tick live; canvas hover preview; reduced-motion override visible; Zitadel login screenshot; every deleted route returns 404.
30. Final whole-branch review.

~30 tasks. Mechanical sweep step (#18) is the largest single task; it shapes the perceived quality of every screen.

---

## 6. Out of scope

- Integration previews — companion roadmap revision, V1.1.
- LLM provider configuration with BYOK + dynamic models — companion roadmap revision, M7.
- Áreas as RBAC + typed Artifact foundation — companion roadmap revision, M8.
- Artifact browsers + Plan template editor — companion roadmap revision, M9.
- Authored MVP content (D01, N02-N05, M03-M05, V02-V08, etc.) — companion roadmap revision, M10.
- Painel do líder + Indicadores básicos — companion roadmap revision, M11.
- Mobile-specific layouts beyond ensuring the new components reduce to single column.
- New onboarding/welcome flow polish.
- Telemetry events for the new interactions.
- Marketplace / store, agent-as-overseer, branching DAG editor (v2 per master).

---

## 7. Open questions

None blocking. Items deferred by explicit decision are listed in §6.

---

## 8. Closes / references

- Master design spec §3 (personas — collapse closed here), §4.5 (overseer per-Task — preserved with implicit default), §10 (M6 line item — significantly expanded by this spec).
- M2 design spec — transitional redirects deleted here per [[feedback-no-pre-v1-compat]].
- M3 design spec §2.4 — AbortSignal pattern reused for new subscriptions.
- M5 design spec §2.1 / §2.2 / §2.11 — assistant state machine and status transitions; reshaped here.
- Companion `docs/superpowers/specs/2026-06-22-aiuna-mvp-roadmap-revision.md` — post-M6 partition.
- Memory files: [[project-positioning]], [[project-artifact-architecture]], [[project-areas-rbac]], [[project-aiuna-mvp-deadline]], [[feedback-no-pre-v1-compat]], [[feedback-design-tokens]], [[project-dev-login]].
