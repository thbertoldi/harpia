# Harpia Conversational Plan Workspace - Template Inputs + Editable Artifacts

**Date:** 2026-06-29
**Status:** Approved direction from brainstorming; awaiting implementation plan
**Branch context:** trunk-based development, starting from `trunk`
**Related specs:**
- `docs/superpowers/specs/2026-06-22-aiuna-mvp-roadmap-revision.md`
- `docs/superpowers/specs/2026-06-26-harpia-m7-linkedin-e2e-content-generation-design.md`

The Monday demo proved the execution engine, but it also exposed the next product gap: the experience still feels too technical for Ana. Ana should not think in terms of slot bindings, seed artifacts, workflow steps, and debug tables. She should describe the outcome she wants, adjust a few understandable inputs, watch the work progress, review generated assets, make small edits when needed, and continue.

This spec turns the hard-coded LinkedIn sports demo path into a reusable product pattern:

1. Plan templates expose friendly input parameters such as theme, language, tone, date range, source group, and audience.
2. Plan runs happen in a conversational workspace inspired by Manus: chat on one side, visible activity and artifacts on the other.
3. Every generated artifact has a browsable home, a stable detail page, and a preview.
4. Text artifacts can be edited and versioned without forcing a full regeneration.

The technical DAG and execution tables remain available, but they become secondary detail surfaces for Platform Engineers and debugging.

---

## 1. Product Decision

The primary plan surface becomes a **Conversational Plan Workspace**.

Ana's default path:

1. Ana says what she wants in natural language, or chooses a plan template.
2. Harpia maps that intent to template input parameters.
3. Ana can adjust those parameters in a lightweight form embedded in the conversation.
4. The plan runs in the same workspace.
5. Activity events explain progress in human terms.
6. Generated artifacts appear as cards as soon as they exist.
7. Ana can preview, edit, approve, reject, or ask for changes from the artifact card.
8. All artifacts remain available in an artifact library after the run.

The product object Ana manipulates is not "a workflow execution". It is "a workspace where Aiuna is helping produce business outputs".

---

## 2. Scope

### 2.1 Template input parameters v1

Plan templates declare user-facing inputs. The first LinkedIn content template should stop hard-coding `sports` and instead expose at least:

| Key | Type | Example | Required | Runtime mapping |
|---|---|---|---|---|
| `theme` | text or select | `Sports`, `Retail`, `B2B SaaS` | yes | content preferences `topic` |
| `language` | language select | `pt-BR`, `en-US`, `es` | yes | content preferences `language` |
| `tone` | select | `analytical`, `friendly`, `executive` | yes | content preferences `tone` |
| `audience` | text | `founders`, `sales leaders` | no | content preferences `audience` |
| `topics_to_avoid` | textarea | competitor names, sensitive topics | no | content preferences `topics_to_avoid` |
| `source_group` | integration selector | `Sports headlines` | yes | `fetch-news` slot binding |
| `date_range` | date range | last 7 days | yes | `fetch-news` `date_range` seed artifact |
| `approval_mode` | select | require approval | yes | behavior policies |

Each parameter descriptor has:

- `key`: stable machine key.
- `label`: short user-facing label.
- `description`: optional helper text for the form and chat assistant.
- `type`: primitive UI/control type.
- `required`: validation flag.
- `default_value`: optional JSON value.
- `options`: optional static options, dynamic lookup descriptor, or both.
- `runtime_mapping`: where the value goes when the configuration is saved or run.

Runtime mapping supports three targets:

1. `seed_artifact`: materialize a typed seed artifact or `SeedArtifactBinding.literal_json`.
2. `slot_binding`: bind a step to an installation, agent, or integration.
3. `behavior_policy`: update existing plan behavior policies.

The first implementation can store descriptors in code or static template metadata. The design direction is that descriptors eventually become part of Plan template authoring in M9.

### 2.2 Artifact workspace v1

Generated artifacts become first-class UI objects.

Required surfaces:

- A per-run artifact rail in the Conversational Plan Workspace.
- A stable artifact detail route.
- A global or plan-scoped artifact library.
- Artifact preview cards embedded in chat/activity.

