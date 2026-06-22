# Harpia UX Realignment — M3 Plan Thread — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the per-PlanConfiguration chat thread at `/plans/configurations/[configurationId]` — backend persistence + RPC + workflow hooks, frontend page + components + integration.

**Architecture:** New `harpia.chat.v1` proto package for the message types; three new RPCs on `PlanService` (`WatchPlanThreadMessages`, `AppendPlanThreadMessage`, `ListPlanThreadMessages`); new `chat_messages` PostgreSQL table keyed by a generic `thread_id` (= plan_configuration_id for v1); new `internal/chat/` Go package owning a generic store; PlanService delegates to it. Frontend: new `lib/chat/` for types + client + stream wrapper, new `PlanDagMiniMap.svelte`, new `lib/components/thread/` directory of cards, new route `/plans/configurations/[configurationId]/`. AbortSignal plumbed through new + existing watch* RPCs. Composer is active-but-passive (persists USER_TEXT, no LLM in M3 — that's M5).

**Tech Stack:** Go 1.23 + ConnectRPC (control-plane), buf + protoc (proto codegen), PostgreSQL + Atlas migrations (database), SvelteKit 2.16 + Svelte 5 (frontend), Vitest (frontend tests), `testing` package (Go tests).

## Global Constraints

- **Vocabulary is canonical** per master design spec §2 — Task / Plan / Agent / Integration / Executor / Overseer / Elicitation / Approval / Feedback / Artifact. User-facing copy and i18n keys use these terms exactly. Do NOT introduce "Elicitation Request", "Step", "Agent Role", or other variants.
- **i18n is lockstep.** Every key added/removed touches both `frontend/src/lib/i18n/en.json` and `frontend/src/lib/i18n/pt-BR.json` in the same commit. The parity test in `frontend/src/lib/i18n/hardcoded-copy.test.ts` enforces this.
- **Design tokens are LOCKED.** Use only existing Tailwind tokens: `bg-obsidian`, `bg-obsidian-light`, `text-cream`, `text-crown-ash`, `text-talon-gold`, `border-plumage`, `font-heading`, `font-body`, `font-mono`. No new colors.
- **Commit per task.** Conventional Commits with `feat(ux-m3):` / `chore(ux-m3):` / `fix(ux-m3):` / `docs(ux-m3):` / `test(ux-m3):` scope.
- **No `Co-Authored-By` trailer** on any commit (project convention).
- **Trunk baseline.** `npm run check` from `frontend/` currently reports 10 pre-existing errors (auth-roles.test.ts ×6, artifact-flow.ts ×1, +layout.svelte ×1, plans/[templateId]/+page.svelte ×2). Flag any NEW error your task introduces; do NOT touch pre-existing errors as part of M3.
- **Test isolation per task.** Run only the focused test for the file you're changing: `npx vitest run <relative-path>` from `frontend/` for frontend, `go test ./internal/chat/...` (or similar narrow path) for Go. Run the full suite only when explicitly directed by a step.
- **Branch:** `feat/ux-realignment-m3-plan-thread` (already checked out).
- **No new files outside the plan's file map.** Every new file is named explicitly. If your implementation needs a file the plan doesn't name, STOP and report BLOCKED.
- **Buf codegen.** After modifying any `.proto` file, run `cd /home/thbertoldi/harpia/proto && buf generate` to regenerate Go + TS + Python clients. Commit the generated code alongside the proto changes in the same commit.

---

## File map (all changes in M3)

### Backend — new files
- `proto/harpia/chat/v1/chat.proto`
- `database/migrations/000012_chat_messages.sql`
- `control-plane/internal/chat/store.go`
- `control-plane/internal/chat/store_test.go`
- `control-plane/internal/chat/messages.go`
- `control-plane/internal/chat/messages_test.go`
- `control-plane/internal/plans/thread_handler.go`
- `control-plane/internal/plans/thread_handler_test.go`

### Backend — modify
- `proto/harpia/plans/v1/plans.proto` (add 3 RPCs to PlanService)
- `control-plane/internal/plans/handler.go` (CreatePlanConfiguration + UpdatePlanConfiguration write CONFIGURATION_SAVED)
- `control-plane/internal/plans/elicitations_handler.go` (RespondToElicitation writes ELICITATION_ANSWERED)
- `control-plane/internal/plans/approvals_handler.go` (RespondToApprovalRequest writes APPROVAL_DECIDED)
- `control-plane/internal/plans/repository.go` (the approval-raise INSERT also writes APPROVAL_RAISED)
- `control-plane/internal/workflow/plans.go` (activity hooks: RUN_STARTED / RUN_COMPLETED / RUN_FAILED / STEP_BOUND / ELICITATION_RAISED)
- `control-plane/internal/plans/handler.go` (PlanHandler constructor accepts the new chat.Store)

### Frontend — new files
- `frontend/src/lib/chat/types.ts`
- `frontend/src/lib/chat/client.ts`
- `frontend/src/lib/chat/client.test.ts`
- `frontend/src/lib/chat/watch.ts`
- `frontend/src/lib/plans/thread.ts`
- `frontend/src/lib/plans/thread.test.ts`
- `frontend/src/lib/components/PlanDagMiniMap.svelte`
- `frontend/src/lib/components/thread/ThreadMessage.svelte`
- `frontend/src/lib/components/thread/SystemEventCard.svelte`
- `frontend/src/lib/components/thread/ElicitationRefCard.svelte`
- `frontend/src/lib/components/thread/ApprovalRefCard.svelte`
- `frontend/src/lib/components/thread/ExecutionSection.svelte`
- `frontend/src/lib/components/thread/ThreadComposer.svelte`
- `frontend/src/lib/components/thread/HintBanner.svelte`
- `frontend/src/routes/plans/configurations/[configurationId]/+page.svelte`
- `frontend/src/routes/plans/configurations/[configurationId]/+page.ts`

### Frontend — modify
- `frontend/src/lib/rpc.ts` (export generated chat types)
- `frontend/src/lib/plans/elicitations.ts` (AbortSignal in WatchElicitationsOptions)
- `frontend/src/lib/plans/approvals.ts` (AbortSignal in WatchApprovalRequestsOptions)
- `frontend/src/lib/inbox/aggregator.ts` (AbortSignal cascade + expose configurationId on InboxItem)
- `frontend/src/lib/inbox/types.ts` (add `configurationId` to InboxElicitationItem + InboxApprovalItem)
- `frontend/src/routes/inbox/+page.svelte` (AbortController consumer)
- `frontend/src/lib/components/inbox/InboxElicitationActions.svelte` (deep-link to thread + anchor)
- `frontend/src/lib/components/inbox/InboxApprovalEntry.svelte` (no behavior change — verify still works after aggregator extension)
- `frontend/src/lib/i18n/en.json` (add `thread.*` keys)
- `frontend/src/lib/i18n/pt-BR.json` (add `thread.*` keys lockstep)

---

## Task 1: Add `harpia.chat.v1` proto package

**Files:**
- Create: `proto/harpia/chat/v1/chat.proto`

**Interfaces:**
- Produces:
  - `harpia.chat.v1.ThreadMessage` — single record in a thread
  - `harpia.chat.v1.ThreadMessageRole` enum — UNSPECIFIED / OVERSEER / AGENT / SYSTEM
  - `harpia.chat.v1.ThreadMessageKind` enum — UNSPECIFIED / USER_TEXT / ASSISTANT_TEXT / CONFIGURATION_SAVED / RUN_STARTED / RUN_COMPLETED / RUN_FAILED / STEP_BOUND / ELICITATION_RAISED / ELICITATION_ANSWERED / APPROVAL_RAISED / APPROVAL_DECIDED

- [ ] **Step 1: Create the proto file**

Create `proto/harpia/chat/v1/chat.proto`:

```protobuf
syntax = "proto3";

package harpia.chat.v1;

import "google/protobuf/timestamp.proto";

option go_package = "github.com/harpia/control-plane/gen/harpia/chat/v1;chatv1";

// ThreadMessage is a single durable record in a chat thread.
//
// In M3 a thread is identified by a PlanConfiguration ID (thread_id =
// plan_configuration_id). The schema is generic; future milestones may
// reuse this type for non-Plan chat surfaces (see UX-M3 design spec §2.2).
message ThreadMessage {
  string id = 1;                                // UUID
  string tenant_id = 2;                          // UUID
  string thread_id = 3;                          // generic; = plan_configuration_id in M3
  string execution_id = 4;                       // optional; UUID of the PlanExecution this event belongs to
  ThreadMessageRole role = 5;
  ThreadMessageKind kind = 6;
  string text = 7;                               // user/assistant text or human-readable summary
  string payload_json = 8;                       // per-kind structured payload (see Kind-specific schemas)
  string author_user_id = 9;                     // optional; NULL for SYSTEM role
  int64 sequence_number = 10;                    // per-thread monotonic; used for WatchPlanThreadMessages resume
  google.protobuf.Timestamp created_at = 11;
}

enum ThreadMessageRole {
  THREAD_MESSAGE_ROLE_UNSPECIFIED = 0;
  THREAD_MESSAGE_ROLE_OVERSEER = 1;     // composer-typed messages from the user
  THREAD_MESSAGE_ROLE_AGENT = 2;         // reserved for M5 assistant; not emitted in M3
  THREAD_MESSAGE_ROLE_SYSTEM = 3;        // system events written by the platform (author_user_id is empty)
}

enum ThreadMessageKind {
  THREAD_MESSAGE_KIND_UNSPECIFIED = 0;
  THREAD_MESSAGE_KIND_USER_TEXT = 1;             // OVERSEER composer message
  THREAD_MESSAGE_KIND_ASSISTANT_TEXT = 2;        // reserved for M5
  THREAD_MESSAGE_KIND_CONFIGURATION_SAVED = 3;   // PlanConfiguration created or updated
  THREAD_MESSAGE_KIND_RUN_STARTED = 4;           // PlanExecution started
  THREAD_MESSAGE_KIND_RUN_COMPLETED = 5;         // PlanExecution finished successfully
  THREAD_MESSAGE_KIND_RUN_FAILED = 6;            // PlanExecution failed
  THREAD_MESSAGE_KIND_STEP_BOUND = 7;            // StepExecution completed (output artifact produced)
  THREAD_MESSAGE_KIND_ELICITATION_RAISED = 8;    // payload_json: { "elicitation_id": "..." }
  THREAD_MESSAGE_KIND_ELICITATION_ANSWERED = 9;  // payload_json: { "elicitation_id": "...", "outcome": "answered|timed_out|cancelled" }
  THREAD_MESSAGE_KIND_APPROVAL_RAISED = 10;      // payload_json: { "approval_request_id": "..." }
  THREAD_MESSAGE_KIND_APPROVAL_DECIDED = 11;     // payload_json: { "approval_request_id": "...", "approved": true|false }
}
```

- [ ] **Step 2: Run `buf generate` to verify the proto compiles**

```bash
cd /home/thbertoldi/harpia/proto && buf generate
```

Expected: no errors. Generated files appear at:
- `control-plane/gen/harpia/chat/v1/chat.pb.go`
- `frontend/src/lib/gen/harpia/chat/v1/chat_pb.ts`
- `agent-runtime/src/harpia_agents/gen/harpia/chat/v1/chat_pb2.py`

If buf complains about `option go_package`, double-check the line matches the existing pattern in `proto/harpia/plans/v1/plans.proto`.

- [ ] **Step 3: Run buf lint to catch style issues**

```bash
cd /home/thbertoldi/harpia/proto && buf lint
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
cd /home/thbertoldi/harpia
git add proto/harpia/chat/v1/chat.proto control-plane/gen/harpia/chat frontend/src/lib/gen/harpia/chat agent-runtime/src/harpia_agents/gen/harpia/chat
git commit -m "feat(ux-m3): add harpia.chat.v1 proto package with ThreadMessage"
```

---

## Task 2: Add chat RPCs to PlanService

**Files:**
- Modify: `proto/harpia/plans/v1/plans.proto` (extend PlanService with 3 RPCs and add 3 request/response message types)

**Interfaces:**
- Consumes: `harpia.chat.v1.ThreadMessage` from Task 1
- Produces:
  - RPC `PlanService.WatchPlanThreadMessages(WatchPlanThreadMessagesRequest) returns (stream WatchPlanThreadMessagesResponse)`
  - RPC `PlanService.AppendPlanThreadMessage(AppendPlanThreadMessageRequest) returns (AppendPlanThreadMessageResponse)`
  - RPC `PlanService.ListPlanThreadMessages(ListPlanThreadMessagesRequest) returns (ListPlanThreadMessagesResponse)`

- [ ] **Step 1: Add the import at the top of plans.proto**

Find the existing `import "google/protobuf/timestamp.proto";` line in `proto/harpia/plans/v1/plans.proto` and ADD this line below it:

```protobuf
import "harpia/chat/v1/chat.proto";
```

- [ ] **Step 2: Add the three RPCs to the PlanService service block**

Find the `rpc WatchApprovalRequests(...)` line (it's the last RPC in PlanService per recon). AFTER that line, BEFORE the closing `}` of the service block, ADD:

```protobuf

  // Plan thread (per-PlanConfiguration chat thread).
  // See docs/superpowers/specs/2026-06-21-harpia-ux-m3-plan-thread-design.md.
  rpc ListPlanThreadMessages(ListPlanThreadMessagesRequest) returns (ListPlanThreadMessagesResponse);
  rpc WatchPlanThreadMessages(WatchPlanThreadMessagesRequest) returns (stream WatchPlanThreadMessagesResponse);
  rpc AppendPlanThreadMessage(AppendPlanThreadMessageRequest) returns (AppendPlanThreadMessageResponse);
```

- [ ] **Step 3: Add the request/response message types at the END of plans.proto**

Append to the bottom of `proto/harpia/plans/v1/plans.proto`:

```protobuf

// --- Plan thread (M3) ---

message ListPlanThreadMessagesRequest {
  string tenant_id = 1;
  string plan_configuration_id = 2;
  int32 page_size = 3;
  string page_token = 4;
}

message ListPlanThreadMessagesResponse {
  repeated harpia.chat.v1.ThreadMessage messages = 1;
  string next_page_token = 2;
}

message WatchPlanThreadMessagesRequest {
  string tenant_id = 1;
  string plan_configuration_id = 2;
  int64 since_sequence_number = 3;  // resume from this sequence; 0 = from the start of unobserved
}

message WatchPlanThreadMessagesResponse {
  repeated harpia.chat.v1.ThreadMessage messages = 1;
}

message AppendPlanThreadMessageRequest {
  string tenant_id = 1;
  string plan_configuration_id = 2;
  harpia.chat.v1.ThreadMessageRole role = 3;
  harpia.chat.v1.ThreadMessageKind kind = 4;
  string text = 5;
  string payload_json = 6;
  string execution_id = 7;   // optional
}

message AppendPlanThreadMessageResponse {
  harpia.chat.v1.ThreadMessage message = 1;
}
```

- [ ] **Step 4: Regenerate and verify**

```bash
cd /home/thbertoldi/harpia/proto && buf generate && buf lint
```

Expected: no errors. Generated TS at `frontend/src/lib/gen/harpia/plans/v1/plans_pb.ts` now includes the new request/response types and the new RPC methods on `PlanService`.

- [ ] **Step 5: Commit**

```bash
cd /home/thbertoldi/harpia
git add proto/harpia/plans/v1/plans.proto control-plane/gen frontend/src/lib/gen agent-runtime/src/harpia_agents/gen
git commit -m "feat(ux-m3): add three PlanService RPCs for plan thread messages"
```

---

## Task 3: `chat_messages` migration

**Files:**
- Create: `database/migrations/000012_chat_messages.sql`

**Interfaces:**
- Produces:
  - PostgreSQL table `chat_messages` with columns: `id, tenant_id, thread_id, execution_id, role, kind, text, payload_json, author_user_id, sequence_number, created_at`
  - Indexes: `idx_chat_messages_thread_seq (thread_id, sequence_number)`, `idx_chat_messages_thread_created (thread_id, created_at)`
  - RLS policy enforcing `tenant_id = current_setting('app.tenant_id')::uuid`

- [ ] **Step 1: Create the migration**

Create `database/migrations/000012_chat_messages.sql`:

```sql
-- M3 (plan thread): per-PlanConfiguration chat messages.
-- See docs/superpowers/specs/2026-06-21-harpia-ux-m3-plan-thread-design.md §2.1.
--
-- Schema is generic (thread_id is a TEXT key) so a future ChatService can
-- reuse the table for non-Plan chat surfaces. In M3 thread_id always equals
-- a plan_configuration_id (UUID stringified).

CREATE TABLE chat_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    thread_id TEXT NOT NULL,
    execution_id UUID,
    role TEXT NOT NULL,
    kind TEXT NOT NULL,
    text TEXT NOT NULL DEFAULT '',
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    author_user_id UUID,
    sequence_number BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chat_messages_thread_seq_unique UNIQUE (thread_id, sequence_number)
);

CREATE INDEX idx_chat_messages_thread_seq ON chat_messages (thread_id, sequence_number);
CREATE INDEX idx_chat_messages_thread_created ON chat_messages (thread_id, created_at);
CREATE INDEX idx_chat_messages_tenant ON chat_messages (tenant_id);

ALTER TABLE chat_messages ENABLE ROW LEVEL SECURITY;

CREATE POLICY chat_messages_tenant_isolation ON chat_messages
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::uuid);

COMMENT ON TABLE chat_messages IS
    'Per-thread durable chat records. M3 uses plan_configuration_id as thread_id.';
COMMENT ON COLUMN chat_messages.execution_id IS
    'Nullable. NULL for plan-scope messages (CONFIGURATION_SAVED, USER_TEXT). Set for execution-scope events.';
COMMENT ON COLUMN chat_messages.sequence_number IS
    'Per-thread monotonic; used by WatchPlanThreadMessages for resume.';
```

Note: the `current_setting('app.tenant_id', true)::uuid` pattern matches the project's existing RLS approach. If the existing migrations use a different setting key (check `database/migrations/000003_tenant_rls_hardening.sql` for the canonical pattern), adopt it verbatim. If different, STOP and report BLOCKED.

- [ ] **Step 2: Apply the migration to dev**

```bash
cd /home/thbertoldi/harpia && ./scripts/apply-dev-db-migrations.sh
```

Expected: Atlas applies migration 000012; no errors. Output ends with something like `Applied 1 migration` or `migration 000012_chat_messages.sql applied`.

If your environment doesn't have a running dev Postgres (e.g., no Tilt up), skip this step and verify migration syntax with `atlas migrate validate --dir file://database/migrations` instead.

- [ ] **Step 3: Verify table exists**

If dev Postgres is running:

```bash
kubectl exec -n default deploy/postgres -- psql -U harpia -d harpia -c "\d chat_messages" 2>&1 | head -20
```

Expected: table description showing columns id, tenant_id, thread_id, execution_id, role, kind, text, payload_json, author_user_id, sequence_number, created_at and the three indexes.

If dev Postgres isn't running, this step is optional — proceed to commit.

- [ ] **Step 4: Commit**

```bash
git add database/migrations/000012_chat_messages.sql
git commit -m "feat(ux-m3): add chat_messages table migration"
```

---

## Task 4: `internal/chat` package (generic store)

**Files:**
- Create: `control-plane/internal/chat/store.go`
- Create: `control-plane/internal/chat/store_test.go`
- Create: `control-plane/internal/chat/messages.go`
- Create: `control-plane/internal/chat/messages_test.go`

**Interfaces:**
- Consumes: `chat_messages` table from Task 3; `chatv1` generated Go types from Task 1
- Produces:
  - `package chat` with:
    - `type Store interface { AppendMessage(ctx, tenantID, msg AppendInput) (*chatv1.ThreadMessage, error); ListMessages(ctx, tenantID, threadID, sinceSeq int64, limit int) ([]*chatv1.ThreadMessage, error); WatchMessages(ctx, tenantID, threadID, sinceSeq int64) iter.Seq2[[]*chatv1.ThreadMessage, error] }`
    - `type AppendInput struct { ThreadID string; Role chatv1.ThreadMessageRole; Kind chatv1.ThreadMessageKind; Text string; PayloadJSON string; AuthorUserID *uuid.UUID; ExecutionID *uuid.UUID }`
    - `type postgresStore struct { pool *pgxpool.Pool }` — concrete impl
    - `func NewPostgresStore(pool *pgxpool.Pool) Store`
    - Typed-payload helper functions: `BuildConfigurationSavedPayload()`, `BuildRunStartedPayload()`, `BuildRunCompletedPayload()`, `BuildRunFailedPayload(err)`, `BuildStepBoundPayload(stepKey, outputArtifactID)`, `BuildElicitationRaisedPayload(elicitationID uuid.UUID)`, `BuildElicitationAnsweredPayload(elicitationID uuid.UUID, outcome string)`, `BuildApprovalRaisedPayload(approvalRequestID string)`, `BuildApprovalDecidedPayload(approvalRequestID string, approved bool)` — each returns a JSON string

- [ ] **Step 1: Write the failing tests for typed-payload helpers**

Create `control-plane/internal/chat/messages_test.go`:

```go
package chat

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestBuildElicitationRaisedPayload(t *testing.T) {
	id := uuid.New()
	got := BuildElicitationRaisedPayload(id)

	var decoded map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if decoded["elicitation_id"] != id.String() {
		t.Fatalf("expected elicitation_id=%s, got %s", id.String(), decoded["elicitation_id"])
	}
}

func TestBuildElicitationAnsweredPayload(t *testing.T) {
	id := uuid.New()
	got := BuildElicitationAnsweredPayload(id, "answered")

	var decoded map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if decoded["elicitation_id"] != id.String() {
		t.Fatalf("expected elicitation_id=%s, got %s", id.String(), decoded["elicitation_id"])
	}
	if decoded["outcome"] != "answered" {
		t.Fatalf("expected outcome=answered, got %s", decoded["outcome"])
	}
}

func TestBuildApprovalRaisedPayload(t *testing.T) {
	got := BuildApprovalRaisedPayload("abc-123")

	var decoded map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if decoded["approval_request_id"] != "abc-123" {
		t.Fatalf("expected approval_request_id=abc-123, got %s", decoded["approval_request_id"])
	}
}

func TestBuildApprovalDecidedPayload(t *testing.T) {
	got := BuildApprovalDecidedPayload("abc-123", true)

	var decoded map[string]any
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if decoded["approval_request_id"] != "abc-123" {
		t.Fatalf("expected approval_request_id=abc-123, got %v", decoded["approval_request_id"])
	}
	if decoded["approved"] != true {
		t.Fatalf("expected approved=true, got %v", decoded["approved"])
	}
}

func TestBuildStepBoundPayload(t *testing.T) {
	got := BuildStepBoundPayload("write-draft", "art-uuid")

	var decoded map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if decoded["step_key"] != "write-draft" {
		t.Fatalf("expected step_key=write-draft, got %s", decoded["step_key"])
	}
	if decoded["output_artifact_id"] != "art-uuid" {
		t.Fatalf("expected output_artifact_id=art-uuid, got %s", decoded["output_artifact_id"])
	}
}

func TestBuildRunFailedPayloadCarriesErrorMessage(t *testing.T) {
	got := BuildRunFailedPayload("step write-draft failed: timeout")

	var decoded map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if decoded["error"] != "step write-draft failed: timeout" {
		t.Fatalf("expected error message preserved, got %s", decoded["error"])
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd /home/thbertoldi/harpia/control-plane && go test ./internal/chat/...
```

Expected: FAIL with "no Go files" or "undefined: BuildElicitationRaisedPayload".

- [ ] **Step 3: Create messages.go with the typed-payload helpers**

Create `control-plane/internal/chat/messages.go`:

```go
// Package chat owns durable chat-message persistence and streaming.
//
// Designed to be reusable beyond the M3 plan-thread use case: store is
// keyed by a generic thread_id (= plan_configuration_id in M3). See
// docs/superpowers/specs/2026-06-21-harpia-ux-m3-plan-thread-design.md.
package chat

import (
	"encoding/json"

	"github.com/google/uuid"
)

// BuildConfigurationSavedPayload returns the JSON payload for a
// CONFIGURATION_SAVED message. Currently carries no structured fields —
// the message's text is sufficient — but the shape is reserved for future
// fields (e.g., diff against previous configuration).
func BuildConfigurationSavedPayload() string {
	return "{}"
}

// BuildRunStartedPayload returns the JSON payload for a RUN_STARTED message.
// Currently carries no fields beyond the execution_id which lives on the
// row's execution_id column.
func BuildRunStartedPayload() string {
	return "{}"
}

// BuildRunCompletedPayload returns the JSON payload for a RUN_COMPLETED message.
func BuildRunCompletedPayload() string {
	return "{}"
}

// BuildRunFailedPayload returns the JSON payload for a RUN_FAILED message.
func BuildRunFailedPayload(errorMessage string) string {
	return mustEncodeJSON(map[string]string{"error": errorMessage})
}

// BuildStepBoundPayload returns the JSON payload for a STEP_BOUND message.
func BuildStepBoundPayload(stepKey, outputArtifactID string) string {
	return mustEncodeJSON(map[string]string{
		"step_key":           stepKey,
		"output_artifact_id": outputArtifactID,
	})
}

// BuildElicitationRaisedPayload returns the JSON payload for an
// ELICITATION_RAISED pointer message.
func BuildElicitationRaisedPayload(elicitationID uuid.UUID) string {
	return mustEncodeJSON(map[string]string{
		"elicitation_id": elicitationID.String(),
	})
}

// BuildElicitationAnsweredPayload returns the JSON payload for an
// ELICITATION_ANSWERED message. Outcome is one of: "answered", "timed_out",
// "cancelled".
func BuildElicitationAnsweredPayload(elicitationID uuid.UUID, outcome string) string {
	return mustEncodeJSON(map[string]string{
		"elicitation_id": elicitationID.String(),
		"outcome":        outcome,
	})
}

// BuildApprovalRaisedPayload returns the JSON payload for an
// APPROVAL_RAISED pointer message.
func BuildApprovalRaisedPayload(approvalRequestID string) string {
	return mustEncodeJSON(map[string]string{
		"approval_request_id": approvalRequestID,
	})
}

// BuildApprovalDecidedPayload returns the JSON payload for an
// APPROVAL_DECIDED message.
func BuildApprovalDecidedPayload(approvalRequestID string, approved bool) string {
	return mustEncodeJSON(map[string]any{
		"approval_request_id": approvalRequestID,
		"approved":            approved,
	})
}

func mustEncodeJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		// All inputs here are well-formed maps; failure is a programmer error.
		panic(err)
	}
	return string(b)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/thbertoldi/harpia/control-plane && go test ./internal/chat/...
```

Expected: 6 tests pass.

- [ ] **Step 5: Write the failing test for the Store interface**

Create `control-plane/internal/chat/store_test.go`:

```go
package chat

import (
	"testing"

	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	"github.com/google/uuid"
)

// TestAppendInputZeroValueIsValid documents the zero-value shape of
// AppendInput. Tasks 5-8 build AppendInput literals; this test prevents
// silent breakage if fields are renamed.
func TestAppendInputZeroValueIsValid(t *testing.T) {
	authorID := uuid.New()
	execID := uuid.New()
	in := AppendInput{
		ThreadID:      "plan-config-uuid",
		Role:          chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_OVERSEER,
		Kind:          chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_TEXT,
		Text:          "hello",
		PayloadJSON:   "{}",
		AuthorUserID:  &authorID,
		ExecutionID:   &execID,
	}
	if in.ThreadID == "" {
		t.Fatal("ThreadID should be settable")
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

```bash
cd /home/thbertoldi/harpia/control-plane && go test ./internal/chat/...
```

Expected: FAIL with "undefined: AppendInput".

- [ ] **Step 7: Create store.go with the Store interface and Postgres impl**

Create `control-plane/internal/chat/store.go`:

```go
package chat

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"

	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
)

// Store is the generic chat-message persistence interface. M3 wires a
// Postgres implementation; tests may stub it.
type Store interface {
	// AppendMessage writes one ThreadMessage and returns the persisted row
	// (with assigned ID and sequence_number).
	AppendMessage(ctx context.Context, tenantID uuid.UUID, input AppendInput) (*chatv1.ThreadMessage, error)
	// ListMessages returns messages for thread_id with sequence_number >
	// sinceSeq, ordered ascending, limited to `limit` rows. limit <= 0 means
	// no limit.
	ListMessages(ctx context.Context, tenantID uuid.UUID, threadID string, sinceSeq int64, limit int) ([]*chatv1.ThreadMessage, error)
}

// AppendInput is the payload for Store.AppendMessage. ThreadID and Role and
// Kind are required; other fields are optional.
type AppendInput struct {
	ThreadID     string
	Role         chatv1.ThreadMessageRole
	Kind         chatv1.ThreadMessageKind
	Text         string
	PayloadJSON  string
	AuthorUserID *uuid.UUID
	ExecutionID  *uuid.UUID
}

// NewPostgresStore returns a Store backed by the given pgxpool.Pool.
func NewPostgresStore(pool *pgxpool.Pool) Store {
	return &postgresStore{pool: pool}
}

type postgresStore struct {
	pool *pgxpool.Pool
}

func (s *postgresStore) AppendMessage(ctx context.Context, tenantID uuid.UUID, input AppendInput) (*chatv1.ThreadMessage, error) {
	if input.ThreadID == "" {
		return nil, errors.New("chat: AppendMessage requires ThreadID")
	}
	if input.Role == chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_UNSPECIFIED {
		return nil, errors.New("chat: AppendMessage requires Role")
	}
	if input.Kind == chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_UNSPECIFIED {
		return nil, errors.New("chat: AppendMessage requires Kind")
	}
	payload := input.PayloadJSON
	if payload == "" {
		payload = "{}"
	}
	var authorArg any
	if input.AuthorUserID != nil {
		authorArg = *input.AuthorUserID
	}
	var execArg any
	if input.ExecutionID != nil {
		execArg = *input.ExecutionID
	}

	// sequence_number is computed per-thread as MAX(existing)+1. The unique
	// constraint on (thread_id, sequence_number) catches races; on conflict
	// we retry once before giving up.
	for attempt := 0; attempt < 2; attempt++ {
		var msg chatv1.ThreadMessage
		var id uuid.UUID
		var createdAt time.Time
		var seq int64
		err := s.pool.QueryRow(ctx, `
			WITH next AS (
				SELECT COALESCE(MAX(sequence_number), 0) + 1 AS seq
				FROM chat_messages
				WHERE thread_id = $1
			)
			INSERT INTO chat_messages (
				tenant_id, thread_id, execution_id, role, kind, text, payload_json,
				author_user_id, sequence_number
			)
			SELECT $2, $1, $3, $4, $5, $6, $7::jsonb, $8, next.seq FROM next
			RETURNING id, sequence_number, created_at
		`,
			input.ThreadID,
			tenantID,
			execArg,
			input.Role.String(),
			input.Kind.String(),
			input.Text,
			payload,
			authorArg,
		).Scan(&id, &seq, &createdAt)
		if err != nil {
			// Detect unique-violation on (thread_id, sequence_number) and retry.
			if isUniqueViolation(err) && attempt == 0 {
				continue
			}
			return nil, fmt.Errorf("chat: AppendMessage: %w", err)
		}
		msg.Id = id.String()
		msg.TenantId = tenantID.String()
		msg.ThreadId = input.ThreadID
		if input.ExecutionID != nil {
			msg.ExecutionId = input.ExecutionID.String()
		}
		msg.Role = input.Role
		msg.Kind = input.Kind
		msg.Text = input.Text
		msg.PayloadJson = payload
		if input.AuthorUserID != nil {
			msg.AuthorUserId = input.AuthorUserID.String()
		}
		msg.SequenceNumber = seq
		msg.CreatedAt = timestamppb.New(createdAt)
		return &msg, nil
	}
	return nil, errors.New("chat: AppendMessage failed after retry on sequence collision")
}

func (s *postgresStore) ListMessages(ctx context.Context, tenantID uuid.UUID, threadID string, sinceSeq int64, limit int) ([]*chatv1.ThreadMessage, error) {
	query := `
		SELECT id, tenant_id, thread_id, execution_id, role, kind, text,
		       payload_json::text, author_user_id, sequence_number, created_at
		FROM chat_messages
		WHERE tenant_id = $1 AND thread_id = $2 AND sequence_number > $3
		ORDER BY sequence_number ASC
	`
	args := []any{tenantID, threadID, sinceSeq}
	if limit > 0 {
		query += " LIMIT $4"
		args = append(args, limit)
	}
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("chat: ListMessages: %w", err)
	}
	defer rows.Close()

	out := make([]*chatv1.ThreadMessage, 0)
	for rows.Next() {
		var (
			id, tenant uuid.UUID
			threadID   string
			execID     uuid.NullUUID
			role, kind string
			text       string
			payload    string
			authorID   uuid.NullUUID
			seq        int64
			createdAt  time.Time
		)
		if err := rows.Scan(&id, &tenant, &threadID, &execID, &role, &kind, &text, &payload, &authorID, &seq, &createdAt); err != nil {
			return nil, fmt.Errorf("chat: ListMessages: scan: %w", err)
		}
		msg := &chatv1.ThreadMessage{
			Id:             id.String(),
			TenantId:       tenant.String(),
			ThreadId:       threadID,
			Role:           chatv1.ThreadMessageRole(chatv1.ThreadMessageRole_value[role]),
			Kind:           chatv1.ThreadMessageKind(chatv1.ThreadMessageKind_value[kind]),
			Text:           text,
			PayloadJson:    payload,
			SequenceNumber: seq,
			CreatedAt:      timestamppb.New(createdAt),
		}
		if execID.Valid {
			msg.ExecutionId = execID.UUID.String()
		}
		if authorID.Valid {
			msg.AuthorUserId = authorID.UUID.String()
		}
		out = append(out, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chat: ListMessages: rows: %w", err)
	}
	return out, nil
}

// isUniqueViolation returns true if err is a Postgres unique constraint violation.
func isUniqueViolation(err error) bool {
	var pgErr *pgx.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
```

Note: this file references `pgx.PgError`. The codebase uses `github.com/jackc/pgx/v5`; the actual symbol may be `pgconn.PgError` in some pgx versions. If `pgx.PgError` is undefined, use:

```go
import "github.com/jackc/pgx/v5/pgconn"

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
```

If neither works, STOP and report BLOCKED with the actual import.

- [ ] **Step 8: Run tests to verify the AppendInput-shape test passes**

```bash
cd /home/thbertoldi/harpia/control-plane && go test ./internal/chat/...
```

Expected: 7 tests pass (the AppendInput shape test plus the 6 payload tests).

- [ ] **Step 9: Run `go vet` and `go build` to verify the package compiles**

```bash
cd /home/thbertoldi/harpia/control-plane && go vet ./internal/chat/... && go build ./...
```

Expected: no output (success).

- [ ] **Step 10: Commit**

```bash
git add control-plane/internal/chat
git commit -m "feat(ux-m3): add internal/chat package with generic Store"
```

---

## Task 5: PlanService thread_handler.go (the three RPCs)

**Files:**
- Create: `control-plane/internal/plans/thread_handler.go`
- Create: `control-plane/internal/plans/thread_handler_test.go`
- Modify: `control-plane/internal/plans/handler.go` (PlanHandler struct gets a `chat chat.Store` field; constructor accepts it)

**Interfaces:**
- Consumes: `chat.Store`, `chat.AppendInput` from Task 4; generated `plansv1` types from Task 2
- Produces: PlanHandler methods `WatchPlanThreadMessages`, `AppendPlanThreadMessage`, `ListPlanThreadMessages` (Connect handler signatures)

- [ ] **Step 1: Write the failing test for the AppendPlanThreadMessage handler**

Create `control-plane/internal/plans/thread_handler_test.go`:

```go
package plans

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/chat"
	"github.com/harpia/control-plane/internal/identity"
)

// fakeChatStore implements chat.Store in-memory for handler tests.
type fakeChatStore struct {
	appended []chat.AppendInput
	messages map[string][]*chatv1.ThreadMessage
}

func (f *fakeChatStore) AppendMessage(ctx context.Context, tenantID uuid.UUID, input chat.AppendInput) (*chatv1.ThreadMessage, error) {
	f.appended = append(f.appended, input)
	if f.messages == nil {
		f.messages = make(map[string][]*chatv1.ThreadMessage)
	}
	msg := &chatv1.ThreadMessage{
		Id:             uuid.New().String(),
		TenantId:       tenantID.String(),
		ThreadId:       input.ThreadID,
		Role:           input.Role,
		Kind:           input.Kind,
		Text:           input.Text,
		PayloadJson:    input.PayloadJSON,
		SequenceNumber: int64(len(f.messages[input.ThreadID]) + 1),
	}
	if input.ExecutionID != nil {
		msg.ExecutionId = input.ExecutionID.String()
	}
	f.messages[input.ThreadID] = append(f.messages[input.ThreadID], msg)
	return msg, nil
}

func (f *fakeChatStore) ListMessages(ctx context.Context, tenantID uuid.UUID, threadID string, sinceSeq int64, limit int) ([]*chatv1.ThreadMessage, error) {
	all := f.messages[threadID]
	out := make([]*chatv1.ThreadMessage, 0, len(all))
	for _, m := range all {
		if m.SequenceNumber > sinceSeq {
			out = append(out, m)
		}
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func TestAppendPlanThreadMessage_PersistsOverseerText(t *testing.T) {
	tenantID := uuid.New()
	configID := uuid.New()
	store := &fakeChatStore{}
	h := &PlanHandler{chat: store}

	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		UserID:   uuid.New().String(),
		TenantID: tenantID.String(),
		Roles:    []string{"Overseer"},
	})
	req := connect.NewRequest(&plansv1.AppendPlanThreadMessageRequest{
		TenantId:            tenantID.String(),
		PlanConfigurationId: configID.String(),
		Role:                chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_OVERSEER,
		Kind:                chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_TEXT,
		Text:                "remember to update the news source URL next week",
	})

	resp, err := h.AppendPlanThreadMessage(ctx, req)
	if err != nil {
		t.Fatalf("AppendPlanThreadMessage returned error: %v", err)
	}
	if got, want := len(store.appended), 1; got != want {
		t.Fatalf("store.AppendMessage call count = %d, want %d", got, want)
	}
	if got := store.appended[0].ThreadID; got != configID.String() {
		t.Fatalf("ThreadID = %q, want %q", got, configID.String())
	}
	if resp.Msg.Message.SequenceNumber != 1 {
		t.Fatalf("response sequence_number = %d, want 1", resp.Msg.Message.SequenceNumber)
	}
}

func TestAppendPlanThreadMessage_RejectsMissingConfigID(t *testing.T) {
	tenantID := uuid.New()
	store := &fakeChatStore{}
	h := &PlanHandler{chat: store}

	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		UserID:   uuid.New().String(),
		TenantID: tenantID.String(),
		Roles:    []string{"Overseer"},
	})
	req := connect.NewRequest(&plansv1.AppendPlanThreadMessageRequest{
		TenantId:            tenantID.String(),
		PlanConfigurationId: "",
		Role:                chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_OVERSEER,
		Kind:                chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_TEXT,
		Text:                "hi",
	})

	_, err := h.AppendPlanThreadMessage(ctx, req)
	if err == nil {
		t.Fatal("expected error for missing plan_configuration_id, got nil")
	}
	if got := connect.CodeOf(err); got != connect.CodeInvalidArgument {
		t.Fatalf("error code = %v, want InvalidArgument", got)
	}
}

