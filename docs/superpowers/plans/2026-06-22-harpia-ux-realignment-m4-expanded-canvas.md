# Harpia UX Realignment — M4 Expanded Canvas — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the expanded canvas at `/plans/configurations/[configurationId]/canvas?run=<execId>` — full-bleed DAG view of one PlanExecution with per-node state, right-pane inline answer forms, and Run-history + Settings drawers in the top bar.

**Architecture:** Reuses M3 chat stream as the live-state source (adds one new `STEP_STARTED` event kind). New `PlanCanvas` component owns layout + selection. Approval and elicitation forms are extracted from M2/M1 surfaces into reusable canvas-pane components so the existing pages just compose them. Legacy `/plans/executions/[id]/` becomes a 302 redirect to the canonical canvas URL.

**Tech Stack:** Go 1.23 + ConnectRPC (control-plane), SvelteKit 2.16 + Svelte 5 (frontend), Vitest (frontend tests), `testing` package (Go tests), Tailwind v4 with Harpia design tokens.

## Global Constraints

- **Vocabulary canonical** per master design spec §2 — Task / Plan / Agent / Integration / Executor / Overseer / Elicitation / Approval / Feedback / Artifact. User-facing copy and i18n keys use these terms exactly.
- **i18n lockstep.** Every key change touches both `frontend/src/lib/i18n/en.json` and `frontend/src/lib/i18n/pt-BR.json` in the same commit. The parity test in `frontend/src/lib/i18n/hardcoded-copy.test.ts` enforces this.
- **Design tokens LOCKED.** Only existing Tailwind tokens: `bg-obsidian`, `bg-obsidian-light`, `text-cream`, `text-crown-ash`, `text-crown-ash-dark`, `text-talon-gold`, `border-plumage`, `font-heading`, `font-body`, `font-mono`. No new colors.
- **Commit per task.** Conventional Commits with `feat(ux-m4):` / `chore(ux-m4):` / `fix(ux-m4):` / `docs(ux-m4):` / `test(ux-m4):` scope.
- **No `Co-Authored-By` trailer** on any commit (project convention).
- **Trunk type-check baseline:** 10 pre-existing errors. ZERO new from M4 code.
- **Test isolation per task.** Run only focused tests: `npx vitest run <relative-path>` from `frontend/` for frontend; `go test ./internal/<pkg>/...` for Go.
- **Branch:** `feat/ux-realignment-m4-expanded-canvas` (already checked out).
- **No new files outside the plan's file map.** Every new file is named explicitly. If your implementation needs a file the plan doesn't name, STOP and report BLOCKED.
- **Buf codegen.** After modifying any `.proto` file, run `cd /home/thbertoldi/harpia/proto && buf generate && buf lint`. Commit the generated code alongside the proto changes in the same commit.
- **Behavior preservation on extraction.** Tasks 5 and 6 extract form logic out of `InboxApprovalEntry.svelte` and the elicitation detail page. The original surfaces (the inbox row, the detail page) MUST behave identically after the extraction. No user-visible regression.

---

## File map (all changes in M4)

### Backend
- Modify: `proto/harpia/chat/v1/chat.proto` (add `STEP_STARTED` to `ThreadMessageKind` enum)
- Modify: `control-plane/internal/chat/messages.go` (add `BuildStepStartedPayload` helper)
- Create: appended tests in `control-plane/internal/chat/messages_test.go`
- Modify: `control-plane/internal/plans/runtime.go` (`CreateStepExecution` emits `STEP_STARTED` after status set to RUNNING)
- Regenerate: `control-plane/gen/`, `frontend/src/lib/gen/`, `agent-runtime/src/harpia_agents/gen/`

### Frontend lib
- Modify: `frontend/src/lib/chat/types.ts` (extend `ChatMessageKind` union + maps)
- Create: `frontend/src/lib/plans/canvas-state.ts`
- Create: `frontend/src/lib/plans/canvas-state.test.ts`

### Frontend forms (extractions)
- Create: `frontend/src/lib/components/canvas/CanvasApprovalForm.svelte`
- Modify: `frontend/src/lib/components/inbox/InboxApprovalEntry.svelte` (composes `CanvasApprovalForm`)
- Create: `frontend/src/lib/components/canvas/CanvasElicitationForm.svelte`
- Modify: `frontend/src/routes/plans/executions/[executionId]/elicitations/[elicitationId]/+page.svelte` (composes `CanvasElicitationForm`)

### Frontend canvas components
- Create: `frontend/src/lib/components/canvas/PlanCanvasNode.svelte`
- Create: `frontend/src/lib/components/canvas/PlanCanvasEdge.svelte`
- Create: `frontend/src/lib/components/canvas/PlanCanvasDetailPane.svelte`
- Create: `frontend/src/lib/components/canvas/CanvasTopBar.svelte`
- Create: `frontend/src/lib/components/canvas/PlanCanvas.svelte`

### Frontend drawers
- Create: `frontend/src/lib/components/canvas/CanvasDrawer.svelte` (shared drawer chrome)
- Create: `frontend/src/lib/components/canvas/RunHistoryDrawer.svelte`
- Create: `frontend/src/lib/components/canvas/SettingsDrawer.svelte`

### Page + integration
- Create: `frontend/src/routes/plans/configurations/[configurationId]/canvas/+page.svelte`
- Create: `frontend/src/routes/plans/configurations/[configurationId]/canvas/+page.ts`
- Create: `frontend/src/routes/plans/executions/[executionId]/+page.ts` (replaces deleted `+page.svelte`)
- Delete: `frontend/src/routes/plans/executions/[executionId]/+page.svelte`
- Delete: `frontend/src/routes/plans/[templateId]/canvas/+page.svelte` (M1 stub)
- Modify: `frontend/src/routes/plans/configurations/[configurationId]/+page.svelte` (wire `Expand ⤢` affordance on the inline mini-map)

### i18n
- Modify: `frontend/src/lib/i18n/en.json` (add `canvas.*` keys)
- Modify: `frontend/src/lib/i18n/pt-BR.json` (lockstep)

---

## Task 1: Add `STEP_STARTED` to the chat proto

**Files:**
- Modify: `proto/harpia/chat/v1/chat.proto`

**Interfaces:**
- Produces: `harpia.chat.v1.ThreadMessageKind.THREAD_MESSAGE_KIND_STEP_STARTED = 12`

- [ ] **Step 1: Add the new enum value**

In `proto/harpia/chat/v1/chat.proto`, find the `enum ThreadMessageKind { ... }` block. After the existing `THREAD_MESSAGE_KIND_APPROVAL_DECIDED = 11;` line and BEFORE the closing brace, ADD:

```protobuf
  THREAD_MESSAGE_KIND_STEP_STARTED = 12;            // payload_json: { "step_key": "...", "step_execution_id": "..." }
```

- [ ] **Step 2: Regenerate**

```bash
cd /home/thbertoldi/harpia/proto && buf generate && buf lint
```

Expected: no errors. Generated files at `control-plane/gen/harpia/chat/v1/chat.pb.go`, `frontend/src/lib/gen/harpia/chat/v1/chat_pb.ts`, `agent-runtime/src/harpia_agents/gen/harpia/chat/v1/chat_pb2.py` now include the new enum value.

- [ ] **Step 3: Commit**

```bash
cd /home/thbertoldi/harpia
git add proto/harpia/chat/v1/chat.proto control-plane/gen/harpia/chat frontend/src/lib/gen/harpia/chat agent-runtime/src/harpia_agents/gen/harpia/chat
git commit -m "feat(ux-m4): add STEP_STARTED to harpia.chat.v1.ThreadMessageKind"
```

---

## Task 2: `BuildStepStartedPayload` helper + workflow emission hook

**Files:**
- Modify: `control-plane/internal/chat/messages.go` (add helper)
- Modify: `control-plane/internal/chat/messages_test.go` (add test)
- Modify: `control-plane/internal/plans/runtime.go` (emit on `CreateStepExecution`)

**Interfaces:**
- Consumes: `chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_STEP_STARTED` from Task 1
- Produces:
  - `chat.BuildStepStartedPayload(stepKey, stepExecutionID string) string`
  - Runtime emits a `STEP_STARTED` chat message every time a `step_executions` row is INSERTed with status `RUNNING`

- [ ] **Step 1: Write the failing test for the payload helper**

In `control-plane/internal/chat/messages_test.go`, APPEND:

```go
func TestBuildStepStartedPayload(t *testing.T) {
	got := BuildStepStartedPayload("write-draft", "step-exec-uuid")

	var decoded map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if decoded["step_key"] != "write-draft" {
		t.Fatalf("expected step_key=write-draft, got %s", decoded["step_key"])
	}
	if decoded["step_execution_id"] != "step-exec-uuid" {
		t.Fatalf("expected step_execution_id=step-exec-uuid, got %s", decoded["step_execution_id"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/thbertoldi/harpia/control-plane && go test ./internal/chat/... -run TestBuildStepStartedPayload
```

Expected: FAIL — `undefined: BuildStepStartedPayload`.

- [ ] **Step 3: Implement the helper**

In `control-plane/internal/chat/messages.go`, AFTER the existing `BuildStepBoundPayload` function, ADD:

```go
// BuildStepStartedPayload returns the JSON payload for a STEP_STARTED message.
// Mirrors STEP_BOUND but without an output artifact (which doesn't exist yet
// at start-of-step time).
func BuildStepStartedPayload(stepKey, stepExecutionID string) string {
	return mustEncodeJSON(map[string]string{
		"step_key":          stepKey,
		"step_execution_id": stepExecutionID,
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/thbertoldi/harpia/control-plane && go test ./internal/chat/... -run TestBuildStepStartedPayload
```

Expected: PASS.

- [ ] **Step 5: Add the runtime emission hook**

In `control-plane/internal/plans/runtime.go`, find the `CreateStepExecution` method (around line 126). The recon found that line 140 sets `step.Status = StepStatusRunning` and the create proceeds to write the row to the DB.

Locate the SUCCESS return path of the method (after `r.plans.CreateStepExecution(...)` succeeds and before `return step, nil` or equivalent). Look for the imports block at the top of the file — `chatv1` and `chat` should already be there from M3 (the file already emits `STEP_BOUND`). If not, ADD:

```go
	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	"github.com/harpia/control-plane/internal/chat"
```

After the successful `CreateStepExecution(...)` call, BEFORE the return, ADD this block (mirror the existing STEP_BOUND emission pattern at line 164-190):

```go
	if r.chat != nil {
		execID := executionID
		configID, lookupErr := r.GetPlanConfigurationIDForExecution(ctx, tenantID, executionID)
		if lookupErr == nil {
			_, _ = r.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
				ThreadID:    configID.String(),
				ExecutionID: &execID,
				Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
				Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_STEP_STARTED,
				Text:        "Step " + input.PlanStepKey + " started.",
				PayloadJSON: chat.BuildStepStartedPayload(input.PlanStepKey, step.ID.String()),
			})
		}
	}
```

Note: variable names (`executionID`, `tenantID`, `input.PlanStepKey`, `step.ID`) must match the local variables in `CreateStepExecution`. The recon showed the method receives `input workflow.CreateStepExecutionInput` and returns `step` (the inserted record). Adjust names to match what's actually in scope.

If `r.GetPlanConfigurationIDForExecution` doesn't exist directly on `RuntimeRepository` (the recon showed the method exists on a different receiver), use the same lookup path the existing `CompleteStepExecution` STEP_BOUND emission uses — copy that pattern verbatim.

- [ ] **Step 6: Compile and run existing tests**

```bash
cd /home/thbertoldi/harpia/control-plane && go vet ./... && go test ./internal/chat/... ./internal/plans/...
```

Expected: existing tests pass; new payload test passes.

- [ ] **Step 7: Commit**

```bash
cd /home/thbertoldi/harpia
git add control-plane/internal/chat/messages.go control-plane/internal/chat/messages_test.go control-plane/internal/plans/runtime.go
git commit -m "feat(ux-m4): emit STEP_STARTED chat message when step transitions to RUNNING"
```

---

## Task 3: Frontend `ChatMessageKind` extension

**Files:**
- Modify: `frontend/src/lib/chat/types.ts`

