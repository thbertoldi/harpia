## Why

Harpia's OpenCode roster lacks a frontier planning/expert tier, relies on a single normal executor, and still carries stale model aliases, Gemini-only exploration, obsolete architecture guidance, and overly broad mutation permissions. The available GPT-5.6, GLM, and DeepSeek model families make it possible to improve planning quality, executor choice, context capacity, review diversity, and safety now.

## What Changes

- Add a GPT-5.6 Sol primary planner for system-development planning and a directly queryable, read-only Sol expert for the hardest focused problems.
- Add GPT-5.6 Terra as a higher-judgment, model-diverse executor while retaining GLM-5.2 as the default implementation workhorse.
- Replace Gemini exploration with GLM-5.2 and its one-million-token context window.
- Replace the retiring `deepseek-reasoner` reviewer alias with DeepSeek V4 Pro and upgrade GPT-5.5 adjudication to Sol.
- Update orchestration so exactly one executor owns overlapping files, sensitive changes always receive independent senior review, and only actual reasoned disputes reach adjudication.
- Remove stale `README.md` instruction injection, align prompts and the Harpia skill with `AGENTS.md` and the Platform Constitution, and enforce least-privilege Git, GitHub, and nested-agent permissions.
- **BREAKING:** Replace the overlapping `@hard-problem` agent with the explicit `@sol` expert name.

## Capabilities

### New Capabilities

- `opencode-agent-roster`: Defines the available OpenCode planning, exploration, implementation, expert, review, and adjudication roles; their model tiers; routing invariants; authoritative project context; and mutation boundaries.

### Modified Capabilities

<!-- No existing product capability requirements change. -->

## Impact

- Affects `opencode.json`, tracked `.opencode/agent/*.md`, and `.opencode/skills/harpia/SKILL.md`.
- Requires only provider/model access already confirmed by the local OpenCode catalog; credentials and global OpenCode configuration remain unchanged.
- Does not change Harpia runtime code, public APIs, schemas, product behavior, or user-facing copy.
