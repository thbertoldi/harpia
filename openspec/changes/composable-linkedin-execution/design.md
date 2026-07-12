## Context

The current `linkedin-content-studio` template is a branched DAG whose declaration order lets
`publish` run immediately after `author-content`; `draft-carousel` and `generate-image` are
later independent branches. The LinkedIn adapter only loads `LinkedInPostDraft` and creates a
legacy `/v2/ugcPosts` request with `shareMediaCategory: "NONE"`, so carousel output is both
unordered and unpublished. The completion activity also omits the execution id and step key,
preventing the runtime from emitting the `STEP_BOUND` event that binds the preview to a run.

The applicable product model is Constitution §§9 and 15: conversation is the lifecycle surface
for a PlanConfiguration and its executions; external mutations are governed and irreversible
ones require approval. This change extends the existing configuration-first flow only through
the execution it starts; it does not make `planassistant` execution-driven.

The existing artifact stream already versions payloads and the Temporal workflow already pauses
for elicitation and approval. The design reuses both rather than creating a per-LinkedIn editor
or another interaction transport. The authoritative LinkedIn REST document sequence is:
`POST /rest/documents?action=initializeUpload`, binary `PUT` to the returned upload URL, then
`POST /rest/posts` with the returned document URN; requests use the OAuth credential and REST
headers held by the LinkedIn `ExecutorInstallation`.

## Goals / Non-Goals

**Goals:**

- Make every execution publish one version-pinned, composable `LinkedInPost`, in the declared
  order, with optional enrichments that preserve that contract when skipped.
- Give each significant generated-content step a durable Elicitation → candidate → Review/
  Revision loop, followed by an Approval gate before the only external mutation.
- Make the active execution card and artifact preview accurately represent the frozen execution
  rather than a template or configuration that has since changed.
- Publish approved text and carousel documents through the real LinkedIn REST API, without
  moving OAuth, provider, storage, or HTTP concerns into domain packages.

**Non-Goals:**

- Execution-driven `planassistant` orchestration (`execution-driven-conversation`).
- The reusable structured-preview/Flint renderer substrate, a carousel editor, or a generic
  document-rendering product surface.
- Jr/Pleno tiers, multi-channel publishing, or an image-provider redesign.

## Decisions

### 1. One version-addressable composable Artifact is the flow contract

Add `ArtifactRef { artifact_id, artifact_version_id, artifact_type_key, content_hash }` and
`LinkedInPost { LinkedInPostDraft text; optional CarouselDraft carousel; repeated ArtifactRef
images }` to `proto/harpia/artifacts/v1/artifacts.proto`. `author-content` creates the
`LinkedInPost` Artifact; each enrichment writes a new `ArtifactVersion` under that same Artifact
id and returns an `ArtifactRef` for that version. No step publishes a `CarouselDraft` or
`ImageAsset` directly.

`CarouselDraft` remains the structured review source. When it is attached, the carousel executor
also creates an immutable, tenant-scoped PDF `LinkedInCarouselDocument` derivative Artifact and
records its ref and byte hash in the `CarouselDraft` version. This deliberately narrow
CarouselDraft→PDF serializer is part of the LinkedIn content adapter path, not the deferred
structured-preview/Flint substrate: the preview continues to render structured slides and the
publisher uploads the already-stored derivative bytes. The document ref makes the exact bytes
reviewed/pinned/auditable business state rather than an untracked HTTP-side conversion.

Alternative rejected: three format-specific publish steps or an untyped blob. Both would repeat
the defect that lets the publisher lose part of the post and contradict the required single
final-post contract.

### 2. Optional enrichments are identity-shaped aliases

The template becomes a linear DAG:

```text
fetch-news:       DateRange    → NewsList
write-draft:      NewsList     → TextDraft
author-content:   TextDraft    → LinkedInPost
draft-carousel:   LinkedInPost → LinkedInPost   (optional)
generate-image:   LinkedInPost → LinkedInPost   (optional)
publish:          LinkedInPost → PublishConfirmation
```

At template validation, any optional middle step MUST have equal input and output artifact types;
an optional terminal or type-changing node is rejected. At execution creation, resolve optional
capabilities once and store an immutable `active_step_keys` graph in the execution snapshot. For
an excluded identity-shaped step, create the auditable SKIPPED StepExecution with input and output
ArtifactRefs equal, place the alias in workflow output resolution, and do not emit a visible
execution step event. Downstream steps therefore receive the same typed ref, while the UI renders
only `active_step_keys` from the frozen snapshot.

Alternative rejected: omit a skipped node without an output alias. That makes the linear
downstream resolver fail and reintroduces format-specific branching logic.

### 3. Review/Revision is a third Human Interaction state

Add `HumanInteractionPolicy` to `PlanStep`, with `elicitation_mode` and `review_mode`. The
content steps `author-content`, `draft-carousel`, and `generate-image` declare
`WHEN_REQUIRED` / `REQUIRED`; `publish` declares its existing irreversible approval behavior.

