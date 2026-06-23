# Harpia UX Realignment — M6 Lapidação — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish the M1-M5 scaffolding into a product that *feels real*. Replace the per-step chip walk with a binding matrix card; rebuild the conversation ending as a three-action Landing card; make the inline DAG and expanded canvas live + properly anchored + hover-previewable; introduce a small motion library and apply it consistently across every Ana-facing surface; extend surface tokens from 2 to 5 layers and enforce gold-accent discipline; add live count badges to sidebar nav; brand Zitadel auth surfaces; complete the original IA cleanup residue (delete legacy admin duplicates, delete transitional redirects, collapse dev-login personas).

**Architecture:** Backend change is small: collapse four `planassistant` states into one `BINDING_MATRIX` state, delete the `CONFIRM` gate (matrix card's Save promotes status directly), wire `UpdatePlanConfiguration` to call `NextTurn` on status promotion so the LandingCard emits. Frontend change is broad: new `lib/motion/` library (Svelte transitions + spring stores), new `Skeleton`, new `BindingMatrixCard` / `LandingCard` / `NodeHoverCard`, extended `PlanCanvas` (live state + edge anchors + vignette), extended `Sidebar` (count badges from existing streams), mechanical sweep adopting new tokens + motion across all card/list components. No proto changes, no migrations, no new RPCs.

**Tech Stack:** Go 1.23 + ConnectRPC (control-plane), SvelteKit 2.16 + Svelte 5 (frontend), Vitest 2 + Playwright (frontend tests), `testing` package (Go), Tailwind v4 with Harpia design tokens, Zitadel private-label-policy API (auth branding).

## Global Constraints

- **Vocabulary canonical** per master design spec §2 — Task / Plan / Agent / Integration / Executor / Overseer / Elicitation / Approval / Feedback / Artifact. User-facing copy and i18n keys use these terms exactly.
- **i18n lockstep.** Every key change touches both `frontend/src/lib/i18n/en.json` and `frontend/src/lib/i18n/pt-BR.json` in the same commit.
- **Design tokens ADD-ONLY.** Existing tokens (`--token-primary`, `--token-surface`, `--token-text`, etc.) are LOCKED — do not modify their values. M6 *adds* three new surface tokens (`--token-surface-deep`, `--token-surface-hover`, `--token-surface-pop`) per spec §2.5. New tokens follow the same `--token-` naming + `@theme` alias pattern.
- **No pre-v1 compatibility.** Delete cleanly: no transitional redirects, no deprecation shims, no schema migrations. Break things; pre-v1 has no released surface to protect.
- **Commit per task.** Conventional Commits with `feat(ux-m6):` / `chore(ux-m6):` / `fix(ux-m6):` / `docs(ux-m6):` / `test(ux-m6):` scope. Subject lowercase after colon, ≤72 chars.
- **No `Co-Authored-By` trailer** on any commit (project convention).
- **No new files outside this plan's file map.** If implementation requires an unnamed file, STOP and report BLOCKED.
- **Branch:** create `feat/ux-realignment-m6-lapidacao` from `trunk` before Task 1.
- **Test isolation per task.** Frontend: `cd frontend && npx vitest run <relative-path>` for unit; `npx playwright test <relative-path>` for e2e; `npm run check` for type checks. Go: `cd control-plane && go test ./internal/<pkg>/...`.
- **Spec reference:** `docs/superpowers/specs/2026-06-22-harpia-m6-lapidacao-design.md`. When this plan says "per spec §X", read that section before deviating.
- **Self-contained task briefs.** Each task is implementable from its own brief — do not require executors to scroll back to prior tasks for definitions. If a function from an earlier task is needed, repeat its signature in the consuming task's `Interfaces` block.
- **Executor tier hint per task.** Each task header carries a `**Suggested executor:**` line. Mechanical sweeps → Composer 2.5. Reasoning-heavy work (state-machine surgery, math) → GPT 5.5 xhigh. Hints, not mandates.

---

## File map (all changes in M6)

### Backend (control-plane)

- Modify: `control-plane/internal/planassistant/state.go` (collapse 4 states → 1)
- Modify: `control-plane/internal/planassistant/state_test.go` (rewrite fixtures)
- Modify: `control-plane/internal/planassistant/prompts.go` (new `BINDING_MATRIX` + `landing` builders; delete obsolete)
- Modify: `control-plane/internal/planassistant/prompts_test.go`
- Modify: `control-plane/internal/planassistant/controller.go` (NextTurn emits new state values; SeedThread emits BINDING_MATRIX)
- Modify: `control-plane/internal/planassistant/controller_test.go`
- Modify: `control-plane/internal/plans/handler.go` (`UpdatePlanConfiguration` calls `NextTurn` on status promotion)
- Modify: `control-plane/internal/plans/handler_test.go`

### Frontend — foundation

- Create: `frontend/src/lib/motion/transitions.ts`
- Create: `frontend/src/lib/motion/transitions.test.ts`
- Create: `frontend/src/lib/motion/springs.ts`
- Create: `frontend/src/lib/motion/reducedMotion.ts`
- Create: `frontend/src/lib/motion/reducedMotion.test.ts`
- Create: `frontend/src/lib/components/Skeleton.svelte`
- Modify: `frontend/src/lib/themes/tokens.css` (append 3 new surface tokens + `@theme` aliases)
- Modify: `frontend/src/lib/themes/default.css` (theme variant for new tokens)
- Modify: `frontend/src/lib/themes/aiuna.css` (theme variant for new tokens)
- Modify: `frontend/src/app.css` (global `:focus-visible` rule + reduced-motion overrides)
- Create: `docs/design/m6-tone-system.md`

### Frontend — thread

- Create: `frontend/src/lib/components/thread/BindingMatrixCard.svelte`
- Create: `frontend/src/lib/components/thread/LandingCard.svelte`
- Modify: `frontend/src/lib/components/thread/AssistantPromptCard.svelte` (branch on `state` value)
- Delete: `frontend/src/lib/components/thread/ConfirmCard.svelte`
- Create: `frontend/src/lib/plans/matrix.ts` (matrix data helpers)
- Create: `frontend/src/lib/plans/matrix.test.ts`

### Frontend — canvas

- Modify: `frontend/src/lib/components/canvas/PlanCanvas.svelte` (live state subscription + node state vocabulary + vignette)
- Create: `frontend/src/lib/components/canvas/NodeHoverCard.svelte`
- Modify: `frontend/src/lib/components/canvas/EdgePath.svelte` (or equivalent — anchor-point computation)

### Frontend — sidebar

- Modify: `frontend/src/lib/components/sidebar/SidebarItem.svelte` (add count prop + badge)
- Modify: `frontend/src/lib/components/sidebar/Sidebar.svelte` (wire count subscriptions)
- Modify: `frontend/src/routes/+layout.svelte` (subscription glue if needed)

### Frontend — routes (deletions + persona collapse)

- Delete (entire directories): `frontend/src/routes/agents/`, `/integrations/`, `/audit/`, `/settings/`
- Delete (entire directories): `frontend/src/routes/oversee/`, `/elicitations/`, `/approvals/`, `/tasks/`
- Delete (entire directories): `frontend/src/routes/plans/[templateId]/configure/` (including all four sub-routes)
- Modify: `frontend/src/routes/login/+page.svelte` (drop Leader/Overseer; add Ana; rename Engineer → Platform Engineer)

### Frontend — i18n

- Modify: `frontend/src/lib/i18n/en.json` (new keys per spec §3.2)
- Modify: `frontend/src/lib/i18n/pt-BR.json` (lockstep)

### Zitadel branding

- Create: `deploy/dev/kind/assets/logo.svg`
- Create: `deploy/dev/kind/assets/background.png`
- Create: `deploy/dev/kind/assets/fonts/BodoniModa-Regular.woff2`
- Create: `deploy/dev/kind/assets/fonts/DMSans-Regular.woff2`
- Create: `deploy/dev/kind/assets/fonts/Manrope-Regular.woff2`
- Create: `deploy/dev/kind/assets/emails/password-reset.html`
- Create: `deploy/dev/kind/assets/emails/password-reset.txt`
- Create: `deploy/dev/kind/assets/emails/invite.html`
- Create: `deploy/dev/kind/assets/emails/invite.txt`
- Create: `deploy/dev/kind/assets/emails/mfa.html`
- Create: `deploy/dev/kind/assets/emails/mfa.txt`
- Modify: `deploy/dev/kind/zitadel-branding-configmap.yaml` (custom CSS + email refs)
- Modify: `deploy/dev/kind/zitadel-init.yaml` (branding-upload steps via private-label-policy API)
- Modify: `deploy/harpia/templates/zitadel.yaml` (production parity)

### Verification

- Create: `docs/superpowers/plans/2026-06-22-harpia-ux-realignment-m6-lapidacao.verification.md`

### Memory

- Modify: `~/.claude/projects/-home-thbertoldi-harpia/memory/project-dev-login.md` (2-persona reality)

---

## Phase 0 — Branch

### Task 1: Create the M6 branch

**Suggested executor:** Composer 2.5 (trivial)

**Files:** none (git only)

- [ ] **Step 1:** From the repo root, verify trunk is current.

```bash
cd /home/thbertoldi/harpia
git fetch origin
git checkout trunk
git pull --ff-only origin trunk
```

Expected: trunk up to date, no merge needed.

- [ ] **Step 2:** Create and switch to the M6 branch.

```bash
git checkout -b feat/ux-realignment-m6-lapidacao
git status
```

Expected: `On branch feat/ux-realignment-m6-lapidacao` and a clean working tree.

---

## Phase 1 — Foundation

### Task 2: Reduced-motion guard

**Suggested executor:** Composer 2.5

**Files:**
- Create: `frontend/src/lib/motion/reducedMotion.ts`
- Create: `frontend/src/lib/motion/reducedMotion.test.ts`

**Interfaces:**
- Produces: `prefersReducedMotion(): boolean` — true when the user has `prefers-reduced-motion: reduce` set; false otherwise; false in SSR.

- [ ] **Step 1: Write the failing test**

Create `frontend/src/lib/motion/reducedMotion.test.ts`:

```ts
import { describe, expect, it, vi, afterEach } from "vitest";
import { prefersReducedMotion } from "./reducedMotion";

describe("prefersReducedMotion", () => {
  const originalMatchMedia = globalThis.matchMedia;
  afterEach(() => {
    globalThis.matchMedia = originalMatchMedia;
  });

  it("returns false when matchMedia is unavailable (SSR)", () => {
    // @ts-expect-error simulate SSR
    delete globalThis.matchMedia;
    expect(prefersReducedMotion()).toBe(false);
  });

  it("returns true when prefers-reduced-motion: reduce matches", () => {
    globalThis.matchMedia = vi.fn().mockReturnValue({ matches: true }) as unknown as typeof matchMedia;
    expect(prefersReducedMotion()).toBe(true);
  });

  it("returns false when reduce does not match", () => {
    globalThis.matchMedia = vi.fn().mockReturnValue({ matches: false }) as unknown as typeof matchMedia;
    expect(prefersReducedMotion()).toBe(false);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd frontend && npx vitest run src/lib/motion/reducedMotion.test.ts
```

Expected: FAIL — `Cannot find module './reducedMotion'`.

- [ ] **Step 3: Implement**

Create `frontend/src/lib/motion/reducedMotion.ts`:

```ts
/**
 * Returns true when the user has requested reduced motion via the
 * `prefers-reduced-motion: reduce` media query. Returns false in any
 * environment without `matchMedia` (e.g., server-side rendering).
 *
 * Consumed by every transition helper in `lib/motion/` to collapse
 * animations to 0ms opacity-only crossfades when reduce is set.
 */
export function prefersReducedMotion(): boolean {
  if (typeof globalThis.matchMedia !== "function") return false;
  return globalThis.matchMedia("(prefers-reduced-motion: reduce)").matches;
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd frontend && npx vitest run src/lib/motion/reducedMotion.test.ts
```

Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/motion/reducedMotion.ts frontend/src/lib/motion/reducedMotion.test.ts
git commit -m "feat(ux-m6): reduced-motion guard"
```

---

### Task 3: Named motion transitions

**Suggested executor:** GPT 5.5 xhigh (animation parameters need careful judgement)

**Files:**
- Create: `frontend/src/lib/motion/transitions.ts`
- Create: `frontend/src/lib/motion/transitions.test.ts`

**Interfaces:**
- Consumes: `prefersReducedMotion()` from `./reducedMotion`.
- Produces:
  - `chatEnter(node, options?)` — fade + 8px slide-up over 180ms; opacity-only 0ms when reduce.
  - `cardLift(node, options?)` — translateY(-1px) + shadow on hover, 120ms cubic-out.
  - `chipFlash(node, options?)` — momentary gold wash (50ms) + decay (80ms).
  - `errorShake(node, options?)` — 60ms red shake on rejection.
  - `saveCelebration(node, options?)` — gold accent breathe (2 cycles, 1.4s total) + SVG stroke draw + staggered fade-in.

All functions return Svelte transition shape `{ delay, duration, css, easing }`.

- [ ] **Step 1: Write the failing test**

Create `frontend/src/lib/motion/transitions.test.ts`:

```ts
import { describe, expect, it, vi, afterEach } from "vitest";
import { chatEnter, cardLift, chipFlash, errorShake, saveCelebration } from "./transitions";

const originalMatchMedia = globalThis.matchMedia;
afterEach(() => {
  globalThis.matchMedia = originalMatchMedia;
});

function noReduce() {
  globalThis.matchMedia = vi.fn().mockReturnValue({ matches: false }) as unknown as typeof matchMedia;
}
function withReduce() {
  globalThis.matchMedia = vi.fn().mockReturnValue({ matches: true }) as unknown as typeof matchMedia;
}

describe("chatEnter", () => {
  it("returns a transition with 180ms duration and fade+slide css", () => {
    noReduce();
    const node = document.createElement("div");
    const t = chatEnter(node);
    expect(t.duration).toBe(180);
    expect(t.css?.(0, 1)).toMatch(/translateY\(8px\)/);
    expect(t.css?.(1, 0)).toMatch(/translateY\(0px\)/);
  });
  it("collapses to opacity-only 0ms under reduced motion", () => {
    withReduce();
    const node = document.createElement("div");
    const t = chatEnter(node);
    expect(t.duration).toBe(0);
    expect(t.css?.(0, 1)).not.toMatch(/translateY/);
  });
});

describe("cardLift", () => {
  it("returns a 120ms transition", () => {
    noReduce();
    const t = cardLift(document.createElement("div"));
    expect(t.duration).toBe(120);
  });
});

describe("chipFlash", () => {
  it("totals 130ms (50ms flash + 80ms decay)", () => {
    noReduce();
    const t = chipFlash(document.createElement("div"));
    expect(t.duration).toBe(130);
  });
});

describe("errorShake", () => {
  it("returns a 60ms transition", () => {
    noReduce();
    const t = errorShake(document.createElement("div"));
    expect(t.duration).toBe(60);
  });
});

describe("saveCelebration", () => {
  it("returns a 1400ms transition", () => {
    noReduce();
    const t = saveCelebration(document.createElement("div"));
    expect(t.duration).toBe(1400);
  });
  it("collapses to 0ms under reduced motion", () => {
    withReduce();
    const t = saveCelebration(document.createElement("div"));
    expect(t.duration).toBe(0);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd frontend && npx vitest run src/lib/motion/transitions.test.ts
```

Expected: FAIL — `Cannot find module './transitions'`.

- [ ] **Step 3: Implement**

Create `frontend/src/lib/motion/transitions.ts`:

```ts
import { cubicOut, cubicInOut } from "svelte/easing";
import { prefersReducedMotion } from "./reducedMotion";

type TransitionConfig = {
  delay?: number;
  duration: number;
  easing?: (t: number) => number;
  css?: (t: number, u: number) => string;
};

function opacityOnly(_: HTMLElement): TransitionConfig {
  return { duration: 0, css: (t) => `opacity: ${t};` };
}

/** Fade + 8px slide-up; 180ms cubic-out. Used for new assistant prompts. */
export function chatEnter(node: HTMLElement): TransitionConfig {
  if (prefersReducedMotion()) return opacityOnly(node);
  return {
    duration: 180,
    easing: cubicOut,
    css: (t, u) => `opacity: ${t}; transform: translateY(${u * 8}px);`,
  };
}

/** Hover lift: translateY(-1px) + shadow. 120ms cubic-out. */
export function cardLift(node: HTMLElement): TransitionConfig {
  if (prefersReducedMotion()) return opacityOnly(node);
  return {
    duration: 120,
    easing: cubicOut,
    css: (t) =>
      `transform: translateY(${(1 - t) * 0 + t * -1}px); box-shadow: 0 ${t * 4}px ${t * 12}px rgba(0,0,0,${t * 0.18});`,
  };
}

/** 50ms gold wash + 80ms decay; total 130ms. Used on chip + button activation. */
export function chipFlash(node: HTMLElement): TransitionConfig {
  if (prefersReducedMotion()) return opacityOnly(node);
  return {
    duration: 130,
    easing: cubicOut,
    css: (t) => {
      const wash = t < 50 / 130 ? 1 : Math.max(0, 1 - (t - 50 / 130) * (130 / 80));
      return `background-color: rgba(200, 146, 15, ${wash * 0.35});`;
    },
  };
}

/** 60ms red shake. Used when an optimistic action is rejected. */
export function errorShake(node: HTMLElement): TransitionConfig {
  if (prefersReducedMotion()) return opacityOnly(node);
  return {
    duration: 60,
    css: (t) => {
      const offset = Math.sin(t * Math.PI * 4) * 3;
      return `transform: translateX(${offset}px); background-color: rgba(217, 83, 79, ${0.18 * (1 - t)});`;
    },
  };
}

/** Save celebration: gold breathe (2 cycles) + staggered reveal. 1400ms cubic-in-out. */
export function saveCelebration(node: HTMLElement): TransitionConfig {
  if (prefersReducedMotion()) return opacityOnly(node);
  return {
    duration: 1400,
    easing: cubicInOut,
    css: (t) => {
      const breathe = 0.5 + 0.5 * Math.sin(t * Math.PI * 4);
      const fadeIn = Math.min(1, t * 2);
      return `opacity: ${fadeIn}; box-shadow: 0 0 ${breathe * 32}px rgba(200, 146, 15, ${breathe * 0.45});`;
    },
  };
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd frontend && npx vitest run src/lib/motion/transitions.test.ts
```

Expected: PASS (7 tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/motion/transitions.ts frontend/src/lib/motion/transitions.test.ts
git commit -m "feat(ux-m6): named motion transitions (chatEnter/cardLift/chipFlash/errorShake/saveCelebration)"
```

---

### Task 4: Spring + tween stores

**Suggested executor:** Composer 2.5

**Files:**
- Create: `frontend/src/lib/motion/springs.ts`

**Interfaces:**
- Produces:
  - `createCountTween(initial: number): Tweened<number>` — 200ms cubic-out tween for counter badges.
  - `createProgressSpring(initial: number): Spring<number>` — stiffness 0.15, damping 0.6 spring for cost-pill amount changes.
  - `createNodeStatePulse(): Tweened<number>` — 1400ms cubic-in-out loop tween 0→1→0 for `running` node opacity pulse.

- [ ] **Step 1: Implement**

Create `frontend/src/lib/motion/springs.ts`:

```ts
import { tweened, type Tweened } from "svelte/motion";
import { spring, type Spring } from "svelte/motion";
import { cubicOut, cubicInOut } from "svelte/easing";
import { prefersReducedMotion } from "./reducedMotion";

/** 200ms cubic-out tween for sidebar count badges (3 → 2 animates). */
export function createCountTween(initial: number): Tweened<number> {
  return tweened(initial, {
    duration: prefersReducedMotion() ? 0 : 200,
    easing: cubicOut,
  });
}

/** Spring for cost-pill amount changes. Settles softly as bindings shift. */
export function createProgressSpring(initial: number): Spring<number> {
  return spring(initial, {
    stiffness: 0.15,
    damping: 0.6,
  });
}

/** Looping 0 → 1 → 0 tween over 1400ms for `running` node opacity pulse. */
export function createNodeStatePulse(): Tweened<number> {
  return tweened(0, {
    duration: prefersReducedMotion() ? 0 : 1400,
    easing: cubicInOut,
  });
}
```

- [ ] **Step 2: Verify it type-checks**

```bash
cd frontend && npm run check
```

Expected: 0 errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/motion/springs.ts
git commit -m "feat(ux-m6): motion spring + tween stores"
```

---

### Task 5: Skeleton component

**Suggested executor:** Composer 2.5

**Files:**
- Create: `frontend/src/lib/components/Skeleton.svelte`

**Interfaces:**
- Props:
  - `shape: "rect" | "circle"` (default `"rect"`)
  - `width: string` (CSS value, e.g., `"160px"` or `"100%"`)
  - `height: string` (CSS value)
  - `count: number` (default 1; renders N stacked skeletons with 8px gap)

Renders a content-shaped placeholder with a slow shimmer (`transform: translateX(-100% → 100%)` over 1.4s, infinite). Respects `prefers-reduced-motion`: shimmer halts.

- [ ] **Step 1: Implement**

Create `frontend/src/lib/components/Skeleton.svelte`:

```svelte
<script lang="ts">
  type Props = {
    shape?: "rect" | "circle";
    width: string;
    height: string;
    count?: number;
  };
  let { shape = "rect", width, height, count = 1 }: Props = $props();
</script>

<div class="stack" style="--gap: 8px;">
  {#each Array(count) as _, i (i)}
    <div
      class="skeleton"
      class:circle={shape === "circle"}
      style="width: {width}; height: {height};"
    >
      <div class="shimmer"></div>
    </div>
  {/each}
</div>

<style>
  .stack { display: flex; flex-direction: column; gap: var(--gap); }
  .skeleton {
    position: relative;
    overflow: hidden;
    background: var(--token-surface-elevated, #1a1b24);
    border-radius: 6px;
  }
  .skeleton.circle { border-radius: 50%; }
  .shimmer {
    position: absolute;
    inset: 0;
    transform: translateX(-100%);
    background: linear-gradient(
      90deg,
      transparent 0%,
      rgba(255, 255, 255, 0.04) 50%,
      transparent 100%
    );
    animation: shimmer 1.4s linear infinite;
  }
  @media (prefers-reduced-motion: reduce) {
    .shimmer { animation: none; }
  }
  @keyframes shimmer {
    to { transform: translateX(100%); }
  }
</style>
```

- [ ] **Step 2: Verify it type-checks**

```bash
cd frontend && npm run check
```

Expected: 0 errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/components/Skeleton.svelte
git commit -m "feat(ux-m6): content-shaped Skeleton component"
```

---

### Task 6: Extend surface tokens (5-layer system)

**Suggested executor:** Composer 2.5 (mechanical)

**Files:**
- Modify: `frontend/src/lib/themes/tokens.css`
- Modify: `frontend/src/lib/themes/default.css` (if it exists — confirm during step 1)
- Modify: `frontend/src/lib/themes/aiuna.css` (if it exists — confirm during step 1)

- [ ] **Step 1: Inspect existing theme structure**

```bash
ls /home/thbertoldi/harpia/frontend/src/lib/themes/
cat /home/thbertoldi/harpia/frontend/src/lib/themes/tokens.css | head -100
```

Confirm `tokens.css` exists with the existing `--token-surface` (`#121318`) and `--token-surface-elevated` (`#1a1b24`) tokens. Note whether `default.css` and `aiuna.css` already exist; if not, theme variants are inside `tokens.css` under `[data-theme=...]` selectors — adapt step 3 accordingly.

- [ ] **Step 2: Add new tokens to `tokens.css`**

Append to the `:root` block in `frontend/src/lib/themes/tokens.css`:

```css
  /* M6 surface layering — 5-layer system per spec §2.5 */
  --token-surface-deep: #0a0b0e;
  --token-surface-hover: #20222c;
  --token-surface-pop: #2a2d3a;
```

Append to the `@theme` block in the same file:

```css
  --color-surface-deep: var(--token-surface-deep);
  --color-surface-hover: var(--token-surface-hover);
  --color-surface-pop: var(--token-surface-pop);
```

- [ ] **Step 3: Add tinted variants in theme files**

If `default.css` exists, append:

```css
:root[data-theme="default"] {
  --token-surface-deep: #0a0b0e;
  --token-surface-hover: #20222c;
  --token-surface-pop: #2a2d3a;
}
```

If `aiuna.css` exists, append (cooler greys for Aiuna):

```css
:root[data-theme="aiuna"] {
  --token-surface-deep: #08090d;
  --token-surface-hover: #1d1f29;
  --token-surface-pop: #272a36;
}
```

If theme variants live inside `tokens.css` under `[data-theme=...]` blocks instead, add the same declarations there.

- [ ] **Step 4: Verify Tailwind generates the new utility classes**

```bash
cd frontend && npm run check
```

Expected: 0 errors. (Class generation happens at build time; type checker only confirms CSS imports.)

- [ ] **Step 5: Smoke-test with a one-off element**

In any existing component file (e.g., `frontend/src/routes/+page.svelte`), temporarily add `<div class="bg-surface-deep bg-surface-hover bg-surface-pop">test</div>` and run:

```bash
cd frontend && npm run dev
```

Visit the page; verify no console errors and the classes apply (inspect element). Revert the change.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/themes/tokens.css frontend/src/lib/themes/default.css frontend/src/lib/themes/aiuna.css
git commit -m "feat(ux-m6): add 5-layer surface tokens (deep/hover/pop)"
```

Note: include only the files that exist; if `default.css` / `aiuna.css` don't exist, only commit `tokens.css`.

---

### Task 7: Global focus-ring + reduced-motion CSS

**Suggested executor:** Composer 2.5

**Files:**
- Modify: `frontend/src/app.css`

- [ ] **Step 1: Read current `app.css` to find a stable insertion point**

```bash
cat /home/thbertoldi/harpia/frontend/src/app.css
```

Find the end of the file (or the position after `@theme` if one exists).

- [ ] **Step 2: Append global focus + reduced-motion rules**

Append to `frontend/src/app.css`:

```css
/* M6 global focus-visible — gold ring, 2px, 4px offset on every interactive element. */
:where(button, [role="button"], a, input, select, textarea, [tabindex]):focus-visible {
  outline: 2px solid var(--token-primary);
  outline-offset: 4px;
  border-radius: 4px;
}

/* M6 reduced-motion fallback — collapse non-essential animation. */
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.001ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.001ms !important;
    scroll-behavior: auto !important;
  }
}
```

- [ ] **Step 3: Verify type-check and dev server**

```bash
cd frontend && npm run check
cd frontend && npm run dev
```

Tab through any page; confirm a visible gold ring appears around the focused element. Open devtools, toggle "Emulate CSS prefers-reduced-motion: reduce" and confirm transitions collapse.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/app.css
git commit -m "feat(ux-m6): global focus-ring + reduced-motion overrides"
```

---

### Task 8: Tone-system companion doc

**Suggested executor:** Composer 2.5

**Files:**
- Create: `docs/design/m6-tone-system.md`

- [ ] **Step 1: Create the doc**

Write `docs/design/m6-tone-system.md`:

```markdown
# M6 Tone System

Reference for the 5-layer surface tokens and gold-accent discipline introduced in M6 (`docs/superpowers/specs/2026-06-22-harpia-m6-lapidacao-design.md` §2.5). Future components stay in line by consulting this doc instead of re-discovering the rules.

## Surface layers

| Token | Hex (default) | Purpose |
|---|---|---|
| `--token-surface-deep` | `#0a0b0e` | Sidebar, canvas vignette outer |
| `--token-surface` | `#121318` | Page background (existing — locked) |
| `--token-surface-elevated` | `#1a1b24` | Cards, top bars (existing — locked) |
| `--token-surface-hover` | `#20222c` | Row hover, picker dropdown |
| `--token-surface-pop` | `#2a2d3a` | Floating popovers, the highest layer |

Tailwind class names: `bg-surface-deep`, `bg-surface`, `bg-surface-elevated`, `bg-surface-hover`, `bg-surface-pop`.

## Gold-accent discipline

Gold is signal, not decoration.

| Earns gold | Doesn't earn gold |
|---|---|
| Primary action button | Generic dividers |
| Active sidebar / nav item | Static section labels |
| Count pill on `Needs you · 3` | Informational text |
| Focus ring (every interactive element) | Body copy emphasis |
| Cost amount in the pill | Default chip borders |
| Bound-node accent in graph | Resolved-item states |
| Save celebration breathe | Empty states |

## Muted text usage

- `--token-text` (`#f5f2eb`) — body text, primary labels.
- `--token-text-muted` (`#9da1ab`) — secondary labels, timestamps, contracts. Default for "information that isn't primary."
- `--token-text-muted-dark` (`#6b7080`) — tertiary chrome only: placeholders inside inputs, "or" between buttons, decorative captions. Never use for information.

## Reviewing a new component

Ask:
1. Does it sit on the right surface tier?
2. Does it use gold only where the discipline allows?
3. Is muted text used for information (then `-muted`) or chrome (then `-muted-dark`)?
4. Does it import motion primitives from `lib/motion/` rather than rolling its own transitions?
```

- [ ] **Step 2: Commit**

```bash
git add docs/design/m6-tone-system.md
git commit -m "docs(ux-m6): tone system reference (5-layer surfaces + gold discipline)"
```

---


## Phase 2 — Backend planassistant rewrite

Per spec §2.1 and §3.1: collapse `BINDING_STEP`, `SET_OVERSEER`, `SET_POLICIES`, and `CONFIRM` into a single `BINDING_MATRIX` state. `SAVED` stays as the post-promotion ack. Derivation: return `BINDING_MATRIX` whenever the configuration has any unbound step OR unset policies OR status is still DRAFT; return `SAVED` once status leaves DRAFT.

### Task 9: Collapse state.go to BINDING_MATRIX

**Suggested executor:** GPT 5.5 xhigh

**Files:**
- Modify: `control-plane/internal/planassistant/state.go`
- Modify: `control-plane/internal/planassistant/state_test.go`

**Interfaces (the file's exported surface AFTER this task):**
- `StateKind` constants: `StateAwaitingTemplate`, `StateBindingMatrix`, `StateSaved`. (All others deleted.)
- `AssistantState{Kind StateKind, StepKey string}` — `StepKey` remains in the struct (still useful for future per-step prompts) but `BINDING_MATRIX` returns it empty.
- `DeriveState(template *plansv1.PlanTemplate, config *plansv1.PlanConfiguration, messages []*chatv1.ThreadMessage) AssistantState` — signature unchanged.
- `policiesSet(p *plansv1.PlanBehaviorPolicies) bool` — private, unchanged.

- [ ] **Step 1: Rewrite `state_test.go` to drive the new shape**

Replace the contents of `control-plane/internal/planassistant/state_test.go`:

```go
package planassistant

import (
	"testing"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
)

func tpl(stepKeys ...string) *plansv1.PlanTemplate {
	steps := make([]*plansv1.PlanStep, len(stepKeys))
	for i, k := range stepKeys {
		steps[i] = &plansv1.PlanStep{Key: k}
	}
	return &plansv1.PlanTemplate{Steps: steps}
}

func cfg(templateID string, status plansv1.PlanConfigurationStatus, bindings map[string]string, policies *plansv1.PlanBehaviorPolicies) *plansv1.PlanConfiguration {
	sb := make([]*plansv1.SlotBinding, 0, len(bindings))
	for k, v := range bindings {
		sb = append(sb, &plansv1.SlotBinding{StepKey: k, ExecutorInstallationId: v})
	}
	return &plansv1.PlanConfiguration{
		PlanTemplateId:   templateID,
		Status:           status,
		SlotBindings:     sb,
		BehaviorPolicies: policies,
	}
}

func validPolicies() *plansv1.PlanBehaviorPolicies {
	return &plansv1.PlanBehaviorPolicies{
		ElicitationTimeoutBehavior: plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_ASK_OVERSEER,
		PublishApprovalMode:        plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_REQUIRED,
	}
}

func TestDeriveState_AwaitingTemplate(t *testing.T) {
	got := DeriveState(tpl("a", "b"), cfg("", plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT, nil, nil), nil)
	if got.Kind != StateAwaitingTemplate {
		t.Fatalf("want AwaitingTemplate, got %v", got.Kind)
	}
}

func TestDeriveState_BindingMatrix_WhenStepUnbound(t *testing.T) {
	got := DeriveState(
		tpl("a", "b"),
		cfg("tmpl-1", plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT, map[string]string{"a": "exec-1"}, validPolicies()),
		nil,
	)
	if got.Kind != StateBindingMatrix {
		t.Fatalf("want BindingMatrix, got %v", got.Kind)
	}
	if got.StepKey != "" {
		t.Fatalf("StepKey must be empty for BindingMatrix, got %q", got.StepKey)
	}
}

func TestDeriveState_BindingMatrix_WhenPoliciesUnset(t *testing.T) {
	got := DeriveState(
		tpl("a"),
		cfg("tmpl-1", plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT, map[string]string{"a": "exec-1"}, nil),
		nil,
	)
	if got.Kind != StateBindingMatrix {
		t.Fatalf("want BindingMatrix (policies unset), got %v", got.Kind)
	}
}

func TestDeriveState_BindingMatrix_WhenStatusStillDraft(t *testing.T) {
	got := DeriveState(
		tpl("a"),
		cfg("tmpl-1", plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT, map[string]string{"a": "exec-1"}, validPolicies()),
		nil,
	)
	if got.Kind != StateBindingMatrix {
		t.Fatalf("want BindingMatrix (still DRAFT), got %v", got.Kind)
	}
}

func TestDeriveState_Saved_WhenStatusRunnable(t *testing.T) {
	got := DeriveState(
		tpl("a"),
		cfg("tmpl-1", plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE, map[string]string{"a": "exec-1"}, validPolicies()),
		nil,
	)
	if got.Kind != StateSaved {
		t.Fatalf("want Saved (status leaves DRAFT), got %v", got.Kind)
	}
}

func TestDeriveState_Saved_WhenStatusScheduled(t *testing.T) {
	got := DeriveState(
		tpl("a"),
		cfg("tmpl-1", plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_SCHEDULED, map[string]string{"a": "exec-1"}, validPolicies()),
		nil,
	)
	if got.Kind != StateSaved {
		t.Fatalf("want Saved (status leaves DRAFT), got %v", got.Kind)
	}
}
```

- [ ] **Step 2: Run tests — they must fail because the old `StateBindingStep` etc. constants still exist and `StateBindingMatrix` does not**

```bash
cd control-plane && go test ./internal/planassistant/... -run TestDeriveState -v
```

Expected: compilation FAIL (`StateBindingMatrix` undefined; old state names still referenced in other tests).

- [ ] **Step 3: Rewrite `state.go`**

Replace the entire contents of `control-plane/internal/planassistant/state.go`:

```go
// Package planassistant owns the deterministic configuration-assistant
// state machine. See docs/superpowers/specs/2026-06-22-harpia-m6-lapidacao-design.md.
package planassistant

import (
	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
)

// StateKind identifies the assistant's position in the configuration flow.
// M6 collapsed BINDING_STEP / SET_OVERSEER / SET_POLICIES / CONFIRM into
// the single BINDING_MATRIX state per spec §2.1. The matrix card is the
// configuration surface; per-step prompts no longer exist.
type StateKind string

const (
	StateAwaitingTemplate StateKind = "AWAITING_TEMPLATE"
	// StateBindingMatrix is the sole pre-SAVED state. The matrix card
	// renders all step rows + policy fields inline. Derivation returns
	// BindingMatrix whenever any step is unbound OR policies are unset
	// OR status is still DRAFT (i.e., the matrix card's Save button has
	// not been clicked).
	StateBindingMatrix StateKind = "BINDING_MATRIX"
	// StateSaved is returned once status leaves DRAFT. Controller.NextTurn
	// detects the transition and emits the LandingCard.
	StateSaved StateKind = "SAVED"
)

// AssistantState is the derived state for a single PlanConfiguration.
// StepKey is retained for future per-step prompts but is unused (empty)
// in v1 — BindingMatrix and Saved both leave it empty.
type AssistantState struct {
	Kind    StateKind
	StepKey string
}

// DeriveState is a pure function over (template, configuration, messages)
// that returns the assistant's next state. Messages are accepted for
// future use but unused in v1 derivation — the configuration alone is
// authoritative.
func DeriveState(template *plansv1.PlanTemplate, config *plansv1.PlanConfiguration, messages []*chatv1.ThreadMessage) AssistantState {
	_ = messages
	if config == nil || config.GetPlanTemplateId() == "" {
		return AssistantState{Kind: StateAwaitingTemplate}
	}

	// Status promoted past DRAFT means the matrix-card Save fired and
	// validation passed. Next assistant turn is the LandingCard.
	if config.GetStatus() != plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT &&
		config.GetStatus() != plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_UNSPECIFIED {
		return AssistantState{Kind: StateSaved}
	}

	// Otherwise we're still in the matrix. Whether any step is unbound or
	// policies are unset, the matrix card surfaces it — derivation returns
	// BindingMatrix uniformly.
	return AssistantState{Kind: StateBindingMatrix}
}

// policiesSet remains exported-package-private; consumed by prompts.go to
// gate Save-button enablement.
func policiesSet(p *plansv1.PlanBehaviorPolicies) bool {
	if p == nil {
		return false
	}
	return p.GetElicitationTimeoutBehavior() != plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_UNSPECIFIED &&
		p.GetPublishApprovalMode() != plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_UNSPECIFIED
}
```

- [ ] **Step 4: Run tests — should pass (the package's other files still reference deleted constants, so compile may still fail — that's expected; move on to Task 10)**

```bash
cd control-plane && go test ./internal/planassistant/... -run TestDeriveState -v
```

Expected: tests in `state_test.go` pass logically but the package may not compile yet because `prompts.go` / `controller.go` still reference `StateBindingStep` / `StateSetOverseer` / `StateSetPolicies` / `StateConfirm`. That's acceptable — Tasks 10 and 11 clean them up.

- [ ] **Step 5: Commit (allow compile-broken state — next tasks fix it)**

```bash
git add control-plane/internal/planassistant/state.go control-plane/internal/planassistant/state_test.go
git commit -m "feat(ux-m6): collapse planassistant states to BINDING_MATRIX

State.go now exposes only AwaitingTemplate / BindingMatrix / Saved.
Per-step states (BINDING_STEP, SET_OVERSEER, SET_POLICIES) and
the CONFIRM gate are deleted; the matrix card is the configuration
surface. SAVED returns once status leaves DRAFT.

prompts.go and controller.go still reference deleted constants and
will not compile until the follow-on tasks land."
```

---

### Task 10: Rewrite prompts.go for BINDING_MATRIX + landing

**Suggested executor:** GPT 5.5 xhigh

**Files:**
- Modify: `control-plane/internal/planassistant/prompts.go`
- Modify: `control-plane/internal/planassistant/prompts_test.go`

**Interfaces (the file's exported surface AFTER this task):**
- `ExecutorOption{ID, Label, Sublabel, Value string; PriceBRL float64}` — unchanged.
- `PromptInput{Template *plansv1.PlanTemplate; Config *plansv1.PlanConfiguration; Catalog map[string][]ExecutorOption; CurrentUserDisplayName string}` — `Catalog` keyed by step key; `CurrentUserDisplayName` defaults to "You" if empty.
- `BuildPrompt(state AssistantState, in PromptInput) (text, payload string)` — signature unchanged. Returns empty `text` for `AWAITING_TEMPLATE` (renderer uses default copy). For `BINDING_MATRIX`, returns the assistant intro line plus a JSON payload with `state: "BINDING_MATRIX"`, `policies_set` bool, and `rows: [{step_key, step_title, contracts: {input, output}, options: [...], current_executor_id, current_overseer_id, current_overseer_label}, ...]`. For `SAVED`, returns the LandingCard's narrative line plus a JSON payload with `state: "landing"` and `actions: [{id: "run-now", label}, {id: "schedule", label}, {id: "walk-away", label}]`.

- [ ] **Step 1: Rewrite `prompts_test.go`**

Replace the contents of `control-plane/internal/planassistant/prompts_test.go`:

```go
package planassistant

import (
	"encoding/json"
	"strings"
	"testing"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
)

func TestBuildPrompt_BindingMatrix(t *testing.T) {
	template := &plansv1.PlanTemplate{
		Steps: []*plansv1.PlanStep{
			{Key: "fetch", Title: "Fetch newsletter"},
			{Key: "summarize", Title: "Summarize"},
		},
	}
	in := PromptInput{
		Template: template,
		Config: &plansv1.PlanConfiguration{
			PlanTemplateId: "tmpl-1",
			SlotBindings: []*plansv1.SlotBinding{
				{StepKey: "fetch", ExecutorInstallationId: "exec-bloomberg"},
			},
		},
		Catalog: map[string][]ExecutorOption{
			"fetch":     {{ID: "exec-bloomberg", Label: "RSS · Bloomberg", PriceBRL: 0}},
			"summarize": {{ID: "exec-writer", Label: "Writer · Senior", PriceBRL: 0.42}},
		},
		CurrentUserDisplayName: "Ana",
	}
	text, payload := BuildPrompt(AssistantState{Kind: StateBindingMatrix}, in)
	if text == "" {
		t.Fatal("expected non-empty assistant text")
	}
	var parsed struct {
		State       string `json:"state"`
		PoliciesSet bool   `json:"policies_set"`
		Rows        []struct {
			StepKey            string `json:"step_key"`
			StepTitle          string `json:"step_title"`
			CurrentExecutorID  string `json:"current_executor_id"`
			CurrentOverseerLbl string `json:"current_overseer_label"`
		} `json:"rows"`
	}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		t.Fatalf("payload not valid JSON: %v\n%s", err, payload)
	}
	if parsed.State != "BINDING_MATRIX" {
		t.Fatalf("want state BINDING_MATRIX, got %q", parsed.State)
	}
	if len(parsed.Rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(parsed.Rows))
	}
	if parsed.Rows[0].CurrentExecutorID != "exec-bloomberg" {
		t.Fatalf("row 0 should reflect existing binding, got %q", parsed.Rows[0].CurrentExecutorID)
	}
	if parsed.Rows[1].CurrentExecutorID != "" {
		t.Fatalf("row 1 should be unbound, got %q", parsed.Rows[1].CurrentExecutorID)
	}
	for _, r := range parsed.Rows {
		if r.CurrentOverseerLbl != "Ana" {
			t.Fatalf("overseer label should default to current user 'Ana', got %q", r.CurrentOverseerLbl)
		}
	}
}

func TestBuildPrompt_Landing(t *testing.T) {
	text, payload := BuildPrompt(AssistantState{Kind: StateSaved}, PromptInput{
		Config: &plansv1.PlanConfiguration{Status: plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE},
	})
	if !strings.Contains(strings.ToLower(text), "saved") && !strings.Contains(strings.ToLower(text), "salv") {
		t.Fatalf("landing text should narrate save, got %q", text)
	}
	var parsed struct {
		State   string `json:"state"`
		Actions []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"actions"`
	}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		t.Fatalf("payload not valid JSON: %v\n%s", err, payload)
	}
	if parsed.State != "landing" {
		t.Fatalf("want state landing, got %q", parsed.State)
	}
	if len(parsed.Actions) != 3 {
		t.Fatalf("want 3 actions, got %d", len(parsed.Actions))
	}
	want := map[string]bool{"run-now": false, "schedule": false, "walk-away": false}
	for _, a := range parsed.Actions {
		want[a.ID] = true
	}
	for id, found := range want {
		if !found {
			t.Fatalf("missing landing action %q", id)
		}
	}
}

func TestBuildPrompt_AwaitingTemplate_ReturnsEmpty(t *testing.T) {
	text, payload := BuildPrompt(AssistantState{Kind: StateAwaitingTemplate}, PromptInput{})
	if text != "" || payload != "" {
		t.Fatalf("awaiting-template prompt should return empty strings, got text=%q payload=%q", text, payload)
	}
}
```

- [ ] **Step 2: Run tests — they must fail (current `prompts.go` only knows old states)**

```bash
cd control-plane && go test ./internal/planassistant/... -run TestBuildPrompt -v
```

Expected: FAIL.

- [ ] **Step 3: Rewrite `prompts.go`**

Replace the entire contents of `control-plane/internal/planassistant/prompts.go`:

```go
package planassistant

import (
	"encoding/json"
	"fmt"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
)

// ExecutorOption is a candidate executor for a step's picker.
type ExecutorOption struct {
	ID       string
	Label    string
	Sublabel string
	Value    string
	PriceBRL float64
}

// PromptInput is everything BuildPrompt needs. Catalog is keyed by step
// key; CurrentUserDisplayName defaults to "You" if empty.
type PromptInput struct {
	Template               *plansv1.PlanTemplate
	Config                 *plansv1.PlanConfiguration
	Catalog                map[string][]ExecutorOption
	CurrentUserDisplayName string
}

// BuildPrompt returns the assistant's text + JSON payload for the given
// state. The frontend renders the assistant turn from these two strings.
func BuildPrompt(state AssistantState, in PromptInput) (text, payload string) {
	switch state.Kind {
	case StateAwaitingTemplate:
		return "", ""
	case StateBindingMatrix:
		return buildMatrixPrompt(in)
	case StateSaved:
		return buildLandingPrompt(in)
	}
	return "", ""
}

func buildMatrixPrompt(in PromptInput) (string, string) {
	user := in.CurrentUserDisplayName
	if user == "" {
		user = "You"
	}
	text := fmt.Sprintf("Here's the plan. Pick an executor for each task — overseer defaults to %s.", user)

	bindings := map[string]string{}
	for _, sb := range in.Config.GetSlotBindings() {
		if sb.GetExecutorInstallationId() != "" {
			bindings[sb.GetStepKey()] = sb.GetExecutorInstallationId()
		}
	}
	overseers := map[string]string{}
	for _, ob := range in.Config.GetOverseerBindings() {
		if ob.GetOverseerUserId() != "" {
			overseers[ob.GetStepKey()] = ob.GetOverseerUserId()
		}
	}

	type contract struct {
		Input  string `json:"input"`
		Output string `json:"output"`
	}
	type row struct {
		StepKey              string           `json:"step_key"`
		StepTitle            string           `json:"step_title"`
		Contracts            contract         `json:"contracts"`
		Options              []ExecutorOption `json:"options"`
		CurrentExecutorID    string           `json:"current_executor_id"`
		CurrentOverseerID    string           `json:"current_overseer_id"`
		CurrentOverseerLabel string           `json:"current_overseer_label"`
	}
	rows := make([]row, 0, len(in.Template.GetSteps()))
	for _, step := range in.Template.GetSteps() {
		overseerLabel := user
		if id, ok := overseers[step.GetKey()]; ok && id != "" && id != "self" {
			overseerLabel = id
		}
		rows = append(rows, row{
			StepKey:   step.GetKey(),
			StepTitle: step.GetTitle(),
			Contracts: contract{
				Input:  step.GetInputContractType(),
				Output: step.GetOutputContractType(),
			},
			Options:              in.Catalog[step.GetKey()],
			CurrentExecutorID:    bindings[step.GetKey()],
			CurrentOverseerID:    overseers[step.GetKey()],
			CurrentOverseerLabel: overseerLabel,
		})
	}

	body := struct {
		State       string `json:"state"`
		PoliciesSet bool   `json:"policies_set"`
		Rows        []row  `json:"rows"`
	}{
		State:       "BINDING_MATRIX",
		PoliciesSet: policiesSet(in.Config.GetBehaviorPolicies()),
		Rows:        rows,
	}
	b, _ := json.Marshal(body)
	return text, string(b)
}

func buildLandingPrompt(in PromptInput) (string, string) {
	_ = in
	text := "Saved. Run it now, schedule a recurring run, or walk away — it'll be here when you come back."
	body := struct {
		State   string `json:"state"`
		Actions []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"actions"`
	}{
		State: "landing",
		Actions: []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		}{
			{ID: "run-now", Label: "Run now"},
			{ID: "schedule", Label: "Schedule…"},
			{ID: "walk-away", Label: "Save and walk away"},
		},
	}
	b, _ := json.Marshal(body)
	return text, string(b)
}
```

- [ ] **Step 4: Run tests**

```bash
cd control-plane && go test ./internal/planassistant/... -run TestBuildPrompt -v
```

Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add control-plane/internal/planassistant/prompts.go control-plane/internal/planassistant/prompts_test.go
git commit -m "feat(ux-m6): rewrite prompts.go for BINDING_MATRIX + landing"
```

---

### Task 11: Rewire controller.go SeedThread + NextTurn

**Suggested executor:** GPT 5.5 xhigh

**Files:**
- Modify: `control-plane/internal/planassistant/controller.go`
- Modify: `control-plane/internal/planassistant/controller_test.go`

**Interfaces (after this task):**
- `Controller` (struct unchanged in shape — same constructor signature).
- `SeedThread(ctx context.Context, tenantID, configID uuid.UUID) error` — emits `CONFIGURATION_STARTED` system message + the first `ASSISTANT_PROMPT` with `state: "BINDING_MATRIX"` payload.
- `NextTurn(ctx context.Context, tenantID, configID uuid.UUID) error` — re-derives state. If `BINDING_MATRIX`, emits a fresh `ASSISTANT_PROMPT` (so row catalogues update when bindings change). If `SAVED` and no `ASSISTANT_PROMPT` with `state: "landing"` has been emitted yet for this configuration, emits the LandingCard. Idempotent: re-calls on the same state are no-ops.

- [ ] **Step 1: Read current controller.go**

```bash
cat /home/thbertoldi/harpia/control-plane/internal/planassistant/controller.go
```

Identify: the `messages.Store` interface methods used (`AppendThreadMessage`, `ListThreadMessages`), the executor catalog method (`ListForStep` or similar), the configuration store method (`GetConfiguration`).

- [ ] **Step 2: Rewrite `controller_test.go`**

Replace the contents of `control-plane/internal/planassistant/controller_test.go`:

```go
package planassistant

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
)

type fakeConfigStore struct {
	cfg *plansv1.PlanConfiguration
}

func (f *fakeConfigStore) GetConfiguration(_ context.Context, _ uuid.UUID, _ uuid.UUID) (*plansv1.PlanConfiguration, error) {
	return f.cfg, nil
}

type fakeTemplateStore struct {
	tpl *plansv1.PlanTemplate
}

func (f *fakeTemplateStore) GetTemplate(_ context.Context, _ string) (*plansv1.PlanTemplate, error) {
	return f.tpl, nil
}

type fakeCatalog struct{}

func (f *fakeCatalog) ListForStep(_ context.Context, _ uuid.UUID, _ string) ([]ExecutorOption, error) {
	return nil, nil
}

type fakeMessages struct {
	written []*chatv1.ThreadMessage
}

func (f *fakeMessages) AppendThreadMessage(_ context.Context, msg *chatv1.ThreadMessage) error {
	f.written = append(f.written, msg)
	return nil
}

func (f *fakeMessages) ListThreadMessages(_ context.Context, _ uuid.UUID, _ uuid.UUID) ([]*chatv1.ThreadMessage, error) {
	return f.written, nil
}

func newCtl() (*Controller, *fakeMessages, *fakeConfigStore) {
	msgs := &fakeMessages{}
	cs := &fakeConfigStore{}
	c := NewController(cs, &fakeTemplateStore{tpl: &plansv1.PlanTemplate{Steps: []*plansv1.PlanStep{{Key: "a", Title: "A"}}}}, &fakeCatalog{}, msgs)
	return c, msgs, cs
}

func TestSeedThread_EmitsConfigStartedAndBindingMatrix(t *testing.T) {
	c, msgs, cs := newCtl()
	cs.cfg = &plansv1.PlanConfiguration{PlanTemplateId: "tmpl-1", Status: plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT}
	if err := c.SeedThread(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatalf("SeedThread: %v", err)
	}
	if len(msgs.written) != 2 {
		t.Fatalf("want 2 messages, got %d", len(msgs.written))
	}
	if msgs.written[0].Kind != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_CONFIGURATION_STARTED {
		t.Fatalf("first message should be CONFIGURATION_STARTED, got %v", msgs.written[0].Kind)
	}
	if msgs.written[1].Kind != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_PROMPT {
		t.Fatalf("second message should be ASSISTANT_PROMPT, got %v", msgs.written[1].Kind)
	}
	var p struct {
		State string `json:"state"`
	}
	_ = json.Unmarshal([]byte(msgs.written[1].GetPayloadJson()), &p)
	if p.State != "BINDING_MATRIX" {
		t.Fatalf("seeded prompt should be BINDING_MATRIX, got %q", p.State)
	}
}

func TestNextTurn_EmitsLandingOnPromotion(t *testing.T) {
	c, msgs, cs := newCtl()
	cs.cfg = &plansv1.PlanConfiguration{
		PlanTemplateId: "tmpl-1",
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
	}
	if err := c.NextTurn(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatalf("NextTurn: %v", err)
	}
	if len(msgs.written) != 1 {
		t.Fatalf("want 1 landing message, got %d", len(msgs.written))
	}
	var p struct {
		State string `json:"state"`
	}
	_ = json.Unmarshal([]byte(msgs.written[0].GetPayloadJson()), &p)
	if p.State != "landing" {
		t.Fatalf("want landing payload, got %q", p.State)
	}
	if !strings.Contains(strings.ToLower(msgs.written[0].GetText()), "saved") {
		t.Fatalf("landing text should narrate save, got %q", msgs.written[0].GetText())
	}
}

func TestNextTurn_LandingIsIdempotent(t *testing.T) {
	c, msgs, cs := newCtl()
	cs.cfg = &plansv1.PlanConfiguration{
		PlanTemplateId: "tmpl-1",
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
	}
	if err := c.NextTurn(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatalf("NextTurn 1: %v", err)
	}
	if err := c.NextTurn(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatalf("NextTurn 2: %v", err)
	}
	if len(msgs.written) != 1 {
		t.Fatalf("idempotent NextTurn should still produce 1 landing, got %d", len(msgs.written))
	}
}
```

- [ ] **Step 3: Run tests — they fail (controller still references deleted states)**

```bash
cd control-plane && go test ./internal/planassistant/... -run TestSeedThread -run TestNextTurn -v
```

Expected: FAIL (compile errors).

- [ ] **Step 4: Rewrite `controller.go`**

Open `control-plane/internal/planassistant/controller.go` and replace the body so it:
1. Keeps the `ExecutorCatalog`, `ConfigurationStore`, `TemplateStore`, `Controller` interfaces/struct.
2. Adds a `MessageStore` interface with `AppendThreadMessage(ctx, *chatv1.ThreadMessage) error` and `ListThreadMessages(ctx, tenantID, configID uuid.UUID) ([]*chatv1.ThreadMessage, error)` if not already present.
3. Implements `SeedThread`: load config + template, build the matrix prompt, append two messages (`CONFIGURATION_STARTED` then `ASSISTANT_PROMPT` carrying the prompt).
4. Implements `NextTurn`: load config + template, derive state. If `BindingMatrix`, build prompt and append `ASSISTANT_PROMPT` only if the most recent `ASSISTANT_PROMPT` differs in payload (cheap dedup: compare payload string). If `Saved`, check via `ListThreadMessages` whether an `ASSISTANT_PROMPT` with `state=="landing"` already exists; if not, emit it; if so, no-op.

Use this skeleton (fill in any project-specific helpers as you go):

```go
package planassistant

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
)

type ExecutorCatalog interface {
	ListForStep(ctx context.Context, tenantID uuid.UUID, stepKey string) ([]ExecutorOption, error)
}

type ConfigurationStore interface {
	GetConfiguration(ctx context.Context, tenantID, configID uuid.UUID) (*plansv1.PlanConfiguration, error)
}

type TemplateStore interface {
	GetTemplate(ctx context.Context, templateID string) (*plansv1.PlanTemplate, error)
}

type MessageStore interface {
	AppendThreadMessage(ctx context.Context, msg *chatv1.ThreadMessage) error
	ListThreadMessages(ctx context.Context, tenantID, configID uuid.UUID) ([]*chatv1.ThreadMessage, error)
}

type Controller struct {
	configs   ConfigurationStore
	templates TemplateStore
	catalog   ExecutorCatalog
	messages  MessageStore
}

func NewController(c ConfigurationStore, t TemplateStore, x ExecutorCatalog, m MessageStore) *Controller {
	return &Controller{configs: c, templates: t, catalog: x, messages: m}
}

func (c *Controller) SeedThread(ctx context.Context, tenantID, configID uuid.UUID) error {
	cfg, err := c.configs.GetConfiguration(ctx, tenantID, configID)
	if err != nil {
		return err
	}
	if err := c.messages.AppendThreadMessage(ctx, &chatv1.ThreadMessage{
		ConfigurationId: configID.String(),
		Kind:            chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_CONFIGURATION_STARTED,
		PayloadJson:     `{"template_id":"` + cfg.GetPlanTemplateId() + `"}`,
	}); err != nil {
		return err
	}
	return c.emitMatrixPrompt(ctx, tenantID, configID, cfg)
}

func (c *Controller) NextTurn(ctx context.Context, tenantID, configID uuid.UUID) error {
	cfg, err := c.configs.GetConfiguration(ctx, tenantID, configID)
	if err != nil {
		return err
	}
	tpl, err := c.templates.GetTemplate(ctx, cfg.GetPlanTemplateId())
	if err != nil {
		return err
	}
	state := DeriveState(tpl, cfg, nil)
	switch state.Kind {
	case StateBindingMatrix:
		return c.emitMatrixPrompt(ctx, tenantID, configID, cfg)
	case StateSaved:
		emitted, err := c.landingAlreadyEmitted(ctx, tenantID, configID)
		if err != nil {
			return err
		}
		if emitted {
			return nil
		}
		text, payload := BuildPrompt(state, PromptInput{Config: cfg})
		return c.messages.AppendThreadMessage(ctx, &chatv1.ThreadMessage{
			ConfigurationId: configID.String(),
			Kind:            chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_PROMPT,
			Text:            text,
			PayloadJson:     payload,
		})
	}
	return nil
}

func (c *Controller) emitMatrixPrompt(ctx context.Context, tenantID, configID uuid.UUID, cfg *plansv1.PlanConfiguration) error {
	tpl, err := c.templates.GetTemplate(ctx, cfg.GetPlanTemplateId())
	if err != nil {
		return err
	}
	catalog := map[string][]ExecutorOption{}
	for _, step := range tpl.GetSteps() {
		opts, err := c.catalog.ListForStep(ctx, tenantID, step.GetKey())
		if err != nil {
			return err
		}
		catalog[step.GetKey()] = opts
	}
	text, payload := BuildPrompt(AssistantState{Kind: StateBindingMatrix}, PromptInput{
		Template: tpl,
		Config:   cfg,
		Catalog:  catalog,
	})
	if dup, err := c.isDuplicatePrompt(ctx, tenantID, configID, payload); err != nil || dup {
		return err
	}
	return c.messages.AppendThreadMessage(ctx, &chatv1.ThreadMessage{
		ConfigurationId: configID.String(),
		Kind:            chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_PROMPT,
		Text:            text,
		PayloadJson:     payload,
	})
}

func (c *Controller) landingAlreadyEmitted(ctx context.Context, tenantID, configID uuid.UUID) (bool, error) {
	msgs, err := c.messages.ListThreadMessages(ctx, tenantID, configID)
	if err != nil {
		return false, err
	}
	for _, m := range msgs {
		if m.GetKind() != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_PROMPT {
			continue
		}
		var p struct {
			State string `json:"state"`
		}
		if json.Unmarshal([]byte(m.GetPayloadJson()), &p) == nil && p.State == "landing" {
			return true, nil
		}
	}
	return false, nil
}

func (c *Controller) isDuplicatePrompt(ctx context.Context, tenantID, configID uuid.UUID, payload string) (bool, error) {
	msgs, err := c.messages.ListThreadMessages(ctx, tenantID, configID)
	if err != nil {
		return false, err
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m.GetKind() == chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_PROMPT {
			return m.GetPayloadJson() == payload, nil
		}
	}
	return false, nil
}
```

If the current `controller.go` has additional helpers (e.g., `findSelectionAndState`, `emitSavedAck`, `emitCurrentPrompt`) tied to deleted states, delete them.

- [ ] **Step 5: Run all planassistant tests**

```bash
cd control-plane && go test ./internal/planassistant/... -v
```

Expected: ALL PASS.

- [ ] **Step 6: Commit**

```bash
git add control-plane/internal/planassistant/controller.go control-plane/internal/planassistant/controller_test.go
git commit -m "feat(ux-m6): rewire controller for BINDING_MATRIX + landing emission

NextTurn now derives state from the post-update configuration. On
BINDING_MATRIX it emits a fresh prompt (deduped by payload). On SAVED
it emits the LandingCard once, idempotently."
```

---

### Task 12: Trigger NextTurn from UpdatePlanConfiguration on status promotion

**Suggested executor:** GPT 5.5 xhigh

**Files:**
- Modify: `control-plane/internal/plans/handler.go`
- Modify: `control-plane/internal/plans/handler_test.go`

- [ ] **Step 1: Locate the handler**

```bash
grep -n "UpdatePlanConfiguration" /home/thbertoldi/harpia/control-plane/internal/plans/*.go
```

Find the function implementing the `UpdatePlanConfiguration` RPC. Read it end-to-end.

- [ ] **Step 2: Add a test asserting NextTurn fires on status promotion**

Append to `control-plane/internal/plans/handler_test.go` (mirror the existing test setup; substitute your `planassistant.Controller` mock):

```go
func TestUpdatePlanConfiguration_TriggersNextTurnOnStatusPromotion(t *testing.T) {
	h, deps := newTestHandler(t)
	// Seed a DRAFT configuration with all bindings + policies set.
	cfgID := seedRunnableReadyDraft(t, deps)
	// Promote DRAFT → RUNNABLE.
	_, err := h.UpdatePlanConfiguration(context.Background(), connect.NewRequest(&plansv1.UpdatePlanConfigurationRequest{
		ConfigurationId: cfgID.String(),
		Status:          plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
	}))
	if err != nil {
		t.Fatalf("UpdatePlanConfiguration: %v", err)
	}
	if deps.assistant.NextTurnCalls != 1 {
		t.Fatalf("want 1 NextTurn call after status promotion, got %d", deps.assistant.NextTurnCalls)
	}
}

func TestUpdatePlanConfiguration_DoesNotTriggerNextTurnWhenStatusUnchanged(t *testing.T) {
	h, deps := newTestHandler(t)
	cfgID := seedRunnableReadyDraft(t, deps)
	// Update slot bindings but keep status DRAFT.
	_, err := h.UpdatePlanConfiguration(context.Background(), connect.NewRequest(&plansv1.UpdatePlanConfigurationRequest{
		ConfigurationId: cfgID.String(),
		SlotBindings:    []*plansv1.SlotBinding{{StepKey: "a", ExecutorInstallationId: "exec-2"}},
	}))
	if err != nil {
		t.Fatalf("UpdatePlanConfiguration: %v", err)
	}
	if deps.assistant.NextTurnCalls != 0 {
		t.Fatalf("want 0 NextTurn calls when status unchanged, got %d", deps.assistant.NextTurnCalls)
	}
}
```

If `newTestHandler` / `seedRunnableReadyDraft` / the assistant-mock-with-call-counter don't exist yet, add them as test helpers in the same file. Keep the mock minimal — just a counter on `NextTurn`.

- [ ] **Step 3: Run tests — they fail (no NextTurn wiring yet)**

```bash
cd control-plane && go test ./internal/plans/... -run TestUpdatePlanConfiguration_Triggers -v
```

Expected: FAIL.

- [ ] **Step 4: Wire NextTurn into the handler**

In `control-plane/internal/plans/handler.go`'s `UpdatePlanConfiguration` method, after the persistence write succeeds:

```go
// Trigger the assistant turn when status leaves DRAFT — this is what
// fires the LandingCard. Per M6 spec §2.1. Idempotent; harmless on
// repeat.
if previousStatus == plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT &&
	updatedConfig.GetStatus() != plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT &&
	updatedConfig.GetStatus() != plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_UNSPECIFIED {
	if err := h.assistant.NextTurn(ctx, tenantID, configID); err != nil {
		// Log but do not fail the update — the assistant turn is a
		// best-effort UI-side effect; the user has already seen their
		// status change. A subsequent NextTurn call will catch up.
		h.logger.Warn("planassistant.NextTurn after status promotion failed", "err", err, "config", configID.String())
	}
}
```

Use whatever logger field the handler already has; use the existing `tenantID` / `configID` variables from the function scope. If the handler doesn't already hold a `planassistant.Controller` reference, add it to the handler struct + constructor.

- [ ] **Step 5: Run tests**

```bash
cd control-plane && go test ./internal/plans/... -v
```

Expected: ALL PASS.

- [ ] **Step 6: Commit**

```bash
git add control-plane/internal/plans/handler.go control-plane/internal/plans/handler_test.go
git commit -m "feat(ux-m6): trigger planassistant.NextTurn on status promotion"
```

---

## Phase 3 — Plan thread components

Per spec §2.1, §2.2, §2.4. Implements the matrix card, landing card, and assistant prompt dispatcher branch.

### Task 13: Matrix data helpers + cost calculator

**Suggested executor:** Composer 2.5

**Files:**
- Create: `frontend/src/lib/plans/matrix.ts`
- Create: `frontend/src/lib/plans/matrix.test.ts`

**Interfaces:**
- `type MatrixRow = { stepKey: string; stepTitle: string; contracts: { input: string; output: string }; options: ExecutorOption[]; currentExecutorId: string; currentOverseerId: string; currentOverseerLabel: string }`
- `type MatrixPayload = { state: "BINDING_MATRIX"; policies_set: boolean; rows: MatrixRow[] }`
- `parseMatrixPayload(json: string): MatrixPayload` — throws on malformed JSON or wrong state value.
- `computeRunCostBRL(rows: MatrixRow[]): { totalBrl: number; unboundCount: number }` — sums `pricePerRunBrl` of each row's `currentExecutorId` from its `options` (0 if unbound; increment `unboundCount`).
- `isMatrixComplete(payload: MatrixPayload): boolean` — true iff every row has a `currentExecutorId` AND `policies_set` is true.

Tests cover: parsing happy path, parse failure on wrong state, cost sum with mixed bound/unbound rows, isMatrixComplete with empty / partial / complete.

- [ ] **Step 1:** Write `matrix.test.ts` with 5 test cases covering the above. Use the existing `frontend/src/lib/plans/cost.test.ts` as a style reference for vitest patterns in this codebase.
- [ ] **Step 2:** Run `cd frontend && npx vitest run src/lib/plans/matrix.test.ts` — verify FAIL (`Cannot find module './matrix'`).
- [ ] **Step 3:** Implement `matrix.ts` per the interface above.
- [ ] **Step 4:** Run the test — verify PASS (5 tests).
- [ ] **Step 5:** Commit: `feat(ux-m6): matrix.ts data helpers + cost calculator`.

---

### Task 14: BindingMatrixCard component

**Suggested executor:** GPT 5.5 xhigh (layout + interaction logic)

**Files:**
- Create: `frontend/src/lib/components/thread/BindingMatrixCard.svelte`

**Visual reference:** the mockups at `.superpowers/brainstorm/512184-1782168073/content/binding-matrix.html` and `binding-comparison.html` (right panel). Match spacing, type weights, row layout.

**Props:**
- `message: ThreadMessage` (carries the `payload_json` parsed via `parseMatrixPayload`)
- `configurationId: string`
- `tenantId: string`
- `executorCatalog: ExecutorCatalog` (for cost lookups; reuse existing type from M5)
- `currentUserDisplayName: string` (defaults to "You")

**Behavior:**
- Renders the assistant intro line (`message.text`) above the card.
- Card header: template name (load via `configurationId` → existing `getConfiguration` helper), `N of M bound` progress pill, template meta line (`M tasks · ~X min/run` if available).
- Per-row: step number circle (gold-filled when bound, plumage-bordered when unbound), task name + contract chip (`(input) → output`), executor picker (renders `row.options` in a dropdown — clicking commits via `selectExecutor` from `lib/plans/assistant.ts`), overseer cell (avatar + name + pencil on hover for override).
- Policies footer row: three columns (Timeout / Approval / Schedule) with inline editable fields. Saving any field calls `UpdatePlanConfiguration` directly via the existing `lib/plans/client.ts` (or equivalent) RPC wrapper.
- Card footer: cost pill on the left (uses `computeRunCostBRL`), Save buttons on the right (`Save draft` secondary, `Save & make runnable` primary; primary disabled until `isMatrixComplete`).
- Save & make runnable: calls `UpdatePlanConfiguration` with `status: PLAN_CONFIGURATION_STATUS_RUNNABLE` AND `status: PLAN_CONFIGURATION_STATUS_SCHEDULED` if `schedule.cron_expression` is non-empty. On success, optimistically writes a `USER_SELECTION` chat message with `payload_json: {state:"binding-matrix",action:"save",bindings_summary:[...]}` for audit (per spec §2.1 trigger model).
- Use `cardLift` from `lib/motion/transitions.ts` on row hover. Use surface tokens: card body `bg-surface-elevated`, row hover `bg-surface-hover`, picker dropdown `bg-surface-pop`.

- [ ] **Step 1:** Create the component file. Use the mockup HTML as the visual reference; translate to Svelte 5 component syntax matching the codebase style (see `frontend/src/lib/components/thread/AssistantPromptCard.svelte` for current Svelte 5 patterns in this project).
- [ ] **Step 2:** Verify type-check: `cd frontend && npm run check`. Expected: 0 errors.
- [ ] **Step 3:** Spot-check in dev: `cd frontend && npm run dev`, navigate to any configuration's thread, confirm the card renders with the right surface layering and the Save button enables/disables correctly when bindings change.
- [ ] **Step 4:** Commit: `feat(ux-m6): BindingMatrixCard component`.

---

### Task 15: LandingCard component with save-celebration motion

**Suggested executor:** GPT 5.5 xhigh

**Files:**
- Create: `frontend/src/lib/components/thread/LandingCard.svelte`

**Props:**
- `message: ThreadMessage` (carries `payload_json` with `state: "landing"` and `actions` array)
- `configurationId: string`
- `tenantId: string`

**Behavior:**
- Renders the assistant narrative line (`message.text`) above the card.
- Card body: a checkmark SVG that stroke-draws on mount, three primary action chips (`Run now`, `Schedule…`, `Save and walk away`) stagger-fading in (60ms each).
- Action wiring:
  - `Run now` → calls the existing `TriggerExecution` (or equivalent) RPC for `configurationId`; on success appends a `SystemEventCard` "Run started" via the existing message pattern.
  - `Schedule…` → opens `ScheduleDialog` (existing M5 component at `frontend/src/lib/components/canvas/ScheduleDialog.svelte`).
  - `Save and walk away` → no-op; the LandingCard dismisses (writes a small `ASSISTANT_TEXT` "Plan saved." or similar quiet system note).
- Apply `saveCelebration` transition from `lib/motion/transitions.ts` to the card on mount (gold breathe + reveal). Use `chatEnter` for the action chip stagger.
- Surfaces: card body `bg-surface-elevated`, primary button `bg-primary text-on-primary`, secondary chips `bg-surface-hover`.
- Reduced motion: `saveCelebration` collapses via the guard in `transitions.ts` automatically.

- [ ] **Step 1:** Create the component file.
- [ ] **Step 2:** Verify type-check: `cd frontend && npm run check`.
- [ ] **Step 3:** Spot-check in dev: trigger a save flow end-to-end (BindingMatrix → Save → LandingCard appears). Verify the celebration breathe is subtle (not gimmicky) and the three actions are clearly distinct.
- [ ] **Step 4:** Commit: `feat(ux-m6): LandingCard with save-celebration motion`.

---

### Task 16: AssistantPromptCard dispatcher branches on state

**Suggested executor:** Composer 2.5

**Files:**
- Modify: `frontend/src/lib/components/thread/AssistantPromptCard.svelte`

**Change:** the existing dispatcher branches on `message.payload_json.state`. Add cases:
- `state === "BINDING_MATRIX"` → render `<BindingMatrixCard {message} {configurationId} {tenantId} {executorCatalog} {currentUserDisplayName} />`
- `state === "landing"` → render `<LandingCard {message} {configurationId} {tenantId} />`
- Existing chip-group fallback stays as the default branch for any unknown state (defensive — shouldn't fire post-M6 since BINDING_STEP / SET_OVERSEER / CONFIRM no longer emit).

- [ ] **Step 1:** Read current `AssistantPromptCard.svelte` to find its dispatch site. Add the two new branches before the fallback.
- [ ] **Step 2:** Verify type-check: `cd frontend && npm run check`.
- [ ] **Step 3:** Commit: `feat(ux-m6): AssistantPromptCard branches on BINDING_MATRIX + landing`.

---

### Task 17: Delete ConfirmCard

**Suggested executor:** Composer 2.5

**Files:**
- Delete: `frontend/src/lib/components/thread/ConfirmCard.svelte`

- [ ] **Step 1:** Confirm no remaining imports: `cd frontend && grep -rn "ConfirmCard" src/`. Expected: only the file itself.
- [ ] **Step 2:** Delete the file: `rm frontend/src/lib/components/thread/ConfirmCard.svelte`.
- [ ] **Step 3:** Verify type-check + smoke run: `cd frontend && npm run check && npm run dev`. Navigate the configuration flow; confirm nothing references ConfirmCard.
- [ ] **Step 4:** Commit: `chore(ux-m6): delete ConfirmCard (folded into LandingCard)`.

---

## Phase 4 — Canvas: live state + edge anchors + hover preview

Per spec §2.3.

### Task 18: PlanCanvas live state subscription + node-state vocabulary + vignette

**Suggested executor:** GPT 5.5 xhigh

**Files:**
- Modify: `frontend/src/lib/components/canvas/PlanCanvas.svelte`

**Change summary:**
- Add a Svelte 5 `$effect` subscribing to the configuration store the BindingMatrixCard reads from (use whichever pattern M5 introduced — likely a derived store keyed by `configurationId`). When bindings change, re-derive each node's state and re-render.
- Implement the node-state vocabulary per spec §2.3:
  - `unbound`: fill `--token-surface-elevated`, 1px dashed `--token-text-muted-dark` border.
  - `bound`: gold inner ring (2px `--token-primary`), embedded executor icon (from `executorCatalog` lookup), solid border `--token-border`.
  - `running`: pulse via `createNodeStatePulse` tween (opacity 1.0 ↔ 0.55 over 1.4s, loop).
  - `waiting`: static 3px outer gold halo.
  - `done`: solid check glyph (use `lucide-svelte` `Check` icon at 16px gold), full opacity.
  - `failed`: 1px red border (`#d9534f`), error icon.
- Replace the dotted-grid canvas background with a CSS radial vignette: `background: radial-gradient(circle at center, var(--token-surface-deep) 0%, #080a0d 100%);`.
- Empty state: when `template` is null/empty, render a single dashed-outline placeholder node with text "Pick a template to see the plan."
- Loading state: when configuration is still loading, render `<Skeleton count={3} width="120px" height="60px" />` arranged horizontally.

**Acceptance:**
- In dev: navigate to `/plans/configurations/[id]` (existing M5 route). The inline DAG card at the top should show all nodes in correct initial state.
- Open the BindingMatrixCard, pick an executor for an unbound row. The corresponding node in the inline DAG should flip from `unbound` to `bound` within ~200ms (same frame as the matrix card row update).
- Navigate to `/plans/[id]/canvas` (existing M4 route). Same component, full-bleed. Background should show the radial vignette.

- [ ] **Step 1:** Read current `PlanCanvas.svelte` end-to-end. Identify the configuration data source it uses and the node-render function.
- [ ] **Step 2:** Add the subscription + state derivation. Inline new CSS rules in the `<style>` block; do not introduce a new CSS file.
- [ ] **Step 3:** Verify type-check: `cd frontend && npm run check`.
- [ ] **Step 4:** Spot-check in dev: walk the acceptance scenario above; screenshot the live state transition for the verification log.
- [ ] **Step 5:** Commit: `feat(ux-m6): PlanCanvas live state + node vocabulary + vignette`.

---

### Task 19: NodeHoverCard popover

**Suggested executor:** Composer 2.5

**Files:**
- Create: `frontend/src/lib/components/canvas/NodeHoverCard.svelte`

**Props:**
- `node: { stepKey: string; stepTitle: string; contracts: { input: string; output: string }; executorName: string | null; executorTier: string | null; pricePerRunBrl: number | null; overseerLabel: string; state: NodeState }`
- `anchorRect: DOMRect` (position above the node)

**Visual:**
- Floating popover, `bg-surface-pop`, 1px `border-border`, 8px rounded, 12px padding.
- Contents: task title (bold, 13px), contract chips (`(input) → output` in mono 10px muted), executor row (icon + name + tier + price), overseer row (avatar + name), state label.
- Anchored above the node with a 6px gap.

**Integration in `PlanCanvas.svelte`:**
- On `mouseenter` of a node, set a 150ms timeout, then render `<NodeHoverCard {node} {anchorRect} />`.
- On `mouseleave`, set an 80ms timeout, then unmount.
- Cancel pending timeouts on opposite events.
- Click still opens the right-side detail pane (existing M4 behavior — leave it alone). The popover dismisses on any click.

- [ ] **Step 1:** Create `NodeHoverCard.svelte`.
- [ ] **Step 2:** Wire the hover handlers in `PlanCanvas.svelte`.
- [ ] **Step 3:** Type-check + dev verify.
- [ ] **Step 4:** Commit: `feat(ux-m6): NodeHoverCard popover for canvas`.

---

### Task 20: Edge anchor-point fix

**Suggested executor:** GPT 5.5 xhigh (geometry)

**Files:**
- Modify: whichever file currently renders edges (likely `frontend/src/lib/components/canvas/EdgePath.svelte` or inline inside `PlanCanvas.svelte` — locate via `grep -rn "<path" frontend/src/lib/components/canvas/`).

**Change:**
- Replace any node-center-to-node-center edge path with anchor-point computation: `source.x + source.width` (right edge midpoint Y) → `target.x` (left edge midpoint Y).
- Use a bezier with a horizontal control offset of `(target.x - sourceRight) * 0.4` for natural flow.
- Render the edge type label as a small chip (`bg-surface-elevated`, 9px mono, 4px padding) anchored at the bezier midpoint via `getPointAtLength(pathLength / 2)`.

**Pseudocode:**
```ts
const sourceRight = source.x + source.width;
const sourceMidY = source.y + source.height / 2;
const targetLeft = target.x;
const targetMidY = target.y + target.height / 2;
const controlOffset = (targetLeft - sourceRight) * 0.4;
const path = `M ${sourceRight} ${sourceMidY} C ${sourceRight + controlOffset} ${sourceMidY}, ${targetLeft - controlOffset} ${targetMidY}, ${targetLeft} ${targetMidY}`;
```

- [ ] **Step 1:** Locate current edge rendering code.
- [ ] **Step 2:** Replace with anchor-based path. Add the midpoint chip.
- [ ] **Step 3:** Verify visually in dev: edges should leave node right edges and arrive at node left edges precisely; no floating gaps.
- [ ] **Step 4:** Commit: `fix(ux-m6): edge anchoring + midpoint type chips`.

---

## Phase 5 — Sidebar count badges

Per spec §2.6.

### Task 21: SidebarItem accepts count prop + renders badge

**Suggested executor:** Composer 2.5

**Files:**
- Modify: `frontend/src/lib/components/sidebar/SidebarItem.svelte` (locate; may be named differently — `grep -l "sidebar" frontend/src/lib/components/` to find).

**Change:**
- Add optional `count?: number` prop (default `undefined`).
- When `count !== undefined && count > 0`, render a small gold pill to the right of the label: `bg-primary text-on-primary`, 11px, 4px horizontal padding, 999px radius. Use `createCountTween` from `lib/motion/springs.ts` so the displayed number animates on change.
- When `count === 0` or undefined, render nothing (no "0" badge).

- [ ] **Step 1:** Locate the sidebar item component.
- [ ] **Step 2:** Add prop + badge render with the count tween.
- [ ] **Step 3:** Type-check + visual verify (the next task wires real counts; this one just renders correctly when given a count).
- [ ] **Step 4:** Commit: `feat(ux-m6): SidebarItem count badge with tween`.

---

### Task 22: Wire Needs you + Your plans count subscriptions

**Suggested executor:** Composer 2.5

**Files:**
- Modify: `frontend/src/lib/components/sidebar/Sidebar.svelte` (or the file that mounts sidebar items; locate via grep).

**Change:**
- For `Needs you`: subscribe to the M2 inbox aggregator stream (existing — find the helper in `frontend/src/lib/inbox/` or similar). Pass `count={pendingCount}` to the SidebarItem.
- For `Your plans`: subscribe to `ListPlanConfigurations` (existing M5 stream in `frontend/src/lib/plans/`). Filter client-side to status `RUNNABLE` or `SCHEDULED`. Pass `count={runnableCount}` to the SidebarItem.
- Use Svelte 5 `$effect` + AbortController to clean up subscriptions on unmount (mirror the pattern M5 §2.12 established).

- [ ] **Step 1:** Locate the existing inbox and configurations subscriptions used elsewhere in M2/M5.
- [ ] **Step 2:** Wire them into the sidebar component with cleanup.
- [ ] **Step 3:** Verify in dev: trigger an inbox item to appear/resolve — the `Needs you · N` badge should tick live without page reload.
- [ ] **Step 4:** Commit: `feat(ux-m6): live Needs you + Your plans count badges in sidebar`.

---

## Phase 6 — Mechanical sweep (motion + tokens)

This is the bulk-quality step. Every existing card, list, and chip component gets reviewed and aligned to the M6 motion library + 5-layer surface system + gold discipline.

### Task 23: Sweep — adopt motion + surface tokens across components

**Suggested executor:** Composer 2.5 (mechanical, repetitive — well suited to it; reference `docs/design/m6-tone-system.md` for the rules)

**Files (all under `frontend/src/lib/components/`):**
- Every `*.svelte` file in `lib/components/` and `lib/components/{thread,canvas,sidebar,inbox}/`. Use `find frontend/src/lib/components -name "*.svelte"` to enumerate.

**Per-file changes:**
1. Replace inline hover transitions / shadow rules with imports from `lib/motion/transitions.ts` (`cardLift` for hover-able cards, `chipFlash` for buttons/chips).
2. Replace spinner usage with `<Skeleton>` from `lib/components/Skeleton.svelte`. Grep for `<Spinner` / `spinner` / `loading...` to find spinner sites.
3. Audit surface tones against the 5-layer system in `docs/design/m6-tone-system.md`:
   - Sidebar containers → `bg-surface-deep`.
   - Card bodies → `bg-surface-elevated`.
   - List row hover → `bg-surface-hover`.
   - Dropdowns / popovers → `bg-surface-pop`.
4. Audit gold usage against the discipline table. Strip gold from decorative dividers, generic section labels, body text emphasis. Keep gold on: primary buttons, active nav items, count pills, focus rings, cost amounts, bound-node accents, save celebration.
5. Audit muted text usage:
   - Information (timestamps, contracts, secondary labels) → `text-text-muted` (`#9da1ab`).
   - Tertiary chrome (placeholders, "or" separators) → `text-text-muted-dark` (`#6b7080`).
   - Never use `-muted-dark` for information.

**Per-component checklist:**
- [ ] Hover transitions go through `cardLift`.
- [ ] Click feedback uses `chipFlash`.
- [ ] Loading uses `<Skeleton>`, not a spinner.
- [ ] Surface tier matches the layer map.
- [ ] Gold appears only per the discipline table.
- [ ] Muted text follows information-vs-chrome rule.

**Commit strategy:** one commit per ~5-10 components, grouped by area (thread, canvas, inbox, sidebar). Conventional commit format: `chore(ux-m6): apply motion + tokens to <area> components`.

**Acceptance:**
- `npm run check`: 0 errors.
- `npm run lint`: clean.
- Manual: navigate every Ana-facing surface (`/inbox`, `/plans/configurations/[id]`, `/plans/[id]/canvas`, `/discover`, `/new`). Each should show consistent hover behavior, no raw spinners, intentional gold-accent usage, and proper layer differentiation.

- [ ] **Step 1:** Enumerate component files: `find frontend/src/lib/components -name "*.svelte" | sort > /tmp/m6-sweep-files.txt`. Review the list; for each file, perform the per-file changes above.
- [ ] **Step 2:** Commit progressively per area. Run `npm run check` after each area commit.
- [ ] **Step 3:** Final lint pass: `cd frontend && npm run lint`. Fix any issues.

---

## Phase 7 — IA cleanup

Per spec §2.8 and the no-pre-v1-compat constraint: delete cleanly, no grace periods.

### Task 24: Delete legacy admin-duplicate routes

**Suggested executor:** Composer 2.5 (trivial)

**Files (delete entire directories):**
- `frontend/src/routes/agents/`
- `frontend/src/routes/integrations/`
- `frontend/src/routes/audit/`
- `frontend/src/routes/settings/`

The canonical versions live under `frontend/src/routes/admin/{agents,integrations,audit,settings}/`.

- [ ] **Step 1:** Confirm no internal links reference the legacy paths.

```bash
cd frontend && grep -rn "href=['\"]/\(agents\|integrations\|audit\|settings\)" src/ \
  && echo "REMAINING REFERENCES — fix them before deletion" || echo "clean — safe to delete"
```

Expected: clean. If any references exist, update them to `/admin/<name>` first.

- [ ] **Step 2:** Delete:

```bash
rm -rf frontend/src/routes/agents frontend/src/routes/integrations frontend/src/routes/audit frontend/src/routes/settings
```

- [ ] **Step 3:** Verify build + smoke run:

```bash
cd frontend && npm run check && npm run dev
```

Navigate manually to `/admin/agents`, `/admin/integrations`, `/admin/audit`, `/admin/settings`. Each should load. `curl -I http://localhost:5173/agents` should return 404.

- [ ] **Step 4:** Commit: `chore(ux-m6): delete legacy /agents /integrations /audit /settings (admin/ canonical)`.

---

### Task 25: Delete M2 transitional redirects

**Suggested executor:** Composer 2.5

**Files (delete entire directories):**
- `frontend/src/routes/oversee/`
- `frontend/src/routes/elicitations/`
- `frontend/src/routes/approvals/`
- `frontend/src/routes/tasks/`

These were 302 redirects to `/inbox` (and `/tasks` was already gone-but-stubbed). No grace period needed.

- [ ] **Step 1:** Confirm no internal links reference these paths.

```bash
cd frontend && grep -rn "href=['\"]/\(oversee\|elicitations\|approvals\|tasks\)" src/ \
  && echo "REMAINING REFERENCES" || echo "clean"
```

Expected: clean (M2's commit already swept these).

- [ ] **Step 2:** Delete:

```bash
rm -rf frontend/src/routes/oversee frontend/src/routes/elicitations frontend/src/routes/approvals frontend/src/routes/tasks
```

- [ ] **Step 3:** Verify: `cd frontend && npm run check`. `curl -I http://localhost:5173/oversee` returns 404.
- [ ] **Step 4:** Commit: `chore(ux-m6): delete /oversee /elicitations /approvals /tasks redirects`.

---

### Task 26: Delete M5 wizard redirects

**Suggested executor:** Composer 2.5

**Files:**
- Delete entire directory: `frontend/src/routes/plans/[templateId]/configure/`

This directory contained 302 redirects to `/new?template=<id>` (added by M5). No grace period needed.

- [ ] **Step 1:** Confirm no internal links: `cd frontend && grep -rn "configure" src/routes/ src/lib/`. Expected: only docs references / commit-message-style strings; no live `href`.
- [ ] **Step 2:** Delete: `rm -rf frontend/src/routes/plans/\[templateId\]/configure`.
- [ ] **Step 3:** Verify: `cd frontend && npm run check`. The "Use this template" CTA on `/plans/[templateId]/+page.svelte` (existing) should still route to `/new?template=<id>` directly (M5 already retargeted it; confirm).
- [ ] **Step 4:** Commit: `chore(ux-m6): delete M5 wizard transitional redirects`.

---

### Task 27: Dev-login persona collapse

**Suggested executor:** Composer 2.5

**Files:**
- Modify: `frontend/src/routes/login/+page.svelte` (or wherever dev-login lives — locate via `grep -rn "devLogin\|persona" frontend/src/routes/login/`)
- Possibly: `frontend/src/lib/auth/` helpers if persona constants are centralized.

**Change:**
- Drop the `Leader` and `Overseer` persona buttons entirely.
- Rename the `Engineer` button to `Platform Engineer`.
- Add an `Ana` button mapped to `seat:operator` permission.
- Ensure the persona toggle in the user menu (already shipped per recent commit `fix(nav): persona toggle routes to new persona's home`) still works after the collapse — it should only see Ana and Platform Engineer as switchable options.

- [ ] **Step 1:** Locate the dev-login persona definitions and the persona-toggle wiring.
- [ ] **Step 2:** Apply the collapse: 2 buttons (Ana, Platform Engineer) instead of 3.
- [ ] **Step 3:** Verify in dev: visit `/login` (or wherever dev-login is exposed), confirm only 2 buttons render. Sign in as each; confirm sidebar shows the persona-appropriate items.
- [ ] **Step 4:** Commit: `chore(ux-m6): collapse dev-login personas to Ana + Platform Engineer`.

---

### Task 28: Update project-dev-login memory file

**Suggested executor:** Composer 2.5 (memory file edit)

**Files:**
- Modify: `~/.claude/projects/-home-thbertoldi-harpia/memory/project-dev-login.md`

- [ ] **Step 1:** Read current contents. The current file claims "3 personas (Leader/Overseer/Engineer)" which is stale after Task 27.
- [ ] **Step 2:** Update the description and body to reflect "2 personas (Ana operator / Platform Engineer)". Note that Leader and Overseer were removed in M6. Keep the rest of the file's claims about bypassing Zitadel intact.
- [ ] **Step 3:** This memory file is outside the repo — no git commit. The change persists in `~/.claude/`.

---

## Phase 8 — Zitadel branding

Per spec §2.7.

### Task 29: Create asset directory + logo + background + fonts

**Suggested executor:** Composer 2.5 (asset assembly — needs human or designer for actual SVG; here we scaffold)

**Files:**
- Create: `deploy/dev/kind/assets/` (new directory)
- Create: `deploy/dev/kind/assets/logo.svg` (Harpia wordmark in Bodoni Moda — light variant for dark Zitadel surface)
- Create: `deploy/dev/kind/assets/background.png` (1920x1080 obsidian gradient, ~30KB)
- Create: `deploy/dev/kind/assets/fonts/BodoniModa-Regular.woff2`
- Create: `deploy/dev/kind/assets/fonts/DMSans-Regular.woff2`
- Create: `deploy/dev/kind/assets/fonts/Manrope-Regular.woff2`

**Asset sourcing:**
- Logo SVG: render "Harpia" in Bodoni Moda 64pt, cream `#f5f2eb` fill, no decoration. Save as `logo.svg`. If a designer needs to produce this, leave a placeholder SVG with the wordmark in any serif and mark a TODO comment inside `<!-- TODO: replace with designer Bodoni Moda render -->` — but ship a valid SVG.
- Background PNG: a 1920x1080 image with `radial-gradient(circle at center, #121318 0%, #0a0b0e 100%)`. Render via any image tool or use ImageMagick: `convert -size 1920x1080 radial-gradient:'#121318'-'#0a0b0e' background.png`.
- Fonts: download from Google Fonts (Bodoni Moda, DM Sans, Manrope), convert to WOFF2 using `pyftsubset` or similar, place in `assets/fonts/`. Use the Regular weight only to keep size down. License files (OFL) for each font go in the same directory: `BodoniModa-OFL.txt`, etc.

- [ ] **Step 1:** Create the directory + assets.
- [ ] **Step 2:** Verify sizes are reasonable (`du -sh deploy/dev/kind/assets/`): total <500KB.
- [ ] **Step 3:** Commit: `feat(ux-m6): Zitadel branding assets (logo, fonts, background)`.

---

### Task 30: Email template overrides

**Suggested executor:** Composer 2.5

**Files:**
- Create: `deploy/dev/kind/assets/emails/password-reset.html`
- Create: `deploy/dev/kind/assets/emails/password-reset.txt`
- Create: `deploy/dev/kind/assets/emails/invite.html`
- Create: `deploy/dev/kind/assets/emails/invite.txt`
- Create: `deploy/dev/kind/assets/emails/mfa.html`
- Create: `deploy/dev/kind/assets/emails/mfa.txt`

**Template structure** (use Zitadel's variable syntax — see Zitadel docs at `https://zitadel.com/docs/guides/manage/customize/texts`):
- HTML: inline-styled (no external CSS), dark theme matching Harpia (`#121318` background, `#1a1b24` card, `#f5f2eb` text, `#c8920f` primary), wordmark at top, action button at center, plain `<p>` body.
- TXT: plain text fallback with the same content, no markup.
- Localize: provide both `en` and `pt-BR` text in each template — Zitadel selects per user locale. Use `{{.PreferredLanguage}}` conditional blocks.

- [ ] **Step 1:** Create the six template files with Harpia-branded HTML + TXT for each event type.
- [ ] **Step 2:** Validate HTML in a browser: open each `.html` file directly; confirm it renders correctly.
- [ ] **Step 3:** Commit: `feat(ux-m6): Zitadel email templates (password-reset, invite, mfa)`.

---

### Task 31: Branding ConfigMap (CSS + asset references)

**Suggested executor:** Composer 2.5

**Files:**
- Modify: `deploy/dev/kind/zitadel-branding-configmap.yaml`

**Change:**
- Add a `theme.css` key with the Harpia CSS overrides:

```css
:root {
  --zitadel-bg: #121318;
  --zitadel-surface: #1a1b24;
  --zitadel-text: #f5f2eb;
  --zitadel-primary: #c8920f;
  --zitadel-border: #2a2d3a;
}
body { background: var(--zitadel-bg); color: var(--zitadel-text); font-family: 'DM Sans', system-ui, sans-serif; }
.ztdl-card { background: var(--zitadel-surface); border: 1px solid var(--zitadel-border); }
.ztdl-button-primary { background: var(--zitadel-primary); color: var(--zitadel-bg); }
.ztdl-heading { font-family: 'Manrope', system-ui, sans-serif; }
@font-face { font-family: 'Bodoni Moda'; src: url('/assets/fonts/BodoniModa-Regular.woff2') format('woff2'); }
@font-face { font-family: 'DM Sans'; src: url('/assets/fonts/DMSans-Regular.woff2') format('woff2'); }
@font-face { font-family: 'Manrope'; src: url('/assets/fonts/Manrope-Regular.woff2') format('woff2'); }
```

(Use the actual Zitadel selectors — the above are placeholders; consult Zitadel's CSS class reference at `https://zitadel.com/docs/guides/manage/customize/branding`.)

- Add keys mounting the email templates as configmap entries.

- [ ] **Step 1:** Read current configmap to understand its structure.
- [ ] **Step 2:** Add `theme.css` + email-template keys.
- [ ] **Step 3:** Apply locally: `kubectl apply -f deploy/dev/kind/zitadel-branding-configmap.yaml -n harpia-dev` (or whichever namespace is used). Restart Zitadel pod to pick up.
- [ ] **Step 4:** Commit: `feat(ux-m6): Zitadel branding configmap (theme.css + email templates)`.

---

### Task 32: Zitadel init Job branding upload

**Suggested executor:** Composer 2.5

**Files:**
- Modify: `deploy/dev/kind/zitadel-init.yaml`

**Change:**
- Add init-container or Job steps that POST the assets to Zitadel's private-label-policy API (`/management/v1/policies/label`) and the email-template API (`/management/v1/policies/notification/...`). The Job already uses a PAT (per `project-zitadel-oidc-bootstrap` memory); reuse that auth.
- Steps:
  1. Wait for Zitadel API readiness (existing pattern; likely a curl polling loop).
  2. POST private-label payload referencing `/assets/logo.svg` and `/assets/background.png`.
  3. POST custom-text payloads for `init`, `password-reset`, `verify-email`, `passwordless-registration`, etc. — pulling content from the configmap mounts.

- [ ] **Step 1:** Read current `zitadel-init.yaml` end-to-end.
- [ ] **Step 2:** Add the branding-upload step(s) after Zitadel readiness check.
- [ ] **Step 3:** Apply locally; tear down and rebuild the dev cluster via `tilt up` (or whichever bootstrap command this project uses). Visit `http://zitadel.localhost` (or the configured dev URL); confirm the login screen shows Harpia branding (logo + dark theme + DM Sans).
- [ ] **Step 4:** Commit: `feat(ux-m6): Zitadel init Job uploads branding via private-label API`.

---

### Task 33: Production helm chart parity

**Suggested executor:** Composer 2.5

**Files:**
- Modify: `deploy/harpia/templates/zitadel.yaml`

**Change:**
- Mirror the dev-cluster configmap pattern into the production helm chart. The configmap should be parameterized via `values.yaml` so customers can override the wordmark / background per-tenant if needed (out of scope for M6 but the shape should support it — a single `branding` key in values that maps through).

- [ ] **Step 1:** Read current `zitadel.yaml` template.
- [ ] **Step 2:** Add the configmap + init Job pattern with values.yaml hooks.
- [ ] **Step 3:** Render the helm template locally: `helm template deploy/harpia/ -f deploy/harpia/values.yaml | grep -A10 "branding"`. Confirm output is well-formed YAML.
- [ ] **Step 4:** Commit: `feat(ux-m6): production helm chart Zitadel branding parity`.

---

## Phase 9 — i18n + verification + close

### Task 34: i18n keys (en + pt-BR lockstep)

**Suggested executor:** Composer 2.5

**Files:**
- Modify: `frontend/src/lib/i18n/en.json`
- Modify: `frontend/src/lib/i18n/pt-BR.json`

**New keys** (per spec §3.2):

```
assistant.bindingMatrix.intro          (e.g., "Here's the plan. Pick an executor for each task — overseer defaults to {user}.")
assistant.bindingMatrix.progress       (e.g., "{bound} of {total} bound")
assistant.bindingMatrix.policiesLabels (timeout, approval, schedule labels)
assistant.bindingMatrix.savePrimary    "Save & make runnable"
assistant.bindingMatrix.saveSecondary  "Save draft"
assistant.landing.text                 "Saved. Run it now, schedule a recurring run, or walk away — it'll be here when you come back."
assistant.landing.actions.runNow       "Run now"
assistant.landing.actions.schedule     "Schedule…"
assistant.landing.actions.walkAway     "Save and walk away"
sidebar.needsYou                       "Needs you"   (count interpolated separately)
sidebar.yourPlans                      "Your plans"
cost.perRun                            "per run"
cost.pendingSteps                      "{count} step(s) pending"
motion.reducedAnnounce                 "Reduced-motion mode is on."   (screen-reader-only)
auth.email.passwordReset.subject       (Portuguese + English)
auth.email.passwordReset.heading
auth.email.passwordReset.body
auth.email.passwordReset.ctaButton
auth.email.invite.subject              (same shape)
auth.email.invite.heading
auth.email.invite.body
auth.email.invite.ctaButton
auth.email.mfa.subject
auth.email.mfa.heading
auth.email.mfa.body
```

- [ ] **Step 1:** Add every key above to both files in the same commit. Maintain alphabetical order within each top-level namespace.
- [ ] **Step 2:** Run the i18n parity test if one exists (M5 added `hardcoded-copy.test.ts` parity test — verify it still passes): `cd frontend && npx vitest run src/lib/i18n/`.
- [ ] **Step 3:** Commit: `feat(ux-m6): i18n keys for matrix, landing, sidebar, motion, auth emails`.

---

### Task 35: End-to-end verification log

**Suggested executor:** Composer 2.5 (data collection + writing)

**Files:**
- Create: `docs/superpowers/plans/2026-06-22-harpia-ux-realignment-m6-lapidacao.verification.md`

**Contents:**
- Walk the new flow end-to-end and screenshot/log each step:
  1. Empty state: navigate to `/new`. Pick a template. Redirected to `/plans/configurations/[id]` with the BindingMatrixCard rendered, all rows unbound.
  2. Bind each row by clicking pickers. Inline DAG card at top reacts live (nodes flip unbound → bound).
  3. Edit policies inline in the matrix card footer.
  4. Click `Save & make runnable`. LandingCard appears with the celebration breathe; checkmark draws in; three action chips fade in staggered.
  5. Click `Run now`. Run starts; canvas shows the running node pulsing.
  6. Trigger an Approval/Elicitation in the inbox; `Needs you` sidebar badge ticks up. Resolve it; badge ticks down with tween.
  7. Hover a node in the expanded canvas. NodeHoverCard renders with executor + overseer + cost; dismisses on click.
  8. Navigate every Ana surface (`/inbox`, `/plans/configurations/[id]`, `/plans/[id]/canvas`, `/discover`, `/new`). Verify: consistent hover lift, no raw spinners, intentional gold-accent placement, 5-layer surface differentiation.
  9. Toggle browser devtools "Emulate prefers-reduced-motion: reduce". Re-walk: transitions collapse, focus rings still visible, badge count still ticks (instantly, no tween).
  10. Visit Zitadel `/login` (auth URL). Confirm Harpia branding (logo, dark theme, DM Sans). Trigger password reset; receive branded email.
- Confirm every deleted route returns 404:
  ```bash
  for path in agents integrations audit settings oversee elicitations approvals tasks \
             plans/template-x/configure plans/template-x/configure/overseer; do
    code=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:5173/$path)
    echo "/$path → $code"
  done
  ```
  Expected: all 404.

- [ ] **Step 1:** Run through the walk; capture screenshots in `docs/superpowers/plans/2026-06-22-harpia-ux-realignment-m6-lapidacao.verification/`.
- [ ] **Step 2:** Write the verification log markdown with embedded screenshot links.
- [ ] **Step 3:** Commit: `docs(ux-m6): end-to-end verification log`.

---

### Task 36: Final whole-branch review

**Suggested executor:** Composer 2.5 (orchestration) — actual review can be GPT 5.5 xhigh or a human.

- [ ] **Step 1:** Type-check + lint clean across both stacks:

```bash
cd frontend && npm run check && npm run lint
cd control-plane && go vet ./... && go test ./...
```

Expected: 0 errors / failures.

- [ ] **Step 2:** Run all M6 tests focused: `cd frontend && npx vitest run src/lib/motion src/lib/plans/matrix src/lib/components/Skeleton` and `cd control-plane && go test ./internal/planassistant/... ./internal/plans/...`.

- [ ] **Step 3:** Skim the diff against trunk:

```bash
git fetch origin trunk
git diff --stat origin/trunk...HEAD
git log --oneline origin/trunk..HEAD
```

Confirm every commit is conventional-format, no `Co-Authored-By` trailers, every commit has `(ux-m6)` scope.

- [ ] **Step 4:** Open a PR (or hand to Thiago to open): title `feat(ux-m6): lapidação — finishing pass + IA cleanup residue`. PR body summarizes the seven workstreams and links to the spec + verification log.