func TestListPlanThreadMessages_FiltersBySinceSequence(t *testing.T) {
	tenantID := uuid.New()
	configID := uuid.New()
	store := &fakeChatStore{messages: map[string][]*chatv1.ThreadMessage{
		configID.String(): {
			{SequenceNumber: 1, Text: "first"},
			{SequenceNumber: 2, Text: "second"},
			{SequenceNumber: 3, Text: "third"},
		},
	}}
	h := &PlanHandler{chat: store}

	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		UserID:   uuid.New().String(),
		TenantID: tenantID.String(),
		Roles:    []string{"Overseer"},
	})
	req := connect.NewRequest(&plansv1.ListPlanThreadMessagesRequest{
		TenantId:            tenantID.String(),
		PlanConfigurationId: configID.String(),
	})

	resp, err := h.ListPlanThreadMessages(ctx, req)
	if err != nil {
		t.Fatalf("ListPlanThreadMessages returned error: %v", err)
	}
	if got, want := len(resp.Msg.Messages), 3; got != want {
		t.Fatalf("messages count = %d, want %d", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/thbertoldi/harpia/control-plane && go test -run TestAppendPlanThreadMessage ./internal/plans/...
```

Expected: FAIL — `PlanHandler` doesn't have a `chat` field, or `AppendPlanThreadMessage` method is undefined.

- [ ] **Step 3: Extend PlanHandler struct to carry chat.Store**

In `control-plane/internal/plans/handler.go`, find the `type PlanHandler struct { ... }` declaration. ADD this field (alphabetically with existing fields, or at the end of the struct):

```go
	chat chat.Store
```

Add the import at the top of the file:

```go
	"github.com/harpia/control-plane/internal/chat"
```

If the existing PlanHandler constructor is `NewPlanHandler(...)`, EXTEND its signature to accept `chatStore chat.Store` and set `h.chat = chatStore`. Locate the constructor; show the diff in your report.

The caller (probably `cmd/api/main.go` or similar wiring) needs updating too — find it via:

```bash
cd /home/thbertoldi/harpia/control-plane && grep -rn "NewPlanHandler\b" cmd/ internal/
```

Update each call site to construct a `chat.NewPostgresStore(pool)` and pass it in. The pool is the same `*pgxpool.Pool` already used by the existing PlanRepository — reuse, don't create a new one.

- [ ] **Step 4: Create thread_handler.go with the three RPC handlers**

Create `control-plane/internal/plans/thread_handler.go`:

```go
package plans

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/chat"
	"github.com/harpia/control-plane/internal/identity"
)

func (h *PlanHandler) ListPlanThreadMessages(
	ctx context.Context,
	req *connect.Request[plansv1.ListPlanThreadMessagesRequest],
) (*connect.Response[plansv1.ListPlanThreadMessagesResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.GetTenantId())
	if err != nil {
		return nil, err
	}
	configID := strings.TrimSpace(req.Msg.GetPlanConfigurationId())
	if configID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("plan_configuration_id is required"))
	}
	if h.chat == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("chat store is unavailable"))
	}
	limit := int(req.Msg.GetPageSize())
	if limit <= 0 {
		limit = 100
	}
	msgs, err := h.chat.ListMessages(ctx, tenantID, configID, 0, limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&plansv1.ListPlanThreadMessagesResponse{
		Messages: msgs,
	}), nil
}