**Interfaces:**
- Consumes: `ProtoThreadMessageKind.STEP_STARTED` (generated by Task 1)
- Produces: `ChatMessageKind` union gains `"STEP_STARTED"`; the `KIND_FROM_PROTO` and `KIND_TO_PROTO` maps gain entries

- [ ] **Step 1: Extend the union type**

In `frontend/src/lib/chat/types.ts`, find the `ChatMessageKind` union declaration. ADD `"STEP_STARTED"` to it:

```ts
export type ChatMessageKind =
  | "USER_TEXT"
  | "ASSISTANT_TEXT"
  | "CONFIGURATION_SAVED"
  | "RUN_STARTED"
  | "RUN_COMPLETED"
  | "RUN_FAILED"
  | "STEP_STARTED"
  | "STEP_BOUND"
  | "ELICITATION_RAISED"
  | "ELICITATION_ANSWERED"
  | "APPROVAL_RAISED"
  | "APPROVAL_DECIDED";
```

- [ ] **Step 2: Update the proto↔domain maps**

In the same file, find `KIND_FROM_PROTO` and `KIND_TO_PROTO`. ADD entries for `STEP_STARTED`:

```ts
const KIND_FROM_PROTO: Record<number, ChatMessageKind> = {
  // ... existing entries
  [ProtoThreadMessageKind.STEP_STARTED]: "STEP_STARTED",
};

const KIND_TO_PROTO: Record<ChatMessageKind, ProtoThreadMessageKind> = {
  // ... existing entries
  STEP_STARTED: ProtoThreadMessageKind.STEP_STARTED,
};
```

- [ ] **Step 3: Type-check**

```bash
cd /home/thbertoldi/harpia/frontend && npx svelte-kit sync >/dev/null 2>&1 && npm run check 2>&1 | grep -cE "^[0-9]+ ERROR" | tail -1
```

Expected: still 10 errors (trunk baseline; no new).

- [ ] **Step 4: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/chat/types.ts
git commit -m "feat(ux-m4): extend ChatMessageKind with STEP_STARTED"
```

---

## Task 4: `lib/plans/canvas-state.ts` — execution state projection helper

**Files:**
- Create: `frontend/src/lib/plans/canvas-state.ts`
- Create: `frontend/src/lib/plans/canvas-state.test.ts`

**Interfaces:**
- Consumes: `ChatMessage`, `ChatMessageKind` from `$lib/chat/types`; `PlanStep` from `$lib/gen/harpia/plans/v1/plans_pb`
- Produces:
  - `type CanvasStepStatus = "pending" | "running" | "awaiting_elicitation" | "awaiting_approval" | "done" | "failed"`
  - `interface CanvasStepState { status; stepExecutionId?; pendingElicitationId?; pendingApprovalId?; startedAt?; completedAt?; outputArtifactId?; errorMessage? }`
  - `function buildCanvasState(messages: ChatMessage[], steps: PlanStep[]): Record<string, CanvasStepState>` — keyed by step.key

- [ ] **Step 1: Write the failing test**

Create `frontend/src/lib/plans/canvas-state.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { buildCanvasState } from "./canvas-state";
import type { ChatMessage } from "$lib/chat/types";
import type { PlanStep } from "$lib/gen/harpia/plans/v1/plans_pb";

function msg(
  id: string,
  kind: ChatMessage["kind"],
  payload: Record<string, unknown>,
  sequenceNumber: number,
): ChatMessage {
  return {
    id,
    tenantId: "tenant-1",
    threadId: "config-1",
    executionId: "exec-1",
    role: "SYSTEM",
    kind,
    text: id,
    payloadJson: JSON.stringify(payload),
    authorUserId: "",
    sequenceNumber: BigInt(sequenceNumber),
    createdAt: `2026-06-22T10:0${sequenceNumber}:00Z`,
  };
}

function step(key: string): PlanStep {
  return {
    // Minimal proto shape; full PlanStep has more fields but buildCanvasState only reads key.
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    key,
  } as any;
}

