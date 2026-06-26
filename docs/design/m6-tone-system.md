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