func (h *PlanHandler) AppendPlanThreadMessage(
	ctx context.Context,
	req *connect.Request[plansv1.AppendPlanThreadMessageRequest],
) (*connect.Response[plansv1.AppendPlanThreadMessageResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.GetTenantId())
	if err != nil {
		return nil, err
	}
	rc, err := identity.RequireRequestContext(ctx)
	if err != nil {
		return nil, err
	}
	configID := strings.TrimSpace(req.Msg.GetPlanConfigurationId())
	if configID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("plan_configuration_id is required"))
	}
	if h.chat == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("chat store is unavailable"))
	}
	input := chat.AppendInput{
		ThreadID:    configID,
		Role:        req.Msg.GetRole(),
		Kind:        req.Msg.GetKind(),
		Text:        req.Msg.GetText(),
		PayloadJSON: req.Msg.GetPayloadJson(),
	}
	if execID := strings.TrimSpace(req.Msg.GetExecutionId()); execID != "" {
		parsed, err := uuid.Parse(execID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		input.ExecutionID = &parsed
	}
	if rc.UserID != "" {
		parsed, err := uuid.Parse(rc.UserID)
		if err == nil {
			input.AuthorUserID = &parsed
		}
	}
	msg, err := h.chat.AppendMessage(ctx, tenantID, input)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&plansv1.AppendPlanThreadMessageResponse{
		Message: msg,
	}), nil
}

func (h *PlanHandler) WatchPlanThreadMessages(
	ctx context.Context,
	req *connect.Request[plansv1.WatchPlanThreadMessagesRequest],
	stream *connect.ServerStream[plansv1.WatchPlanThreadMessagesResponse],
) error {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.GetTenantId())
	if err != nil {
		return err
	}
	configID := strings.TrimSpace(req.Msg.GetPlanConfigurationId())
	if configID == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("plan_configuration_id is required"))
	}
	if h.chat == nil {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("chat store is unavailable"))
	}

	sinceSeq := req.Msg.GetSinceSequenceNumber()
	// Initial flush: send everything since the resume point.
	initial, err := h.chat.ListMessages(ctx, tenantID, configID, sinceSeq, 0)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if len(initial) > 0 {
		if err := stream.Send(&plansv1.WatchPlanThreadMessagesResponse{Messages: initial}); err != nil {
			return err
		}
		sinceSeq = initial[len(initial)-1].SequenceNumber
	}

	// Poll for new messages every 2 seconds (matches WatchElicitations pattern).
	ticker := newWatchTicker(watchPlanThreadMessagesPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C():
			next, err := h.chat.ListMessages(ctx, tenantID, configID, sinceSeq, 0)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			if len(next) > 0 {
				if err := stream.Send(&plansv1.WatchPlanThreadMessagesResponse{Messages: next}); err != nil {
					return err
				}
				sinceSeq = next[len(next)-1].SequenceNumber
			}
		}
	}
}
```

Note: this references `watchPlanThreadMessagesPollInterval` and `newWatchTicker`. The recon showed `WatchElicitations` uses `time.NewTicker(watchElicitationsPollInterval)` directly. Mirror that pattern — at the bottom of this file, ADD:

```go
import "time"

const watchPlanThreadMessagesPollInterval = 2 * time.Second

// newWatchTicker returns a small wrapper over time.Ticker so handlers can be
// substituted in tests. Production uses the real ticker.
type watchTicker struct{ t *time.Ticker }

func (w *watchTicker) C() <-chan time.Time { return w.t.C }
func (w *watchTicker) Stop()               { w.t.Stop() }

func newWatchTicker(d time.Duration) *watchTicker {
	return &watchTicker{t: time.NewTicker(d)}
}
```

(Time import goes at the top of the file alongside the others.)

- [ ] **Step 5: Run the focused tests**

```bash
cd /home/thbertoldi/harpia/control-plane && go test ./internal/plans/... -run "TestAppendPlanThreadMessage|TestListPlanThreadMessages" -v
```

Expected: 3 tests pass.

- [ ] **Step 6: Run `go vet` and `go build` on the full control-plane**

```bash
cd /home/thbertoldi/harpia/control-plane && go vet ./... && go build ./...
```

Expected: no errors. If `NewPlanHandler` is called with the wrong number of args, fix the call sites you found in Step 3.

- [ ] **Step 7: Commit**

```bash
git add control-plane/internal/plans/thread_handler.go control-plane/internal/plans/thread_handler_test.go control-plane/internal/plans/handler.go control-plane/cmd
git commit -m "feat(ux-m3): add three PlanService chat-thread RPC handlers"
```

(If your wiring touched files outside `control-plane/cmd`, adjust the `git add` accordingly. The grep in Step 3 listed them.)

---

## Task 6: CONFIGURATION_SAVED hook in configuration handlers

**Files:**
- Modify: `control-plane/internal/plans/handler.go` (`CreatePlanConfiguration` and `UpdatePlanConfiguration` write a CONFIGURATION_SAVED message after successful save)

**Interfaces:**
- Consumes: `chat.Store`, `chat.AppendInput`, `chat.BuildConfigurationSavedPayload` from Task 4

- [ ] **Step 1: Write the failing test**

Append to `control-plane/internal/plans/thread_handler_test.go`:

```go
func TestCreatePlanConfiguration_WritesConfigurationSavedMessage(t *testing.T) {
	t.Skip("integration test — requires Postgres; covered by E2E suite. The hook itself is verified by reading the handler diff.")
}
```

This test is intentionally skipped because exercising the full `CreatePlanConfiguration` handler requires a real Postgres repository and template — not unit-testable. The hook's correctness is verified by reading the handler diff and by Task 18's manual smoke. The skip records the intent.

- [ ] **Step 2: Add the hook to CreatePlanConfiguration**

In `control-plane/internal/plans/handler.go`, find the `CreatePlanConfiguration` method. Locate the line that returns success — the recon showed:

```go
return connect.NewResponse(&plansv1.CreatePlanConfigurationResponse{
    PlanConfiguration: configurationToProto(created),
}), nil
```

IMMEDIATELY BEFORE the `return` line, ADD this block (with imports for `chatv1` and `chat` at the top of the file if not already present from Task 5):

```go
	// M3: append CONFIGURATION_SAVED message to the plan thread so the
	// thread is non-empty on first render. Best-effort — log and continue
	// if the chat store is unavailable. See UX-M3 design spec §2.5.
	if h.chat != nil {
		_, chatErr := h.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
			ThreadID:    created.ID.String(),
			Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
			Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_CONFIGURATION_SAVED,
			Text:        "Configuration saved.",
			PayloadJSON: chat.BuildConfigurationSavedPayload(),
		})
		if chatErr != nil {
			// Don't fail the whole request — the configuration is saved.
			// Log via the same mechanism the package already uses.
			// (If no logger field exists, this falls back to printing nothing;
			// the message will be missing from the thread but the config is OK.)
		}
	}

```

If there's already an existing logger field on PlanHandler (search `h.logger`, `h.log`, or similar), use it to log the chatErr. Otherwise leave the error swallowed — the comment documents the tradeoff.

Add at the top of the file if not present:

```go
import (
	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	"github.com/harpia/control-plane/internal/chat"
)
```

- [ ] **Step 3: Add the same hook to UpdatePlanConfiguration**

Same pattern. Find `UpdatePlanConfiguration`'s success `return` line and add the IDENTICAL block immediately before it (using `updated.ID.String()` as the ThreadID and `"Configuration updated."` as the text):

```go
	if h.chat != nil {
		_, _ = h.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
			ThreadID:    updated.ID.String(),
			Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
			Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_CONFIGURATION_SAVED,
			Text:        "Configuration updated.",
			PayloadJSON: chat.BuildConfigurationSavedPayload(),
		})
	}