Each artifact card shows:

- title or best available label.
- artifact type, such as `NewsList`, `TextDraft`, or `LinkedInPostDraft`.
- source plan and source step in human language.
- created and updated timestamps.
- current version.
- status: generated, edited, approved, rejected, superseded, or failed.
- primary action: preview, edit, approve, or continue, depending on context.

No generated artifact should be visible only inside an execution log.

### 2.3 Text artifact editing and versioning

Ana must be able to make small text changes without a full regeneration.

Supported in v1:

- `TextDraft`
- `LinkedInPostDraft`
- future text-like artifacts that can render as plain text or Markdown

Editing rules:

1. Opening a text artifact detail page shows read mode first, with an edit action when the user has permission.
2. Saving an edit creates a new artifact version. It does not overwrite the original payload.
3. The latest version is the default for preview, approval, and downstream steps.
4. Version history remains visible.
5. Each version records provenance: generated by step, edited by user, created from another version, or produced by regeneration.
6. Downstream steps that already consumed an older version are marked as using that version, not silently changed.

The user-facing model is "edit the draft", but the backend model is append-only versioning.

### 2.4 Conversational run workspace

The run surface should feel like a lightweight conversation with visible work happening, not a workflow console.

Layout direction:

- Main column: conversation and activity timeline.
- Side rail: artifacts from the current configuration/run.
- Details drawer: step DAG, raw execution state, technical IDs, retry tools.

Timeline event examples:

- "I found 5 relevant articles from Sports headlines."
- "I drafted the newsletter in Portuguese."
- "I adapted the draft for LinkedIn."
- "The LinkedIn post is ready for review."
- "Publishing is waiting for your approval."

Events can be generated from existing `PlanExecution`, `StepExecution`, elicitation, approval, and artifact records. They do not require an LLM narrator in v1.

Chat actions:

- "Make this shorter" should create an artifact edit or targeted regeneration request, depending on available implementation.
- "Change the language to English" should update the template input parameter and ask whether to rerun affected steps.
- "Use a more executive tone" should update parameters and rerun or patch the affected artifact.

The first implementation can support a small command set and clear fallbacks. The UI should still be conversational even when some actions route to forms.

---

## 3. UX Principles

1. **Outcome language first.** Ana sees "Draft", "Sources", "Ready for approval", and "Needs your input" before she sees step keys or workflow states.
2. **Artifacts are the center of gravity.** Runs are meaningful because they produce assets. The assets must be easy to find after the run.
3. **Edits are cheap.** A typo, extra line, or tone adjustment should not require restarting the whole plan.
4. **Technical controls are still available.** Platform Engineers need the DAG, bindings, and execution details, but those controls should live behind details/debug affordances.
5. **Every important state is visible.** Running, waiting for input, failed, ready for approval, edited, and superseded states must be visible without opening logs.
6. **Errors are conversational and actionable.** The surface says what failed, what the user can do, and whether retry is possible.

---

## 4. Architecture

```text
PlanTemplate
  -> TemplateInputParameter descriptors
  -> PlanConfiguration parameter values
  -> Runtime materialization
       -> seed artifacts
       -> slot bindings
       -> behavior policies

PlanExecution
  -> StepExecution events
  -> generated Artifact versions
  -> activity timeline
  -> approval / elicitation actions

Artifact
  -> version history
  -> preview
  -> editor
  -> library/read model
```

### 4.1 Template input descriptors

Add a template input descriptor model. It can start as frontend/shared registry data if that lets us ship faster, but the shape should be compatible with moving into `PlanTemplate` later.

Proposed descriptor shape:

```text
TemplateInputParameter
  key
  label
  description
  type
  required
  default_value_json
  options_json
  runtime_mappings[]
```

The materializer converts parameter values into the existing runtime structures where possible:

- `SeedArtifactBinding` for date range and content preferences.
- `SlotBinding` for source groups and executor choices.
- `PlanBehaviorPolicies` for approval mode.

This avoids a risky workflow rewrite. The workflow can continue consuming the existing configuration snapshot while the product surface becomes parameter-driven.

### 4.2 Plan configuration parameter values

