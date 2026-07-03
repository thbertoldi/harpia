# ADR-016: Design Token Refresh (Harpy Eclipse)

**Status:** Accepted
**Date:** 2026-07-02
**Deciders:** thbertoldi

**References:**
- ADR-005: Development and Delivery (mise, Tilt, trunk-based)
- ADR-012: Plan-Centric Task Model (Artifact, ArtifactType, "UI surfaces are browsers over the Artifact stream")
- AGENTS.md: "Design tokens (colors, fonts) are LOCKED. Animate layout/opacity/transform only."

## Context

Harpia's design tokens — gold-on-warm-obsidian with the harpy-eagle palette (`talon-gold`,
`obsidian`, `plumage`, `cream`, `crown-ash`) and Manrope/DM Sans/Bodoni typography — were
locked early to keep the surface stable. The token layer is well structured: a 5-layer
surface system (`--token-surface-deep/-/…-elevated/-hover/-pop`), semantic Tailwind v4
mappings, shadcn-style aliases, and per-theme overrides (`default`, `aiuna`, `tenant-base`,
`brand`) via `data-theme` on `:root` (see `frontend/src/lib/themes/tokens.css`).

The upcoming chat UX — a **live execution tracker**, an **artifact preview slide-over**, and
**catalog-driven suggestions** (inspired by a Fable reference design) — needs richer surface
treatment than the current palette expresses cleanly. Three forces:

1. **Generative/active state has no dedicated accent.** Today "running", "generating",
   streaming cursors, and active pills all reuse gold (`--token-primary`), which is also the
   *brand/identity* color. Gold-for-everything flattens the visual hierarchy: a running step
   is visually equal to a primary CTA.
2. **Status semantics are implicit.** Success/running/pending are ad-hoc hex literals
   scattered in components. There is no `--token-status-*` role, so contrast and
   consistency are unchecked.
3. **Depth reads as flat.** The reference design's "wow" comes from layered depth (a cooler
   near-black base + gradient glows), not from any single color. Our warm obsidian base is
   fine but slightly muddies the accent layering.

Fonts are good and stay unchanged. This is a **color/depth** refresh only.

## Decision

Introduce a dual-accent token system — **Harpy Eclipse** — that keeps gold as brand identity
and adds an **electric teal "energy" accent** for generative/active state, cools the obsidian
base, and formalizes semantic status tokens. All motion continues to animate layout/opacity/
transform only; gradients are static background fills.

### 1. Retain gold as brand/premium identity

`--token-primary` / `--token-primary-bright` (gold) remain the **identity** layer: wordmark,
brand lockups, the gold stripe, premium CTAs, and the focus ring. Gold is **not** used for
generative/active/running state after this change — that role moves to the energy accent.

### 2. Add an electric "energy" accent for generative/active state

New tokens, theme-overridable but **teal by default across all themes** so the "generative"
signal is universally recognizable:

```
--token-energy: #2dd4bf;            /* primary active/generative accent */
--token-energy-bright: #5eead4;
--token-energy-soft: rgba(45, 212, 191, 0.14);   /* fills, chips, halos */
--token-energy-gradient-from: #5eead4;
--token-energy-gradient-via:  #2dd4bf;
--token-energy-gradient-to:   rgba(45, 212, 191, 0.25);
```

Used for: the **running** step state, the **generating** skeleton shimmer, the live
**"Executing…"** header pill, the streaming cursor, and active/selected artifact cards. This
is the Fable "violet energy" role, recolored to be distinct from generic AI products.

### 3. Cool the obsidian base for depth

Refine the 5-layer surface scale toward a slightly bluer near-black so accent layering
reads with more depth (Fable uses a `zinc-950`-like base; we adopt a comparable cool obsidian
without changing the token *names*):

```
--token-surface-deep:    #08090d;
--token-surface:         #101218;
--token-surface-elevated:#181a22;
--token-surface-hover:   #1f212b;
--token-surface-pop:     #282b36;
```

Border and text tokens shift a few points to match. This is a value refinement, not a rename;
all existing semantic aliases (`--color-surface`, `-elevated`, …) and Harpia palette aliases
(`--color-obsidian*`, `-plumage`, …) continue to resolve unchanged.