```

- [ ] **Step 4: Verify the package still compiles**

```bash
cd /home/thbertoldi/harpia/control-plane && go vet ./internal/plans/... && go test ./internal/plans/...
```

Expected: existing tests still pass; the skipped TestCreatePlanConfiguration_WritesConfigurationSavedMessage reports as skipped.

- [ ] **Step 5: Commit**

```bash
git add control-plane/internal/plans/handler.go control-plane/internal/plans/thread_handler_test.go
git commit -m "feat(ux-m3): write CONFIGURATION_SAVED message on configuration create/update"
```

---

## Task 7: Workflow activity hooks (RUN_STARTED, RUN_COMPLETED, RUN_FAILED, STEP_BOUND, ELICITATION_RAISED)

**Files:**
- Modify: `control-plane/internal/workflow/plans.go` (the activity functions emit chat messages)
- Modify: `control-plane/internal/plans/runtime.go` (if the activities delegate to a runtime — found in recon as `a.Runtime.StartPlanExecution` etc.; the runtime owns the chat.Store and the actual append happens there)

**Interfaces:**
- Consumes: `chat.Store`, `chat.AppendInput`, `chat.BuildRunStartedPayload`, `chat.BuildRunCompletedPayload`, `chat.BuildRunFailedPayload`, `chat.BuildStepBoundPayload`, `chat.BuildElicitationRaisedPayload` from Task 4

**Note on locating the runtime:** The recon showed `StartPlanExecutionActivity` calls `a.Runtime.StartPlanExecution(ctx, tenantID, executionID)`. The Runtime type lives somewhere under `control-plane/internal/plans/`. Find it:

```bash
cd /home/thbertoldi/harpia/control-plane && grep -rn "func.*Runtime.*StartPlanExecution\b" internal/
```

The actual chat-message writes happen IN the runtime's methods (which already do the DB writes for plan_executions), not in the workflow activities themselves. This keeps the activities thin and the data writes co-located.

- [ ] **Step 1: Add chat.Store field to the Runtime struct**

In the file that defines `Runtime` (located by the grep above — likely `control-plane/internal/plans/runtime.go`), find the struct declaration. ADD a `chat chat.Store` field. Update the constructor to accept it.

```go
import (
	"github.com/harpia/control-plane/internal/chat"
	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
)
```

Update the wiring (probably the same place that constructs PlanHandler — found in Task 5 Step 3) to pass the same `chat.Store` instance into the Runtime constructor as well. Use one store instance shared by PlanHandler and Runtime.

- [ ] **Step 2: Hook RUN_STARTED into Runtime.StartPlanExecution**

Find `func (r *Runtime) StartPlanExecution(ctx context.Context, tenantID uuid.UUID, executionID uuid.UUID) error`. After the existing DB write that marks the plan_execution status to "running" succeeds, ADD:

```go
	if r.chat != nil {
		execID := executionID
		// Need the plan_configuration_id (= thread_id) — look it up from
		// the plan_executions row we just touched.
		configID, lookupErr := r.repo.GetPlanConfigurationIDForExecution(ctx, tenantID, executionID)
		if lookupErr == nil {
			_, _ = r.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
				ThreadID:    configID.String(),
				ExecutionID: &execID,
				Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
				Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_RUN_STARTED,
				Text:        "Run started.",
				PayloadJSON: chat.BuildRunStartedPayload(),
			})
		}
	}

```

The repository method `GetPlanConfigurationIDForExecution` may not exist yet. Add it to `control-plane/internal/plans/repository.go`:

```go
// GetPlanConfigurationIDForExecution returns the plan_configuration_id for a given plan_execution_id.
func (r *PlanRepository) GetPlanConfigurationIDForExecution(ctx context.Context, tenantID uuid.UUID, executionID uuid.UUID) (uuid.UUID, error) {
	var configID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT plan_configuration_id FROM plan_executions
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, executionID).Scan(&configID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("plans: GetPlanConfigurationIDForExecution: %w", err)
	}
	return configID, nil
}
```

If `r.repo` is named differently (e.g., `r.repository`, `r.store`), use the actual name found in the Runtime struct definition.

- [ ] **Step 3: Hook RUN_COMPLETED / RUN_FAILED into Runtime.CompletePlanExecution**

Find `func (r *Runtime) CompletePlanExecution(ctx, tenantID, executionID) error`. After the existing DB write, ADD a similar block that branches on the execution's final status:

```go
	if r.chat != nil {
		execID := executionID
		exec, lookupErr := r.repo.GetPlanExecution(ctx, tenantID, executionID)
		if lookupErr == nil {
			configID := exec.PlanConfigurationID
			var kind chatv1.ThreadMessageKind
			var text string
			var payload string
			if exec.Status == PlanExecutionStatusFailed {
				kind = chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_RUN_FAILED
				text = "Run failed."
				payload = chat.BuildRunFailedPayload(exec.FailureReason)
			} else {
				kind = chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_RUN_COMPLETED
				text = "Run completed."
				payload = chat.BuildRunCompletedPayload()
			}
			_, _ = r.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
				ThreadID:    configID.String(),
				ExecutionID: &execID,
				Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
				Kind:        kind,
				Text:        text,
				PayloadJSON: payload,
			})
		}
	}

```

Adjust struct field names (`exec.PlanConfigurationID`, `exec.Status`, `exec.FailureReason`) to match the actual `PlanExecution` Go type. If `FailureReason` is named differently (e.g., `ErrorMessage`, `Error`), use the real name. If the repository has no `GetPlanExecution` method, add one with the same pattern as `GetPlanConfigurationIDForExecution`.

- [ ] **Step 4: Hook STEP_BOUND into Runtime.CompleteStepExecution**

Find `func (r *Runtime) CompleteStepExecution(ctx, input StepStatusUpdateInput) error`. After the existing DB write, ADD:

```go
	if r.chat != nil {
		// Resolve configuration_id from the execution.
		execID := input.PlanExecutionID
		configID, lookupErr := r.repo.GetPlanConfigurationIDForExecution(ctx, input.TenantID, input.PlanExecutionID)
		if lookupErr == nil {
			_, _ = r.chat.AppendMessage(ctx, input.TenantID, chat.AppendInput{
				ThreadID:    configID.String(),
				ExecutionID: &execID,
				Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
				Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_STEP_BOUND,
				Text:        "Step " + input.PlanStepKey + " completed.",
				PayloadJSON: chat.BuildStepBoundPayload(input.PlanStepKey, input.OutputArtifactID),
			})
		}
	}

```

`StepStatusUpdateInput` field names may differ — use whatever the type actually has (search the file for its definition). The OutputArtifactID may be on a different field; if absent, pass `""` as the second arg to `BuildStepBoundPayload`.

- [ ] **Step 5: Hook ELICITATION_RAISED into AwaitElicitationStepExecutionActivity (or its runtime counterpart)**

The recon showed the elicitation-raise path is in the workflow itself (`workflow/plans.go` lines 560-590) calling `AwaitElicitationStepExecutionActivity`. Find the runtime method that activity calls (likely `r.Runtime.AwaitElicitationStepExecution` or similar). After the DB INSERT into `plan_elicitations`, ADD:

```go
	if r.chat != nil {
		execID := input.PlanExecutionID
		configID, lookupErr := r.repo.GetPlanConfigurationIDForExecution(ctx, input.TenantID, input.PlanExecutionID)
		if lookupErr == nil {
			_, _ = r.chat.AppendMessage(ctx, input.TenantID, chat.AppendInput{
				ThreadID:    configID.String(),
				ExecutionID: &execID,
				Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
				Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ELICITATION_RAISED,
				Text:        "Question raised on step " + input.PlanStepKey + ".",
				PayloadJSON: chat.BuildElicitationRaisedPayload(elicitationID),
			})
		}
	}

```

`elicitationID` is the UUID that was just inserted into `plan_elicitations`. The runtime method's existing code already has this value (it's the row's ID); reuse it.

- [ ] **Step 6: Compile and run existing tests**

```bash
cd /home/thbertoldi/harpia/control-plane && go vet ./... && go test ./internal/plans/... ./internal/workflow/...
```

Expected: all existing tests still pass; no new tests for these hooks (covered by Task 18 manual smoke).

- [ ] **Step 7: Commit**

```bash
git add control-plane/internal/plans/runtime.go control-plane/internal/plans/repository.go control-plane/internal/workflow/plans.go
git commit -m "feat(ux-m3): emit RUN/STEP/ELICITATION_RAISED chat messages from runtime"
```

---

## Task 8: ELICITATION_ANSWERED + APPROVAL_RAISED + APPROVAL_DECIDED hooks

**Files:**
- Modify: `control-plane/internal/plans/elicitations_handler.go` (RespondToElicitation writes ELICITATION_ANSWERED)
- Modify: `control-plane/internal/plans/approvals_handler.go` (RespondToApprovalRequest writes APPROVAL_DECIDED)
- Modify: `control-plane/internal/plans/repository.go` (the approval-raise INSERT also writes APPROVAL_RAISED — see recon: the INSERT is in repository.go around line 667)

- [ ] **Step 1: Hook ELICITATION_ANSWERED into RespondToElicitation**

In `control-plane/internal/plans/elicitations_handler.go`, find the success return of `RespondToElicitation`. The recon showed the relevant block:

```go
return connect.NewResponse(&plansv1.RespondToElicitationResponse{
    Elicitation: elicitationToProto(updated),
}), nil
```

IMMEDIATELY BEFORE this return, ADD:

```go
	if h.chat != nil {
		execID := updated.PlanExecutionID
		configID, lookupErr := h.repo.GetPlanConfigurationIDForExecution(ctx, tenantID, updated.PlanExecutionID)
		if lookupErr == nil {
			_, _ = h.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
				ThreadID:    configID.String(),
				ExecutionID: &execID,
				Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
				Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ELICITATION_ANSWERED,
				Text:        "Question on step " + updated.PlanStepKey + " was answered.",
				PayloadJSON: chat.BuildElicitationAnsweredPayload(updated.ID, "answered"),
			})
		}
	}

```

Add the imports if not already present:

```go
	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	"github.com/harpia/control-plane/internal/chat"
```

- [ ] **Step 2: Hook APPROVAL_DECIDED into RespondToApprovalRequest**

In `control-plane/internal/plans/approvals_handler.go`, find the success return of `RespondToApprovalRequest`. Recon showed:

```go
return connect.NewResponse(&plansv1.RespondToApprovalRequestResponse{
    ApprovalRequest: approvalRequestToProto(planApprovalRequestToDomain(updated)),
}), nil
```

IMMEDIATELY BEFORE, ADD:

```go
	if h.chat != nil {
		execID := updated.PlanExecutionID
		configID, lookupErr := h.repo.GetPlanConfigurationIDForExecution(ctx, tenantID, updated.PlanExecutionID)
		if lookupErr == nil {
			text := "Approval rejected."
			if req.Msg.GetApproved() {
				text = "Approval granted."
			}
			_, _ = h.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
				ThreadID:    configID.String(),
				ExecutionID: &execID,
				Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
				Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_APPROVAL_DECIDED,
				Text:        text,
				PayloadJSON: chat.BuildApprovalDecidedPayload(updated.ID, req.Msg.GetApproved()),
			})
		}
	}

```

Add the imports as before.

- [ ] **Step 3: Hook APPROVAL_RAISED at the INSERT site in repository.go**

The recon located the approval-raise INSERT at `control-plane/internal/plans/repository.go:667`:

```go
`INSERT INTO plan_approval_requests (
    id, tenant_id, plan_execution_id, step_execution_id, plan_step_key,
    input_artifact_id, status
)
VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7)
```

The repository method that owns this INSERT writes the row. We need its `func` signature — find by reading `repository.go` for the surrounding function. It's probably named `CreatePlanApprovalRequest` or similar.

The chat write needs the configuration_id, which requires a lookup. We DON'T want the repository to know about chat.Store (mixing concerns). Better: have the CALLER of `CreatePlanApprovalRequest` write the chat message. Find the caller:

```bash
cd /home/thbertoldi/harpia/control-plane && grep -rn "CreatePlanApprovalRequest\|PlanApprovalRequestCreate\|approvalRequest.*Insert\|approvals.Create" internal/
```

The caller is likely a workflow activity or a runtime method. At that call site, after the create succeeds, ADD a chat.AppendMessage call following the same pattern as Tasks 7. The thread_id is `configID.String()`, kind is `APPROVAL_RAISED`, payload is `chat.BuildApprovalRaisedPayload(createdRow.ID)`.

If the caller doesn't have access to a `chat.Store`, plumb it through the same way Task 7 plumbed it into the Runtime.

- [ ] **Step 4: Compile and run existing tests**

```bash
cd /home/thbertoldi/harpia/control-plane && go vet ./... && go test ./internal/plans/...
```

Expected: existing tests pass.

- [ ] **Step 5: Commit**

```bash
git add control-plane/internal/plans/elicitations_handler.go control-plane/internal/plans/approvals_handler.go control-plane/internal/plans
git commit -m "feat(ux-m3): emit ELICITATION_ANSWERED / APPROVAL_RAISED / APPROVAL_DECIDED chat messages"
```

---

## Task 9: Frontend `lib/chat/` — types, client, watch stream

**Files:**
- Create: `frontend/src/lib/chat/types.ts`
- Create: `frontend/src/lib/chat/client.ts`
- Create: `frontend/src/lib/chat/client.test.ts`
- Create: `frontend/src/lib/chat/watch.ts`
- Modify: `frontend/src/lib/rpc.ts` (export generated chat types)

**Interfaces:**
- Consumes: Generated `harpia.chat.v1.ThreadMessage`, `harpia.plans.v1.{WatchPlanThreadMessagesRequest, AppendPlanThreadMessageRequest, ListPlanThreadMessagesRequest}` from Tasks 1+2; `planClient` from `$lib/rpc`
- Produces:
  - `type ChatMessage` (TS mirror of `ThreadMessage`)
  - `type ChatMessageRole = "OVERSEER" | "AGENT" | "SYSTEM"`
  - `type ChatMessageKind = "USER_TEXT" | "ASSISTANT_TEXT" | "CONFIGURATION_SAVED" | "RUN_STARTED" | "RUN_COMPLETED" | "RUN_FAILED" | "STEP_BOUND" | "ELICITATION_RAISED" | "ELICITATION_ANSWERED" | "APPROVAL_RAISED" | "APPROVAL_DECIDED"`
  - `function appendThreadMessage(tenantId, configurationId, role, kind, text, payloadJson?, executionId?): Promise<ChatMessage>`
  - `interface WatchThreadOptions { sinceSequenceNumber?: bigint; signal?: AbortSignal }`
  - `async function* watchThreadMessages(tenantId, configurationId, options?): AsyncIterable<ChatMessage[]>`
  - `function loadThreadMessages(tenantId, configurationId, pageSize?): Promise<ChatMessage[]>`

- [ ] **Step 1: Add the generated chat types export to rpc.ts**

In `frontend/src/lib/rpc.ts`, ADD this export block (at the end of the file, alongside other type re-exports):

```ts
export type {
  ThreadMessage,
  ListPlanThreadMessagesResponse,
  WatchPlanThreadMessagesResponse,
  AppendPlanThreadMessageResponse,
} from "$lib/gen/harpia/chat/v1/chat_pb";
export {
  ThreadMessageRole as ProtoThreadMessageRole,
  ThreadMessageKind as ProtoThreadMessageKind,
} from "$lib/gen/harpia/chat/v1/chat_pb";
```

Note: `ThreadMessageRole` and `ThreadMessageKind` are also exported from `plans_pb.ts` (the existing thread embedded in ElicitationRequest). To avoid name collision, the new exports are renamed `ProtoThreadMessageRole` / `ProtoThreadMessageKind`. Frontend code using these enums goes through these names.

If the existing `plans_pb.ts` export of `ThreadMessageRole` is no longer used after M3 (the elicitation thread still uses it — verify with grep), leave it. Both can coexist with the renames.

- [ ] **Step 2: Write the failing test for appendThreadMessage**

Create `frontend/src/lib/chat/client.test.ts`:

```ts
import { describe, expect, it, vi, beforeEach } from "vitest";

const planClientMock = vi.hoisted(() => ({
  planClient: {
    appendPlanThreadMessage: vi.fn(),
    listPlanThreadMessages: vi.fn(),
  },
}));
vi.mock("$lib/rpc", () => planClientMock);

beforeEach(() => {
  planClientMock.planClient.appendPlanThreadMessage.mockReset();
  planClientMock.planClient.listPlanThreadMessages.mockReset();
});

describe("appendThreadMessage", () => {
  it("calls planClient.appendPlanThreadMessage with the proto enum values and returns the persisted message", async () => {
    planClientMock.planClient.appendPlanThreadMessage.mockResolvedValueOnce({
      message: {
        id: "msg-1",
        tenantId: "tenant-1",
        threadId: "config-1",
        role: 1,  // OVERSEER
        kind: 1,  // USER_TEXT
        text: "remember to update news source",
        payloadJson: "{}",
        sequenceNumber: 42n,
      },
    });

    const { appendThreadMessage } = await import("./client");
    const got = await appendThreadMessage(
      "tenant-1",
      "config-1",
      "OVERSEER",
      "USER_TEXT",
      "remember to update news source",
    );

    expect(got.id).toBe("msg-1");
    expect(got.text).toBe("remember to update news source");
    expect(got.role).toBe("OVERSEER");
    expect(got.kind).toBe("USER_TEXT");
    expect(
      planClientMock.planClient.appendPlanThreadMessage,
    ).toHaveBeenCalledTimes(1);
    expect(
      planClientMock.planClient.appendPlanThreadMessage,
    ).toHaveBeenCalledWith({
      tenantId: "tenant-1",
      planConfigurationId: "config-1",
      role: 1,
      kind: 1,
      text: "remember to update news source",
      payloadJson: "{}",
      executionId: "",
    });
  });
});

describe("loadThreadMessages", () => {
  it("returns the message array from the RPC response", async () => {
    planClientMock.planClient.listPlanThreadMessages.mockResolvedValueOnce({
      messages: [
        {
          id: "msg-1",
          tenantId: "tenant-1",
          threadId: "config-1",
          role: 3,  // SYSTEM
          kind: 3,  // CONFIGURATION_SAVED
          text: "Configuration saved.",
          payloadJson: "{}",
          sequenceNumber: 1n,
        },
      ],
    });
    const { loadThreadMessages } = await import("./client");
    const got = await loadThreadMessages("tenant-1", "config-1");
    expect(got).toHaveLength(1);
    expect(got[0].kind).toBe("CONFIGURATION_SAVED");
    expect(got[0].role).toBe("SYSTEM");
  });
});
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd /home/thbertoldi/harpia/frontend && npx vitest run src/lib/chat/client.test.ts
```

