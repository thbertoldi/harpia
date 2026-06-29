# Harpia M7 — LinkedIn E2E Content Generation

**Date:** 2026-06-26  
**Status:** Approved design from brainstorming; awaiting implementation plan  
**Target demo date:** Monday, 2026-06-29  
**Supersedes:** the broad post-M6 M7 "LLM providers + pricing" slice for this milestone. Pricing stays out.

M7 is a demo-first milestone. By Monday, Ana must be able to run the LinkedIn content plan from the app and watch a real agent chain generate content from real RSS feeds, ending at a pending approval for the LinkedIn draft. The demo is not a CLI or backend-only path.

The future direction remains smarter `/new` recommendation: a customer can type "I want a sports newsletter for LinkedIn" and Harpia recommends the LinkedIn plan with sports defaults. That is not in the Monday slice. M7 should shape the configuration data so that future flow can reuse it.

---

## 1. Scope

M7 ships the smallest true product path:

1. **Platform Engineer configures DeepSeek BYOK** in Admin Settings.
2. **Platform Engineer configures RSS feed groups** in Admin Integrations. RSS is multi-instance: several named groups, each with several feed URLs.
3. **Ana manually chooses the LinkedIn plan** from the existing plan picker.
4. **Plan configuration suggests/fills defaults** for topic, feed group, content brief, bindings, and approval policy while preserving the M6 Binding Matrix as the editable source of truth.
5. **Run now executes the real plan path**: RSS integration -> newsletter writer agent -> LinkedIn voice agent -> publish approval gate.
6. **UI ends at pending approval** with previews for `NewsList`, `TextDraft`, and `LinkedInPostDraft`.
7. **All waiting-for-user states surface in the unified inbox** so Ana can find approvals, elicitations, and feedback from the same open-actions screen.

Out of scope:

- Customer-facing pricing, pricing override UI, or model price management.
- Generic OpenAI-compatible custom provider UX.
- `/new` natural-language plan recommendation.
- Public LinkedIn publish.
- A generic seed-artifact framework for every possible plan input.
- Integration previews as a standalone authoring feature.

---

## 2. Demo Path

The Monday script should be:

1. Platform Engineer opens `/admin/settings#llm-providers`.
2. They add a DeepSeek API key and select `deepseek-v4-flash` or `deepseek-v4-pro`.
3. Platform Engineer opens `/admin/integrations`.
4. They create or confirm an RSS feed group:
   - `Sports headlines`
   - feeds: `https://feeds.folha.uol.com.br/esporte/rss091.xml`
5. Ana opens `/new`, manually chooses the LinkedIn plan, and lands in the plan thread/configuration screen.
6. Ana clicks **Suggest**, then answers "What do you want to talk about?" with `sports`.
7. Harpia fills:
   - `fetch-news` -> `Sports headlines`
   - `write-newsletter` -> newsletter writer agent
   - `adapt-for-linkedin` -> LinkedIn voice agent
   - `publish-linkedin` -> LinkedIn publish approval-only/dry-run installation
   - content brief/tone defaults so the writer does not stop for avoidable elicitation
   - RSS date range: last 7 calendar days
   - publish approval mode: require approval
8. Ana runs the plan.
9. The plan executes through content generation and pauses at the publish approval.
10. Ana sees the pending approval in the run surface and in `/inbox`, then can preview the LinkedIn draft. The actual publish button is not clicked during the Monday success path.

Success is a plan waiting for approval, not a completed published run.

---

## 3. Architecture

```text
Platform Engineer
  -> Admin Settings: DeepSeek tenant key + model
  -> Admin Integrations: named RSS feed groups

Ana
  -> manually chooses LinkedIn plan
  -> Suggest fills plan configuration
  -> Run now

Plan workflow
  fetch-news          RSS integration          -> NewsList artifact
  write-newsletter    newsletter-writer agent  -> TextDraft artifact
  adapt-for-linkedin  linkedin-voice agent     -> LinkedInPostDraft artifact
  publish-linkedin    approval-only publish    -> pending approval before external publish
```