### 4. Formalize semantic status tokens

Introduce status roles so success/running/pending/danger are never ad-hoc literals:

```
--token-status-running: var(--token-energy);
--token-status-done:    #34d399;     /* emerald-400 on dark */
--token-status-pending: var(--token-text-muted-dark);
--token-danger:         #f87171;     /* brighten for dark-bg contrast (was #d9534f) */
```

### 5. Motion policy unchanged (restated)

Gradients (brand gold + energy teal) are **static** background fills. The only animated
properties are opacity, transform, and layout dimensions — already enforced by the global
reduced-motion guard in `app.css`. No transition animates color, font, or token hue.

### 6. Themes

- `default.css` adopts the Harpy Eclipse values above.
- `aiuna.css` keeps its purple **identity** primary, but adopts the **same teal energy
  accent** and status tokens, so generative state is consistent across themes.
- `tenant-base.css` / `brand.css` override mechanism is unchanged; they gain the new tokens
  as optional overrides (falling back to defaults).

### 7. Usage discipline

New components MUST prefer semantic tokens (`--color-energy`, `--color-status-running`,
`--color-surface-*`) over raw hex. A follow-up audit removes raw color literals from existing
components that currently hardcode status colors.

## Rationale

- **Identity vs. energy, separated.** Keeping gold for brand and adding teal for generative
  state restores hierarchy: a running step is visibly distinct from a primary CTA. One accent
  (gold) was overloaded.
- **Distinctive, not generic.** Every AI product uses violet/indigo for "generative". Gold +
  electric teal is striking and on-brand for Harpia without copying the reference palette.
- **Cheap and contained.** This is a token-value refresh plus a handful of new roles — no
  component API changes, no new primitives, no migration. Pre-v1, it ships forward-only.
- **Accessibility-first.** Centralizing status colors into tokens lets us WCAG-check them once
  and guarantee contrast on the dark base, instead of per-component.

### Alternatives Considered

| Alternative | Why not |
|---|---|
| **B — Gold + indigo/violet** | Closest to the Fable reference's energy; familiar "AI" cue. Rejected: indistinct — indistinguishable from every other AI product. |
| **C — Full "Forge" departure (amber→magenta)** | Most dramatic. Rejected: abandons the established harpy-eagle identity and needs the most rework for limited gain. |
| **Keep status quo (gold-for-everything)** | No new token work. Rejected: does not solve the overloaded-accent or implicit-status problems that the new UX depends on. |
| **Make the energy accent theme-specific (purple for aiuna)** | Maximally flexible. Rejected: the "generative/active" signal should be consistent across themes for recognition; identity (gold/purple) is the only theme-variable color. |

## Consequences

### What becomes easier

- Expressing generative/active UI (live execution, generating skeletons, status pills) with a
  dedicated, high-contrast accent.
- Consistent, WCAG-checkable status colors via semantic tokens.
- Deeper, more premium-feeling surfaces without changing any component API.

### What becomes harder

- Every theme package must define the new tokens (defaults cover gaps, but explicit is better).
- A usage-discipline audit to migrate raw status literals to the new semantic tokens.
- Designers/contributors must learn the **identity (gold) vs. energy (teal)** distinction and
  not reach for gold for active state.

### Implementation status

1. Accept ADR — Accepted 2026-07-02.
2. Extend `frontend/src/lib/themes/tokens.css` with energy + status tokens and the cooled
   surface scale; update `@theme` mappings and Harpia palette aliases.
3. Update `default.css` and `aiuna.css`; verify `tenant-base.css`/`brand.css` fallbacks.
4. WCAG AA contrast pass on the new accents against the dark base.
5. Audit existing components for raw status literals → migrate to `--color-status-*`.
6. Land before the three dependent OpenSpec changes so they inherit the tokens.

### Out of scope

- Typography changes (fonts stay).
- Installing shadcn-svelte (orthogonal; decided against in the artifact-preview-panel change).
- Token *naming* changes / alias removals (pure value + role additions).
- Animating colors or token hues (forbidden by the motion policy).
