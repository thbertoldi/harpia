## 1. Project Configuration and New Roles

- [x] 1.1 Update `opencode.json` to rely on auto-loaded `AGENTS.md`, allowlist safe read-only Git inspection, and approval-gate Git mutations and GitHub MCP actions; verify with `jq empty opencode.json`.
- [x] 1.2 Add `.opencode/agent/planner.md` as a GPT-5.6 Sol primary planning agent with maximum reasoning, planning-first permissions, and an executor-cold deliverable contract.
- [x] 1.3 Replace `.opencode/agent/hard-problem.md` with `.opencode/agent/sol.md` as the directly queryable, read-only GPT-5.6 Sol expert and remove every `@hard-problem` reference.
- [x] 1.4 Add `.opencode/agent/terra.md` as the GPT-5.6 Terra higher-judgment executor with the same repository invariants and applicable verification gates as the workhorse.

## 2. Routing, Review, and Guidance Alignment

- [x] 2.1 Update `.opencode/agent/orchestrator.md` with an explicit subagent allowlist, Sol/Terra routing, single-writer ownership, corrected sensitive-path gates, mandatory layered review, and adjudication only for actual disputes.
- [x] 2.2 Move `.opencode/agent/explorer.md` to `zai-coding-plan/glm-5.2` and align its evidence contract and domain source-of-truth wording with the Platform Constitution.
- [x] 2.3 Update `.opencode/agent/implementer.md` and `.opencode/agent/scut.md` to prevent nested delegation/external writes, escalate hard decisions to `@sol`, and run every applicable gate from `AGENTS.md`.
- [x] 2.4 Upgrade `.opencode/agent/reviewer-senior.md` to `deepseek/deepseek-v4-pro`, upgrade `.opencode/agent/adjudicator.md` to GPT-5.6 Sol, and align both reviewer prompts plus `.opencode/agent/reviewer-fast.md` with current guidance and least privilege.
- [x] 2.5 Rewrite `.opencode/skills/harpia/SKILL.md` as a concise router to `AGENTS.md`, the Platform Constitution, and task-relevant companion documents without duplicating mutable architecture.

## 3. Validation

- [x] 3.1 Confirm every configured model ID with `opencode models openai`, `opencode models zai-coding-plan`, and `opencode models deepseek`.
- [x] 3.2 Resolve the roster with `opencode agent list`, verify `planner`, `sol`, and `terra` are present and `hard-problem` is absent, and inspect the resolved permission rules for the new/changed agents.
- [x] 3.3 Search tracked OpenCode configuration for stale Gemini explorer, GPT-5.5, `deepseek-reasoner`, `@hard-problem`, old five-context/task-management guidance, and broad silent `git *` allowance; resolve every hit in scope.
- [x] 3.4 Run `openspec validate modernize-opencode-agent-roster --strict`, `git diff --check -- opencode.json .opencode/agent .opencode/skills/harpia/SKILL.md openspec/changes/modernize-opencode-agent-roster`, and confirm no unrelated file was changed by this work.