Add `ReviewRequest` and `ReviewDecision` contracts and tenant/RLS-backed persistence. A request
stores the subject `ArtifactRef`, producing StepExecution, overseer, status, and revision
feedback. An accepted request records the exact ref. A revision decision carries feedback; the
workflow resumes the real agent with it, creates a new ArtifactVersion under the same Artifact,
and raises a fresh ReviewRequest. Review is neither an ElicitationRequest (missing context before
work) nor an ApprovalRequest (permission for an irreversible transition).

The workflow loop for a declared content step is: run agent; durably wait/resume an elicitation
when the agent requests one; persist the candidate ref; durably wait for ReviewDecision; on
revision rerun the same real agent with feedback; on acceptance complete the step. The maximum
elicitation/review rounds and timeout behavior follow the current bounded/Temporal-safe pattern;
workers never wait on a human synchronously.

Alternative rejected: use ApprovalRequest for content feedback. Approval answers whether to take
an irreversible action; it cannot express feedback and would not guarantee a newly reviewed
version.

### 4. Approval pins the exact final ref before the publisher can run

`ApprovalRequest` replaces the string-only `input_artifact_id` subject with a full immutable
`subject_artifact_ref` (including content hash); persistence stores artifact id, version id, and
hash in separately queryable fields. The workflow creates it only after all active enrichment
reviews accept. A resolve activity verifies the decision still refers to the pinned ref before
resuming `publish`; the LinkedIn handler loads that exact version, never an Artifact's mutable
current version. Reject or cancel terminates the publish StepExecution before the integration
activity and makes zero LinkedIn calls.

This uses a first-class `ArtifactRef` rather than a bare version id so the handler can verify
tenant ownership, type, and hash at every persistence and adapter boundary.

### 5. The LinkedIn adapter owns REST and document upload semantics

Refactor `control-plane/internal/executors/integrations/linkedin/` so its handler accepts a
version-pinned `LinkedInPost`, uses only an adapter-facing artifact reader to load the post and
the referenced immutable carousel-document bytes, and passes a normalized publish request to
`LinkedInPublisher`. The HTTP publisher chooses exactly one path:

- no carousel: create an organic text post through `POST /rest/posts` using the approved text;
- carousel: initialize `/rest/documents?action=initializeUpload`, PUT the pinned PDF bytes to the
  returned URL, then create one `/rest/posts` document post referencing that returned URN.

The author/owner identifier, OAuth credential, API version/header policy, and HTTP client stay in
the LinkedIn installation/config and adapter. The client never accepts arbitrary user URLs or
credentials from artifacts. HTTP tests use `httptest`; acceptance invokes the production adapter
with a recording local server, while the only live-LinkedIn assertion remains an explicit TODO
requiring sandbox credentials.

### 6. Runtime events and chat derive from execution state

Every completion and identity-alias completion passes `PlanExecutionID`, `PlanStepKey`, output
artifact id, output version id, and content hash to the activity. The runtime emits a deduplicated
`STEP_BOUND` scoped to that execution. Add review raised/decided thread-message kinds and payload
helpers alongside existing elicitation/approval events. The frontend uses those events and a
fetched immutable execution snapshot to build the card; it does not infer active steps from the
latest template or configuration. Review and approval cards open the canonical preview for their
pinned subject ref, including carousel slides.

### 7. Documentation and prior change reconciliation are explicit

The implementation task updates Constitution §9 and glossary/interaction language together so
conversation covers executions and declared checkpoints surface. It also marks the incomplete
rich LinkedIn branch/publish work and the deferred unified-post item as superseded by this change,
without silently editing their completed historical requirements.

## Risks / Trade-offs

- **PDF derivative may differ from the structured preview** → generate and store the deterministic
  document before approval, pin its hash in the approved post version, and test upload bytes
  against that artifact; do not regenerate it during publish.
- **Review loops can create unbounded cost/time** → use durable temporal waits, bounded rounds,
  existing timeout policy, and explicit terminal failure/cancellation paths.
- **Version pin can be bypassed by loading current Artifact state** → expose a version-specific
  artifact reader and make the publisher interface accept the pinned ref/bytes, with regression
  tests that change the current version after approval.
- **LinkedIn API permissions/version behavior can vary** → centralize REST-version headers and
  status classification in the adapter; run contract tests locally and mark live sandbox evidence
  as an environmental acceptance TODO only.
- **Previous change artifacts contradict this design** → the reconciliation task updates forward
  links before either change is archived; this change is authoritative for unfinished work.

## Migration Plan

1. Introduce the breaking proto, database, seed, and generated-client changes together; pre-v1
   has no data migration or compatibility shim.
2. Deploy the runtime/review/approval and adapter changes before seeding template version 3, so no
   runnable configuration can snapshot the new shape against an old worker.
3. Use the real embedded-template and Temporal tests to verify the snapshot contains the linear
   active graph, then run the local production-adapter acceptance scenario.
4. Roll back by redeploying the prior application and template seed only before any new-shape
   execution is started. After a new snapshot exists, rollback is not supported; retain the
   immutable execution/artifact records for audit and fix forward.

## Open Questions

None. The carousel PDF derivative is intentionally a narrow publish prerequisite, not a decision
to build the deferred general renderer substrate.