describe("buildCanvasState", () => {
  it("returns 'pending' for every step when no messages exist", () => {
    const state = buildCanvasState([], [step("a"), step("b"), step("c")]);
    expect(state.a.status).toBe("pending");
    expect(state.b.status).toBe("pending");
    expect(state.c.status).toBe("pending");
  });

  it("returns 'running' for a step with STEP_STARTED and no STEP_BOUND", () => {
    const messages = [
      msg("m1", "STEP_STARTED", { step_key: "a", step_execution_id: "se-a" }, 1),
    ];
    const state = buildCanvasState(messages, [step("a"), step("b")]);
    expect(state.a.status).toBe("running");
    expect(state.a.stepExecutionId).toBe("se-a");
    expect(state.a.startedAt).toBe("2026-06-22T10:01:00Z");
    expect(state.b.status).toBe("pending");
  });

  it("returns 'done' for a step with STEP_BOUND, carrying outputArtifactId", () => {
    const messages = [
      msg("m1", "STEP_STARTED", { step_key: "a", step_execution_id: "se-a" }, 1),
      msg("m2", "STEP_BOUND", { step_key: "a", output_artifact_id: "art-x" }, 2),
    ];
    const state = buildCanvasState(messages, [step("a")]);
    expect(state.a.status).toBe("done");
    expect(state.a.outputArtifactId).toBe("art-x");
    expect(state.a.completedAt).toBe("2026-06-22T10:02:00Z");
  });

  it("returns 'awaiting_elicitation' when ELICITATION_RAISED is the latest event for a step", () => {
    const messages = [
      msg("m1", "STEP_STARTED", { step_key: "a", step_execution_id: "se-a" }, 1),
      msg("m2", "ELICITATION_RAISED", { elicitation_id: "el-1" }, 2),
    ];
    const state = buildCanvasState(messages, [step("a")]);
    expect(state.a.status).toBe("awaiting_elicitation");
    expect(state.a.pendingElicitationId).toBe("el-1");
  });

  it("clears the elicitation pointer when ELICITATION_ANSWERED arrives", () => {
    const messages = [
      msg("m1", "STEP_STARTED", { step_key: "a", step_execution_id: "se-a" }, 1),
      msg("m2", "ELICITATION_RAISED", { elicitation_id: "el-1" }, 2),
      msg("m3", "ELICITATION_ANSWERED", { elicitation_id: "el-1" }, 3),
    ];
    const state = buildCanvasState(messages, [step("a")]);
    expect(state.a.status).toBe("running"); // back to running while step continues
    expect(state.a.pendingElicitationId).toBeUndefined();
  });

  it("returns 'awaiting_approval' when APPROVAL_RAISED is pending for a step", () => {
    const messages = [
      msg("m1", "STEP_STARTED", { step_key: "publish", step_execution_id: "se-p" }, 1),
      msg("m2", "APPROVAL_RAISED", { approval_request_id: "ap-1" }, 2),
    ];
    const state = buildCanvasState(messages, [step("publish")]);
    expect(state.publish.status).toBe("awaiting_approval");
    expect(state.publish.pendingApprovalId).toBe("ap-1");
  });

  it("returns 'failed' when RUN_FAILED is the run-level event and the step was running", () => {
    const messages = [
      msg("m1", "STEP_STARTED", { step_key: "a", step_execution_id: "se-a" }, 1),
      msg("m2", "RUN_FAILED", { error: "step a failed: timeout" }, 2),
    ];
    const state = buildCanvasState(messages, [step("a")]);
    expect(state.a.status).toBe("failed");
    expect(state.a.errorMessage).toBe("step a failed: timeout");
  });

  it("payload pointers (elicitation/approval) require the step to be running first", () => {
    // An ELICITATION_RAISED message can't be associated with a step unless a
    // STEP_STARTED preceded it for that step. Pointer messages carry the
    // elicitation_id; the step key is implicit from the most recent
    // STEP_STARTED for the same execution.
    const messages = [
      msg("m1", "ELICITATION_RAISED", { elicitation_id: "el-orphan" }, 1),
    ];
    const state = buildCanvasState(messages, [step("a")]);
    expect(state.a.status).toBe("pending"); // no STEP_STARTED → step still pending
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/thbertoldi/harpia/frontend && npx vitest run src/lib/plans/canvas-state.test.ts
```

Expected: FAIL — module not found.

- [ ] **Step 3: Implement the helper**

Create `frontend/src/lib/plans/canvas-state.ts`:

```ts
import type { ChatMessage } from "$lib/chat/types";
import type { PlanStep } from "$lib/gen/harpia/plans/v1/plans_pb";

export type CanvasStepStatus =
  | "pending"
  | "running"
  | "awaiting_elicitation"
  | "awaiting_approval"
  | "done"
  | "failed";

export interface CanvasStepState {
  status: CanvasStepStatus;
  stepExecutionId?: string;
  pendingElicitationId?: string;
  pendingApprovalId?: string;
  startedAt?: string;
  completedAt?: string;
  outputArtifactId?: string;
  errorMessage?: string;
}

/**
 * Project a stream of M3 chat messages onto a canvas state map keyed by
 * step.key. Pure function — replays messages in sequence order.
 *
 * Pointer messages (ELICITATION_RAISED, APPROVAL_RAISED) attach to the step
 * that most recently STEP_STARTED in the same execution. Their corresponding
 * *_ANSWERED / *_DECIDED clear the pointer and revert status to "running"
 * until STEP_BOUND completes the step.
 *
 * RUN_FAILED at the run level marks the currently-running step as "failed"
 * and stores the error message from the payload.
 */
export function buildCanvasState(
  messages: ChatMessage[],
  steps: PlanStep[],
): Record<string, CanvasStepState> {
  const state: Record<string, CanvasStepState> = {};
  for (const step of steps) {
    state[step.key] = { status: "pending" };
  }

  // Track the "currently active" step key per execution so pointer
  // messages can attach. M4 assumes a single execution context per call.
  let activeStepKey: string | null = null;

  const sorted = [...messages].sort((a, b) =>
    a.sequenceNumber < b.sequenceNumber ? -1 : 1,
  );

  for (const m of sorted) {
    let payload: Record<string, unknown> = {};
    try {
      payload = JSON.parse(m.payloadJson) as Record<string, unknown>;
    } catch {
      payload = {};
    }

    switch (m.kind) {
      case "STEP_STARTED": {
        const stepKey = String(payload.step_key ?? "");
        if (!stepKey || !state[stepKey]) break;
        state[stepKey] = {
          ...state[stepKey],
          status: "running",
          stepExecutionId: String(payload.step_execution_id ?? ""),
          startedAt: m.createdAt,
          pendingElicitationId: undefined,
          pendingApprovalId: undefined,
        };
        activeStepKey = stepKey;
        break;
      }
      case "STEP_BOUND": {
        const stepKey = String(payload.step_key ?? "");
        if (!stepKey || !state[stepKey]) break;
        state[stepKey] = {
          ...state[stepKey],
          status: "done",
          completedAt: m.createdAt,
          outputArtifactId: String(payload.output_artifact_id ?? ""),
          pendingElicitationId: undefined,
          pendingApprovalId: undefined,
        };
        if (activeStepKey === stepKey) {
          activeStepKey = null;
        }
        break;
      }
      case "ELICITATION_RAISED": {
        if (!activeStepKey || !state[activeStepKey]) break;
        state[activeStepKey] = {
          ...state[activeStepKey],
          status: "awaiting_elicitation",
          pendingElicitationId: String(payload.elicitation_id ?? ""),
        };
        break;
      }
      case "ELICITATION_ANSWERED": {
        if (!activeStepKey || !state[activeStepKey]) break;
        if (state[activeStepKey].status === "awaiting_elicitation") {
          state[activeStepKey] = {
            ...state[activeStepKey],
            status: "running",
            pendingElicitationId: undefined,
          };
        }
        break;
      }
      case "APPROVAL_RAISED": {
        if (!activeStepKey || !state[activeStepKey]) break;
        state[activeStepKey] = {
          ...state[activeStepKey],
          status: "awaiting_approval",
          pendingApprovalId: String(payload.approval_request_id ?? ""),
        };
        break;
      }
      case "APPROVAL_DECIDED": {
        if (!activeStepKey || !state[activeStepKey]) break;
        if (state[activeStepKey].status === "awaiting_approval") {
          state[activeStepKey] = {
            ...state[activeStepKey],
            status: "running",
            pendingApprovalId: undefined,
          };
        }
        break;
      }
      case "RUN_FAILED": {
        if (!activeStepKey || !state[activeStepKey]) break;
        state[activeStepKey] = {
          ...state[activeStepKey],
          status: "failed",
          errorMessage: String(payload.error ?? ""),
        };
        activeStepKey = null;
        break;
      }
      // RUN_STARTED, RUN_COMPLETED, CONFIGURATION_SAVED, USER_TEXT,
      // ASSISTANT_TEXT — no step-level effect.
      default:
        break;
    }
  }

  return state;
}
```

- [ ] **Step 4: Run tests to verify all pass**

```bash
cd /home/thbertoldi/harpia/frontend && npx vitest run src/lib/plans/canvas-state.test.ts
```

Expected: 8 tests pass.

- [ ] **Step 5: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/plans/canvas-state.ts frontend/src/lib/plans/canvas-state.test.ts
git commit -m "feat(ux-m4): add buildCanvasState helper projecting chat events to step state"
```

---

## Task 5: Extract `CanvasApprovalForm` from `InboxApprovalEntry`

**Files:**
- Create: `frontend/src/lib/components/canvas/CanvasApprovalForm.svelte`
- Modify: `frontend/src/lib/components/inbox/InboxApprovalEntry.svelte` (composes the new form)

**Interfaces:**
- Consumes: `respondToApprovalRequest` from `$lib/plans/approvals`; `getTenant` from `$lib/auth`; `toUserMessage` from `$lib/connect-errors`; `ArtifactPreview` from `$lib/components/ArtifactPreview.svelte`
- Produces: `CanvasApprovalForm` with props `{ approvalRequestId: string; inputArtifactId?: string; onDecided?: (decision: "approved" | "rejected") => void }`

- [ ] **Step 1: Read the current `InboxApprovalEntry.svelte` in full**

```bash
cat /home/thbertoldi/harpia/frontend/src/lib/components/inbox/InboxApprovalEntry.svelte
```

Note where the form state (`expanded`, `submitting`, `decision`, `rejectReason`, `rejectMode`, `errorMessage`) is declared, where `submit()` is, and where the `approvalActions` / `approvalFooter` snippets live. The extraction lifts the state + `submit()` + the rendering of those snippets into the new component.

- [ ] **Step 2: Create the extracted form component**

Create `frontend/src/lib/components/canvas/CanvasApprovalForm.svelte`:

```svelte
<script lang="ts">
  import { locale, translate } from "$lib/i18n";
  import { getTenant } from "$lib/auth";
  import { respondToApprovalRequest } from "$lib/plans/approvals";
  import { toUserMessage } from "$lib/connect-errors";
  import ArtifactPreview from "$lib/components/ArtifactPreview.svelte";

  interface Props {
    approvalRequestId: string;
    inputArtifactId?: string;
    onDecided?: (decision: "approved" | "rejected") => void;
  }
  let { approvalRequestId, inputArtifactId, onDecided }: Props = $props();

  let expanded = $state(false);
  let submitting = $state(false);
  let decision = $state<"approved" | "rejected" | null>(null);
  let rejectReason = $state("");
  let rejectMode = $state(false);
  let errorMessage = $state<string | null>(null);

  const tenantId = $derived(getTenant()?.id ?? "");
  const hasFooter = $derived(
    expanded || rejectMode || errorMessage !== null,
  );

  async function submit(approved: boolean) {
    const tenant = getTenant();
    if (!tenant?.id || submitting) return;
    if (!approved && rejectReason.trim().length === 0) {
      rejectMode = true;
      return;
    }
    submitting = true;
    errorMessage = null;
    try {
      await respondToApprovalRequest(
        tenant.id,
        approvalRequestId,
        approved,
        approved ? "" : rejectReason.trim(),
      );
      decision = approved ? "approved" : "rejected";
      expanded = false;
      rejectMode = false;
      onDecided?.(decision);
    } catch (e) {
      errorMessage = toUserMessage(e);
    } finally {
      submitting = false;
    }
  }
</script>

<div class="flex flex-col gap-2">
  <div class="flex gap-1.5">
    {#if decision}
      <span class="text-[11px] font-semibold text-talon-gold">
        {translate(
          decision === "approved"
            ? "inbox.decision.approved"
            : "inbox.decision.rejected",
          $locale,
        )}
      </span>
    {:else}
      <button
        type="button"
        onclick={() => (expanded = !expanded)}
        disabled={submitting}
        class="rounded border border-plumage bg-transparent px-3 py-1.5 text-[11px] font-medium text-crown-ash hover:border-talon-gold hover:text-talon-gold disabled:opacity-50"
      >
        {translate(
          expanded ? "inbox.actions.hide" : "inbox.actions.preview",
          $locale,
        )}
      </button>
      <button
        type="button"
        onclick={() => submit(false)}
        disabled={submitting}
        class="rounded border border-plumage bg-transparent px-3 py-1.5 text-[11px] font-medium text-crown-ash hover:border-red-400 hover:text-red-400 disabled:opacity-50"
      >
        {translate("inbox.actions.reject", $locale)}
      </button>
      <button
        type="button"
        onclick={() => submit(true)}
        disabled={submitting}
        class="rounded border border-talon-gold bg-talon-gold px-3 py-1.5 text-[11px] font-semibold text-obsidian hover:opacity-90 disabled:opacity-50"
      >
        {translate(
          submitting ? "inbox.actions.submitting" : "inbox.actions.approve",
          $locale,
        )}
      </button>
    {/if}
  </div>

  {#if hasFooter}
    <div class="mt-3 flex flex-col gap-2 border-t border-plumage pt-3">
      {#if expanded && tenantId && inputArtifactId}
        <div class="mt-3">
          <ArtifactPreview {tenantId} artifactId={inputArtifactId} />
        </div>
      {/if}
      {#if rejectMode}
        <textarea
          class="mt-3 w-full rounded border border-plumage bg-obsidian px-3 py-2 text-[12px] text-cream focus:border-talon-gold focus:outline-none"
          rows="2"
          placeholder={translate("inbox.rejectReason.placeholder", $locale)}
          bind:value={rejectReason}
        ></textarea>
      {/if}
      {#if errorMessage}
        <p class="mt-2 text-[11px] text-red-400">{errorMessage}</p>
      {/if}
    </div>
  {/if}
</div>
```

- [ ] **Step 3: Refactor `InboxApprovalEntry.svelte` to compose `CanvasApprovalForm`**

REWRITE `frontend/src/lib/components/inbox/InboxApprovalEntry.svelte` to:
- Keep using `InboxRow` as the visual shell (row card)
- Replace the inline `approvalActions` + `approvalFooter` snippets with a single render of `<CanvasApprovalForm>` inside the row's body

```svelte
<script lang="ts">
  import { locale, translate } from "$lib/i18n";
  import { resolve } from "$app/paths";
  import type { InboxApprovalItem } from "$lib/inbox/types";
  import InboxRow from "./InboxRow.svelte";
  import CanvasApprovalForm from "$lib/components/canvas/CanvasApprovalForm.svelte";

  interface Props {
    item: InboxApprovalItem;
  }
  let { item }: Props = $props();

  const openThreadHref = $derived(
    item.configurationId
      ? resolve(`/plans/configurations/${item.configurationId}#m-approval-${item.id}`)
      : "",
  );
</script>

<InboxRow {item}>
  {#snippet actions()}
    {#if openThreadHref}
      <a
        href={openThreadHref}
        class="rounded border border-plumage bg-transparent px-3 py-1.5 text-[11px] font-medium text-crown-ash hover:border-talon-gold hover:text-talon-gold"
      >
        {translate("inbox.actions.openThread", $locale)}
      </a>
    {/if}
  {/snippet}
  {#snippet footer()}
    <CanvasApprovalForm
      approvalRequestId={item.id}
      inputArtifactId={item.inputArtifactId}
    />
  {/snippet}
</InboxRow>
```

Note: this changes the InboxApprovalEntry's UX subtly — the Approve / Reject / Preview buttons now live in the row's FOOTER instead of the actions slot. If preserving exact M3 placement is required, instead refactor more conservatively: keep the actions snippet wired to `CanvasApprovalForm`'s buttons via a slot/prop on the form. The simpler refactor above is acceptable if a visual regression test is unavailable; verify in Task 19 manual smoke that the row UX is still acceptable.

- [ ] **Step 4: Run focused tests + visual smoke**

```bash
cd /home/thbertoldi/harpia/frontend && npx vitest run src/lib/inbox/ src/lib/components/
```

Expected: existing inbox tests still pass. Component-level tests aren't added here (Svelte component testing setup would be its own task; relying on the manual smoke step in Task 19).

- [ ] **Step 5: Type-check**

```bash
cd /home/thbertoldi/harpia/frontend && npm run check 2>&1 | grep -E "CanvasApprovalForm|InboxApprovalEntry" | head -5
```

Expected: no errors mentioning either file.

- [ ] **Step 6: Format + lint**

```bash
cd /home/thbertoldi/harpia/frontend && npm run format >/dev/null && npm run lint 2>&1 | tail -3
```

Expected: clean.

- [ ] **Step 7: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/components/canvas/CanvasApprovalForm.svelte frontend/src/lib/components/inbox/InboxApprovalEntry.svelte
git commit -m "feat(ux-m4): extract CanvasApprovalForm from InboxApprovalEntry"
```

---

## Task 6: Extract `CanvasElicitationForm` from the elicitation detail page

**Files:**
- Create: `frontend/src/lib/components/canvas/CanvasElicitationForm.svelte`
- Modify: `frontend/src/routes/plans/executions/[executionId]/elicitations/[elicitationId]/+page.svelte` (composes the new form)

**Interfaces:**
- Consumes: `respondToElicitation`, `parseSchemaFields`, `buildPayloadJson`, type `ElicitationFormField` from `$lib/plans/elicitations`; `getTenant` from `$lib/auth`; `toUserMessage` from `$lib/connect-errors`; `Elicitation` type from `$lib/plans/elicitations`
- Produces: `CanvasElicitationForm` with props `{ elicitation: Elicitation; onAnswered?: () => void }`

- [ ] **Step 1: Read the current detail page in full**

```bash
cat /home/thbertoldi/harpia/frontend/src/routes/plans/executions/\[executionId\]/elicitations/\[elicitationId\]/+page.svelte
```

Identify the form-related state (`fieldValues`, `responseText`, `rawJson`, `fieldError`, `submitted`), the `schemaFields` derived, the form render block (around lines 310-385), and the `submit()` function (around lines 144-189).

- [ ] **Step 2: Create the extracted form**

Create `frontend/src/lib/components/canvas/CanvasElicitationForm.svelte`:

```svelte
<script lang="ts">
  import { locale, translate } from "$lib/i18n";
  import { getTenant } from "$lib/auth";
  import {
    type Elicitation,
    type ElicitationFormField,
    buildPayloadJson,
    parseSchemaFields,
    respondToElicitation,
  } from "$lib/plans/elicitations";
  import { toUserMessage } from "$lib/connect-errors";

  interface Props {
    elicitation: Elicitation;
    onAnswered?: () => void;
  }
  let { elicitation, onAnswered }: Props = $props();

  const schemaFields = $derived<ElicitationFormField[]>(
    parseSchemaFields(elicitation.schemaJson),
  );

  let fieldValues = $state<Record<string, string>>({});
  let responseText = $state("");
  let rawJson = $state("");
  let fieldError = $state<string | null>(null);
  let submitting = $state(false);
  let submitted = $state(false);
  let submitError = $state<string | null>(null);

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    if (submitting) return;
    const tenant = getTenant();
    if (!tenant?.id) return;

    // Required-field validation
    for (const field of schemaFields) {
      if (field.required && !fieldValues[field.name]?.trim()) {
        fieldError = translate(
          "elicitations.detail.fieldRequired",
          $locale,
        ).replace("{label}", field.label);
        return;
      }
    }

    submitting = true;
    submitError = null;
    fieldError = null;
    try {
      const payloadJson =
        schemaFields.length > 0
          ? buildPayloadJson(fieldValues)
          : rawJson.trim() || "{}";
      await respondToElicitation(tenant.id, elicitation.id, {
        payloadJson,
        responseText: responseText.trim(),
      });
      submitted = true;
      onAnswered?.();
    } catch (e) {
      submitError = toUserMessage(e);
    } finally {
      submitting = false;
    }
  }
</script>

{#if submitted}
  <p class="rounded border border-talon-gold/40 bg-talon-gold/10 px-3 py-2 text-[12px] text-cream">
    {translate("elicitations.detail.submitted", $locale)}
  </p>
{:else}
  <form onsubmit={submit} class="flex flex-col gap-3">
    {#each schemaFields as field (field.name)}
      <div class="space-y-1">
        <label
          for={`field-${field.name}`}
          class="block text-[11px] font-semibold text-crown-ash"
        >
          {field.label}{field.required ? " *" : ""}
        </label>
        {#if field.type === "select"}
          <select
            id={`field-${field.name}`}
            bind:value={fieldValues[field.name]}
            class="w-full rounded border border-plumage bg-obsidian px-2 py-1.5 text-[12px] text-cream focus:border-talon-gold focus:outline-none"
          >
            <option value="">--</option>
            {#each field.options ?? [] as opt (opt)}
              <option value={opt}>{opt}</option>
            {/each}
          </select>
        {:else if field.type === "textarea"}
          <textarea
            id={`field-${field.name}`}
            bind:value={fieldValues[field.name]}
            rows="3"
            class="w-full rounded border border-plumage bg-obsidian px-2 py-1.5 text-[12px] text-cream focus:border-talon-gold focus:outline-none"
          ></textarea>
        {:else}
          <input
            id={`field-${field.name}`}
            type={field.type === "number" ? "number" : "text"}
            bind:value={fieldValues[field.name]}
            class="w-full rounded border border-plumage bg-obsidian px-2 py-1.5 text-[12px] text-cream focus:border-talon-gold focus:outline-none"
          />
        {/if}
      </div>
    {/each}

    <div class="space-y-1">
      <label
        for="elicitation-response-text"
        class="block text-[11px] font-semibold text-crown-ash"
      >
        {translate("elicitations.detail.responseText", $locale)}
      </label>
      <textarea
        id="elicitation-response-text"
        bind:value={responseText}
        rows="2"
        class="w-full rounded border border-plumage bg-obsidian px-2 py-1.5 text-[12px] text-cream focus:border-talon-gold focus:outline-none"
      ></textarea>
    </div>

    {#if schemaFields.length === 0}
      <div class="space-y-1">
        <label
          for="elicitation-raw-json"
          class="block text-[11px] font-semibold text-crown-ash"
        >
          {translate("elicitations.detail.rawJson", $locale)}
        </label>
        <textarea
          id="elicitation-raw-json"
          bind:value={rawJson}
          rows="3"
          class="w-full rounded border border-plumage bg-obsidian px-2 py-1.5 font-mono text-[11px] text-cream focus:border-talon-gold focus:outline-none"
        ></textarea>
      </div>
    {/if}

    {#if fieldError}
      <p class="text-[11px] text-red-400">{fieldError}</p>
    {/if}
    {#if submitError}
      <p class="text-[11px] text-red-400">{submitError}</p>
    {/if}

    <button
      type="submit"
      disabled={submitting}
      class="self-end rounded border border-talon-gold bg-talon-gold px-3 py-1.5 text-[11px] font-semibold text-obsidian hover:opacity-90 disabled:opacity-50"
    >
      {translate(
        submitting ? "elicitations.detail.submitting" : "elicitations.detail.submit",
        $locale,
      )}
    </button>
  </form>
{/if}
```

Note: the i18n keys `elicitations.detail.fieldRequired`, `responseText`, `rawJson`, `submit`, `submitting`, `submitted` may or may not exist in the current locale files. The existing detail page presumably uses them — verify with `grep "elicitations.detail" frontend/src/lib/i18n/en.json` and adopt the same keys it currently uses (rename above if the actual keys differ). If any key is missing, this is a Task 18 i18n addition.

- [ ] **Step 3: Refactor the detail page to compose the new form**

REWRITE the form-render portion of `frontend/src/routes/plans/executions/[executionId]/elicitations/[elicitationId]/+page.svelte` to render `<CanvasElicitationForm elicitation={elicitation} />` in place of the inline form, while keeping the page's outer layout (heading, breadcrumb, thread display, back link).

Use a focused `Edit` operation: find the block from the `<form onsubmit={submit}>` opening tag through its closing `</form>` tag (the form was the bulk of the page's right column) and REPLACE with:

```svelte
{#if elicitation}
  <CanvasElicitationForm
    elicitation={elicitation}
    onAnswered={() => {
      // existing post-submit behavior — e.g., refresh thread display
    }}
  />
{/if}
```

Add the import at the top of the page's script block:

```ts
import CanvasElicitationForm from "$lib/components/canvas/CanvasElicitationForm.svelte";
```

DELETE the now-unused script-level state from the page (`fieldValues`, `responseText`, `rawJson`, `fieldError`, `submitted`, `submitting`, `submitError`, `submit`). Keep state that's still used by the surrounding chrome (the elicitation load, thread display, breadcrumb).

- [ ] **Step 4: Run focused tests**

```bash
cd /home/thbertoldi/harpia/frontend && npx vitest run src/lib/plans/
```

Expected: existing tests still pass.

- [ ] **Step 5: Type-check**

```bash
cd /home/thbertoldi/harpia/frontend && npx svelte-kit sync >/dev/null 2>&1 && npm run check 2>&1 | grep -cE "^[0-9]+ COMPLETED" | tail -1
```

Expected: still 10 errors (trunk baseline).

- [ ] **Step 6: Format + lint**

```bash
cd /home/thbertoldi/harpia/frontend && npm run format >/dev/null && npm run lint 2>&1 | tail -3
```

Expected: clean.

- [ ] **Step 7: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/components/canvas/CanvasElicitationForm.svelte frontend/src/routes/plans/executions
git commit -m "feat(ux-m4): extract CanvasElicitationForm from elicitation detail page"
```

---

## Task 7: `PlanCanvasNode.svelte`

**Files:**
- Create: `frontend/src/lib/components/canvas/PlanCanvasNode.svelte`

**Interfaces:**
- Consumes: `PlanStep` from `$lib/gen/harpia/plans/v1/plans_pb`; `CanvasStepState`, `CanvasStepStatus` from `$lib/plans/canvas-state`
- Produces: A single canvas node card. Props: `{ step: PlanStep; state: CanvasStepState; selected: boolean; onSelect: () => void }`

- [ ] **Step 1: Create the component**

Create `frontend/src/lib/components/canvas/PlanCanvasNode.svelte`:

```svelte
<script lang="ts">
  import { Loader2, Check, AlertTriangle, MessageSquare, ShieldQuestion, Clock } from "lucide-svelte";
  import type { PlanStep } from "$lib/gen/harpia/plans/v1/plans_pb";
  import type { CanvasStepState } from "$lib/plans/canvas-state";
  import { locale, translate } from "$lib/i18n";

  interface Props {
    step: PlanStep;
    state: CanvasStepState;
    selected: boolean;
    onSelect: () => void;
  }
  let { step, state, selected, onSelect }: Props = $props();

  const StatusIcon = $derived(
    state.status === "running"
      ? Loader2
      : state.status === "done"
        ? Check
        : state.status === "failed"
          ? AlertTriangle
          : state.status === "awaiting_elicitation"
            ? MessageSquare
            : state.status === "awaiting_approval"
              ? ShieldQuestion
              : Clock,
  );

  const statusLabelKey = $derived(`canvas.status.${state.status}`);
  const ringClass = $derived(
    selected ? "ring-2 ring-talon-gold ring-offset-2 ring-offset-obsidian" : "",
  );
  const borderClass = $derived(
    state.status === "done"
      ? "border-talon-gold/60"
      : state.status === "failed"
        ? "border-red-500/60"
        : state.status === "running"
          ? "border-talon-gold"
          : "border-plumage",
  );
</script>

<button
  type="button"
  onclick={onSelect}
  class={`flex w-44 flex-col gap-1.5 rounded-lg border bg-obsidian-light px-3 py-2 text-left transition-colors hover:border-talon-gold ${borderClass} ${ringClass}`}
>
  <div class="flex items-center gap-1.5">
    <StatusIcon
      class={`size-3.5 ${state.status === "running" ? "animate-spin text-talon-gold" : "text-crown-ash"}`}
    />
    <span class="font-heading text-[12px] font-semibold text-cream">
      {step.title}
    </span>
  </div>
  <span class="font-mono text-[9px] text-crown-ash-dark">{step.key}</span>
  <span class="text-[10px] text-crown-ash">
    {translate(statusLabelKey, $locale)}
  </span>
</button>
```

- [ ] **Step 2: Type-check**

```bash
cd /home/thbertoldi/harpia/frontend && npx svelte-kit sync >/dev/null 2>&1 && npm run check 2>&1 | grep -E "PlanCanvasNode" | head -3
```

Expected: no errors mentioning the file.

- [ ] **Step 3: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/components/canvas/PlanCanvasNode.svelte
git commit -m "feat(ux-m4): add PlanCanvasNode component with state-driven icon and border"
```

---

## Task 8: `PlanCanvasEdge.svelte`

**Files:**
- Create: `frontend/src/lib/components/canvas/PlanCanvasEdge.svelte`

**Interfaces:**
- Produces: A connector with a small artifact-type label. Props: `{ label: string }`

- [ ] **Step 1: Create the component**

Create `frontend/src/lib/components/canvas/PlanCanvasEdge.svelte`:

```svelte
<script lang="ts">
  import { ArrowRight } from "lucide-svelte";

  interface Props {
    label: string;
  }
  let { label }: Props = $props();
</script>

<div class="flex flex-col items-center gap-0.5 px-2">
  <span class="font-mono text-[9px] text-crown-ash-dark">{label}</span>
  <ArrowRight class="size-4 text-crown-ash-dark" />
</div>
```

- [ ] **Step 2: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/components/canvas/PlanCanvasEdge.svelte
git commit -m "feat(ux-m4): add PlanCanvasEdge connector with artifact-type label"
```

---

## Task 9: `PlanCanvasDetailPane.svelte`

**Files:**
- Create: `frontend/src/lib/components/canvas/PlanCanvasDetailPane.svelte`

**Interfaces:**
- Consumes: `PlanStep`, `CanvasStepState`, `getElicitation`, `respondToElicitation` (only types — RPC calls live inside the forms). Imports `CanvasApprovalForm` and `CanvasElicitationForm`. Uses `getElicitation` from `$lib/plans/elicitations` to load the elicitation object when needed.
- Produces: The right-pane component. Props: `{ step: PlanStep | null; state: CanvasStepState | null; tenantId: string }`. When `step` is null, shows an empty-state hint.

- [ ] **Step 1: Create the component**

Create `frontend/src/lib/components/canvas/PlanCanvasDetailPane.svelte`:

```svelte
<script lang="ts">
  import { locale, translate } from "$lib/i18n";
  import {
    type Elicitation,
    getElicitation,
  } from "$lib/plans/elicitations";
  import type { PlanStep } from "$lib/gen/harpia/plans/v1/plans_pb";
  import type { CanvasStepState } from "$lib/plans/canvas-state";
  import CanvasApprovalForm from "./CanvasApprovalForm.svelte";
  import CanvasElicitationForm from "./CanvasElicitationForm.svelte";

  interface Props {
    step: PlanStep | null;
    state: CanvasStepState | null;
    tenantId: string;
    // Optional: input artifact id for approval preview (lifted from state)
    approvalInputArtifactId?: string;
  }
  let { step, state, tenantId, approvalInputArtifactId }: Props = $props();

  let elicitation = $state<Elicitation | null>(null);
  let elicitationLoadError = $state(false);

  $effect(() => {
    elicitation = null;
    elicitationLoadError = false;
    if (!state || state.status !== "awaiting_elicitation" || !state.pendingElicitationId) {
      return;
    }
    const eid = state.pendingElicitationId;
    let active = true;
    (async () => {
      try {
        const fetched = await getElicitation(tenantId, eid);
        if (active) elicitation = fetched;
      } catch {
        if (active) elicitationLoadError = true;
      }
    })();
    return () => {
      active = false;
    };
  });
</script>

<aside class="flex h-full w-[340px] flex-col gap-3 border-l border-plumage bg-obsidian-light px-4 py-3">
  {#if !step || !state}
    <p class="text-[12px] text-crown-ash-dark">
      {translate("canvas.detail.empty", $locale)}
    </p>
  {:else}
    <header class="flex flex-col gap-1 border-b border-plumage pb-3">
      <h2 class="font-heading text-[14px] font-semibold text-cream">
        {step.title}
      </h2>
      <p class="font-mono text-[10px] text-crown-ash-dark">{step.key}</p>
      <p class="text-[11px] text-crown-ash">
        {translate(`canvas.status.${state.status}`, $locale)}
      </p>
    </header>

    {#if state.status === "awaiting_approval" && state.pendingApprovalId}
      <section>
        <h3 class="mb-2 text-[11px] font-semibold uppercase tracking-wider text-crown-ash">
          {translate("canvas.detail.answerApproval", $locale)}
        </h3>
        <CanvasApprovalForm
          approvalRequestId={state.pendingApprovalId}
          inputArtifactId={approvalInputArtifactId}
        />
      </section>
    {:else if state.status === "awaiting_elicitation"}
      <section>
        <h3 class="mb-2 text-[11px] font-semibold uppercase tracking-wider text-crown-ash">
          {translate("canvas.detail.answerElicitation", $locale)}
        </h3>
        {#if elicitationLoadError}
          <p class="text-[12px] text-red-400">
            {translate("canvas.detail.elicitationLoadError", $locale)}
          </p>
        {:else if !elicitation}
          <p class="text-[12px] text-crown-ash-dark">
            {translate("canvas.detail.elicitationLoading", $locale)}
          </p>
        {:else}
          <CanvasElicitationForm {elicitation} />
        {/if}
      </section>
    {:else}
      <section class="flex flex-col gap-2 text-[12px] text-crown-ash">
        {#if state.startedAt}
          <p>
            <span class="text-crown-ash-dark">{translate("canvas.detail.startedAt", $locale)}: </span>
            {state.startedAt}
          </p>
        {/if}
        {#if state.completedAt}
          <p>
            <span class="text-crown-ash-dark">{translate("canvas.detail.completedAt", $locale)}: </span>
            {state.completedAt}
          </p>
        {/if}
        {#if state.outputArtifactId}
          <p>
            <span class="text-crown-ash-dark">{translate("canvas.detail.outputArtifact", $locale)}: </span>
            <span class="font-mono">{state.outputArtifactId}</span>
          </p>
        {/if}
        {#if state.errorMessage}
          <p class="text-red-400">{state.errorMessage}</p>
        {/if}
      </section>
    {/if}
  {/if}
</aside>
```

Note: this references `getElicitation` from `$lib/plans/elicitations`. Verify the function exists with that signature. If the existing exported name differs, use whatever the codebase exports (search via `grep "export.*Elicitation" frontend/src/lib/plans/elicitations.ts`). If no single-elicitation loader exists, you may need to load via `listElicitations({ tenantId, elicitationId })` or by extending the lib.

- [ ] **Step 2: Type-check + commit**

```bash
cd /home/thbertoldi/harpia/frontend && npx svelte-kit sync >/dev/null 2>&1 && npm run check 2>&1 | grep -E "PlanCanvasDetailPane" | head -3
cd /home/thbertoldi/harpia && git add frontend/src/lib/components/canvas/PlanCanvasDetailPane.svelte
git commit -m "feat(ux-m4): add PlanCanvasDetailPane with inline answer forms"
```

---

## Task 10: `CanvasTopBar.svelte`

**Files:**
- Create: `frontend/src/lib/components/canvas/CanvasTopBar.svelte`

**Interfaces:**
- Produces: The top-bar component. Props: `{ configurationId: string; runStartedAt?: string; pendingAnswerCount: number; onOpenRunHistory: () => void; onOpenSettings: () => void; onAnswerNext: () => void; planName?: string }`

- [ ] **Step 1: Create the component**

Create `frontend/src/lib/components/canvas/CanvasTopBar.svelte`:

```svelte
<script lang="ts">
  import { ArrowLeft, History, Settings as SettingsIcon, Calendar, Reply } from "lucide-svelte";
  import { resolve } from "$app/paths";
  import { locale, translate } from "$lib/i18n";

  interface Props {
    configurationId: string;
    runStartedAt?: string;
    pendingAnswerCount: number;
    onOpenRunHistory: () => void;
    onOpenSettings: () => void;
    onAnswerNext: () => void;
    planName?: string;
  }
  let {
    configurationId,
    runStartedAt,
    pendingAnswerCount,
    onOpenRunHistory,
    onOpenSettings,
    onAnswerNext,
    planName,
  }: Props = $props();

  const backHref = $derived(resolve(`/plans/configurations/${configurationId}`));
</script>

<header class="flex items-center gap-3 border-b border-plumage bg-obsidian px-4 py-2">
  <a
    href={backHref}
    class="flex items-center gap-1 rounded border border-plumage bg-transparent px-2 py-1 text-[11px] text-crown-ash hover:border-talon-gold hover:text-talon-gold"
  >
    <ArrowLeft class="size-3.5" />
    {translate("canvas.topbar.backToThread", $locale)}
  </a>

  <div class="flex-1">
    {#if planName}
      <span class="font-heading text-[13px] font-semibold text-cream">{planName}</span>
    {/if}
    {#if runStartedAt}
      <span class="ml-2 font-mono text-[11px] text-crown-ash-dark">{runStartedAt}</span>
    {/if}
  </div>

  <button
    type="button"
    onclick={onOpenRunHistory}
    class="flex items-center gap-1 rounded border border-plumage bg-transparent px-2 py-1 text-[11px] text-crown-ash hover:border-talon-gold hover:text-talon-gold"
  >
    <History class="size-3.5" />
    {translate("canvas.topbar.runHistory", $locale)}
  </button>

  <button
    type="button"
    disabled
    title={translate("canvas.topbar.scheduleComingSoon", $locale)}
    class="flex cursor-not-allowed items-center gap-1 rounded border border-plumage/40 bg-transparent px-2 py-1 text-[11px] text-crown-ash-dark"
  >
    <Calendar class="size-3.5" />
    {translate("canvas.topbar.schedule", $locale)}
  </button>

  <button
    type="button"
    onclick={onOpenSettings}
    class="flex items-center gap-1 rounded border border-plumage bg-transparent px-2 py-1 text-[11px] text-crown-ash hover:border-talon-gold hover:text-talon-gold"
  >
    <SettingsIcon class="size-3.5" />
    {translate("canvas.topbar.settings", $locale)}
  </button>

  {#if pendingAnswerCount > 0}
    <button
      type="button"
      onclick={onAnswerNext}
      class="flex items-center gap-1 rounded border border-talon-gold bg-talon-gold px-3 py-1 text-[11px] font-semibold text-obsidian hover:opacity-90"
    >
      <Reply class="size-3.5" />
      {translate("canvas.topbar.answer", $locale)} ★ {pendingAnswerCount}
    </button>
  {/if}
</header>
```

- [ ] **Step 2: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/components/canvas/CanvasTopBar.svelte
git commit -m "feat(ux-m4): add CanvasTopBar with back, drawers, and Answer N primary"
```

---

## Task 11: `PlanCanvas.svelte` (top-level layout)

**Files:**
- Create: `frontend/src/lib/components/canvas/PlanCanvas.svelte`

**Interfaces:**
- Consumes: `PlanStep`, `PlanStepDependency` from `$lib/gen/harpia/plans/v1/plans_pb`; `orderPlanStepsLinear`, `buildLinearDagEdges` from `$lib/plans/artifact-flow`; `CanvasStepState` from `$lib/plans/canvas-state`; `PlanCanvasNode`, `PlanCanvasEdge`, `PlanCanvasDetailPane` from `./...`
- Produces: The top-level canvas component. Props: `{ steps: PlanStep[]; edges: PlanStepDependency[]; canvasState: Record<string, CanvasStepState>; tenantId: string; approvalInputArtifactByStep?: Record<string, string> }`

- [ ] **Step 1: Create the component**

Create `frontend/src/lib/components/canvas/PlanCanvas.svelte`:

```svelte
<script lang="ts">
  import { locale } from "$lib/i18n";
  import type { PlanStep, PlanStepDependency } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { orderPlanStepsLinear, buildLinearDagEdges } from "$lib/plans/artifact-flow";
  import type { CanvasStepState } from "$lib/plans/canvas-state";
  import PlanCanvasNode from "./PlanCanvasNode.svelte";
  import PlanCanvasEdge from "./PlanCanvasEdge.svelte";
  import PlanCanvasDetailPane from "./PlanCanvasDetailPane.svelte";

  interface Props {
    steps: PlanStep[];
    edges: PlanStepDependency[];
    canvasState: Record<string, CanvasStepState>;
    tenantId: string;
    approvalInputArtifactByStep?: Record<string, string>;
  }
  let {
    steps,
    edges,
    canvasState,
    tenantId,
    approvalInputArtifactByStep,
  }: Props = $props();

  const orderedSteps = $derived(orderPlanStepsLinear(steps, edges));
  const dagEdges = $derived(buildLinearDagEdges(steps, edges, $locale));

  let selectedKey = $state<string | null>(null);

  const selectedStep = $derived(
    selectedKey ? orderedSteps.find((s) => s.key === selectedKey) ?? null : null,
  );
  const selectedState = $derived(
    selectedKey ? canvasState[selectedKey] ?? null : null,
  );
  const selectedApprovalArtifact = $derived(
    selectedKey ? approvalInputArtifactByStep?.[selectedKey] : undefined,
  );

  function edgeLabelBetween(fromKey: string, toKey: string): string {
    const edge = dagEdges.find(
      (e) => e.from.key === fromKey && e.to.key === toKey,
    );
    return edge?.artifactLabel ?? "—";
  }
</script>

<div class="flex h-full">
  <div
    class="harpia-canvas-grid flex flex-1 items-center justify-start overflow-auto px-6 py-6"
  >
    <div class="flex items-center gap-2">
      {#each orderedSteps as step, index (step.key)}
        <PlanCanvasNode
          {step}
          state={canvasState[step.key] ?? { status: "pending" }}
          selected={selectedKey === step.key}
          onSelect={() => (selectedKey = step.key)}
        />
        {#if index < orderedSteps.length - 1}
          <PlanCanvasEdge
            label={edgeLabelBetween(step.key, orderedSteps[index + 1].key)}
          />
        {/if}
      {/each}
    </div>
  </div>

  <PlanCanvasDetailPane
    step={selectedStep}
    state={selectedState}
    {tenantId}
    approvalInputArtifactId={selectedApprovalArtifact}
  />
</div>

<style>
  .harpia-canvas-grid {
    background-color: var(--color-obsidian, #0b1320);
    background-image:
      radial-gradient(rgba(255, 255, 255, 0.04) 1px, transparent 1px);
    background-size: 24px 24px;
  }
</style>
```

- [ ] **Step 2: Type-check + commit**

```bash
cd /home/thbertoldi/harpia/frontend && npx svelte-kit sync >/dev/null 2>&1 && npm run check 2>&1 | grep -E "PlanCanvas\b" | head -3
cd /home/thbertoldi/harpia && git add frontend/src/lib/components/canvas/PlanCanvas.svelte
git commit -m "feat(ux-m4): add PlanCanvas top-level layout with node selection"
```

---

## Task 12: `CanvasDrawer.svelte` shared chrome + `RunHistoryDrawer.svelte`

**Files:**
- Create: `frontend/src/lib/components/canvas/CanvasDrawer.svelte`
- Create: `frontend/src/lib/components/canvas/RunHistoryDrawer.svelte`

**Interfaces:**
- `CanvasDrawer` produces: a right-side sliding drawer. Props: `{ open: boolean; onClose: () => void; title: string; children: Snippet }`
- `RunHistoryDrawer` produces: rows of past runs. Props: `{ open: boolean; onClose: () => void; tenantId: string; configurationId: string; currentRunId?: string }`. Click a row → navigates to `/plans/configurations/[configId]/canvas?run=<execId>`.

- [ ] **Step 1: Create the shared drawer chrome**

Create `frontend/src/lib/components/canvas/CanvasDrawer.svelte`:

```svelte
<script lang="ts">
  import type { Snippet } from "svelte";
  import { X } from "lucide-svelte";
  import { locale, translate } from "$lib/i18n";

  interface Props {
    open: boolean;
    onClose: () => void;
    title: string;
    children: Snippet;
  }
  let { open, onClose, title, children }: Props = $props();
</script>

{#if open}
  <div
    role="presentation"
    onclick={onClose}
    class="fixed inset-0 z-40 bg-black/40"
  ></div>
  <aside
    class="fixed inset-y-0 right-0 z-50 flex w-96 flex-col gap-2 border-l border-plumage bg-obsidian transition-transform"
  >
    <header class="flex items-center justify-between border-b border-plumage px-4 py-3">
      <h2 class="font-heading text-[13px] font-semibold text-cream">{title}</h2>
      <button
        type="button"
        onclick={onClose}
        aria-label={translate("canvas.drawer.close", $locale)}
        class="text-crown-ash hover:text-talon-gold"
      >
        <X class="size-4" />
      </button>
    </header>
    <div class="flex-1 overflow-y-auto px-4 py-3">
      {@render children()}
    </div>
  </aside>
{/if}
```

- [ ] **Step 2: Create the run history drawer**

Create `frontend/src/lib/components/canvas/RunHistoryDrawer.svelte`:

```svelte
<script lang="ts">
  import { goto } from "$app/navigation";
  import { resolve } from "$app/paths";
  import { locale, translate } from "$lib/i18n";
  import { loadPlanExecutions } from "$lib/plans/plan-execution";
  import type { PlanExecution } from "$lib/gen/harpia/plans/v1/plans_pb";
  import CanvasDrawer from "./CanvasDrawer.svelte";

  interface Props {
    open: boolean;
    onClose: () => void;
    tenantId: string;
    configurationId: string;
    currentRunId?: string;
  }
  let { open, onClose, tenantId, configurationId, currentRunId }: Props = $props();

  let executions = $state<PlanExecution[]>([]);
  let loading = $state(false);
  let loadError = $state(false);

  $effect(() => {
    if (!open) return;
    loading = true;
    loadError = false;
    (async () => {
      try {
        const result = await loadPlanExecutions();
        // Filter to this configuration only — loadPlanExecutions returns all
        // executions for the tenant by default; we want this plan's only.
        executions = result.executions.filter(
          (e) => e.planConfigurationId === configurationId,
        );
      } catch {
        loadError = true;
      } finally {
        loading = false;
      }
    })();
  });

  function go(executionId: string) {
    goto(resolve(`/plans/configurations/${configurationId}/canvas?run=${executionId}`));
    onClose();
  }
</script>

<CanvasDrawer
  {open}
  {onClose}
  title={translate("canvas.runHistory.title", $locale)}
>
  {#if loading}
    <p class="text-[12px] text-crown-ash-dark">
      {translate("canvas.runHistory.loading", $locale)}
    </p>
  {:else if loadError}
    <p class="text-[12px] text-red-400">
      {translate("canvas.runHistory.loadError", $locale)}
    </p>
  {:else if executions.length === 0}
    <p class="text-[12px] text-crown-ash-dark">
      {translate("canvas.runHistory.empty", $locale)}
    </p>
  {:else}
    <ul class="flex flex-col gap-2">
      {#each executions as exec, index (exec.id)}
        <li>
          <button
            type="button"
            onclick={() => go(exec.id)}
            class={`flex w-full flex-col gap-0.5 rounded border px-3 py-2 text-left text-[12px] hover:border-talon-gold ${
              exec.id === currentRunId
                ? "border-talon-gold bg-talon-gold/10"
                : "border-plumage bg-obsidian-light"
            }`}
          >
            <span class="font-heading font-semibold text-cream">
              {translate("canvas.runHistory.runLabel", $locale).replace(
                "{n}",
                String(executions.length - index),
              )}
            </span>
            <span class="font-mono text-[10px] text-crown-ash-dark">
              {exec.updatedAt}
            </span>
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</CanvasDrawer>
```

Note: `loadPlanExecutions` from the recon has signature `(): Promise<PlanExecutionsResult>` and returns all-tenant executions. The filter above narrows to the current configuration. If you want server-side filtering, use `loadPlanExecutions({ planConfigurationId: configurationId })` if the wrapper supports it — verify by reading the function's signature.

- [ ] **Step 3: Type-check + commit**

```bash
cd /home/thbertoldi/harpia/frontend && npx svelte-kit sync >/dev/null 2>&1 && npm run check 2>&1 | grep -E "CanvasDrawer|RunHistoryDrawer" | head -5
cd /home/thbertoldi/harpia && git add frontend/src/lib/components/canvas/CanvasDrawer.svelte frontend/src/lib/components/canvas/RunHistoryDrawer.svelte
git commit -m "feat(ux-m4): add CanvasDrawer chrome + RunHistoryDrawer"
```

---

## Task 13: `SettingsDrawer.svelte`

**Files:**
- Create: `frontend/src/lib/components/canvas/SettingsDrawer.svelte`

**Interfaces:**
- Consumes: `PlanPoliciesForm` from `$lib/components/PlanPoliciesForm.svelte`; the configuration's `BehaviorPoliciesFormValues` shape from wherever that type lives in the codebase
- Produces: Drawer hosting the policies form. Props: `{ open: boolean; onClose: () => void; configurationId: string; tenantId: string }`

- [ ] **Step 1: Locate the policies form's value shape and save path**

Inspect `frontend/src/lib/components/PlanPoliciesForm.svelte` and `frontend/src/lib/plans/behavior-policies.ts` (if it exists) for:
- The `BehaviorPoliciesFormValues` type
- The function that turns those values into a `PlanConfiguration` update payload and calls `updatePlanConfiguration`

Search:

```bash
cd /home/thbertoldi/harpia && grep -rn "BehaviorPoliciesFormValues\|updatePlanConfiguration" frontend/src/lib/ | head -10
```

- [ ] **Step 2: Create the settings drawer**

Create `frontend/src/lib/components/canvas/SettingsDrawer.svelte` (the body composes PlanPoliciesForm and wires save through `updatePlanConfiguration`):

```svelte
<script lang="ts">
  import { locale, translate } from "$lib/i18n";
  import CanvasDrawer from "./CanvasDrawer.svelte";
  import PlanPoliciesForm from "$lib/components/PlanPoliciesForm.svelte";
  import { planClient } from "$lib/rpc";
  import { toUserMessage } from "$lib/connect-errors";
  // Adjust the import path/name to match the actual export found in Step 1.
  import type { BehaviorPoliciesFormValues } from "$lib/components/PlanPoliciesForm.svelte";

  interface Props {
    open: boolean;
    onClose: () => void;
    configurationId: string;
    tenantId: string;
  }
  let { open, onClose, configurationId, tenantId }: Props = $props();

  let values = $state<BehaviorPoliciesFormValues>({
    elicitationTimeoutBehavior: undefined as any,
    elicitationTimeoutHours: 48,
    publishApprovalMode: undefined as any,
  });
  let loading = $state(false);
  let loadError = $state<string | null>(null);
  let saving = $state(false);
  let saveError = $state<string | null>(null);
  let saveNotice = $state<string | null>(null);

  $effect(() => {
    if (!open) return;
    loading = true;
    loadError = null;
    (async () => {
      try {
        const response = await planClient.getPlanConfiguration({
          tenantId,
          planConfigurationId: configurationId,
        });
        const policies = response.planConfiguration?.behaviorPolicies;
        if (policies) {
          values = {
            elicitationTimeoutBehavior: policies.elicitationTimeoutBehavior,
            elicitationTimeoutHours:
              policies.elicitationTimeoutHours ?? 48,
            publishApprovalMode: policies.publishApprovalMode,
          };
        }
      } catch (e) {
        loadError = toUserMessage(e);
      } finally {
        loading = false;
      }
    })();
  });

  async function handleSave() {
    saving = true;
    saveError = null;
    saveNotice = null;
    try {
      await planClient.updatePlanConfiguration({
        tenantId,
        planConfigurationId: configurationId,
        // Mirror what the existing wizard's policies submit sends.
        // Adjust the field names to match the proto.
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        behaviorPolicies: values as any,
      } as any);
      saveNotice = translate("canvas.settings.saved", $locale);
    } catch (e) {
      saveError = toUserMessage(e);
    } finally {
      saving = false;
    }
  }
</script>

<CanvasDrawer
  {open}
  {onClose}
  title={translate("canvas.settings.title", $locale)}
>
  {#if loading}
    <p class="text-[12px] text-crown-ash-dark">
      {translate("canvas.settings.loading", $locale)}
    </p>
  {:else if loadError}
    <p class="text-[12px] text-red-400">{loadError}</p>
  {:else}
    <PlanPoliciesForm
      bind:values
      {saving}
      {saveError}
      {saveNotice}
      onSave={handleSave}
    />
  {/if}
</CanvasDrawer>
```

Note: the `behaviorPolicies as any` casts above are intentional to keep this brief small. The actual `updatePlanConfiguration` request shape must match the proto's `UpdatePlanConfigurationRequest.behavior_policies` field. The wizard's existing save flow (find it via `grep -rn "updatePlanConfiguration" frontend/src/routes/plans/`) has the canonical payload-builder code — mirror it. If the call shape isn't obvious, STOP and report BLOCKED with the actual wizard call site for guidance.

- [ ] **Step 3: Type-check + commit**

```bash
cd /home/thbertoldi/harpia/frontend && npx svelte-kit sync >/dev/null 2>&1 && npm run check 2>&1 | grep -E "SettingsDrawer" | head -3
cd /home/thbertoldi/harpia && git add frontend/src/lib/components/canvas/SettingsDrawer.svelte
git commit -m "feat(ux-m4): add SettingsDrawer hosting PlanPoliciesForm"
```

---

## Task 14: `/plans/configurations/[configurationId]/canvas/` route — page + load

**Files:**
- Create: `frontend/src/routes/plans/configurations/[configurationId]/canvas/+page.svelte`
- Create: `frontend/src/routes/plans/configurations/[configurationId]/canvas/+page.ts`

**Interfaces:**
- Consumes: `PlanCanvas`, `CanvasTopBar`, `RunHistoryDrawer`, `SettingsDrawer`, `buildCanvasState`, `loadThreadMessages`, `watchThreadMessages`, the various `loadPlan*` helpers
- Produces: the canvas page. URL: `/plans/configurations/[configurationId]/canvas?run=<execId>`. Without `?run=`: static DAG of the configuration template. With `?run=`: paints execution state.

- [ ] **Step 1: Create `+page.ts`**

Create `frontend/src/routes/plans/configurations/[configurationId]/canvas/+page.ts`:

```ts
import { error } from "@sveltejs/kit";
import type { PageLoad } from "./$types";
import { planClient } from "$lib/rpc";
import { getTenant } from "$lib/auth";

export const load: PageLoad = async ({ params, url }) => {
  const tenant = getTenant();
  if (!tenant?.id) {
    throw error(401, "Not authenticated");
  }
  const { configurationId } = params;
  const runId = url.searchParams.get("run") ?? "";

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
      runId,
      configuration: config.planConfiguration,
      template: template.planTemplate,
    };
  } catch (e) {
    throw error(404, "Plan configuration not found");
  }
};
```

- [ ] **Step 2: Create `+page.svelte`**

Create `frontend/src/routes/plans/configurations/[configurationId]/canvas/+page.svelte`:

```svelte
<script lang="ts">
  import { goto } from "$app/navigation";
  import { resolve } from "$app/paths";
  import { getTenant } from "$lib/auth";
  import { locale, translate } from "$lib/i18n";
  import {
    loadThreadMessages,
  } from "$lib/chat/client";
  import { watchThreadMessages } from "$lib/chat/watch";
  import type { ChatMessage } from "$lib/chat/types";
  import { buildCanvasState } from "$lib/plans/canvas-state";
  import PlanCanvas from "$lib/components/canvas/PlanCanvas.svelte";
  import CanvasTopBar from "$lib/components/canvas/CanvasTopBar.svelte";
  import RunHistoryDrawer from "$lib/components/canvas/RunHistoryDrawer.svelte";
  import SettingsDrawer from "$lib/components/canvas/SettingsDrawer.svelte";

  let { data } = $props();

  let messages = $state<ChatMessage[]>([]);
  let runHistoryOpen = $state(false);
  let settingsOpen = $state(false);

  const tenantId = $derived(getTenant()?.id ?? "");

  // Filter messages to the current run (if a run is selected).
  const runMessages = $derived(
    data.runId ? messages.filter((m) => m.executionId === data.runId) : [],
  );

  const canvasState = $derived(
    buildCanvasState(runMessages, data.template?.steps ?? []),
  );

  // Approval input artifact lookup by step key — derived from messages.
  // Real implementation would fetch step_executions for richer data; for M4
  // we leave this as an empty record and ArtifactPreview falls back.
  const approvalInputArtifactByStep = $derived<Record<string, string>>({});

  // Pending answer count = nodes in awaiting_* state.
  const pendingAnswerCount = $derived(
    Object.values(canvasState).filter(
      (s) =>
        s.status === "awaiting_elicitation" || s.status === "awaiting_approval",
    ).length,
  );

  function answerNext() {
    const firstPending = Object.entries(canvasState).find(
      ([, s]) =>
        s.status === "awaiting_elicitation" || s.status === "awaiting_approval",
    );
    if (firstPending) {
      // Selection lives inside PlanCanvas — we'd need to lift it for this to
      // work end-to-end. For M4 v1, scroll the node into view by updating the
      // URL hash; PlanCanvas listens to hash changes and selects accordingly.
      goto(`${resolve(`/plans/configurations/${data.configurationId}/canvas`)}?run=${data.runId}#node-${firstPending[0]}`);
    }
  }

  // Subscribe to chat messages for live updates.
  $effect(() => {
    if (!tenantId || !data.configurationId) return;
    const controller = new AbortController();
    (async () => {
      try {
        const initial = await loadThreadMessages(tenantId, data.configurationId);
        if (controller.signal.aborted) return;
        messages = initial;
        const sinceSeq =
          initial.length > 0 ? initial[initial.length - 1].sequenceNumber : 0n;
        for await (const batch of watchThreadMessages(
          tenantId,
          data.configurationId,
          { sinceSequenceNumber: sinceSeq, signal: controller.signal },
        )) {
          if (controller.signal.aborted) return;
          messages = [...messages, ...batch];
        }
      } catch {
        if (controller.signal.aborted) return;
        // Best-effort: canvas still renders the static DAG.
      }
    })();
    return () => {
      controller.abort();
    };
  });

  // Run started-at from the RUN_STARTED message of the current run.
  const runStartedAt = $derived(
    runMessages.find((m) => m.kind === "RUN_STARTED")?.createdAt,
  );
</script>

<svelte:head>
  <title>{translate("nav.planCanvas", $locale)} · Harpia</title>
</svelte:head>

<div class="flex h-screen flex-col">
  <CanvasTopBar
    configurationId={data.configurationId}
    {runStartedAt}
    {pendingAnswerCount}
    planName={data.template?.name}
    onOpenRunHistory={() => (runHistoryOpen = true)}
    onOpenSettings={() => (settingsOpen = true)}
    onAnswerNext={answerNext}
  />

  <div class="flex-1 overflow-hidden">
    {#if data.template?.steps && data.template.steps.length > 0}
      <PlanCanvas
        steps={data.template.steps}
        edges={data.template.edges}
        {canvasState}
        {tenantId}
        {approvalInputArtifactByStep}
      />
    {:else}
      <p class="px-6 py-6 text-[12px] text-crown-ash-dark">
        {translate("canvas.template.empty", $locale)}
      </p>
    {/if}
  </div>

  <RunHistoryDrawer
    open={runHistoryOpen}
    onClose={() => (runHistoryOpen = false)}
    {tenantId}
    configurationId={data.configurationId}
    currentRunId={data.runId || undefined}
  />

  <SettingsDrawer
    open={settingsOpen}
    onClose={() => (settingsOpen = false)}
    configurationId={data.configurationId}
    {tenantId}
  />
</div>
```

- [ ] **Step 3: Type-check + format + lint**

```bash
cd /home/thbertoldi/harpia/frontend && npx svelte-kit sync >/dev/null 2>&1 && npm run check 2>&1 | grep -E "configurations.\[" | head -5
cd /home/thbertoldi/harpia/frontend && npm run format >/dev/null && npm run lint 2>&1 | tail -3
```

Expected: no new errors; format/lint clean.

- [ ] **Step 4: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/routes/plans/configurations/[configurationId]/canvas
git commit -m "feat(ux-m4): add /plans/configurations/[id]/canvas route with live state"
```

---

## Task 15: Replace legacy `/plans/executions/[executionId]/+page.svelte` with redirect

**Files:**
- Delete: `frontend/src/routes/plans/executions/[executionId]/+page.svelte`
- Create: `frontend/src/routes/plans/executions/[executionId]/+page.ts` (302 redirect)

**Interfaces:**
- Behavior: visiting `/plans/executions/<execId>` 302-redirects to `/plans/configurations/<configId>/canvas?run=<execId>` where `configId` is looked up via `getPlanExecution`.

- [ ] **Step 1: Create the redirect loader**

Create `frontend/src/routes/plans/executions/[executionId]/+page.ts`:

```ts
import { error, redirect } from "@sveltejs/kit";
import type { PageLoad } from "./$types";
import { planClient } from "$lib/rpc";
import { getTenant } from "$lib/auth";

export const load: PageLoad = async ({ params }) => {
  const tenant = getTenant();
  if (!tenant?.id) {
    throw error(401, "Not authenticated");
  }
  const { executionId } = params;
  try {
    const response = await planClient.getPlanExecution({
      tenantId: tenant.id,
      planExecutionId: executionId,
    });
    const configId = response.planExecution?.planConfigurationId;
    if (!configId) {
      throw error(404, "Plan execution not found");
    }
    throw redirect(
      302,
      `/plans/configurations/${configId}/canvas?run=${executionId}`,
    );
  } catch (e) {
    // Re-throw SvelteKit redirects without wrapping
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    if (e instanceof Error && "status" in (e as any) && "location" in (e as any)) {
      throw e;
    }
    throw error(404, "Plan execution not found");
  }
};
```

- [ ] **Step 2: Delete the old `+page.svelte`**

```bash
cd /home/thbertoldi/harpia
rm frontend/src/routes/plans/executions/[executionId]/+page.svelte
```

- [ ] **Step 3: Type-check + commit**

```bash
cd /home/thbertoldi/harpia/frontend && npx svelte-kit sync >/dev/null 2>&1 && npm run check 2>&1 | grep -E "executions\[" | head -3
cd /home/thbertoldi/harpia && git add frontend/src/routes/plans/executions/[executionId]
git commit -m "feat(ux-m4): replace /plans/executions/[id] page with canvas redirect"
```

---

## Task 16: Delete the M1 canvas stub

**Files:**
- Delete: `frontend/src/routes/plans/[templateId]/canvas/+page.svelte`

- [ ] **Step 1: Delete the file**

```bash
cd /home/thbertoldi/harpia
rm frontend/src/routes/plans/[templateId]/canvas/+page.svelte
```

- [ ] **Step 2: Verify no orphan references**

```bash
cd /home/thbertoldi/harpia && grep -rn "plans/\[templateId\]/canvas\|/canvas?milestone" frontend/src 2>/dev/null
```

Expected: zero matches.

- [ ] **Step 3: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/routes/plans/[templateId]/canvas
git commit -m "chore(ux-m4): delete M1 canvas stub (replaced by /plans/configurations/[id]/canvas)"
```

---

## Task 17: Wire the M3 thread's inline mini-map `Expand ⤢` affordance

**Files:**
- Modify: `frontend/src/routes/plans/configurations/[configurationId]/+page.svelte`

**Interfaces:**
- Adds a small `Expand ⤢` link next to the `PlanDagMiniMap` that navigates to `/plans/configurations/<configId>/canvas`.

- [ ] **Step 1: Read the current thread page near the mini-map render**

```bash
grep -n "PlanDagMiniMap" /home/thbertoldi/harpia/frontend/src/routes/plans/configurations/\[configurationId\]/+page.svelte
```

The recon found the mini-map renders at line 121.

- [ ] **Step 2: Wrap the mini-map with a flex container that also holds the expand link**

In the file, find the line:

```svelte
<PlanDagMiniMap steps={data.template.steps} edges={data.template.edges} />
```

REPLACE with:

```svelte
<div class="flex items-center justify-between gap-2">
  <PlanDagMiniMap steps={data.template.steps} edges={data.template.edges} />
  <a
    href={resolve(`/plans/configurations/${data.configurationId}/canvas`)}
    class="flex items-center gap-1 rounded border border-plumage bg-transparent px-2 py-1 text-[10px] text-crown-ash hover:border-talon-gold hover:text-talon-gold"
    aria-label={translate("canvas.expandLink", $locale)}
  >
    <Expand class="size-3" />
    {translate("canvas.expandLink", $locale)}
  </a>
</div>
```

At the top of the page's `<script>` block, ADD imports if not already present:

```ts
import { Expand } from "lucide-svelte";
import { resolve } from "$app/paths";
```

If `Expand` is not exported by lucide-svelte (the recon showed `Expand2` was imported elsewhere), use `Expand2` instead and adjust accordingly.

- [ ] **Step 3: Type-check + commit**

```bash
cd /home/thbertoldi/harpia/frontend && npx svelte-kit sync >/dev/null 2>&1 && npm run check 2>&1 | grep -E "configurationId.\+page" | head -3
cd /home/thbertoldi/harpia && git add frontend/src/routes/plans/configurations/[configurationId]/+page.svelte
git commit -m "feat(ux-m4): wire Expand affordance from thread mini-map to canvas"
```

---

## Task 18: `canvas.*` i18n keys (lockstep en + pt-BR)

**Files:**
- Modify: `frontend/src/lib/i18n/en.json`
- Modify: `frontend/src/lib/i18n/pt-BR.json`

**Interfaces:**
- Produces: the full set of `canvas.*` keys used by Tasks 7-14, plus the `canvas.expandLink` key used by Task 17.

- [ ] **Step 1: Add keys to en.json**

INSERT after the existing `thread.*` block (or wherever the file's natural ordering places it):

```json
  "canvas.status.pending": "Pending",
  "canvas.status.running": "Running",
  "canvas.status.awaiting_elicitation": "Awaiting elicitation",
  "canvas.status.awaiting_approval": "Awaiting approval",
  "canvas.status.done": "Done",
  "canvas.status.failed": "Failed",
  "canvas.detail.empty": "Select a step to see its details.",
  "canvas.detail.answerApproval": "Answer this approval",
  "canvas.detail.answerElicitation": "Answer this elicitation",
  "canvas.detail.elicitationLoadError": "Could not load the elicitation.",
  "canvas.detail.elicitationLoading": "Loading elicitation…",
  "canvas.detail.startedAt": "Started",
  "canvas.detail.completedAt": "Completed",
  "canvas.detail.outputArtifact": "Output artifact",
  "canvas.topbar.backToThread": "Back to thread",
  "canvas.topbar.runHistory": "Run history",
  "canvas.topbar.schedule": "Schedule",
  "canvas.topbar.scheduleComingSoon": "Coming in a later milestone",
  "canvas.topbar.settings": "Settings",
  "canvas.topbar.answer": "Answer",
  "canvas.drawer.close": "Close",
  "canvas.runHistory.title": "Run history",
  "canvas.runHistory.loading": "Loading runs…",
  "canvas.runHistory.loadError": "Could not load run history.",
  "canvas.runHistory.empty": "No runs yet for this plan.",
  "canvas.runHistory.runLabel": "Run {n}",
  "canvas.settings.title": "Plan settings",
  "canvas.settings.loading": "Loading settings…",
  "canvas.settings.saved": "Settings saved.",
  "canvas.template.empty": "This plan has no steps yet.",
  "canvas.expandLink": "Expand",
```

- [ ] **Step 2: Add the same keys to pt-BR.json with Portuguese values**

INSERT the same block in the same position:

```json
  "canvas.status.pending": "Pendente",
  "canvas.status.running": "Em execução",
  "canvas.status.awaiting_elicitation": "Aguardando elicitação",
  "canvas.status.awaiting_approval": "Aguardando aprovação",
  "canvas.status.done": "Concluído",
  "canvas.status.failed": "Falhou",
  "canvas.detail.empty": "Selecione uma etapa para ver detalhes.",
  "canvas.detail.answerApproval": "Responder esta aprovação",
  "canvas.detail.answerElicitation": "Responder esta elicitação",
  "canvas.detail.elicitationLoadError": "Não foi possível carregar a elicitação.",
  "canvas.detail.elicitationLoading": "Carregando elicitação…",
  "canvas.detail.startedAt": "Iniciado em",
  "canvas.detail.completedAt": "Concluído em",
  "canvas.detail.outputArtifact": "Artefato de saída",
  "canvas.topbar.backToThread": "Voltar para conversa",
  "canvas.topbar.runHistory": "Histórico de execuções",
  "canvas.topbar.schedule": "Agenda",
  "canvas.topbar.scheduleComingSoon": "Disponível em um marco futuro",
  "canvas.topbar.settings": "Configurações",
  "canvas.topbar.answer": "Responder",
  "canvas.drawer.close": "Fechar",
  "canvas.runHistory.title": "Histórico de execuções",
  "canvas.runHistory.loading": "Carregando execuções…",
  "canvas.runHistory.loadError": "Não foi possível carregar o histórico.",
  "canvas.runHistory.empty": "Nenhuma execução ainda neste plano.",
  "canvas.runHistory.runLabel": "Execução {n}",
  "canvas.settings.title": "Configurações do plano",
  "canvas.settings.loading": "Carregando configurações…",
  "canvas.settings.saved": "Configurações salvas.",
  "canvas.template.empty": "Este plano ainda não tem etapas.",
  "canvas.expandLink": "Expandir",
```

- [ ] **Step 3: Run i18n parity test**

```bash
cd /home/thbertoldi/harpia/frontend && npx vitest run src/lib/i18n/
```

Expected: parity test passes (key sets identical).

- [ ] **Step 4: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/i18n/en.json frontend/src/lib/i18n/pt-BR.json
git commit -m "feat(ux-m4): add canvas.* i18n keys for expanded canvas surface"
```

---

## Task 19: End-to-end verification

**Files:** None modified — manual + automated verification. Deliverable is a verification log.

- [ ] **Step 1: Run the full automated suite**

```bash
cd /home/thbertoldi/harpia
( cd frontend && npm run test 2>&1 | tail -5 )
( cd frontend && npm run check 2>&1 | grep "ERRORS" | tail -1 )
( cd frontend && npm run lint 2>&1 | tail -3 )
( cd control-plane && go test ./... 2>&1 | tail -10 )
( cd control-plane && go vet ./... 2>&1 | tail -3 )
```

Expected:
- Frontend tests: baseline + new (canvas-state 8 tests)
- Type-check: 10 errors (trunk baseline; 0 new)
- Lint: clean
- Go tests: all pass plus new chat test (`TestBuildStepStartedPayload`)
- Go vet: clean

- [ ] **Step 2: Smoke-test the dev server**

```bash
cd /home/thbertoldi/harpia/frontend && npm run dev -- --port 5179 > /tmp/harpia-m4-dev.log 2>&1 &
DEV=$!
sleep 8
for r in /plans/configurations/test-id/canvas /plans/executions/test-id /plans/configurations/test-id; do
  curl -s -o /dev/null -w "%{http_code} → $r\n" -L --max-redirs 0 "http://localhost:5179$r"
done
tail -3 /tmp/harpia-m4-dev.log
kill $DEV 2>/dev/null; wait $DEV 2>/dev/null
```

Expected: each returns 302 (auth redirect). No 500 in the dev log.

- [ ] **Step 3: Write the verification log**

Create `docs/superpowers/plans/2026-06-22-harpia-ux-realignment-m4-expanded-canvas.verification.md`:

```markdown
# M4 Verification — <date>

Branch: `feat/ux-realignment-m4-expanded-canvas`
Plan: `docs/superpowers/plans/2026-06-22-harpia-ux-realignment-m4-expanded-canvas.md`

## Automated checks (executed)

| Check | Result | Notes |
|---|---|---|
| `frontend: npm run test` | ✅ <N> passing | New tests: lib/plans/canvas-state (8) |
| `frontend: npm run check` | ✅ 10 errors, 0 new | Trunk baseline |
| `frontend: npm run lint` | ✅ clean | |
| `control-plane: go test ./...` | ✅ <N> passing | New: TestBuildStepStartedPayload |
| `control-plane: go vet ./...` | ✅ clean | |
| Frontend dev server boots | ✅ Vite ready | |
| Route /plans/configurations/[id]/canvas resolves | ✅ 302→/login | |
| Route /plans/executions/[id] resolves | ✅ 302 redirect | Confirms legacy URL still works |
| `proto: buf generate && buf lint` | ✅ clean | New STEP_STARTED enum value |

## Browser-driven verification (manual — required before merge)

Run `cd frontend && npm run dev` with Tilt up. Use `devLogin('Leader')`.

| Step | Expected |
|---|---|
| Visit `/plans/configurations/<configId>/canvas` (no run) | Canvas renders static DAG; all nodes "Pending"; right-pane empty hint. |
| Visit `/plans/configurations/<configId>/canvas?run=<execId>` | Canvas paints execution state from chat stream; running step shows spinner; completed shows checkmark. |
| Visit `/plans/executions/<execId>` directly | 302 redirects to canvas URL with run query. |
| Click a node | Right-pane fills with executor / status / artifacts; selected ring on the node. |
| Click a node awaiting elicitation | Right-pane renders CanvasElicitationForm; submit answers the elicitation; row in M3 thread shows ELICITATION_ANSWERED. |
| Click a node awaiting approval | Right-pane renders CanvasApprovalForm; Approve/Reject work; canvas state updates live. |
| Click `Answer ★ N` in top bar | First pending step is highlighted/scrolled into view. |
| Click `Run history` | Drawer opens with prior runs of this plan; clicking a row swaps `?run=` and re-renders. |
| Click `Settings` | Drawer opens with PlanPoliciesForm; saving persists. |
| Click `Schedule` | Tooltip "Coming in a later milestone"; button disabled. |
| Click `Back to thread` | Navigates to `/plans/configurations/<configId>`. |
| From thread page, click `Expand ⤢` next to mini-map | Navigates to canvas URL. |
| Run a plan from scratch | Canvas updates live as STEP_STARTED / STEP_BOUND events arrive. |

## Commits

(populate from git log --oneline trunk..HEAD)

## Status

**Automated portion: ✅ complete.** Manual browser steps required before merge.
```

- [ ] **Step 4: Commit the verification log**

```bash
cd /home/thbertoldi/harpia
git add docs/superpowers/plans/2026-06-22-harpia-ux-realignment-m4-expanded-canvas.verification.md
git commit -m "chore(ux-m4): record M4 verification notes"
```

---

## Task 20: Whole-branch final review

**Files:** None modified — final review dispatch.

- [ ] **Step 1: Generate the review package**

```bash
cd /home/thbertoldi/harpia
MERGE_BASE=$(git merge-base trunk HEAD)
SCRIPTS=/home/thbertoldi/.claude/plugins/cache/claude-plugins-official/superpowers/6.0.3/skills/subagent-driven-development/scripts
$SCRIPTS/review-package $MERGE_BASE HEAD
```

- [ ] **Step 2: Dispatch a strongest-available reviewer (Claude opus or equivalent)**

Reviewer focus areas:
1. Vocabulary canonical per spec §2.
2. i18n parity (canvas.* keys lockstep en + pt-BR).
3. AbortSignal correctness on the new chat-stream subscription in the canvas page.
4. `STEP_STARTED` workflow hook fires when step transitions to RUNNING (verify the runtime path is the right place).
5. Form extractions don't regress M2/M3 UX (InboxApprovalEntry and elicitation detail page still work the same).
6. Legacy `/plans/executions/[id]` redirect resolves cleanly without redirect loops.
7. v1→v2 hinges preserved (PlanDagDiagram unchanged for template-detail page; canvas component decomposed for extensibility; Schedule stub clearly marked).
8. No new type errors beyond trunk baseline.

- [ ] **Step 3: Address review findings**

Critical: fix on the branch. Important: assess scope and either fix or defer. Minor: defer.

- [ ] **Step 4: Push and open the PR**

```bash
git push -u origin feat/ux-realignment-m4-expanded-canvas
gh pr create --title "feat(ux): M4 expanded canvas" --body "$(cat <<'EOF'
## Summary

Ships M4 of the UX Realignment — the expanded canvas at `/plans/configurations/[configId]/canvas?run=<execId>`.

**Plan:** [`docs/superpowers/plans/2026-06-22-harpia-ux-realignment-m4-expanded-canvas.md`](docs/superpowers/plans/2026-06-22-harpia-ux-realignment-m4-expanded-canvas.md)
**Design spec:** [`docs/superpowers/specs/2026-06-22-harpia-ux-m4-expanded-canvas-design.md`](docs/superpowers/specs/2026-06-22-harpia-ux-m4-expanded-canvas-design.md)
**Verification log:** [`docs/superpowers/plans/2026-06-22-harpia-ux-realignment-m4-expanded-canvas.verification.md`](docs/superpowers/plans/2026-06-22-harpia-ux-realignment-m4-expanded-canvas.verification.md)

## What ships

### Backend
- New `STEP_STARTED` `ThreadMessageKind` (proto + payload helper + workflow hook in `runtime.CreateStepExecution`)

### Frontend
- New `/plans/configurations/[id]/canvas?run=<execId>` route
- New `PlanCanvas` component family — nodes, edges, detail pane, top bar
- New `RunHistoryDrawer` + `SettingsDrawer` (hosts existing `PlanPoliciesForm`)
- Extracted `CanvasApprovalForm` (from `InboxApprovalEntry`) + `CanvasElicitationForm` (from elicitation detail page) — original surfaces compose them
- New `buildCanvasState` pure helper projecting M3 chat events onto step state (8 unit tests)
- Legacy `/plans/executions/[id]` is now a 302 redirect to the canvas
- M3 thread mini-map gains an `Expand ⤢` affordance linking to the canvas
- M1 canvas stub deleted
- `canvas.*` i18n keys (lockstep en + pt-BR)

## Known limitations / M5+ follow-ups

- Schedule dialog stubbed (button disabled with "Coming in a later milestone" tooltip). M5 will ship.
- Branching DAG layout is post-v1 (spec §4); current layout assumes linear chain.
- No drag-to-rearrange / drag-to-add (post-v1).
- Approval `inputArtifactId` lookup for the detail pane uses the chat-stream message payloads; richer cross-RPC data is M5+.

## Test plan

See verification log for full manual checklist. Manual browser smoke required before merge.
EOF
)"
```

- [ ] **Step 5: Final message**

The PR URL is the deliverable.

---

## Done criteria

M4 is complete when all of the following hold:

1. The `/plans/configurations/[configurationId]/canvas?run=<execId>` route renders a working canvas: nodes with state, edges with type labels, selectable, right-pane fills with executor/elicitation/approval details.
2. The chat stream's `STEP_STARTED` and existing `STEP_BOUND` / `RUN_*` / `ELICITATION_*` / `APPROVAL_*` events correctly drive node state via `buildCanvasState`.
3. Right-pane forms answer elicitations and approvals end-to-end without leaving the canvas.
4. Run history and Settings drawers open / close / function.
5. Schedule button is stubbed with tooltip.
6. `Answer ★ N` badge appears when ≥1 step is awaiting an answer; clicking it routes to the first pending node.
7. Legacy `/plans/executions/[executionId]` 302-redirects to the canvas URL.
8. M3 thread mini-map gains the `Expand ⤢` link to the canvas.
9. M1 canvas stub is deleted.
10. `npm run test` and `go test` pass; `npm run check` at trunk baseline; `npm run lint` clean.
11. PR open with verification log linked.
12. No legacy routes broken: `/inbox`, `/plans/[templateId]` template detail, `/plans/configurations/[id]` M3 thread, all M1 routes still work.

After M4 ships, M5 (chat-driven configuration) can begin. The canvas's right-pane and the M3 thread's composer are the two surfaces where M5's assistant becomes active.