The publish step remains in the plan because the existing workflow raises approval before running publish when policy requires approval. This gives a truthful end-to-end execution while avoiding a public LinkedIn post.

---

## 4. DeepSeek BYOK

DeepSeek is a first-class provider for M7. Do not build a generic custom endpoint provider.

Existing substrate:

- `tenant_llm_configs` already stores one row per `(tenant_id, provider)`.
- public LLM config RPCs already set, rotate, delete, and list metadata.
- provider names are normalized lowercase.
- Admin Settings already has an LLM provider card pattern for OpenAI, Anthropic, and Ollama.

M7 changes:

- Add `deepseek` to frontend provider types and Admin Settings provider catalogue.
- Add model options:
  - `deepseek-v4-flash`
  - `deepseek-v4-pro`
- Add `DeepSeekProvider` in agent runtime using the OpenAI-compatible chat completions shape with base URL `https://api.deepseek.com`.
- Register DeepSeek in `LLMRegistry.default()`.
- Route the LinkedIn content agents to DeepSeek for the Monday path.
  - Update the newsletter writer and LinkedIn voice manifests to use a DeepSeek model id for M7, defaulting to `deepseek-v4-flash`.
  - Map `_default_provider_for_manifest()` for both content agents to `deepseek`.
  - Keep `requested_model` as an optional override, not as the primary M7 route.
- Keep public responses secret-free: only `has_key`, model metadata, and masked state are visible.

DeepSeek docs reference: the official API docs list OpenAI-compatible `base_url` as `https://api.deepseek.com`, current model names `deepseek-v4-flash` and `deepseek-v4-pro`, and mark `deepseek-chat` / `deepseek-reasoner` for deprecation on 2026-07-24.

Pricing note: if the provider interface needs internal `ModelPricing` values to estimate cost, add minimal static metadata in code. Do not add pricing UI, pricing overrides, or roadmap-priced plan surfaces in M7.

---

## 5. RSS Feed Groups

The current UI assumes one installation per integration SKU. That is wrong for RSS.

Backend state already supports the correct model:

- `executor_installations` has no uniqueness constraint on `(tenant_id, executor_sku_id)`.
- RSS config already accepts `config_json.feeds` as an array.
- plan slot binding already points to a single `executor_installation_id`.

M7 UI model:

```text
Admin Integrations

RSS News Feeds
  Tech headlines
    https://hnrss.org/frontpage

  Sports headlines
    https://feeds.folha.uol.com.br/esporte/rss091.xml

  + Add feed group

LinkedIn
  LinkedIn connection / credential card
```

Each RSS feed group is one `executor_installation`:

- `executor_sku_key`: `rss-news-feed`
- `display_name`: group name
- `config_json`: `{ "feeds": ["https://..."] }`
- `enabled`: whether it can be used in plan bindings

Frontend helper changes:

- Stop collapsing installations by SKU in `loadDemoIntegrationContext`.
- Group integration inventory by integration type.
- Show all RSS installations in the RSS group.
- Preserve existing create/update RPCs; no migration is required.
- The Binding Matrix should list multiple RSS feed groups as compatible options for `fetch-news`.

---

## 6. LinkedIn Publish Binding

The Monday path needs the publish step bound so the workflow can reach the existing approval gate. It must not require a real public LinkedIn OAuth connection.

M7 adds an approval-only/dry-run mode for the LinkedIn publish integration:

- one `linkedin-publish` installation can be configured as `mode: "approval_only"` with no real OAuth secret.
- readiness validation treats that installation as bindable for plans whose publish policy is `require approval`.
- the workflow still raises the approval before any publish call.
- if someone approves during rehearsal, the dry-run publisher must return a non-public `PublishConfirmation` instead of calling LinkedIn:
  - `platform`: `linkedin-dry-run`
  - `external_id`: deterministic dry-run id
  - `url`: empty
- UI copy must avoid claiming that LinkedIn is connected for public publishing.