`PlanConfiguration` needs a place to preserve the user's original parameter values, not only the materialized seed artifacts.

The implementation plan should choose one of two approaches:

1. Add `parameter_values_json` to the plan configuration record and proto.
2. Add a repeated typed `TemplateInputValue` proto field.

The recommended v1 path is typed JSON storage plus validation through descriptors. It is faster, easier to evolve before v1, and compatible with future formal proto fields if needed.

### 4.3 Artifact versioning

Artifacts currently point directly at a storage URI and content hash. Editing requires versioning.

Recommended model:

```text
artifacts
  id
  tenant_id
  artifact_type_id
  current_version_id
  created_at
  updated_at

artifact_versions
  id
  artifact_id
  version_number
  storage_uri
  content_hash
  source_plan_execution_id
  source_step_execution_id
  source_version_id
  created_by_user_id
  edit_summary
  created_at
```

The existing `Artifact` response can keep returning the current payload location during the transition. New APIs expose version history and allow saving a new text version.

Required new capabilities:

- list artifact versions.
- get payload for a specific version.
- preview current version or a specific version.
- create a new version from edited text.
- select or derive current version.

### 4.4 Artifact provenance

Artifacts need enough provenance for the UI to answer "where did this come from?"

At minimum:

- tenant.
- artifact type.
- plan configuration id.
- plan execution id.
- step execution id.
- creating actor: system, agent, integration, or user.
- source artifact/version when derived from another artifact.

The implementation can add provenance fields incrementally, but every artifact card should eventually be explainable without reading workflow logs.

### 4.5 Activity timeline

The conversational workspace uses a read model built from existing events:

- plan thread messages.
- plan execution status.
- step execution status.
- artifact creation/version events.
- approval requests.
- elicitation requests.
- errors.

This read model can be assembled client-side first if backend changes would slow the slice. A backend `PlanActivity` stream is the cleaner long-term shape.

---

## 5. Frontend Surfaces

### 5.1 Template input panel

Replace the narrow "Suggest topic" drawer with a reusable parameter panel.

Controls:

- theme/topic text input or select.
- language selector.
- tone selector.
- source group selector.
- date range control.
- topics-to-avoid textarea.
- approval mode selector.

The panel writes parameter values and materialized configuration fields together. The Binding Matrix remains available in details mode for advanced repair.

### 5.2 Conversational plan workspace

Route direction:

- Keep the existing plan thread route as the durable plan home.
- Make the first viewport chat/activity plus artifacts.
- Move DAG/execution details into a drawer or secondary tab.

Expected components:

- `ConversationalWorkspace`
- `PlanActivityTimeline`
- `TemplateInputPanel`
- `ArtifactRail`
- `ArtifactCard`
- `ArtifactViewer`
- `TextArtifactEditor`

### 5.3 Artifact library

Add a browse surface that can answer:

- What did this tenant generate?
- Which plan generated it?
- Which assets are ready for use?
- Which assets were edited or approved?

The first version can be generic and filtered by artifact type, plan, and date. Specialized content/customer/opportunity browsers remain aligned with the M8/M9 typed artifact roadmap.

---

## 6. Backend/API Changes

### 6.1 Plans

Add support for:

- persisting parameter values on configurations.
- validating parameter values against descriptors.
- materializing values into seed artifacts, slot bindings, and behavior policies.
- preserving parameter values in execution snapshots.

The materialization helper should be pure and unit tested. Given descriptors and values, it returns a deterministic configuration patch.

### 6.2 Artifacts

Add support for:

- versioned artifact payloads.
- text artifact edits.
- artifact listing with filters.
- artifact provenance.
- current-version preview.
- specific-version preview.

Text editing should be implemented only for artifact types with a known text projection. Unknown binary or structured artifacts stay read-only in v1.

### 6.3 Activity/read model

The first slice can compose activity client-side from existing APIs. If this becomes brittle, add a backend `ListPlanActivity` or `WatchPlanActivity` RPC that emits normalized events:

- `run_started`
- `step_started`
- `step_completed`
- `artifact_created`
- `artifact_version_created`
- `approval_requested`
- `elicitation_requested`
- `run_failed`
- `run_completed`

