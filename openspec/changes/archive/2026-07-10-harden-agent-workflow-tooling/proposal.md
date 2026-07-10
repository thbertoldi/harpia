## Why

Harpia's custom OpenCode roster is not yet the effective default and remains exposed to broad external skill discovery, configuration drift, and self-reported verification. OpenSpec is canonical but its generated OpenCode adapters, project policy, and strict validation are not consistently version-controlled or enforced in CI.

## What Changes

- Make `orchestrator` the project default, set explicit main and lightweight models, and disable overlapping built-in development agents.
- Restrict the Skill tool to Harpia, OpenSpec, and a reviewed set of stack-relevant engineering skills.
- Add a cheap, read-only verifier that can execute only fixed repository gates and report results independently.
- Centralize shared executor command permissions and add a deterministic roster validator for configuration, role, permission, model, and package invariants.
- Track the generated OpenSpec 1.5.0 OpenCode commands and skills, adapting their cross-agent handoff to Harpia's roster.
- Configure OpenSpec with canonical project context and executor-cold artifact rules, then run strict validation in CI.
- Pin the OpenCode CLI and its local plugin SDK to the same project version.
- **BREAKING:** Hide the built-in `build`, `plan`, and `explore` agents in favor of Harpia's explicit roster roles.

## Capabilities

### New Capabilities

- `openspec-workflow`: Defines Harpia's tracked OpenSpec adapters, centralized artifact-authoring policy, and continuous strict validation.

### Modified Capabilities

- `opencode-agent-roster`: Makes the custom roster the default, constrains skill discovery, adds independent verification and deterministic roster validation, centralizes executor permissions, and aligns OpenCode package versions.

## Impact

- Affects `opencode.json`, `.opencode/agent/`, `.opencode/commands/`, `.opencode/skills/`, `.opencode/package*.json`, `.opencode/.gitignore`, `openspec/config.yaml`, `openspec/specs/`, `scripts/`, `mise.toml`, and `.github/workflows/ci.yml`.
- Adds no Harpia runtime behavior, product-domain contract, public API, schema, migration, or user-facing copy.
- Requires OpenCode 1.17.18, OpenSpec 1.5.0, Bun, and only repository-local deterministic validation during normal CI; authenticated provider-catalog checks remain optional local evidence.