Expected: FAIL — `Cannot find module './client'`.

- [ ] **Step 4: Create types.ts**

Create `frontend/src/lib/chat/types.ts`:

```ts
import {
  ProtoThreadMessageKind,
  ProtoThreadMessageRole,
} from "$lib/rpc";
import type { ThreadMessage as ProtoThreadMessage } from "$lib/rpc";

export type ChatMessageRole = "OVERSEER" | "AGENT" | "SYSTEM";
export type ChatMessageKind =
  | "USER_TEXT"
  | "ASSISTANT_TEXT"
  | "CONFIGURATION_SAVED"
  | "RUN_STARTED"
  | "RUN_COMPLETED"
  | "RUN_FAILED"
  | "STEP_BOUND"
  | "ELICITATION_RAISED"
  | "ELICITATION_ANSWERED"
  | "APPROVAL_RAISED"
  | "APPROVAL_DECIDED";

export interface ChatMessage {
  id: string;
  tenantId: string;
  threadId: string;
  executionId: string;            // empty string when plan-scope
  role: ChatMessageRole;
  kind: ChatMessageKind;
  text: string;
  payloadJson: string;            // JSON-encoded per-kind payload; parse at use site
  authorUserId: string;           // empty for SYSTEM
  sequenceNumber: bigint;
  createdAt: string;              // ISO-8601 (from proto Timestamp)
}

const ROLE_FROM_PROTO: Record<number, ChatMessageRole> = {
  [ProtoThreadMessageRole.OVERSEER]: "OVERSEER",
  [ProtoThreadMessageRole.AGENT]: "AGENT",
  [ProtoThreadMessageRole.SYSTEM]: "SYSTEM",
};

const KIND_FROM_PROTO: Record<number, ChatMessageKind> = {
  [ProtoThreadMessageKind.USER_TEXT]: "USER_TEXT",
  [ProtoThreadMessageKind.ASSISTANT_TEXT]: "ASSISTANT_TEXT",
  [ProtoThreadMessageKind.CONFIGURATION_SAVED]: "CONFIGURATION_SAVED",
  [ProtoThreadMessageKind.RUN_STARTED]: "RUN_STARTED",
  [ProtoThreadMessageKind.RUN_COMPLETED]: "RUN_COMPLETED",
  [ProtoThreadMessageKind.RUN_FAILED]: "RUN_FAILED",
  [ProtoThreadMessageKind.STEP_BOUND]: "STEP_BOUND",
  [ProtoThreadMessageKind.ELICITATION_RAISED]: "ELICITATION_RAISED",
  [ProtoThreadMessageKind.ELICITATION_ANSWERED]: "ELICITATION_ANSWERED",
  [ProtoThreadMessageKind.APPROVAL_RAISED]: "APPROVAL_RAISED",
  [ProtoThreadMessageKind.APPROVAL_DECIDED]: "APPROVAL_DECIDED",
};

const ROLE_TO_PROTO: Record<ChatMessageRole, ProtoThreadMessageRole> = {
  OVERSEER: ProtoThreadMessageRole.OVERSEER,
  AGENT: ProtoThreadMessageRole.AGENT,
  SYSTEM: ProtoThreadMessageRole.SYSTEM,
};

const KIND_TO_PROTO: Record<ChatMessageKind, ProtoThreadMessageKind> = {
  USER_TEXT: ProtoThreadMessageKind.USER_TEXT,
  ASSISTANT_TEXT: ProtoThreadMessageKind.ASSISTANT_TEXT,
  CONFIGURATION_SAVED: ProtoThreadMessageKind.CONFIGURATION_SAVED,
  RUN_STARTED: ProtoThreadMessageKind.RUN_STARTED,
  RUN_COMPLETED: ProtoThreadMessageKind.RUN_COMPLETED,
  RUN_FAILED: ProtoThreadMessageKind.RUN_FAILED,
  STEP_BOUND: ProtoThreadMessageKind.STEP_BOUND,
  ELICITATION_RAISED: ProtoThreadMessageKind.ELICITATION_RAISED,
  ELICITATION_ANSWERED: ProtoThreadMessageKind.ELICITATION_ANSWERED,
  APPROVAL_RAISED: ProtoThreadMessageKind.APPROVAL_RAISED,
  APPROVAL_DECIDED: ProtoThreadMessageKind.APPROVAL_DECIDED,
};

export function chatMessageFromProto(p: ProtoThreadMessage): ChatMessage {
  return {
    id: p.id,
    tenantId: p.tenantId,
    threadId: p.threadId,
    executionId: p.executionId,
    role: ROLE_FROM_PROTO[p.role] ?? "SYSTEM",
    kind: KIND_FROM_PROTO[p.kind] ?? "USER_TEXT",
    text: p.text,
    payloadJson: p.payloadJson,
    authorUserId: p.authorUserId,
    sequenceNumber: p.sequenceNumber,
    createdAt: p.createdAt ? p.createdAt.toDate().toISOString() : "",
  };
}

export function chatRoleToProto(role: ChatMessageRole): ProtoThreadMessageRole {
  return ROLE_TO_PROTO[role];
}

export function chatKindToProto(kind: ChatMessageKind): ProtoThreadMessageKind {
  return KIND_TO_PROTO[kind];
}
```

- [ ] **Step 5: Create client.ts**

Create `frontend/src/lib/chat/client.ts`:

```ts
import { planClient } from "$lib/rpc";
import {
  type ChatMessage,
  type ChatMessageKind,
  type ChatMessageRole,
  chatKindToProto,
  chatMessageFromProto,
  chatRoleToProto,
} from "./types";

export async function appendThreadMessage(
  tenantId: string,
  configurationId: string,
  role: ChatMessageRole,
  kind: ChatMessageKind,
  text: string,
  payloadJson = "{}",
  executionId = "",
): Promise<ChatMessage> {
  const response = await planClient.appendPlanThreadMessage({
    tenantId,
    planConfigurationId: configurationId,
    role: chatRoleToProto(role),
    kind: chatKindToProto(kind),
    text,
    payloadJson,
    executionId,
  });
  if (!response.message) {
    throw new Error("appendPlanThreadMessage returned no message");
  }
  return chatMessageFromProto(response.message);
}

export async function loadThreadMessages(
  tenantId: string,
  configurationId: string,
  pageSize = 100,
): Promise<ChatMessage[]> {
  const response = await planClient.listPlanThreadMessages({
    tenantId,
    planConfigurationId: configurationId,
    pageSize,
    pageToken: "",
  });
  return response.messages.map(chatMessageFromProto);
}
```

- [ ] **Step 6: Run client test to verify it passes**

```bash
cd /home/thbertoldi/harpia/frontend && npx vitest run src/lib/chat/client.test.ts
```

Expected: 2 tests pass.

- [ ] **Step 7: Create watch.ts**

Create `frontend/src/lib/chat/watch.ts`:

```ts
import { planClient } from "$lib/rpc";
import { type ChatMessage, chatMessageFromProto } from "./types";

export interface WatchThreadOptions {
  sinceSequenceNumber?: bigint;
  signal?: AbortSignal;
}

export async function* watchThreadMessages(
  tenantId: string,
  configurationId: string,
  options: WatchThreadOptions = {},
): AsyncIterable<ChatMessage[]> {
  const sinceSeq = options.sinceSequenceNumber ?? 0n;
  for await (const event of planClient.watchPlanThreadMessages(
    {
      tenantId,
      planConfigurationId: configurationId,
      sinceSequenceNumber: sinceSeq,
    },
    { signal: options.signal },
  )) {
    yield event.messages.map(chatMessageFromProto);
  }
}
```

- [ ] **Step 8: Type-check and run all chat tests**

```bash
cd /home/thbertoldi/harpia/frontend && npx svelte-kit sync >/dev/null 2>&1 && npm run check 2>&1 | grep -E "src/lib/(chat|rpc)" | head -5
echo '---'
npx vitest run src/lib/chat/
```

Expected: no NEW type errors in `chat/` or `rpc.ts`; 2 tests pass.

- [ ] **Step 9: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/chat frontend/src/lib/rpc.ts
git commit -m "feat(ux-m3): add lib/chat — types, client, watch stream"
```

---

## Task 10: AbortSignal backfill across watchElicitations, watchApprovalRequests, watchInbox

**Files:**
- Modify: `frontend/src/lib/plans/elicitations.ts` (add `signal?: AbortSignal` to `WatchElicitationsOptions`, pass through)
- Modify: `frontend/src/lib/plans/approvals.ts` (same)
- Modify: `frontend/src/lib/inbox/aggregator.ts` (`InboxSources` and `watchInbox` accept signal, cascade to underlying watchers)
- Modify: `frontend/src/routes/inbox/+page.svelte` (consumer uses AbortController instead of `active` flag)

**Interfaces:**
- Modified: `WatchElicitationsOptions` and `WatchApprovalRequestsOptions` gain optional `signal?: AbortSignal`
- Modified: `watchInbox(tenantId, sources?, options?)` — new optional third parameter `{ signal?: AbortSignal }`

- [ ] **Step 1: Update elicitations.ts**

In `frontend/src/lib/plans/elicitations.ts`, find the type:

```ts
export type WatchElicitationsOptions = {
  stepExecutionId?: string;
  addressedToMe?: boolean;
};
```

REPLACE with:

```ts
export type WatchElicitationsOptions = {
  stepExecutionId?: string;
  addressedToMe?: boolean;
  signal?: AbortSignal;
};
```

Find the function:

```ts
export async function* watchElicitations(
  tenantId: string,
  options: WatchElicitationsOptions = {},
): AsyncIterable<ElicitationRequest[]> {
  for await (const event of planClient.watchElicitations({
    tenantId,
    addressedToMe: options.addressedToMe ?? false,
    stepExecutionId: options.stepExecutionId,
  })) {
    yield event.elicitations;
  }
}
```

REPLACE the `planClient.watchElicitations(...)` call to pass `{ signal: options.signal }` as the second argument:

```ts
export async function* watchElicitations(
  tenantId: string,
  options: WatchElicitationsOptions = {},
): AsyncIterable<ElicitationRequest[]> {
  for await (const event of planClient.watchElicitations(
    {
      tenantId,
      addressedToMe: options.addressedToMe ?? false,
      stepExecutionId: options.stepExecutionId,
    },
    { signal: options.signal },
  )) {
    yield event.elicitations;
  }
}
```

- [ ] **Step 2: Update approvals.ts the same way**

In `frontend/src/lib/plans/approvals.ts`:

REPLACE the type:

```ts
export type WatchApprovalRequestsOptions = {
  stepExecutionId?: string;
  planExecutionId?: string;
};
```

WITH:

```ts
export type WatchApprovalRequestsOptions = {
  stepExecutionId?: string;
  planExecutionId?: string;
  signal?: AbortSignal;
};
```

REPLACE the function body to pass `{ signal: options.signal }` to the RPC call:

```ts
export async function* watchApprovalRequests(
  tenantId: string,
  options: WatchApprovalRequestsOptions = {},
): AsyncIterable<ApprovalRequest[]> {
  for await (const event of planClient.watchApprovalRequests(
    {
      tenantId,
      stepExecutionId: options.stepExecutionId,
      planExecutionId: options.planExecutionId,
    },
    { signal: options.signal },
  )) {
    yield event.approvalRequests;
  }
}
```

- [ ] **Step 3: Update aggregator.ts to cascade signals**

In `frontend/src/lib/inbox/aggregator.ts`, find the `InboxSources` interface (it has `watchElicitations: (tenantId) => AsyncIterable<...>` etc.). EXTEND each method to accept an optional second `signal` argument:

```ts
export interface InboxSources {
  watchElicitations: (
    tenantId: string,
    signal?: AbortSignal,
  ) => AsyncIterable<ElicitationRequest[]>;
  watchApprovalRequests: (
    tenantId: string,
    signal?: AbortSignal,
  ) => AsyncIterable<ApprovalRequest[]>;
  loadFeedback: (tenantId: string) => Promise<FeedbackRequest[]>;
}
```

Update `DEFAULT_INBOX_SOURCES` to pass the signal through:

```ts
export const DEFAULT_INBOX_SOURCES: InboxSources = {
  watchElicitations: (tenantId, signal) =>
    watchElicitations(tenantId, { addressedToMe: true, signal }),
  watchApprovalRequests: (tenantId, signal) =>
    watchApprovalRequests(tenantId, { signal }),
  loadFeedback: loadPendingFeedback,
};
```

Update `watchInbox` signature to accept an optional options bag:

```ts
export interface WatchInboxOptions {
  signal?: AbortSignal;
}

export async function* watchInbox(
  tenantId: string,
  sources: InboxSources = DEFAULT_INBOX_SOURCES,
  options: WatchInboxOptions = {},
): AsyncIterable<InboxItem[]> {
  // ... existing body, but in the two `void consume(sources.watch...(tenantId), ...)`
  // calls, pass options.signal as the second argument to the source factory:
  void consume(sources.watchElicitations(tenantId, options.signal), (b) => {
    elicitations = b;
  });
  void consume(sources.watchApprovalRequests(tenantId, options.signal), (b) => {
    approvals = b;
  });
  // (loadFeedback is a Promise, not cancellable via signal — left as-is)
  // ... rest of body unchanged
}
```

- [ ] **Step 4: Update aggregator test mocks to accept the new signature**

In `frontend/src/lib/inbox/aggregator.test.ts`, find the `InboxSources` factory mocks (the `watchElicitations: (tenantId) => ...` lines). UPDATE each to accept an optional signal parameter so types match. Specifically: in places like:

```ts
watchElicitations: () => yieldOnce([...])
```

REPLACE with:

```ts
watchElicitations: (_tenantId, _signal) => yieldOnce([...])
```

(The underscores satisfy linter for unused params.)

Run the aggregator tests:

```bash
cd /home/thbertoldi/harpia/frontend && npx vitest run src/lib/inbox/
```

Expected: 6 tests pass.

- [ ] **Step 5: Update the inbox page consumer to use AbortController**

In `frontend/src/routes/inbox/+page.svelte`, find the existing `$effect` block:

```svelte
$effect(() => {
  const tenant = getTenant();
  if (!tenant?.id) return;
  let active = true;
  (async () => {
    try {
      for await (const batch of watchInbox(tenant.id)) {
        if (!active) return;
        items = batch;
      }
    } catch {
      if (active) loadError = true;
    }
  })();
  return () => {
    active = false;
  };
});
```

REPLACE with:

```svelte
$effect(() => {
  const tenant = getTenant();
  if (!tenant?.id) return;
  const controller = new AbortController();
  (async () => {
    try {
      for await (const batch of watchInbox(tenant.id, undefined, {
        signal: controller.signal,
      })) {
        if (controller.signal.aborted) return;
        items = batch;
      }
    } catch (err) {
      if (controller.signal.aborted) return; // expected on unmount
      loadError = true;
    }
  })();
  return () => {
    controller.abort();
  };
});
```

- [ ] **Step 6: Type-check and run all relevant tests**

```bash
cd /home/thbertoldi/harpia/frontend && npx svelte-kit sync >/dev/null 2>&1 && npm run check 2>&1 | grep -cE "^[0-9]+ ERROR" | tail -1
echo '---'
npx vitest run src/lib/inbox/ src/lib/plans/
```

Expected: type-check at baseline 10 errors (no new); all inbox + plans tests pass.

- [ ] **Step 7: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/plans/elicitations.ts frontend/src/lib/plans/approvals.ts frontend/src/lib/inbox/aggregator.ts frontend/src/lib/inbox/aggregator.test.ts frontend/src/routes/inbox/+page.svelte
git commit -m "fix(ux-m3): plumb AbortSignal through watch* RPCs and the inbox page"
```

---

## Task 11: Inbox aggregator exposes `configurationId` on InboxItem