---

## 7. Error Handling

Errors should be visible in the conversational workspace and attached to the failed activity.

Required behavior:

- plan start failure shows a clear inline error and retry action.
- step failure shows the friendly step title, failure summary, and retry availability.
- provider/key failures explain that the AI provider is unavailable or misconfigured.
- artifact preview failure does not break the whole workspace.
- artifact edit save failure keeps the draft text in the editor.
- version conflict shows that a newer version exists and asks the user to review it before saving.

Logs and technical IDs are available from details mode, but the primary surface should state the recovery action first.

---

## 8. Non-Goals

- Full M9 template authoring UI.
- Full Manus clone or autonomous browser workspace.
- Collaborative editing.
- Rich image/video/binary asset editing.
- Public artifact sharing links.
- Marketplace of agents or templates.
- Replacing the workflow engine or existing execution model.
- Moving all configuration into natural language only. Forms remain important for precision.

---

## 9. Implementation Milestones

### A. Parameter foundation

- Define parameter descriptor shape.
- Add configuration parameter value persistence.
- Add materialization helper.
- Replace LinkedIn hard-coded suggestion values with parameter-driven values.
- Support non-sports themes and post language selection.

### B. Artifact visibility

- Add artifact list/read model.
- Add artifact cards in the run surface.
- Add artifact detail route.
- Ensure every generated artifact from the LinkedIn path is reachable outside execution logs.

### C. Text editing and versioning

- Add artifact version storage.
- Add APIs for listing versions, previewing versions, and saving edited text.
- Add text editor UI for text-like artifacts.
- Wire approval/downstream actions to the current artifact version.

### D. Conversational workspace

- Convert the plan thread/run page into the primary conversational workspace.
- Add activity timeline.
- Move DAG/execution detail into secondary details mode.
- Surface errors and waiting states as conversational events.

### E. Chat-driven parameter and edit commands

- Parse a small command set from chat.
- Route parameter changes to the parameter panel/materializer.
- Route text changes to artifact edits when safe.
- Ask for confirmation before rerunning affected steps.

---

## 10. Testing

Backend:

- parameter materializer unit tests for theme, language, tone, source group, date range, and approval mode.
- configuration create/update tests preserving parameter values.
- artifact versioning tests: create, edit, list versions, preview current, preview historical.
- authorization tests for cross-tenant artifact access and edits.
- provenance tests for generated and edited versions.

Frontend:

- parameter panel tests for language/theme/tone persistence.
- artifact card rendering tests for generated, edited, failed, and approval states.
- text editor tests for save failure, version conflict, and successful version creation.
- activity timeline tests for run status mapping.

End to end:

1. Create a LinkedIn content plan with a non-sports theme.
2. Select Portuguese, English, or Spanish as the post language.
3. Run the plan.
4. Preview the generated `NewsList`, `TextDraft`, and `LinkedInPostDraft`.
5. Edit the LinkedIn draft text and save a new version.
6. Approve or continue with the edited version.
7. Find the edited artifact in the artifact library.

---

## 11. Open Product Decisions For The Implementation Plan

These decisions should be resolved while writing the implementation plan, not by ad hoc coding:

1. Whether parameter descriptors live in proto/template metadata immediately or start in a shared registry.
2. Which language options ship first: likely `pt-BR`, `en-US`, and `es`.
3. Which tone presets ship first.
4. Whether artifact library appears as a top-level nav item immediately or as a plan-scoped tab first.
5. Whether editing an artifact automatically marks downstream steps stale or only shows provenance until the user asks to rerun.
6. Which roles can edit generated artifacts in v1.

---

## 12. Acceptance Criteria

The first implementation slice is successful when:

1. Ana can run the LinkedIn content plan for a theme other than sports.
2. Ana can choose the generated post language before running.
3. The run workspace shows human-readable progress and generated artifact cards.
4. Ana can preview every generated artifact from the run.
5. Ana can edit a generated text artifact, save a new version, and use that version for approval or continuation.
6. The edited artifact can be found again from an artifact browsing surface.
7. The technical execution detail remains available for debugging, but it is not the default mental model for Ana.