After Monday, real LinkedIn OAuth can replace the dry-run installation without changing the plan shape.

---

## 7. Plan Configuration Suggestion

M7 should not resurrect the pre-M6 wizard. The Binding Matrix remains the editable source of truth.

Add one focused suggestion affordance inside the plan configuration surface:

- A **Suggest** button in the Binding Matrix header.
- Clicking it opens an inline prompt: "What do you want to talk about?"
- Suggested topic chips: `Sports`, `Tech`, `Custom`.

For the Monday sports path, Suggest fills:

- feed group: `Sports headlines`
- content topic: `sports`
- tone: professional, concise, LinkedIn-native
- topics to avoid: empty by default, editable
- RSS date range: last 7 calendar days, encoded with explicit `YYYY-MM-DD` `start_date` and `end_date`
- publish approval: require approval

The suggestion can be deterministic. It does not need an LLM recommender. It should write normal plan configuration fields so later `/new` recommendation can reuse the same shape.

Hidden runtime requirements that must be handled:

- RSS requires a `DateRange` input. M7 must persist an explicit `DateRange` seed binding when Suggest fills the plan. Use `SeedArtifactBinding.literal_json` for the `fetch-news` step. The value should cover the last 7 calendar days and be visible in configuration/run details.
- `newsletter-writer-senior` currently elicits tone when missing. M7 should pass tone/topic defaults from configuration so content generation does not pause before the approval step unless the user deliberately clears required fields.

---

## 8. Agent Execution Runtime

M7 must close the current plan-agent bridge.

Current state:

- Go workflow has a real `RunIntegrationActivity`.
- Go workflow's `RunAgentActivity` is still a failed stub.
- Python agent runtime has manifest-backed newsletter and LinkedIn voice runners.
- Python agent activity currently handles only the newsletter writer shape and returns inline output instead of a persisted artifact.

Target contract:

1. Go plan workflow invokes agent execution for `ExecutorKindAgent`.
2. The agent activity receives:
   - tenant id
   - plan execution id
   - step execution id
   - manifest id/version from installation snapshot
   - input artifact refs
   - output artifact type key
   - configuration values such as topic/tone/requested model when present
3. Agent runtime loads the upstream artifact payload.
4. Agent runtime runs the manifest-backed agent with tenant DeepSeek credentials.
5. Agent runtime creates a validated artifact through `ArtifactService.CreateArtifactWithPayload`.
6. Agent runtime returns `ExecutorActivityResult{status: completed, output_artifact_id}`.

Both agents must be supported:

- `newsletter-writer-senior`: `NewsList` -> `TextDraft`
- `linkedin-voice-senior`: `TextDraft` -> `LinkedInPostDraft`

The runtime should use the same artifact contract as integration steps. Inline IDs such as `inline:text-draft:*` are not acceptable for the app demo because approval previews and step outputs expect persisted artifacts.

---

## 9. Approval And Preview

Approval behavior should use the existing workflow design:

- `publish-linkedin` is present and bound.
- its installation is approval-only/dry-run for Monday.
- behavior policy requires publish approval.
- workflow creates a `StepExecution` for `publish-linkedin`.
- workflow creates an approval request before calling the LinkedIn publisher.
- approval request references the input artifact id, which is the `LinkedInPostDraft`.
- UI preview uses existing artifact preview logic.

The run remains `waiting_human` / awaiting approval. That is the desired Monday terminal state.

UI should make the chain visible:

- `fetch-news`: preview `NewsList` article count/titles
- `write-newsletter`: preview `TextDraft`
- `adapt-for-linkedin`: preview `LinkedInPostDraft`
- approval: preview the same `LinkedInPostDraft`

---

## 10. Unified Inbox Notifications

M7 must preserve the M2/M3 interaction model: when the system waits for a human, the user should not have to stay on the run screen to discover it.

Every pending user action from the LinkedIn run must appear in the unified inbox (`/inbox`):