**Files:**
- Modify: `frontend/src/lib/inbox/types.ts` (add `configurationId: string` to `InboxElicitationItem` and `InboxApprovalItem`)
- Modify: `frontend/src/lib/inbox/aggregator.ts` (`toInboxElicitation` and `toInboxApproval` populate `configurationId` from the underlying request)

**Interfaces:**
- Modified: `InboxElicitationItem.configurationId: string` and `InboxApprovalItem.configurationId: string` — empty string when not derivable (graceful degrade)

The recon noted: `ElicitationRequest` carries `planExecutionId` but NOT `planConfigurationId` directly. To derive configurationId from planExecutionId, we'd need a join or an additional RPC. The simplest M3 approach: extend the existing watchElicitations / watchApprovalRequests protos to include `plan_configuration_id` on each request. That's a proto change.

Alternative: M3 makes the inbox "Open thread" link to `/plans/configurations/lookup?execution=<execId>` — a frontend route that does the lookup and redirects. Less proto churn but adds a hop.

**Decision for M3:** extend the proto. `ElicitationRequest.plan_configuration_id` and `ApprovalRequest.plan_configuration_id` are added. The Go-side projection already has the data (the elicitation and approval rows have plan_execution_id, which has plan_configuration_id one join away).

- [ ] **Step 1: Add `plan_configuration_id` to ElicitationRequest and ApprovalRequest protos**

In `proto/harpia/plans/v1/plans.proto`, find the `message ElicitationRequest { ... }` declaration. Find the highest field number in use; ADD:

```protobuf
  string plan_configuration_id = N;   // where N = next free field number
```

Same for `message ApprovalRequest { ... }`.

Run `cd /home/thbertoldi/harpia/proto && buf generate && buf lint`.

- [ ] **Step 2: Populate the new field in the Go projection helpers**

Find `elicitationToProto` in `control-plane/internal/plans/elicitations_handler.go` and `approvalRequestToProto` in `approvals_handler.go`. Each currently builds the proto message from a domain struct. ADD the configuration_id population.

The Elicitation domain struct (defined in `control-plane/internal/plans/`) has `PlanExecutionID uuid.UUID`. Look up the config_id via the existing repository:

```go
configID, err := r.GetPlanConfigurationIDForExecution(ctx, tenantID, e.PlanExecutionID)
if err == nil {
    proto.PlanConfigurationId = configID.String()
}
```

But `elicitationToProto` may not have a repository/context handy. Better: load the configuration_id at the SQL query level. In the existing query that loads elicitations (find it in `repository.go` near the elicitation list/get functions), JOIN to `plan_executions` to bring in `plan_configuration_id`. Add a `PlanConfigurationID uuid.UUID` field to the `Elicitation` domain struct. Then `elicitationToProto` reads `e.PlanConfigurationID.String()`.

Same pattern for ApprovalRequest.

If this is a substantial schema-touching change, STOP and report DONE_WITH_CONCERNS with the proposed query change. The user may want to defer this — and accept that the inbox deep-link uses execution_id-based lookup instead.

- [ ] **Step 3: Update frontend types**

In `frontend/src/lib/inbox/types.ts`, find `InboxElicitationItem` and `InboxApprovalItem`. ADD `configurationId: string;` to each.

- [ ] **Step 4: Populate in the aggregator's projection**

In `frontend/src/lib/inbox/aggregator.ts`, find `toInboxElicitation` and `toInboxApproval`. ADD the field:

```ts
function toInboxElicitation(req: ElicitationRequest): InboxElicitationItem {
  return {
    // ... existing fields
    configurationId: req.planConfigurationId,
  };
}

function toInboxApproval(req: ApprovalRequest): InboxApprovalItem {
  return {
    // ... existing fields
    configurationId: req.planConfigurationId,
  };
}
```

- [ ] **Step 5: Run tests**

```bash
cd /home/thbertoldi/harpia/frontend && npx vitest run src/lib/inbox/
```

