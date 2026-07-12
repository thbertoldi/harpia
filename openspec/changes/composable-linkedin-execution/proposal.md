## Why

`linkedin-content-studio` currently has a broken execution shape: `publish` is ordered before
the independently branched carousel and image steps, and the text-only LinkedIn publisher cannot
publish a carousel. The run can therefore publish an incomplete post before rich content exists,
while its inline completion event omits execution context and its configuration-only assistant
cannot surface a reviewable execution.

This change makes the existing LinkedIn execution path trustworthy before the separate
`execution-driven-conversation` change evolves the assistant itself. It introduces one
composable final post, explicit human Review/Revision gates for significant generated content,
version-pinned irreversible approval, and real LinkedIn document publishing.

## What Changes

- Re-author the seeded `linkedin-content-studio` template as the linear composable flow
  `fetch-news → write-draft → author-content → [draft-carousel] → [generate-image] → publish`.
  `author-content` produces a `LinkedInPost`; opted-in middle steps enrich that same artifact;
  `publish` consumes exactly one final `LinkedInPost`.
- Add the composable `LinkedInPost` and version-addressable `ArtifactRef` contracts. Carousel
  and image references remain typed Artifact stream values; provider credentials and upload
  configuration remain on the LinkedIn `ExecutorInstallation`.
- **BREAKING:** replace rich-content branch outputs and the text-only LinkedIn publish contract
  with the single `LinkedInPost → PublishConfirmation` contract. The pre-v1 template, proto, and
  generated clients may change without compatibility shims.
- Add `human_interaction_policy` to `PlanStep`, with `WHEN_REQUIRED` elicitation and required
  review on every significant content-generation step. Model Review/Revision as a new Human
  Interaction concept, separate from Elicitation and Approval; a revision creates a new
  `ArtifactVersion` and repeats review.
- Extend the execution workflow for declared elicitation, review, version-pinned approval, and
  identity-shaped optional enrichment skips. A skipped optional middle step aliases its upstream
  ArtifactRef as its output; invalid optional non-identity contracts are rejected when the
  template is validated.
- Correct completion/`STEP_BOUND` activity input so it includes `PlanExecutionID`,
  `PlanStepKey`, and output artifact identity, allowing the existing thread preview path to bind
  the right execution artifact.
- Render an active execution from its immutable `PlanExecution` snapshot, omitting opted-out
  enrichments; surface elicitation, review/revision, approval, and the final composable preview
  in the conversation with `en` and `pt-BR` copy.
- Extend the LinkedIn integration adapter for the real LinkedIn document-upload publish sequence
  when a carousel is present, while preserving the real text-only API path when it is absent.
- Amend Constitution §9 to define a conversation as lifecycle assistance over PlanConfigurations
  **and their executions**, with declared interaction checkpoints surfaced automatically, and to
  name Review/Revision as distinct from Elicitation and Approval.
- Reconcile the superseded rich-content work and deferred unified-post follow-up by pointing
  `rich-linkedin-content` and `agent-team-deferred-followups` at this authoritative execution
  change and removing their contradictory unfinished unified-publish scope.

## Capabilities

### New Capabilities

- `composable-artifacts`: A version-addressable composable `LinkedInPost` ArtifactType and the
  identity-alias rule that lets optional enrichment preserve a typed downstream contract.
- `execution-review-revision`: Declared ReviewRequest/ReviewDecision checkpoints that create and
  review ArtifactVersions independently of elicitation and irreversible approval.
- `composable-linkedin-execution`: The linear LinkedIn Content Studio execution, exact-final
  approval, and real LinkedIn text/document publishing.

### Modified Capabilities

- `plan-execution-runtime`: Execution interaction now includes declared review/revision,
  version-pinned approval, and identity-alias optional skips.
- `artifact-preview-panel`: The canonical preview must render a composable `LinkedInPost`,
  including its carousel before approval.
- `live-execution-chat`: Active execution cards must derive participating steps from the frozen
  execution snapshot and surface declared execution checkpoints.

## Impact

- **Constitution and OpenSpec:** `docs/architecture/harpia-platform.md`; the new delta specs;
  reconciliation notes in `openspec/changes/rich-linkedin-content/` and
  `openspec/changes/agent-team-deferred-followups/`.
- **Contracts and generated clients:** `proto/harpia/artifacts/v1/artifacts.proto`,
  `proto/harpia/plans/v1/plans.proto`, generated Go/TypeScript/Python code, and any persistence
  required for review requests and version pins.
- **Control plane:** template seeding and validation under `control-plane/internal/plans/`,
  plan/runtime repositories, Temporal workflow and activity inputs under
  `control-plane/internal/workflow/`, artifact preview/version access, and the Human Interaction
  handlers.
- **Agent runtime:** the LinkedIn content specialist and carousel agent must return/continue the
  composable post and request real elicitation where context is missing; no provider SDK enters
  the domain or agent package.
- **LinkedIn adapter:** `control-plane/internal/executors/integrations/linkedin/` gains document
  upload/register/publish behavior against the real API, isolated behind its adapter interfaces.
- **Frontend:** execution/checkpoint cards, artifact preview, client wrappers, and flat locale
  keys in both `frontend/src/lib/i18n/en.json` and `frontend/src/lib/i18n/pt-BR.json`.

## Non-goals

- Making `planassistant` execution-driven. That is the separate
  `execution-driven-conversation` change; this change keeps the current configuration wizard and
  makes the run it launches correct and reviewable.
- The structured-preview/Flint renderer substrate (`structured-preview-substrate`). This change
  extends the canonical existing preview only as needed to show the composed post and carousel.
- Júnior/Pleno agent tiers.
