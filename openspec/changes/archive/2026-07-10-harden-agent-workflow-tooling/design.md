## Context

The just-archived roster modernization established Harpia-specific planning, execution, review, and adjudication roles, but intentionally left the default primary agent unchanged. The project config still inherits permissive skill discovery and built-in development agents, writer command policies are copied across three files, generated OpenSpec adapters are untracked, and verification evidence comes from the writer that produced the diff. The installed OpenCode CLI is now 1.17.18 while its ignored local plugin manifest remains on 1.15.13.

This change is repository-development tooling. It must preserve the existing dirty roster work, avoid product-side Executor terminology changes, and keep provider-catalog access out of deterministic CI.

## Goals / Non-Goals

**Goals:**

- Make the Harpia roster the effective default and close obvious bypasses.
- Minimize the skill surface while retaining the repository and stack workflows engineers need.
- Separate execution from verification and make policy/configuration drift machine-detectable.
- Make OpenSpec authoring policy and strict validation consistent across developers and CI.
- Pin mutually coupled OpenCode CLI/plugin versions in tracked configuration.

**Non-Goals:**

- Change Harpia runtime agents, product-side ExecutorSKUs, provider credentials, global user configuration, or model entitlements.
- Guarantee that optional remote model catalogs are reachable in CI.
- Track unrelated Claude, Codex, Cursor, MCP, or personal tooling adapters currently present in the worktree.
- Replace OpenSpec with another planning format or duplicate policy text from canonical documents.

## Decisions

### 1. Select the custom roster and disable bypass roles

Set `default_agent` to `orchestrator`, `model` to the orchestrator's GLM workhorse, and `small_model` to a free lightweight OpenCode model for title/summary work. Disable built-in `build`, `plan`, `explore`, and the broadly capable `general` subagent; retain hidden system roles and non-writing provider/plugin helpers. This is safer than merely documenting preferred roles because directly invoked built-ins can bypass Harpia's routing and review contracts.

### 2. Deny skills by default and allow only named project/stack skills

Configure `permission.skill` with `* = deny`, followed by explicit allowances for `harpia`, all five `openspec-*` skills, and a small stack-relevant set covering gRPC, Python, PostgreSQL, Temporal, Kubernetes/Helm, observability, Svelte/Tailwind/chat UI, and accessibility. OpenCode evaluates the last matching rule, so the catch-all must come first. This keeps BMAD/WDS and arbitrary externally scanned skills unavailable without disabling the Skill tool entirely.

### 3. Centralize executor gates and add a constrained verifier

Move the shared safe Git inspection and fixed whole-surface verification commands into root permissions, then let `implementer`, `terra`, and `scut` inherit them instead of maintaining copies. Agent-specific denial still wins for read-only roles. Add `verifier` as a cheap subagent with edits, delegation, GitHub, external paths, and skills denied; its bash policy starts at deny and allows only fixed lint/test/spec/roster gate commands. It reports command, exit status, and raw output without fixing failures.

### 4. Validate static policy and resolved OpenCode behavior together

Implement a dependency-free Bun validator because Bun is already a project tool and provides JSON, YAML, and TOML parsing. Static checks cover tracked agents, role references, skill order/allowlist, permission invariants, dangerous Git allow rules, stale model aliases, OpenSpec config shape, adapter presence, and CLI/plugin version equality. It also invokes pinned OpenCode in pure mode to prove config parsing, required loaded roles, disabled built-ins, and effective defaults while tolerating unrelated user-global agents. `mise run opencode-validate` is deterministic; an explicit `--models` extension may query authenticated provider catalogs and is not a CI dependency.

### 5. Track and adapt the OpenSpec integration

Version the five generated commands and five generated skills from OpenSpec 1.5.0. Replace the archive adapter's nonexistent `general-purpose` delegation target with Harpia's allowlisted `implementer` role. Configure `openspec/config.yaml` with short pointers to `AGENTS.md` and the Platform Constitution plus per-artifact rules; task rules require executor-cold paths, dependencies, acceptance criteria, and exact verification.

### 6. Gate both contracts in CI and pin coupled tools

Add one tooling job that installs exact OpenSpec 1.5.0 and OpenCode 1.17.18 releases, runs `openspec validate --all --strict --no-interactive`, runs validator tests, and executes the roster validator. Container publication depends on this job. Pin OpenCode in `mise.toml`, track `.opencode/package.json` and its lock, and set `@opencode-ai/plugin` to 1.17.18 so the validator can detect future drift.

## Risks / Trade-offs

- **A useful external skill is denied** → keep the allowlist explicit and add a reviewed entry when a real Harpia workflow needs it.
- **Disabling built-ins breaks saved agent selections** → the breaking behavior is intentional; use `orchestrator`, `planner`, or `explorer` equivalents.
- **Root command inheritance grants fixed gates to more custom agents** → read-only roles retain explicit bash denial, dangerous mutations remain ask/deny, and the validator checks effective invariants.
- **Resolved CLI checks inherit user-global configuration** → run OpenCode with `--pure`, assert required/forbidden roles rather than an exact full roster, and never print resolved configuration that may contain secrets.
- **Package releases move faster than the repository** → pin exact versions and update CLI/plugin together through a reviewed change; remote model availability remains a separate opt-in check.
- **CI installation adds network latency** → install only two exact tooling packages in one small independent job.

## Migration Plan

1. Land the archived roster baseline and generated adapters, then apply the new configuration and validator.
2. Run static/resolved roster validation, strict OpenSpec validation, and independent verifier smoke checks locally.
3. Let CI prove a clean checkout can install the pinned tools and pass both gates.
4. Roll back by restoring the prior project config and unpinning the tooling manifests; no application data or runtime migration is involved.

## Open Questions

None. The user explicitly requested all eight hardening improvements; version 1.17.18 supersedes the earlier 1.17.17 audit value.