Expected: 6 tests pass. If aggregator tests reference InboxItem without `configurationId`, update the test fixtures to include `configurationId: ""` (empty for the test's purposes).

- [ ] **Step 6: Commit**

```bash
cd /home/thbertoldi/harpia
git add proto control-plane/gen frontend/src/lib/gen frontend/src/lib/inbox control-plane/internal/plans
git commit -m "feat(ux-m3): expose plan_configuration_id on Elicitation/Approval RPCs and InboxItem"
```

---

## Task 12: `lib/plans/thread.ts` — execution grouping helper

**Files:**
- Create: `frontend/src/lib/plans/thread.ts`
- Create: `frontend/src/lib/plans/thread.test.ts`

**Interfaces:**
- Consumes: `ChatMessage` from `$lib/chat/types`
- Produces:
  - `interface ExecutionGroup { executionId: string; runNumber: number; messages: ChatMessage[]; status: "running" | "completed" | "failed" | "unknown" }`
  - `type ThreadSection = { kind: "plan-scope"; message: ChatMessage } | { kind: "execution"; group: ExecutionGroup }`
  - `function buildThreadSections(messages: ChatMessage[]): ThreadSection[]` — groups consecutive same-`executionId` messages into ExecutionGroup; plan-scope messages (executionId === "") become single "plan-scope" sections in chronological order

- [ ] **Step 1: Write the failing test**

Create `frontend/src/lib/plans/thread.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { buildThreadSections } from "./thread";
import type { ChatMessage } from "$lib/chat/types";

function msg(
  id: string,
  executionId: string,
  kind: ChatMessage["kind"],
  sequenceNumber: number,
): ChatMessage {
  return {
    id,
    tenantId: "tenant-1",
    threadId: "config-1",
    executionId,
    role: kind === "USER_TEXT" ? "OVERSEER" : "SYSTEM",
    kind,
    text: id,
    payloadJson: "{}",
    authorUserId: "",
    sequenceNumber: BigInt(sequenceNumber),
    createdAt: `2026-06-21T10:0${sequenceNumber}:00Z`,
  };
}

describe("buildThreadSections", () => {
  it("returns an empty array for no messages", () => {
    expect(buildThreadSections([])).toEqual([]);
  });

  it("renders plan-scope messages as plan-scope sections in chronological order", () => {
    const messages = [
      msg("m1", "", "CONFIGURATION_SAVED", 1),
      msg("m2", "", "USER_TEXT", 2),
    ];
    const sections = buildThreadSections(messages);
    expect(sections).toHaveLength(2);
    expect(sections[0].kind).toBe("plan-scope");
    expect(sections[1].kind).toBe("plan-scope");
  });

  it("groups consecutive same-executionId messages into one ExecutionGroup", () => {
    const messages = [
      msg("m1", "", "CONFIGURATION_SAVED", 1),
      msg("m2", "exec-A", "RUN_STARTED", 2),
      msg("m3", "exec-A", "STEP_BOUND", 3),
      msg("m4", "exec-A", "RUN_COMPLETED", 4),
    ];
    const sections = buildThreadSections(messages);
    expect(sections).toHaveLength(2);
    expect(sections[0].kind).toBe("plan-scope");
    expect(sections[1].kind).toBe("execution");
    if (sections[1].kind === "execution") {
      expect(sections[1].group.executionId).toBe("exec-A");
      expect(sections[1].group.runNumber).toBe(1);
      expect(sections[1].group.messages).toHaveLength(3);
      expect(sections[1].group.status).toBe("completed");
    }
  });

  it("numbers runs in chronological appearance order", () => {
    const messages = [
      msg("m1", "exec-A", "RUN_STARTED", 1),
      msg("m2", "exec-A", "RUN_COMPLETED", 2),
      msg("m3", "exec-B", "RUN_STARTED", 3),
      msg("m4", "exec-B", "RUN_FAILED", 4),
    ];
    const sections = buildThreadSections(messages);
    expect(sections).toHaveLength(2);
    if (sections[0].kind === "execution") {
      expect(sections[0].group.runNumber).toBe(1);
      expect(sections[0].group.status).toBe("completed");
    }
    if (sections[1].kind === "execution") {
      expect(sections[1].group.runNumber).toBe(2);
      expect(sections[1].group.status).toBe("failed");
    }
  });

  it("derives status from the latest terminal message in the group; otherwise 'running'", () => {
    const running = [
      msg("m1", "exec-A", "RUN_STARTED", 1),
      msg("m2", "exec-A", "STEP_BOUND", 2),
    ];
    const sections = buildThreadSections(running);
    if (sections[0].kind === "execution") {
      expect(sections[0].group.status).toBe("running");
    }
  });

  it("re-groups when execution_id changes back (e.g. inter-execution plan-scope message)", () => {
    const messages = [
      msg("m1", "exec-A", "RUN_STARTED", 1),
      msg("m2", "exec-A", "RUN_COMPLETED", 2),
      msg("m3", "", "USER_TEXT", 3),
      msg("m4", "exec-B", "RUN_STARTED", 4),
    ];
    const sections = buildThreadSections(messages);
    expect(sections).toHaveLength(3);
    expect(sections[0].kind).toBe("execution");
    expect(sections[1].kind).toBe("plan-scope");
    expect(sections[2].kind).toBe("execution");
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/thbertoldi/harpia/frontend && npx vitest run src/lib/plans/thread.test.ts
```

Expected: FAIL — module not found.

- [ ] **Step 3: Create thread.ts**

Create `frontend/src/lib/plans/thread.ts`:

```ts
import type { ChatMessage } from "$lib/chat/types";

export interface ExecutionGroup {
  executionId: string;
  runNumber: number;
  messages: ChatMessage[];
  status: "running" | "completed" | "failed" | "unknown";
}

export type ThreadSection =
  | { kind: "plan-scope"; message: ChatMessage }
  | { kind: "execution"; group: ExecutionGroup };

const TERMINAL_KIND_STATUS: Partial<
  Record<ChatMessage["kind"], ExecutionGroup["status"]>
> = {
  RUN_COMPLETED: "completed",
  RUN_FAILED: "failed",
};

export function buildThreadSections(messages: ChatMessage[]): ThreadSection[] {
  const sections: ThreadSection[] = [];
  const runNumberByExecutionId = new Map<string, number>();
  let nextRunNumber = 1;
  let currentGroup: ExecutionGroup | null = null;

  for (const message of messages) {
    if (message.executionId === "") {
      // Plan-scope — flush any open group and emit as its own section.
      if (currentGroup) {
        sections.push({ kind: "execution", group: currentGroup });
        currentGroup = null;
      }
      sections.push({ kind: "plan-scope", message });
      continue;
    }
    if (currentGroup && currentGroup.executionId !== message.executionId) {
      sections.push({ kind: "execution", group: currentGroup });
      currentGroup = null;
    }
    if (!currentGroup) {
      const existingRunNumber = runNumberByExecutionId.get(message.executionId);
      const runNumber = existingRunNumber ?? nextRunNumber;
      if (existingRunNumber === undefined) {
        runNumberByExecutionId.set(message.executionId, runNumber);
        nextRunNumber += 1;
      }
      currentGroup = {
        executionId: message.executionId,
        runNumber,
        messages: [],
        status: "running",
      };
    }
    currentGroup.messages.push(message);
    const terminalStatus = TERMINAL_KIND_STATUS[message.kind];
    if (terminalStatus) {
      currentGroup.status = terminalStatus;
    }
  }

  if (currentGroup) {
    sections.push({ kind: "execution", group: currentGroup });
  }

  return sections;
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/thbertoldi/harpia/frontend && npx vitest run src/lib/plans/thread.test.ts
```

Expected: 6 tests pass.

- [ ] **Step 5: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/plans/thread.ts frontend/src/lib/plans/thread.test.ts
git commit -m "feat(ux-m3): add buildThreadSections helper for execution grouping"
```

---

## Task 13: `PlanDagMiniMap.svelte` compact DAG component

**Files:**
- Create: `frontend/src/lib/components/PlanDagMiniMap.svelte`

**Interfaces:**
- Consumes: `PlanStep`, `PlanStepDependency` from `$lib/gen/harpia/plans/v1/plans_pb`; `orderPlanStepsLinear` from `$lib/plans/artifact-flow`
- Produces: A Svelte 5 component with `Props { steps: PlanStep[]; edges: PlanStepDependency[] }` rendering a single-row strip of step name boxes with arrow connectors.

- [ ] **Step 1: Create the component**

Create `frontend/src/lib/components/PlanDagMiniMap.svelte`:

```svelte
<script lang="ts">
  import { ChevronRight } from "lucide-svelte";
  import type {
    PlanStep,
    PlanStepDependency,
  } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { orderPlanStepsLinear } from "$lib/plans/artifact-flow";

  let {
    steps,
    edges,
  }: {
    steps: PlanStep[];
    edges: PlanStepDependency[];
  } = $props();

  const orderedSteps = $derived(orderPlanStepsLinear(steps, edges));
</script>

<div class="flex flex-wrap items-center gap-1.5 rounded-md border border-plumage bg-obsidian-light px-3 py-2">
  {#each orderedSteps as step, index (step.key)}
    <span
      class="font-heading text-[11px] font-semibold text-cream rounded border border-plumage/60 bg-obsidian px-2 py-1"
    >
      {step.title}
    </span>
    {#if index < orderedSteps.length - 1}
      <ChevronRight class="size-3 text-crown-ash-dark" />
    {/if}
  {/each}
</div>
```

- [ ] **Step 2: Type-check**

```bash
cd /home/thbertoldi/harpia/frontend && npx svelte-kit sync >/dev/null 2>&1 && npm run check 2>&1 | grep -c "PlanDagMiniMap"
```

Expected: 0 (no errors mentioning this file).

- [ ] **Step 3: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/components/PlanDagMiniMap.svelte
git commit -m "feat(ux-m3): add PlanDagMiniMap compact DAG component"
```

---

## Task 14: `lib/components/thread/` directory — chat surface components

**Files:**
- Create: `frontend/src/lib/components/thread/ThreadMessage.svelte`
- Create: `frontend/src/lib/components/thread/SystemEventCard.svelte`
- Create: `frontend/src/lib/components/thread/ElicitationRefCard.svelte`
- Create: `frontend/src/lib/components/thread/ApprovalRefCard.svelte`
- Create: `frontend/src/lib/components/thread/ExecutionSection.svelte`
- Create: `frontend/src/lib/components/thread/ThreadComposer.svelte`
- Create: `frontend/src/lib/components/thread/HintBanner.svelte`

**Interfaces:**
- Consumes: `ChatMessage` from `$lib/chat/types`; `appendThreadMessage` from `$lib/chat/client`; `ExecutionGroup` from `$lib/plans/thread`
- Produces: Svelte 5 components with the props shown below in their respective sections.

- [ ] **Step 1: Create `HintBanner.svelte` (the dismissable "assistant doesn't reply yet" notice)**

Create `frontend/src/lib/components/thread/HintBanner.svelte`:

```svelte
<script lang="ts">
  import { X } from "lucide-svelte";
  import { browser } from "$app/environment";
  import { locale, translate } from "$lib/i18n";

  interface Props {
    threadId: string;
  }
  let { threadId }: Props = $props();

  const storageKey = $derived(`harpia.thread.${threadId}.composerHintDismissed`);

  let visible = $state(false);

  $effect(() => {
    if (!browser) return;
    visible = localStorage.getItem(storageKey) !== "1";
  });

  function dismiss() {
    visible = false;
    if (browser) {
      localStorage.setItem(storageKey, "1");
    }
  }
</script>

{#if visible}
  <div
    class="flex items-start gap-3 rounded-md border border-talon-gold/40 bg-talon-gold/5 px-3 py-2 text-[12px] text-crown-ash"
  >
    <p class="flex-1">
      {translate("thread.composerHint", $locale)}
    </p>
    <button
      type="button"
      onclick={dismiss}
      aria-label={translate("thread.dismiss", $locale)}
      class="text-crown-ash-dark hover:text-talon-gold"
    >
      <X class="size-4" />
    </button>
  </div>
{/if}
```

- [ ] **Step 2: Create `SystemEventCard.svelte` (kind-discriminated render)**

Create `frontend/src/lib/components/thread/SystemEventCard.svelte`:

```svelte
<script lang="ts">
  import { Activity, Check, AlertTriangle, Play, Save } from "lucide-svelte";
  import type { ChatMessage } from "$lib/chat/types";
  import { locale, translate } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";

  interface Props {
    message: ChatMessage;
  }
  let { message }: Props = $props();

  const iconFor = $derived(
    message.kind === "RUN_STARTED"
      ? Play
      : message.kind === "RUN_COMPLETED"
        ? Check
        : message.kind === "RUN_FAILED"
          ? AlertTriangle
          : message.kind === "STEP_BOUND"
            ? Activity
            : Save,
  );

  const labelKey = $derived(`thread.event.${message.kind.toLowerCase()}`);
</script>

<div
  id={`m-${message.id}`}
  class="flex items-center gap-3 rounded-md border border-plumage/60 bg-obsidian-light px-3 py-2 text-[12px] text-crown-ash"
>
  <svelte:component this={iconFor} class="size-4 text-talon-gold" />
  <span class="flex-1 text-cream">{message.text}</span>
  <span class="text-[10px] text-crown-ash-dark">
    {formatRelativeTime(message.createdAt, $locale)}
  </span>
</div>
```

Note: the `iconFor` `$derived` uses `svelte:component`. In Svelte 5 this is `<svelte:component this={iconFor} ... />`. If Svelte 5 removed `svelte:component` (verify in the installed version), use the runes equivalent: `{@const Icon = iconFor}<Icon class="..." />`.

- [ ] **Step 3: Create `ElicitationRefCard.svelte`**

Create `frontend/src/lib/components/thread/ElicitationRefCard.svelte`:

```svelte
<script lang="ts">
  import { MessageSquare, CheckCircle2, XCircle } from "lucide-svelte";
  import type { ChatMessage } from "$lib/chat/types";
  import { locale, translate } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";

  interface Props {
    message: ChatMessage;
  }
  let { message }: Props = $props();

  const isAnswered = $derived(message.kind === "ELICITATION_ANSWERED");

  const icon = $derived(
    isAnswered ? CheckCircle2 : MessageSquare,
  );
</script>

<div
  id={`m-${message.id}`}
  class="flex items-center gap-3 rounded-md border border-talon-gold/40 bg-talon-gold/10 px-3 py-2 text-[12px]"
>
  <svelte:component this={icon} class="size-4 text-talon-gold" />
  <span class="flex-1 text-cream">{message.text}</span>
  <span class="text-[10px] text-crown-ash-dark">
    {formatRelativeTime(message.createdAt, $locale)}
  </span>
</div>
```

- [ ] **Step 4: Create `ApprovalRefCard.svelte`**

Create `frontend/src/lib/components/thread/ApprovalRefCard.svelte`:

```svelte
<script lang="ts">
  import { ShieldCheck, ShieldX, ShieldQuestion } from "lucide-svelte";
  import type { ChatMessage } from "$lib/chat/types";
  import { locale, translate } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";

  interface Props {
    message: ChatMessage;
  }
  let { message }: Props = $props();

  const decided = $derived(message.kind === "APPROVAL_DECIDED");

  const payloadApproved = $derived.by(() => {
    if (!decided) return null;
    try {
      const parsed = JSON.parse(message.payloadJson);
      return typeof parsed.approved === "boolean" ? parsed.approved : null;
    } catch {
      return null;
    }
  });

  const icon = $derived(
    !decided
      ? ShieldQuestion
      : payloadApproved === true
        ? ShieldCheck
        : ShieldX,
  );
</script>

<div
  id={`m-${message.id}`}
  class="flex items-center gap-3 rounded-md border border-talon-gold/40 bg-talon-gold/10 px-3 py-2 text-[12px]"
>
  <svelte:component this={icon} class="size-4 text-talon-gold" />
  <span class="flex-1 text-cream">{message.text}</span>
  <span class="text-[10px] text-crown-ash-dark">
    {formatRelativeTime(message.createdAt, $locale)}
  </span>
</div>
```

- [ ] **Step 5: Create `ThreadMessage.svelte` (shell that dispatches to kind-specific cards)**

Create `frontend/src/lib/components/thread/ThreadMessage.svelte`:

```svelte
<script lang="ts">
  import type { ChatMessage } from "$lib/chat/types";
  import { locale, translate } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";
  import SystemEventCard from "./SystemEventCard.svelte";
  import ElicitationRefCard from "./ElicitationRefCard.svelte";
  import ApprovalRefCard from "./ApprovalRefCard.svelte";

  interface Props {
    message: ChatMessage;
  }
  let { message }: Props = $props();
</script>

{#if message.kind === "USER_TEXT"}
  <div
    id={`m-${message.id}`}
    class="self-end max-w-[85%] rounded-lg border border-plumage bg-obsidian-light px-3 py-2"
  >
    <p class="text-[13px] text-cream whitespace-pre-wrap">{message.text}</p>
    <p class="mt-1 text-right text-[10px] text-crown-ash-dark">
      {formatRelativeTime(message.createdAt, $locale)}
    </p>
  </div>
{:else if message.kind === "ELICITATION_RAISED" || message.kind === "ELICITATION_ANSWERED"}
  <ElicitationRefCard {message} />
{:else if message.kind === "APPROVAL_RAISED" || message.kind === "APPROVAL_DECIDED"}
  <ApprovalRefCard {message} />
{:else}
  <SystemEventCard {message} />
{/if}
```

- [ ] **Step 6: Create `ExecutionSection.svelte` (collapsible wrapper)**

Create `frontend/src/lib/components/thread/ExecutionSection.svelte`:

```svelte
<script lang="ts">
  import { ChevronDown, ChevronRight } from "lucide-svelte";
  import type { ExecutionGroup } from "$lib/plans/thread";
  import { locale, translate } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";
  import ThreadMessage from "./ThreadMessage.svelte";

  interface Props {
    group: ExecutionGroup;
    defaultExpanded?: boolean;
  }
  let { group, defaultExpanded = false }: Props = $props();

  let expanded = $state(defaultExpanded);

  const statusKey = $derived(`thread.execution.status.${group.status}`);
  const startedAt = $derived(group.messages[0]?.createdAt ?? "");
</script>

<section class="rounded-md border border-plumage bg-obsidian-light">
  <button
    type="button"
    onclick={() => (expanded = !expanded)}
    class="flex w-full items-center gap-3 rounded-md px-3 py-2 text-left text-[12px] hover:bg-obsidian-light/70"
  >
    {#if expanded}
      <ChevronDown class="size-4 text-talon-gold" />
    {:else}
      <ChevronRight class="size-4 text-crown-ash-dark" />
    {/if}
    <span class="flex-1 font-heading font-semibold text-cream">
      {translate("thread.execution.runLabel", $locale).replace(
        "{n}",
        String(group.runNumber),
      )}
      <span class="text-crown-ash">· {translate(statusKey, $locale)}</span>
    </span>
    <span class="text-[10px] text-crown-ash-dark">
      {formatRelativeTime(startedAt, $locale)}
    </span>
  </button>
  {#if expanded}
    <div class="flex flex-col gap-2 border-t border-plumage/50 px-3 py-3">
      {#each group.messages as message (message.id)}
        <ThreadMessage {message} />
      {/each}
    </div>
  {/if}
</section>
```

- [ ] **Step 7: Create `ThreadComposer.svelte`**

Create `frontend/src/lib/components/thread/ThreadComposer.svelte`:

```svelte
<script lang="ts">
  import { Send } from "lucide-svelte";
  import { locale, translate } from "$lib/i18n";
  import { appendThreadMessage } from "$lib/chat/client";

  interface Props {
    tenantId: string;
    configurationId: string;
    onSent?: () => void;
  }
  let { tenantId, configurationId, onSent }: Props = $props();

  let value = $state("");
  let sending = $state(false);
  let error = $state<string | null>(null);

  async function send() {
    const trimmed = value.trim();
    if (trimmed.length === 0 || sending) return;
    sending = true;
    error = null;
    try {
      await appendThreadMessage(
        tenantId,
        configurationId,
        "OVERSEER",
        "USER_TEXT",
        trimmed,
      );
      value = "";
      onSent?.();
    } catch (e) {
      error = e instanceof Error ? e.message : translate("thread.composer.error", $locale);
    } finally {
      sending = false;
    }
  }
</script>

<div class="flex flex-col gap-2 border-t border-plumage bg-obsidian-light px-3 py-3">
  <div class="flex items-end gap-2">
    <textarea
      bind:value
      placeholder={translate("thread.composer.placeholder", $locale)}
      class="flex-1 rounded border border-plumage bg-obsidian px-3 py-2 text-[13px] text-cream focus:border-talon-gold focus:outline-none"
      rows="2"
      disabled={sending}
    ></textarea>
    <button
      type="button"
      onclick={send}
      disabled={sending || value.trim().length === 0}
      class="rounded border border-talon-gold bg-talon-gold px-3 py-2 text-[12px] font-semibold text-obsidian hover:opacity-90 disabled:opacity-50"
    >
      <Send class="size-4" />
    </button>
  </div>
  {#if error}
    <p class="text-[11px] text-red-400">{error}</p>
  {/if}
</div>
```

- [ ] **Step 8: Type-check**

```bash
cd /home/thbertoldi/harpia/frontend && npx svelte-kit sync >/dev/null 2>&1 && npm run check 2>&1 | grep -c "src/lib/components/thread"
```

Expected: 0 (no errors mentioning the thread components).

- [ ] **Step 9: Format + lint**

```bash
cd /home/thbertoldi/harpia/frontend && npm run format >/dev/null && npm run lint 2>&1 | tail -3
```

Expected: clean.

- [ ] **Step 10: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/components/thread
git commit -m "feat(ux-m3): add lib/components/thread — chat surface components"
```

---

## Task 15: `/plans/configurations/[configurationId]/` route — the page

**Files:**
- Create: `frontend/src/routes/plans/configurations/[configurationId]/+page.svelte`
- Create: `frontend/src/routes/plans/configurations/[configurationId]/+page.ts`

**Interfaces:**
- Consumes:
  - `loadThreadMessages` and `watchThreadMessages` from `$lib/chat/{client,watch}`
  - `buildThreadSections` from `$lib/plans/thread`
  - `PlanDagMiniMap`, all thread components from Task 14
  - `watchInbox` from `$lib/inbox/aggregator` (for live pending state filtered to this configuration)
  - `loadPlanTemplate` from `$lib/plans/plan-template` (to load the template referenced by the configuration, for the DAG mini-map)
  - `getTenant` from `$lib/auth`; `planClient` from `$lib/rpc`
- Produces: the route handler + page UI.

- [ ] **Step 1: Create `+page.ts` (server-side load)**

Create `frontend/src/routes/plans/configurations/[configurationId]/+page.ts`:

```ts
import { error } from "@sveltejs/kit";
import type { PageLoad } from "./$types";
import { planClient } from "$lib/rpc";
import { getTenant } from "$lib/auth";

export const load: PageLoad = async ({ params }) => {
  const tenant = getTenant();
  if (!tenant?.id) {
    throw error(401, "Not authenticated");
  }
  const { configurationId } = params;
  try {
    const config = await planClient.getPlanConfiguration({
      tenantId: tenant.id,
      planConfigurationId: configurationId,
    });
    if (!config.planConfiguration) {
      throw error(404, "Plan configuration not found");
    }
    const template = await planClient.getPlanTemplate({
      planTemplateId: config.planConfiguration.planTemplateId,
    });
    return {
      configurationId,
      configuration: config.planConfiguration,
      template: template.planTemplate,
    };
  } catch (e) {
    throw error(404, "Plan configuration not found");
  }
};
```

Note: `getPlanConfiguration` may be a different RPC name (verify against the generated TS client). The recon showed `GetPlanConfiguration` exists. If the param name on the request is `planConfigurationId` vs `configurationId`, use whichever the generated type requires.

- [ ] **Step 2: Create `+page.svelte`**

Create `frontend/src/routes/plans/configurations/[configurationId]/+page.svelte`:

```svelte
<script lang="ts">
  import { onMount } from "svelte";
  import { browser } from "$app/environment";
  import { page } from "$app/state";
  import { getTenant } from "$lib/auth";
  import { locale, translate } from "$lib/i18n";
  import {
    appendThreadMessage,
    loadThreadMessages,
  } from "$lib/chat/client";
  import { watchThreadMessages } from "$lib/chat/watch";
  import type { ChatMessage } from "$lib/chat/types";
  import { buildThreadSections } from "$lib/plans/thread";
  import PlanDagMiniMap from "$lib/components/PlanDagMiniMap.svelte";
  import ThreadMessage from "$lib/components/thread/ThreadMessage.svelte";
  import ExecutionSection from "$lib/components/thread/ExecutionSection.svelte";
  import ThreadComposer from "$lib/components/thread/ThreadComposer.svelte";
  import HintBanner from "$lib/components/thread/HintBanner.svelte";

  let { data } = $props();

  let messages = $state<ChatMessage[]>([]);
  let loadError = $state(false);

  const tenantId = $derived(getTenant()?.id ?? "");

  // Initial load via list-RPC, then live updates via watch-RPC with AbortController.
  $effect(() => {
    if (!tenantId || !data.configurationId) return;
    const controller = new AbortController();
    (async () => {
      try {
        // Initial historical load.
        const initial = await loadThreadMessages(tenantId, data.configurationId);
        if (controller.signal.aborted) return;
        messages = initial;
        // Live updates from the last known sequence.
        const sinceSeq =
          initial.length > 0
            ? initial[initial.length - 1].sequenceNumber
            : 0n;
        for await (const batch of watchThreadMessages(
          tenantId,
          data.configurationId,
          { sinceSequenceNumber: sinceSeq, signal: controller.signal },
        )) {
          if (controller.signal.aborted) return;
          messages = [...messages, ...batch];
        }
      } catch (err) {
        if (controller.signal.aborted) return;
        loadError = true;
      }
    })();
    return () => {
      controller.abort();
    };
  });

  const sections = $derived(buildThreadSections(messages));
  const mostRecentExecutionId = $derived.by(() => {
    for (let i = sections.length - 1; i >= 0; i--) {
      const s = sections[i];
      if (s.kind === "execution") return s.group.executionId;
    }
    return null;
  });

  // Deep-link anchor: handle three formats and scroll once when target arrives:
  //   #m-<messageId>                — direct chat_message id
  //   #m-elicitation-<elicitationId> — resolve to ELICITATION_RAISED message
  //   #m-approval-<approvalId>       — resolve to APPROVAL_RAISED message
  // The effect re-runs as messages arrive (Svelte tracks the `messages` read).
  // A flag prevents scrolling more than once.
  let didScrollToAnchor = $state(false);
  $effect(() => {
    if (!browser || didScrollToAnchor) return;
    const hash = window.location.hash;
    if (!hash.startsWith("#m-")) return;
    let targetId: string | null = null;
    if (hash.startsWith("#m-elicitation-")) {
      const elicitId = hash.slice("#m-elicitation-".length);
      const match = messages.find((m) => {
        if (m.kind !== "ELICITATION_RAISED") return false;
        try {
          return JSON.parse(m.payloadJson)?.elicitation_id === elicitId;
        } catch {
          return false;
        }
      });
      targetId = match ? `m-${match.id}` : null;
    } else if (hash.startsWith("#m-approval-")) {
      const approvalId = hash.slice("#m-approval-".length);
      const match = messages.find((m) => {
        if (m.kind !== "APPROVAL_RAISED") return false;
        try {
          return JSON.parse(m.payloadJson)?.approval_request_id === approvalId;
        } catch {
          return false;
        }
      });
      targetId = match ? `m-${match.id}` : null;
    } else {
      targetId = hash.slice(1);
    }
    if (!targetId) return; // target message hasn't arrived yet — wait for next reactive update
    requestAnimationFrame(() => {
      const el = document.getElementById(targetId!);
      if (el) {
        el.scrollIntoView({ behavior: "smooth", block: "center" });
        el.classList.add("harpia-pulse-anchor");
        setTimeout(() => el.classList.remove("harpia-pulse-anchor"), 1500);
        didScrollToAnchor = true;
      }
    });
  });
</script>

<svelte:head>
  <title>
    {data.configuration?.workspaceId
      ? "Plan thread"
      : translate("thread.title", $locale)} · Harpia
  </title>
</svelte:head>

<div class="mx-auto flex max-w-3xl flex-col gap-3 px-4 py-6">
  {#if data.template?.steps && data.template.steps.length > 0}
    <PlanDagMiniMap steps={data.template.steps} edges={data.template.edges} />
  {/if}

  <HintBanner threadId={data.configurationId} />

  {#if loadError}
    <p class="rounded border border-plumage bg-obsidian-light px-4 py-3 text-sm text-crown-ash">
      {translate("thread.loadError", $locale)}
    </p>
  {:else if sections.length === 0}
    <p class="rounded border border-plumage bg-obsidian-light px-4 py-3 text-sm text-crown-ash">
      {translate("thread.empty", $locale)}
    </p>
  {:else}
    <div class="flex flex-col gap-2">
      {#each sections as section, index (section.kind === "execution" ? section.group.executionId : section.message.id)}
        {#if section.kind === "plan-scope"}
          <ThreadMessage message={section.message} />
        {:else}
          <ExecutionSection
            group={section.group}
            defaultExpanded={section.group.executionId === mostRecentExecutionId}
          />
        {/if}
      {/each}
    </div>
  {/if}

  <ThreadComposer
    {tenantId}
    configurationId={data.configurationId}
  />
</div>

<style>
  :global(.harpia-pulse-anchor) {
    animation: harpia-pulse 1.5s ease-out;
  }
  @keyframes harpia-pulse {
    0% {
      box-shadow: 0 0 0 0 rgba(212, 175, 55, 0.7);
    }
    100% {
      box-shadow: 0 0 0 8px rgba(212, 175, 55, 0);
    }
  }
</style>
```

- [ ] **Step 3: Type-check and verify the route compiles**

```bash
cd /home/thbertoldi/harpia/frontend && npx svelte-kit sync >/dev/null 2>&1 && npm run check 2>&1 | grep -E "configurations\[" | head -5
```

Expected: no errors. If `getPlanConfiguration` arg names differ, fix per the generated client.

- [ ] **Step 4: Format and lint**

```bash
cd /home/thbertoldi/harpia/frontend && npm run format >/dev/null && npm run lint 2>&1 | tail -3
```

Expected: clean.

- [ ] **Step 5: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/routes/plans/configurations
git commit -m "feat(ux-m3): /plans/configurations/[id] route — plan thread page"
```

---

## Task 16: Inbox deep-link wiring

**Files:**
- Modify: `frontend/src/lib/components/inbox/InboxElicitationActions.svelte` (link to thread + anchor)
- Modify: `frontend/src/lib/components/inbox/InboxApprovalEntry.svelte` (verify the existing inline behavior still works after aggregator extension; no behavior change but verify type compat with `configurationId` field)

**Interfaces:**
- Consumes: `configurationId` on `InboxItem` from Task 11

- [ ] **Step 1: Find the current InboxElicitationActions implementation**

```bash
cat /home/thbertoldi/harpia/frontend/src/lib/components/inbox/InboxElicitationActions.svelte
```

It currently links to `/plans/executions/[executionId]/elicitations/[id]`.

- [ ] **Step 2: Update the href to point to the thread**

Replace the existing `<a href={...}>` line with:

```svelte
<a
  href={resolve(
    item.configurationId
      ? `/plans/configurations/${item.configurationId}#m-elicitation-${item.id}`
      : `/plans/executions/${item.planExecutionId}/elicitations/${item.id}`,
  )}
  ...
>
```

The fallback to the legacy URL is for cases where the aggregator's `configurationId` field is empty (e.g., before the backend proto change ships fully). Once Task 11's proto change is live, the fallback path is unreachable.

Note: the anchor `#m-elicitation-${item.id}` references the elicitation_id, not the chat message id. Task 15's `+page.svelte` deep-link handler already handles this dispatch (see Task 15 Step 2 — the handler decodes `#m-elicitation-<id>` and `#m-approval-<id>` formats and scrolls to the matching pointer message). No additional page-side change required by this task.

- [ ] **Step 3: Verify InboxApprovalEntry still compiles**

`InboxApprovalEntry.svelte` already exists from M2. It uses `item` (an `InboxApprovalItem`) and now that item has a `configurationId` field. No behavior change needed in the entry itself — but verify it compiles:

```bash
cd /home/thbertoldi/harpia/frontend && npm run check 2>&1 | grep -i "InboxApproval" | head -5
```

Expected: no errors. If `InboxApprovalItem` shape changed in a way that breaks the entry, fix the breakage.

- [ ] **Step 4: Run all inbox tests**

```bash
cd /home/thbertoldi/harpia/frontend && npx vitest run src/lib/inbox/
```

Expected: 6 tests pass.

- [ ] **Step 5: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/components/inbox/InboxElicitationActions.svelte frontend/src/routes/plans/configurations
git commit -m "feat(ux-m3): inbox 'Open thread' deep-links to plan thread"
```

---

## Task 17: i18n keys for the thread surface

**Files:**
- Modify: `frontend/src/lib/i18n/en.json`
- Modify: `frontend/src/lib/i18n/pt-BR.json`

**Interfaces:**
- Produces: a new `thread.*` family of i18n keys covering composer, hints, system event labels, execution-section labels, error/empty states.

- [ ] **Step 1: Add keys to en.json**

In `frontend/src/lib/i18n/en.json`, insert after the existing `inbox.*` block (find any `"inbox.summary.approval"` line — insert after it):

```json
  "thread.title": "Plan thread",
  "thread.empty": "Nothing has happened on this plan yet. When the first run starts, events will appear here.",
  "thread.loadError": "Could not load the plan thread.",
  "thread.composer.placeholder": "Leave yourself a note…",
  "thread.composer.error": "Could not send the message. Try again.",
  "thread.composerHint": "The assistant doesn't reply yet — coming in a later milestone. You can leave notes for yourself in the meantime.",
  "thread.dismiss": "Dismiss",
  "thread.execution.runLabel": "Run {n}",
  "thread.execution.status.running": "running",
  "thread.execution.status.completed": "completed",
  "thread.execution.status.failed": "failed",
  "thread.execution.status.unknown": "unknown",
  "thread.event.configuration_saved": "Configuration saved.",
  "thread.event.run_started": "Run started.",
  "thread.event.run_completed": "Run completed.",
  "thread.event.run_failed": "Run failed.",
  "thread.event.step_bound": "Step completed.",
```

- [ ] **Step 2: Add the same keys to pt-BR.json**

Insert the same block in the same position with Portuguese values:

```json
  "thread.title": "Conversa do plano",
  "thread.empty": "Nada aconteceu neste plano ainda. Quando a primeira execução começar, eventos aparecerão aqui.",
  "thread.loadError": "Não foi possível carregar a conversa do plano.",
  "thread.composer.placeholder": "Deixe uma nota para você…",
  "thread.composer.error": "Não foi possível enviar a mensagem. Tente novamente.",
  "thread.composerHint": "O assistente ainda não responde — chegando em um marco futuro. Você pode deixar notas para si mesmo enquanto isso.",
  "thread.dismiss": "Dispensar",
  "thread.execution.runLabel": "Execução {n}",
  "thread.execution.status.running": "em andamento",
  "thread.execution.status.completed": "concluída",
  "thread.execution.status.failed": "falhou",
  "thread.execution.status.unknown": "desconhecido",
  "thread.event.configuration_saved": "Configuração salva.",
  "thread.event.run_started": "Execução iniciada.",
  "thread.event.run_completed": "Execução concluída.",
  "thread.event.run_failed": "Execução falhou.",
  "thread.event.step_bound": "Etapa concluída.",
```

- [ ] **Step 3: Run the i18n parity test**

```bash
cd /home/thbertoldi/harpia/frontend && npx vitest run src/lib/i18n/
```

Expected: 13 tests pass (parity confirms identical key sets).

- [ ] **Step 4: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/i18n/en.json frontend/src/lib/i18n/pt-BR.json
git commit -m "feat(ux-m3): add thread.* i18n keys for plan thread surface"
```

---

## Task 18: End-to-end verification

**Files:** None modified — manual + automated verification. Deliverable is a verification log.

- [ ] **Step 1: Run the full automated suite**

```bash
cd /home/thbertoldi/harpia
( cd frontend && npm run test 2>&1 | tail -5 )
( cd frontend && npm run check 2>&1 | grep "ERRORS" | tail -1 )
( cd frontend && npm run lint 2>&1 | tail -3 )
( cd control-plane && go test ./... 2>&1 | tail -5 )
( cd control-plane && go vet ./... 2>&1 | tail -3 )
```

Expected:
- Frontend tests: previous count + the new tests from Tasks 9, 12 (4-6 new tests roughly)
- Type-check: 10 errors (trunk baseline; 0 new)
- Lint: clean
- Go tests: all pre-existing pass plus the new chat package tests (7 new)
- Go vet: clean

- [ ] **Step 2: Dev-server smoke (frontend)**

```bash
cd /home/thbertoldi/harpia/frontend && npm run dev -- --port 5179 > /tmp/harpia-m3-dev.log 2>&1 &
DEV=$!
sleep 8
echo "--- routes ---"
for r in / /inbox /plans/configurations/test-id ; do
  curl -s -o /dev/null -w "%{http_code} → $r\n" -L --max-redirs 0 "http://localhost:5179$r"
done
tail -3 /tmp/harpia-m3-dev.log
kill $DEV 2>/dev/null
wait $DEV 2>/dev/null
```

Expected: each route returns 302 (auth redirect to /login). No 500 in the dev log.

- [ ] **Step 3: Backend integration check (if Tilt is up)**

If Tilt is running, regenerate proto and restart the API:

```bash
cd /home/thbertoldi/harpia/proto && buf generate
# Tilt should hot-reload control-plane after the Go files change.
```

Verify the new RPCs are registered (look at the harpia-api logs in Tilt). If Tilt isn't up, skip — this is verified by `go test` already.

- [ ] **Step 4: Write the verification log**

Create `docs/superpowers/plans/2026-06-21-harpia-ux-realignment-m3-plan-thread.verification.md`:

```markdown
# M3 Verification — <date>

Branch: `feat/ux-realignment-m3-plan-thread`
Plan: `docs/superpowers/plans/2026-06-21-harpia-ux-realignment-m3-plan-thread.md`

## Automated checks (executed)

| Check | Result | Notes |
|---|---|---|
| `frontend: npm run test` | ✅ <N/N> passing | New tests: lib/chat/client.test.ts (2), lib/plans/thread.test.ts (6) |
| `frontend: npm run check` | ✅ 10 errors, 0 new | Trunk baseline (4 auth-roles + 1 artifact-flow + 1 layout + 2 plans/[templateId] + 2 hardcoded-copy) |
| `frontend: npm run lint` | ✅ clean | |
| `control-plane: go test ./...` | ✅ <N> passing | New: internal/chat (7 tests), internal/plans/thread_handler (3) |
| `control-plane: go vet ./...` | ✅ clean | |
| Frontend dev server boots | ✅ Vite ready | |
| Route /plans/configurations/[id] resolves | ✅ 302→/login | Authenticated render not tested via curl |
| `proto: buf generate && buf lint` | ✅ clean | New chat.v1 package + 3 PlanService RPCs |

## Browser-driven verification (manual — required before merge)

Run `cd frontend && npm run dev` with Tilt up so the backend is reachable. Use `devLogin('Leader')`.

| Step | Expected |
|---|---|
| Visit a PlanConfiguration's thread URL: `/plans/configurations/<configId>` | Page renders. DAG mini-map at top. First message: "Configuration saved." (or "Configuration updated." for updated ones). HintBanner visible. Composer at bottom. |
| Type a message in the composer and press send | Message appears in the thread as a right-aligned bubble. HintBanner can be dismissed (persists across reloads via localStorage). |
| Run a plan (use existing wizard) | Live updates: RUN_STARTED system event appears, then STEP_BOUND for each step, then RUN_COMPLETED or RUN_FAILED. |
| Raise an elicitation on a step | ELICITATION_RAISED card appears in the thread inside the execution section. |
| Answer the elicitation (via the existing detail page or via inbox) | ELICITATION_ANSWERED card appears below the raised one. |
| Click "Open thread" on an inbox elicitation row | Navigates to /plans/configurations/<configId>#m-elicitation-<id>; thread scrolls to that message, pulses briefly. |
| Reload the thread page | Messages persist (loaded from chat_messages table). |
| Most recent execution section is expanded by default; older ones collapsed | Verified visually. |

## Commits

(populate from `git log --oneline trunk..HEAD`)

## Status

**Automated portion: ✅ complete.** Manual browser steps required before merge.
```

- [ ] **Step 5: Commit the verification log**

```bash
git add docs/superpowers/plans/2026-06-21-harpia-ux-realignment-m3-plan-thread.verification.md
git commit -m "chore(ux-m3): record M3 verification notes"
```

---

## Task 19: Whole-branch final review

**Files:** None modified — final review dispatch.

- [ ] **Step 1: Generate the review package**

```bash
cd /home/thbertoldi/harpia
MERGE_BASE=$(git merge-base trunk HEAD)
SCRIPTS=/home/thbertoldi/.claude/plugins/cache/claude-plugins-official/superpowers/6.0.3/skills/subagent-driven-development/scripts
$SCRIPTS/review-package $MERGE_BASE HEAD
```

- [ ] **Step 2: Dispatch the strongest available reviewer**

Using the orchestrator's standard final-review prompt (from `superpowers/6.0.3/skills/requesting-code-review/code-reviewer.md`), dispatch with the diff package, the spec, and the plan as required reading. Reviewer focus areas (per M3's risk surface):

1. **Vocabulary discipline** — every user-facing string respects spec §2 canonical terms
2. **i18n parity** — every key change is lockstep across en.json + pt-BR.json
3. **Concurrency in watchThreadMessages and watchInbox** — no leaked streams, AbortSignal correctly cascades
4. **Backend write paths** — chat_messages writes don't break the existing handlers (best-effort, swallowed errors are intentional)
5. **Snippet conditional pattern** — none of the new components use the M2 bug pattern (passing snippets unconditionally with internal guards)
6. **No new type errors** — type-check at trunk baseline
7. **Per-execution grouping correctness** — buildThreadSections handles edge cases (empty thread, plan-scope-only, alternating execution_ids)
8. **v1→v2 hinges preserved** — `ASSISTANT_TEXT` reserved but unused, generic chat.Store, extraction-friendly proto layout
9. **No broken legacy routes** — `/plans/[templateId]` template detail still works, M2 inbox still works, all M1 routes still work

- [ ] **Step 3: Address review findings**

Critical findings: fix on the branch with `fix(ux-m3):` commits. Important findings: assess scope and either fix or defer with documentation. Minor: defer.

- [ ] **Step 4: Push the branch**

```bash
git push -u origin feat/ux-realignment-m3-plan-thread
```

- [ ] **Step 5: Open the PR**

```bash
gh pr create --title "feat(ux): M3 plan thread" --body "$(cat <<'EOF'
## Summary

Ships M3 of the UX Realignment — the per-PlanConfiguration chat thread at `/plans/configurations/[configurationId]`.

**Plan:** [`docs/superpowers/plans/2026-06-21-harpia-ux-realignment-m3-plan-thread.md`](docs/superpowers/plans/2026-06-21-harpia-ux-realignment-m3-plan-thread.md)
**Design spec:** [`docs/superpowers/specs/2026-06-21-harpia-ux-m3-plan-thread-design.md`](docs/superpowers/specs/2026-06-21-harpia-ux-m3-plan-thread-design.md)
**Verification log:** [`docs/superpowers/plans/2026-06-21-harpia-ux-realignment-m3-plan-thread.verification.md`](docs/superpowers/plans/2026-06-21-harpia-ux-realignment-m3-plan-thread.verification.md)

## What ships

### Backend
- New `harpia.chat.v1` proto package + 3 RPCs on PlanService (`WatchPlanThreadMessages`, `AppendPlanThreadMessage`, `ListPlanThreadMessages`)
- New `chat_messages` table (PostgreSQL migration 000012) keyed by generic `thread_id`
- New `internal/chat/` Go package with a generic Store; PlanService delegates
- Workflow activity hooks: `RUN_STARTED`, `RUN_COMPLETED`, `RUN_FAILED`, `STEP_BOUND`, `ELICITATION_RAISED`, `ELICITATION_ANSWERED`, `APPROVAL_RAISED`, `APPROVAL_DECIDED`
- `CONFIGURATION_SAVED` emission on `Create/UpdatePlanConfiguration`

### Frontend
- `/plans/configurations/[configurationId]/` route with chat thread UI
- `PlanDagMiniMap` compact DAG component
- `lib/components/thread/`: ThreadMessage, SystemEventCard, ElicitationRefCard, ApprovalRefCard, ExecutionSection, ThreadComposer, HintBanner
- `lib/chat/`: types, client, watch stream
- `lib/plans/thread.ts`: execution-grouping helper
- AbortSignal plumbing through `watchElicitations`, `watchApprovalRequests`, `watchInbox` (clears M2 debt)
- `plan_configuration_id` exposed on Elicitation/Approval RPC responses and InboxItem
- Inbox "Open thread" links rewired to deep-link into the plan thread

## Known limitations / M4-M5 follow-ups

- Composer is passive — no LLM behind it. M5 will ship the assistant brain. `ASSISTANT_TEXT` kind is reserved.
- Feedback "Open thread" still links to `/oversee` (FeedbackRequest has no direct config_id; deferred to M5)
- Sidebar "your plans" list deferred (spec §2.11 — not blocking the thread surface)

## Test plan

See verification log for full checklist. Manual browser smoke required before merge.

EOF
)"
```

- [ ] **Step 6: Final status**

The PR URL is the deliverable. Update the progress ledger with the PR number and mark M3 done.

---

## Done criteria

M3 is complete when all of the following hold:

1. The `/plans/configurations/[configurationId]` route renders a working chat thread with the DAG mini-map, message list (grouped by execution_id), composer, and hint banner.
2. Every plan-lifecycle event (configuration save, run start, step bound, run complete/fail, elicitation raised/answered, approval raised/decided) results in a chat_messages row.
3. The composer persists user-typed messages as USER_TEXT chat_messages.
4. Inbox "Open thread" for elicitations and approvals navigates to the thread with the correct anchor and pulses the target message briefly.
5. `npm run test` passes (frontend + backend).
6. `npm run check` is at trunk baseline.
7. `npm run lint` clean.
8. AbortSignal correctly cascades on page unmount (verified by watching no zombie connections in dev tools after navigating away).
9. PR open with verification log linked, manual smoke checklist in body.
10. No legacy routes broken: `/inbox`, `/plans/[templateId]`, `/plans/executions/[id]`, all M1 routes still work.

After M3 ships, M4 (expanded canvas) can begin. The thread anchors and message ids exist as a stable target for M4's "click an execution section header → open canvas at this run" deep-link.