- publish approval requests
- agent elicitations, if any still occur
- feedback requests, if any are raised by the runtime

Inbox items should include:

- action kind (`approval`, `elicitation`, or `feedback`)
- plan/configuration/run context
- step name, such as `publish-linkedin`
- link back to the plan thread or run/canvas
- the existing action UI or preview entry point

The Monday happy path must create a pending approval item in `/inbox` as soon as the workflow pauses before publish. Resolving the approval from either `/inbox` or the plan/run surface must update the other surface through the existing watch/aggregator paths.

---

## 11. Error Handling

Expected failures should be legible:

- Missing DeepSeek key: block run/configuration with a Platform Engineer action link to `/admin/settings#llm-providers`.
- DeepSeek authentication failure: agent step fails with "DeepSeek authentication failed" or equivalent provider error.
- DeepSeek rate/provider error: agent step fails with provider and retry context.
- RSS feed failure: `fetch-news` shows the feed URL that failed.
- Empty RSS result: fail the RSS step with a clear "no articles found for selected feeds/date range" message or surface it as a validation warning before run.
- Missing content brief defaults: configuration should prevent avoidable newsletter elicitation unless the user intentionally clears required values.
- Missing LinkedIn dry-run publish binding: block runnable promotion with an action to create the approval-only publish binding.
- Missing approval policy: default to require approval for publish.

---

## 12. Test Plan

Focused tests:

- Frontend:
  - Admin Settings provider catalogue includes DeepSeek.
  - LLM config client types accept `deepseek`.
  - Admin Integrations can render multiple RSS feed groups under one RSS section.
  - Creating/updating an RSS group writes `config_json.feeds`.
  - Binding Matrix lists multiple RSS feed groups for `fetch-news`.
  - Suggest fills sports defaults and remains editable.
  - pending publish approval appears in `/inbox` with a link/preview action.
- Control plane:
  - multiple RSS `executor_installations` for one tenant/SKU are listed and bindable.
  - LinkedIn approval-only publish installation is bindable without real OAuth.
  - plan validation accepts a selected RSS feed group.
  - publish approval still occurs before LinkedIn publish.
  - approval, elicitation, and feedback pending states remain listable by the inbox aggregator.
  - approving a dry-run publish step does not call LinkedIn and returns a dry-run confirmation.
  - `RunAgentActivity` no longer returns the stub failure.
- Agent runtime:
  - DeepSeek provider routes `deepseek-v4-flash` and `deepseek-v4-pro`.
  - tenant resolver supports `provider=deepseek`.
  - fake LLM test for `NewsList -> TextDraft -> LinkedInPostDraft`.
  - agent activity persists output artifacts and returns artifact IDs.
- End-to-end:
  - run against fake LLM and controlled RSS fixture.
  - one manual rehearsal with real DeepSeek key and real RSS feed before Monday.

Verification commands for implementation should include:

- `go test ./internal/executors/... ./internal/plans/... ./internal/workflow/... ./internal/llm_config/...`
- agent-runtime tests for LLM providers and agents
- frontend unit tests for settings/integrations/plan configuration
- Playwright smoke for the Monday path if the local stack can run it reliably

---

## 13. Future Follow-Up

After Monday, evolve toward the smart recommendation flow:

- `/new` accepts "I want a sports newsletter for LinkedIn".
- deterministic or LLM-assisted recommender picks the LinkedIn plan.
- recommender preselects RSS feed groups and content brief values.
- Ana lands in the same configuration surface M7 introduced.

This follow-up should reuse M7's configuration fields and feed group model. It should not create a parallel wizard.

---

## 14. References

- M6 design: `docs/superpowers/specs/2026-06-22-harpia-m6-lapidacao-design.md`
- Roadmap revision: `docs/superpowers/specs/2026-06-22-aiuna-mvp-roadmap-revision.md`
- DeepSeek API docs: `https://api-docs.deepseek.com/`
- DeepSeek Models & Pricing: `https://api-docs.deepseek.com/quick_start/pricing`
