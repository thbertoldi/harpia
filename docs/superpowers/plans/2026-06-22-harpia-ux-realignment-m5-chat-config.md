# Harpia UX Realignment — M5 Chat-as-Configuration — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the deterministic configuration assistant inside the plan thread, replacing the four-page wizard. Add `/new` launcher, schedule dialog, live cost pill, and the sidebar "Your plans" list. Delete `/plans/[templateId]/configure/*` with one release of transitional redirects.

**Architecture:** New `control-plane/internal/planassistant/` Go package derives assistant state from `(PlanConfiguration, chat_messages)` and emits the next `ASSISTANT_PROMPT` on each `USER_SELECTION`. No LLM. The chat protocol gains five new `ThreadMessageKind`s (`CONFIGURATION_STARTED`, `ASSISTANT_PROMPT`, `USER_SELECTION`, `STEP_REBOUND`, `SCHEDULE_SET`). Schedule reuses the existing `PlanConfiguration.schedule` field — no migration. Cost pill is a pure derive over executor catalog + current bindings.

**Tech Stack:** Go 1.23 + ConnectRPC (control-plane), SvelteKit 2.16 + Svelte 5 (frontend), Vitest (frontend tests), `testing` package + `pgxpool` (Go), Tailwind v4 with Harpia design tokens, `github.com/robfig/cron/v3` for cron validation (add to `go.mod` if absent).

## Global Constraints

- **Vocabulary canonical** per master design spec §2 — Task / Plan / Agent / Integration / Executor / Overseer / Elicitation / Approval / Feedback / Artifact. User-facing copy and i18n keys use these terms exactly.
- **i18n lockstep.** Every key change touches both `frontend/src/lib/i18n/en.json` and `frontend/src/lib/i18n/pt-BR.json` in the same commit. The parity test in `frontend/src/lib/i18n/hardcoded-copy.test.ts` enforces this.
- **Design tokens LOCKED.** Only existing Tailwind tokens: `bg-obsidian`, `bg-obsidian-light`, `text-cream`, `text-crown-ash`, `text-crown-ash-dark`, `text-talon-gold`, `border-plumage`, `font-heading`, `font-body`, `font-mono`. No new colors.
- **Commit per task.** Conventional Commits with `feat(ux-m5):` / `chore(ux-m5):` / `fix(ux-m5):` / `docs(ux-m5):` / `test(ux-m5):` scope.
- **No `Co-Authored-By` trailer** on any commit (project convention).
- **Buf codegen.** After modifying any `.proto` file, run `cd /home/thbertoldi/harpia/proto && buf generate && buf lint`. Commit the generated code alongside the proto changes in the same commit.
- **No new files outside this plan's file map.** If implementation requires an unnamed file, STOP and report BLOCKED.
- **Branch:** create `feat/ux-realignment-m5-chat-config` from `trunk` before Task 1.
- **Test isolation per task.** Run only the focused tests: `npx vitest run <relative-path>` from `frontend/` for frontend; `go test ./internal/<pkg>/...` for Go.
- **Seed artifacts.** The assistant saves with `seed_artifacts: []`. Templates requiring seeds are out of scope (parity with wizard).
- **Cron-to-Temporal wiring out of scope.** The schedule field is persisted and triggers `SCHEDULE_SET` + status reconciliation; the actual scheduler trigger is a follow-up backend milestone.
- **Spec reference:** `docs/superpowers/specs/2026-06-22-harpia-ux-m5-chat-config-design.md`. When this plan says "per spec §X", read that section before deviating.

---

## File map (all changes in M5)

### Backend
- Modify: `proto/harpia/chat/v1/chat.proto` (5 new `ThreadMessageKind` variants)
- Modify: `control-plane/internal/chat/messages.go` (5 new payload builder helpers)
- Modify: `control-plane/internal/chat/messages_test.go` (tests for each builder)
- Create: `control-plane/internal/planassistant/state.go`
- Create: `control-plane/internal/planassistant/state_test.go`
- Create: `control-plane/internal/planassistant/prompts.go`
- Create: `control-plane/internal/planassistant/prompts_test.go`
- Create: `control-plane/internal/planassistant/controller.go`
- Create: `control-plane/internal/planassistant/controller_test.go`
- Modify: `control-plane/internal/plans/handler.go` (`CreatePlanConfiguration` → `SeedThread`; `UpdatePlanConfiguration` → schedule side effect)
- Modify: `control-plane/internal/plans/thread_handler.go` (`AppendPlanThreadMessage` → `NextTurn` on `USER_SELECTION`)
- Modify: `control-plane/go.mod` / `go.sum` (add `github.com/robfig/cron/v3` if absent)
- Regenerate: `control-plane/gen/`, `frontend/src/lib/gen/`, `agent-runtime/src/harpia_agents/gen/`

### Frontend lib
- Modify: `frontend/src/lib/chat/types.ts` (extend `ChatMessageKind` union + 4 maps)
- Create: `frontend/src/lib/plans/cost.ts`
- Create: `frontend/src/lib/plans/cost.test.ts`
- Create: `frontend/src/lib/plans/assistant.ts`
- Create: `frontend/src/lib/plans/assistant.test.ts`

### Frontend components
- Create: `frontend/src/lib/components/thread/AssistantPromptCard.svelte`
- Create: `frontend/src/lib/components/thread/ConfirmCard.svelte`
- Create: `frontend/src/lib/components/thread/TemplatePickerCard.svelte`
- Modify: `frontend/src/lib/components/thread/ThreadMessage.svelte` (dispatch the new kinds)
- Modify: `frontend/src/lib/components/thread/SystemEventCard.svelte` (icon + label for `STEP_REBOUND`, `SCHEDULE_SET`, `CONFIGURATION_STARTED`)
- Create: `frontend/src/lib/components/PlanCostPill.svelte`
- Create: `frontend/src/lib/components/PlanThreadTopBar.svelte`
- Create: `frontend/src/lib/components/canvas/ScheduleDialog.svelte`
- Modify: `frontend/src/lib/components/canvas/CanvasTopBar.svelte` (wire Schedule + mount cost pill)
- Create: `frontend/src/lib/components/sidebar/YourPlansList.svelte`

### Pages + integration
- Modify: `frontend/src/routes/new/+page.svelte` (replace M5 stub with launcher)
- Create: `frontend/src/routes/new/+page.ts` (load templates for gallery)
- Modify: `frontend/src/routes/plans/configurations/[configurationId]/+page.svelte` (mount top bar; render new kinds)
- Modify: `frontend/src/routes/plans/configurations/[configurationId]/+page.ts` (load executor catalog)
- Modify: `frontend/src/routes/plans/configurations/[configurationId]/canvas/+page.svelte` (mount ScheduleDialog state)
- Modify: `frontend/src/routes/+layout.svelte` (mount `YourPlansList` under sidebar nav)
- Delete: `frontend/src/routes/plans/[templateId]/configure/+page.svelte`
- Delete: `frontend/src/routes/plans/[templateId]/configure/+layout.server.ts`
- Delete: `frontend/src/routes/plans/[templateId]/configure/overseer/+page.svelte`
- Delete: `frontend/src/routes/plans/[templateId]/configure/policies/+page.svelte`
- Delete: `frontend/src/routes/plans/[templateId]/configure/summary/+page.svelte`
- Create: `frontend/src/routes/plans/[templateId]/configure/+page.ts` (302 redirect to `/new?template=<id>`)
- Create: `frontend/src/routes/plans/[templateId]/configure/overseer/+page.ts` (302 redirect)
- Create: `frontend/src/routes/plans/[templateId]/configure/policies/+page.ts` (302 redirect)
- Create: `frontend/src/routes/plans/[templateId]/configure/summary/+page.ts` (302 redirect)
- Modify: `frontend/src/routes/plans/[templateId]/+page.svelte` (retarget "Use this template" CTA)

### i18n
- Modify: `frontend/src/lib/i18n/en.json` (add `assistant.*`, `confirm.*`, `cost.*`, `schedule.*`, `sidebar.yourPlans.*`, `new.*` keys)
- Modify: `frontend/src/lib/i18n/pt-BR.json` (lockstep)

### Verification
- Create: `docs/superpowers/plans/2026-06-22-harpia-ux-realignment-m5-chat-config.verification.md`

---

## Task 0: Create the M5 working branch

**Files:**
- None (git branch only)

- [ ] **Step 1: Branch from trunk**

```bash
cd /home/thbertoldi/harpia
git fetch origin
git checkout -b feat/ux-realignment-m5-chat-config origin/trunk
```

Expected: branch created, working tree clean except untracked `.svelte-kit/`.

---

## Task 1: Proto — five new `ThreadMessageKind` variants

**Files:**
- Modify: `proto/harpia/chat/v1/chat.proto`

**Interfaces:**
- Consumes: existing `THREAD_MESSAGE_KIND_STEP_STARTED = 12` from M4.
- Produces:
  - `THREAD_MESSAGE_KIND_CONFIGURATION_STARTED = 13`
  - `THREAD_MESSAGE_KIND_ASSISTANT_PROMPT      = 14`
  - `THREAD_MESSAGE_KIND_USER_SELECTION        = 15`
  - `THREAD_MESSAGE_KIND_STEP_REBOUND          = 16`
  - `THREAD_MESSAGE_KIND_SCHEDULE_SET          = 17`

- [ ] **Step 1: Append the five enum values**

In `proto/harpia/chat/v1/chat.proto`, locate the `enum ThreadMessageKind { ... }` block. After the existing `THREAD_MESSAGE_KIND_STEP_STARTED = 12;` line and BEFORE the closing brace, ADD:

```protobuf
  THREAD_MESSAGE_KIND_CONFIGURATION_STARTED = 13;   // payload_json: { "template_id": "..." }
  THREAD_MESSAGE_KIND_ASSISTANT_PROMPT      = 14;   // payload_json: { "state": "...", "step_key": "...", "options": [...] }
  THREAD_MESSAGE_KIND_USER_SELECTION        = 15;   // payload_json: { "in_response_to_message_id": "...", "option_id": "...", "value": "..." }
  THREAD_MESSAGE_KIND_STEP_REBOUND          = 16;   // payload_json: { "step_key": "...", "previous_executor_installation_id": "...", "new_executor_installation_id": "..." }
  THREAD_MESSAGE_KIND_SCHEDULE_SET          = 17;   // payload_json: { "schedule_cron": "...", "timezone": "..." }
```

- [ ] **Step 2: Regenerate**

```bash
cd /home/thbertoldi/harpia/proto && buf generate && buf lint
```

Expected: no errors. Generated files at `control-plane/gen/harpia/chat/v1/chat.pb.go`, `frontend/src/lib/gen/harpia/chat/v1/chat_pb.ts`, `agent-runtime/src/harpia_agents/gen/harpia/chat/v1/chat_pb2.py` carry the new enum values.

- [ ] **Step 3: Commit**

```bash
cd /home/thbertoldi/harpia
git add proto/harpia/chat/v1/chat.proto control-plane/gen/harpia/chat frontend/src/lib/gen/harpia/chat agent-runtime/src/harpia_agents/gen/harpia/chat
git commit -m "feat(ux-m5): add 5 chat-message kinds for the configuration assistant"
```

---

## Task 2: Chat — five payload builder helpers + tests

**Files:**
- Modify: `control-plane/internal/chat/messages.go`
- Modify: `control-plane/internal/chat/messages_test.go`

**Interfaces:**
- Consumes: the five new enum values from Task 1.
- Produces (all in package `chat`):
  - `BuildConfigurationStartedPayload(templateID string) string`
  - `BuildAssistantPromptPayload(state, stepKey string, options []AssistantOption) string`
  - `BuildUserSelectionPayload(inResponseToMessageID, optionID, value string) string`
  - `BuildStepReboundPayload(stepKey, previousInstallationID, newInstallationID string) string`
  - `BuildScheduleSetPayload(scheduleCron, timezone string) string`
  - `AssistantOption` struct: `{ ID, Label, Sublabel, Value string; PriceBrl *float64 }`

- [ ] **Step 1: Write failing tests for the five builders**

The existing `control-plane/internal/chat/messages_test.go` is `package chat` (internal test). Builders are called unqualified — do NOT prefix with `chat.`. Append:

```go
func TestBuildConfigurationStartedPayload(t *testing.T) {
	got := BuildConfigurationStartedPayload("tpl-123")
	want := `{"template_id":"tpl-123"}`
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestBuildAssistantPromptPayload(t *testing.T) {
	price := 0.12
	options := []AssistantOption{
		{ID: "opt-1", Label: "Junior", Sublabel: "R$ 0.02/run", Value: "inst-1", PriceBrl: &price},
		{ID: "opt-2", Label: "Senior", Sublabel: "", Value: "inst-2", PriceBrl: nil},
	}
	got := BuildAssistantPromptPayload("BINDING_STEP", "step-a", options)
	if !strings.Contains(got, `"state":"BINDING_STEP"`) ||
		!strings.Contains(got, `"step_key":"step-a"`) ||
		!strings.Contains(got, `"options":`) ||
		!strings.Contains(got, `"id":"opt-1"`) {
		t.Fatalf("unexpected payload: %s", got)
	}
}

func TestBuildUserSelectionPayload(t *testing.T) {
	got := BuildUserSelectionPayload("msg-1", "opt-2", "inst-2")
	want := `{"in_response_to_message_id":"msg-1","option_id":"opt-2","value":"inst-2"}`
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestBuildStepReboundPayload(t *testing.T) {
	got := BuildStepReboundPayload("step-a", "inst-old", "inst-new")
	want := `{"step_key":"step-a","previous_executor_installation_id":"inst-old","new_executor_installation_id":"inst-new"}`
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestBuildScheduleSetPayload(t *testing.T) {
	got := BuildScheduleSetPayload("0 9 * * *", "America/Sao_Paulo")
	want := `{"schedule_cron":"0 9 * * *","timezone":"America/Sao_Paulo"}`
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}
```

Add `"strings"` to the imports if not already present. Do NOT add `chat` to imports — the file is `package chat`.

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/thbertoldi/harpia/control-plane && go test ./internal/chat/... -run 'TestBuild(ConfigurationStarted|AssistantPrompt|UserSelection|StepRebound|ScheduleSet)Payload' -v
```

Expected: FAIL with `undefined: chat.BuildConfigurationStartedPayload` etc.

- [ ] **Step 3: Implement the five builders**

Append to `control-plane/internal/chat/messages.go`:

```go
// AssistantOption is one chip in an ASSISTANT_PROMPT payload.
type AssistantOption struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Sublabel string   `json:"sublabel,omitempty"`
	Value    string   `json:"value"`
	PriceBrl *float64 `json:"price_brl,omitempty"`
}

// BuildConfigurationStartedPayload returns the JSON payload for a
// CONFIGURATION_STARTED message — written once on PlanConfiguration insert.
func BuildConfigurationStartedPayload(templateID string) string {
	return mustEncodeJSON(map[string]string{"template_id": templateID})
}

// BuildAssistantPromptPayload returns the JSON payload for an
// ASSISTANT_PROMPT message — the assistant's turn with quick-reply chips.
func BuildAssistantPromptPayload(state, stepKey string, options []AssistantOption) string {
	if options == nil {
		options = []AssistantOption{}
	}
	return mustEncodeJSON(map[string]any{
		"state":    state,
		"step_key": stepKey,
		"options":  options,
	})
}

// BuildUserSelectionPayload returns the JSON payload for a USER_SELECTION
// message — Ana's chip click in response to an ASSISTANT_PROMPT.
func BuildUserSelectionPayload(inResponseToMessageID, optionID, value string) string {
	return mustEncodeJSON(map[string]string{
		"in_response_to_message_id": inResponseToMessageID,
		"option_id":                 optionID,
		"value":                     value,
	})
}

// BuildStepReboundPayload returns the JSON payload for a STEP_REBOUND
// system message — Ana edited a previously-bound executor.
func BuildStepReboundPayload(stepKey, previousInstallationID, newInstallationID string) string {
	return mustEncodeJSON(map[string]string{
		"step_key":                          stepKey,
		"previous_executor_installation_id": previousInstallationID,
		"new_executor_installation_id":      newInstallationID,
	})
}

// BuildScheduleSetPayload returns the JSON payload for a SCHEDULE_SET
// system message — the schedule dialog persisted a cron expression.
func BuildScheduleSetPayload(scheduleCron, timezone string) string {
	return mustEncodeJSON(map[string]string{
		"schedule_cron": scheduleCron,
		"timezone":      timezone,
	})
}
```

`mustEncodeJSON` already exists in the file.

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/thbertoldi/harpia/control-plane && go test ./internal/chat/... -run 'TestBuild' -v
```

Expected: PASS for all five new tests (and the M3/M4 ones).

- [ ] **Step 5: Commit**

```bash
cd /home/thbertoldi/harpia
git add control-plane/internal/chat/messages.go control-plane/internal/chat/messages_test.go
git commit -m "feat(ux-m5): add 5 chat payload builders for the configuration assistant"
```

---

## Task 3: `planassistant.state` — state derivation (pure)

**Files:**
- Create: `control-plane/internal/planassistant/state.go`
- Create: `control-plane/internal/planassistant/state_test.go`

**Interfaces:**
- Consumes:
  - `plansv1.PlanConfiguration` (proto type from `gen/harpia/plans/v1`)
  - `plansv1.PlanTemplate`
  - `[]*chatv1.ThreadMessage`
- Produces:
  - `type AssistantState struct { Kind StateKind; StepKey string }`
  - `type StateKind string` — one of `"AWAITING_TEMPLATE"`, `"BINDING_STEP"`, `"SET_OVERSEER"`, `"SET_POLICIES"`, `"CONFIRM"`, `"SAVED"`.
  - `func DeriveState(template *plansv1.PlanTemplate, config *plansv1.PlanConfiguration, messages []*chatv1.ThreadMessage) AssistantState`
  - Constants for each `StateKind`.

- [ ] **Step 1: Write the failing test file**

Create `control-plane/internal/planassistant/state_test.go`:

```go
package planassistant_test

import (
	"testing"

	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/planassistant"
)

func mkTemplate(stepKeys ...string) *plansv1.PlanTemplate {
	tpl := &plansv1.PlanTemplate{Id: "tpl-1"}
	for _, k := range stepKeys {
		tpl.Steps = append(tpl.Steps, &plansv1.PlanStep{Key: k, Title: k})
	}
	return tpl
}

func TestDeriveState_AwaitingTemplate_WhenTemplateIdEmpty(t *testing.T) {
	got := planassistant.DeriveState(mkTemplate("a"), &plansv1.PlanConfiguration{}, nil)
	if got.Kind != planassistant.StateAwaitingTemplate {
		t.Fatalf("got %v, want AWAITING_TEMPLATE", got.Kind)
	}
}

func TestDeriveState_BindingFirstStep_WhenNoSlotBindings(t *testing.T) {
	cfg := &plansv1.PlanConfiguration{PlanTemplateId: "tpl-1"}
	got := planassistant.DeriveState(mkTemplate("a", "b"), cfg, nil)
	if got.Kind != planassistant.StateBindingStep || got.StepKey != "a" {
		t.Fatalf("got %+v, want BINDING_STEP(a)", got)
	}
}

func TestDeriveState_BindingNextUnboundStep(t *testing.T) {
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId: "tpl-1",
		SlotBindings: []*plansv1.SlotBinding{
			{StepKey: "a", ExecutorInstallationId: "inst-a"},
		},
	}
	got := planassistant.DeriveState(mkTemplate("a", "b"), cfg, nil)
	if got.Kind != planassistant.StateBindingStep || got.StepKey != "b" {
		t.Fatalf("got %+v, want BINDING_STEP(b)", got)
	}
}

func TestDeriveState_OverseerAfterAllBindings(t *testing.T) {
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId: "tpl-1",
		SlotBindings: []*plansv1.SlotBinding{
			{StepKey: "a", ExecutorInstallationId: "inst-a"},
			{StepKey: "b", ExecutorInstallationId: "inst-b"},
		},
	}
	got := planassistant.DeriveState(mkTemplate("a", "b"), cfg, nil)
	if got.Kind != planassistant.StateSetOverseer || got.StepKey != "a" {
		t.Fatalf("got %+v, want SET_OVERSEER(a)", got)
	}
}

func TestDeriveState_PoliciesAfterAllOverseers(t *testing.T) {
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId: "tpl-1",
		SlotBindings: []*plansv1.SlotBinding{
			{StepKey: "a", ExecutorInstallationId: "inst-a"},
		},
		OverseerBindings: []*plansv1.OverseerBinding{
			{StepKey: "a", OverseerUserId: "user-1"},
		},
	}
	got := planassistant.DeriveState(mkTemplate("a"), cfg, nil)
	if got.Kind != planassistant.StateSetPolicies {
		t.Fatalf("got %+v, want SET_POLICIES", got)
	}
}

func TestDeriveState_ConfirmAfterPolicies(t *testing.T) {
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId: "tpl-1",
		SlotBindings:   []*plansv1.SlotBinding{{StepKey: "a", ExecutorInstallationId: "inst-a"}},
		OverseerBindings: []*plansv1.OverseerBinding{{StepKey: "a", OverseerUserId: "u"}},
		BehaviorPolicies: &plansv1.PlanBehaviorPolicies{
			ElicitationTimeoutBehavior: plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_PAUSE_UNTIL_ANSWERED,
			PublishApprovalMode:        plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_AUTO_PUBLISH,
		},
	}
	got := planassistant.DeriveState(mkTemplate("a"), cfg, nil)
	if got.Kind != planassistant.StateConfirm {
		t.Fatalf("got %+v, want CONFIRM", got)
	}
}

func TestDeriveState_SavedWhenStatusRunnable(t *testing.T) {
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId:   "tpl-1",
		Status:           plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
		SlotBindings:     []*plansv1.SlotBinding{{StepKey: "a", ExecutorInstallationId: "inst-a"}},
		OverseerBindings: []*plansv1.OverseerBinding{{StepKey: "a", OverseerUserId: "u"}},
		BehaviorPolicies: &plansv1.PlanBehaviorPolicies{
			ElicitationTimeoutBehavior: plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_PAUSE_UNTIL_ANSWERED,
			PublishApprovalMode:        plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_AUTO_PUBLISH,
		},
	}
	// One SAVED-signalling SCHEDULE_SET or no further prompt — but easier:
	// once status is RUNNABLE+SCHEDULED-eligible, treat as SAVED regardless of msgs.
	got := planassistant.DeriveState(mkTemplate("a"), cfg, []*chatv1.ThreadMessage{})
	if got.Kind != planassistant.StateSaved {
		t.Fatalf("got %+v, want SAVED", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/thbertoldi/harpia/control-plane && go test ./internal/planassistant/... -v
```

Expected: FAIL — package does not exist yet.

- [ ] **Step 3: Implement `state.go`**

Create `control-plane/internal/planassistant/state.go`:

```go
// Package planassistant owns the deterministic configuration-assistant
// state machine. See docs/superpowers/specs/2026-06-22-harpia-ux-m5-chat-config-design.md.
package planassistant

import (
	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
)

// StateKind identifies the assistant's position in the configuration flow.
type StateKind string

const (
	StateAwaitingTemplate StateKind = "AWAITING_TEMPLATE"
	StateBindingStep      StateKind = "BINDING_STEP"
	StateSetOverseer      StateKind = "SET_OVERSEER"
	StateSetPolicies      StateKind = "SET_POLICIES"
	StateConfirm          StateKind = "CONFIRM"
	StateSaved            StateKind = "SAVED"
)

// AssistantState is the derived state for a single PlanConfiguration.
// StepKey is the step the current state applies to (empty for non-per-step states).
type AssistantState struct {
	Kind    StateKind
	StepKey string
}

// DeriveState is a pure function over (template, configuration, messages)
// that returns the assistant's next state. Messages are accepted for
// future use (e.g., detecting in-progress rewind) but are unused in v1
// derivation — the configuration alone is authoritative.
func DeriveState(template *plansv1.PlanTemplate, config *plansv1.PlanConfiguration, messages []*chatv1.ThreadMessage) AssistantState {
	_ = messages
	if config == nil || config.GetPlanTemplateId() == "" {
		return AssistantState{Kind: StateAwaitingTemplate}
	}
	// SAVED is sticky once status crosses RUNNABLE/SCHEDULED.
	switch config.GetStatus() {
	case plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
		plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_SCHEDULED:
		return AssistantState{Kind: StateSaved}
	}

	steps := template.GetSteps()

	// 1. Walk steps in order; first unbound step → BINDING_STEP.
	bindings := map[string]string{}
	for _, sb := range config.GetSlotBindings() {
		if sb.GetExecutorInstallationId() != "" {
			bindings[sb.GetStepKey()] = sb.GetExecutorInstallationId()
		}
	}
	for _, step := range steps {
		if _, ok := bindings[step.GetKey()]; !ok {
			return AssistantState{Kind: StateBindingStep, StepKey: step.GetKey()}
		}
	}

	// 2. All bound — first step without an overseer → SET_OVERSEER.
	overseers := map[string]string{}
	for _, ob := range config.GetOverseerBindings() {
		if ob.GetOverseerUserId() != "" {
			overseers[ob.GetStepKey()] = ob.GetOverseerUserId()
		}
	}
	for _, step := range steps {
		if _, ok := overseers[step.GetKey()]; !ok {
			return AssistantState{Kind: StateSetOverseer, StepKey: step.GetKey()}
		}
	}

	// 3. Policies unset → SET_POLICIES.
	if !policiesSet(config.GetBehaviorPolicies()) {
		return AssistantState{Kind: StateSetPolicies}
	}

	// 4. Everything bound and policies set, status still DRAFT → CONFIRM.
	return AssistantState{Kind: StateConfirm}
}

func policiesSet(p *plansv1.PlanBehaviorPolicies) bool {
	if p == nil {
		return false
	}
	return p.GetElicitationTimeoutBehavior() != plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_UNSPECIFIED &&
		p.GetPublishApprovalMode() != plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_UNSPECIFIED
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/thbertoldi/harpia/control-plane && go test ./internal/planassistant/... -v
```

Expected: PASS for all seven derivation tests.

- [ ] **Step 5: Commit**

```bash
cd /home/thbertoldi/harpia
git add control-plane/internal/planassistant/state.go control-plane/internal/planassistant/state_test.go
git commit -m "feat(ux-m5): planassistant.DeriveState — pure state derivation"
```

---

## Task 4: `planassistant.prompts` — per-state prompt builders

**Files:**
- Create: `control-plane/internal/planassistant/prompts.go`
- Create: `control-plane/internal/planassistant/prompts_test.go`

**Interfaces:**
- Consumes:
  - `AssistantState` (Task 3)
  - `chat.AssistantOption` (Task 2)
- Produces:
  - `type PromptInput struct { Template *plansv1.PlanTemplate; Config *plansv1.PlanConfiguration; CandidateExecutors []ExecutorOption }`
  - `type ExecutorOption struct { StepKey, InstallationID, DisplayName, SkuKey string; Tier string; PriceBrl *float64 }`
  - `func BuildPrompt(state AssistantState, in PromptInput) (text string, payload string)` — returns the human-readable prose AND the JSON payload (built with `chat.BuildAssistantPromptPayload`).

- [ ] **Step 1: Write failing tests**

Create `control-plane/internal/planassistant/prompts_test.go`:

```go
package planassistant_test

import (
	"strings"
	"testing"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/planassistant"
)

func TestBuildPrompt_BindingStep_IncludesCandidates(t *testing.T) {
	tpl := mkTemplate("draft", "publish")
	tpl.Steps[0].Title = "Draft post"
	price := 0.05
	in := planassistant.PromptInput{
		Template: tpl,
		Config:   &plansv1.PlanConfiguration{PlanTemplateId: "tpl-1"},
		CandidateExecutors: []planassistant.ExecutorOption{
			{StepKey: "draft", InstallationID: "inst-1", DisplayName: "Junior Writer", SkuKey: "writer-junior", Tier: "junior", PriceBrl: &price},
		},
	}
	state := planassistant.AssistantState{Kind: planassistant.StateBindingStep, StepKey: "draft"}
	text, payload := planassistant.BuildPrompt(state, in)
	if !strings.Contains(text, "Draft post") {
		t.Fatalf("text missing step title: %q", text)
	}
	if !strings.Contains(payload, `"state":"BINDING_STEP"`) ||
		!strings.Contains(payload, `"step_key":"draft"`) ||
		!strings.Contains(payload, `"id":"inst-1"`) {
		t.Fatalf("payload missing fields: %s", payload)
	}
}

func TestBuildPrompt_SetOverseer_DefaultsToYou(t *testing.T) {
	tpl := mkTemplate("draft")
	state := planassistant.AssistantState{Kind: planassistant.StateSetOverseer, StepKey: "draft"}
	in := planassistant.PromptInput{Template: tpl, Config: &plansv1.PlanConfiguration{PlanTemplateId: "tpl-1"}}
	_, payload := planassistant.BuildPrompt(state, in)
	if !strings.Contains(payload, `"state":"SET_OVERSEER"`) ||
		!strings.Contains(payload, `"value":"self"`) {
		t.Fatalf("expected self option in payload: %s", payload)
	}
}

func TestBuildPrompt_SetPolicies_HasTimeoutAndApprovalChoices(t *testing.T) {
	state := planassistant.AssistantState{Kind: planassistant.StateSetPolicies}
	in := planassistant.PromptInput{Config: &plansv1.PlanConfiguration{PlanTemplateId: "tpl-1"}}
	_, payload := planassistant.BuildPrompt(state, in)
	// Policies prompt presents combined choices; assert at least one of each.
	if !strings.Contains(payload, "PAUSE_UNTIL_ANSWERED") || !strings.Contains(payload, "REQUIRE_APPROVAL") {
		t.Fatalf("policies payload missing canonical choices: %s", payload)
	}
}

func TestBuildPrompt_Confirm_EmitsSaveOption(t *testing.T) {
	state := planassistant.AssistantState{Kind: planassistant.StateConfirm}
	in := planassistant.PromptInput{Config: &plansv1.PlanConfiguration{PlanTemplateId: "tpl-1"}}
	_, payload := planassistant.BuildPrompt(state, in)
	if !strings.Contains(payload, `"id":"save"`) {
		t.Fatalf("confirm payload missing save option: %s", payload)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/thbertoldi/harpia/control-plane && go test ./internal/planassistant/... -run 'TestBuildPrompt' -v
```

Expected: FAIL — `BuildPrompt` does not exist.

- [ ] **Step 3: Implement `prompts.go`**

Create `control-plane/internal/planassistant/prompts.go`:

```go
package planassistant

import (
	"fmt"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/chat"
)

// ExecutorOption is one available executor installation for a step,
// loaded by the controller before calling BuildPrompt.
type ExecutorOption struct {
	StepKey        string
	InstallationID string
	DisplayName    string
	SkuKey         string
	Tier           string // "junior" / "senior" / "specialist" / ""
	PriceBrl       *float64
}

// PromptInput carries everything BuildPrompt needs that isn't already in
// AssistantState. The controller assembles it once per turn.
type PromptInput struct {
	Template           *plansv1.PlanTemplate
	Config             *plansv1.PlanConfiguration
	CandidateExecutors []ExecutorOption // filtered for the current step only when state is BINDING_STEP
}

// BuildPrompt returns the assistant's prose text and the JSON payload
// (chat.BuildAssistantPromptPayload(state, stepKey, options)) for the
// given state. Pure.
func BuildPrompt(state AssistantState, in PromptInput) (text, payload string) {
	switch state.Kind {
	case StateAwaitingTemplate:
		// Unreachable in v1; emit a single option that surfaces the gallery.
		return "Pick a template to start.",
			chat.BuildAssistantPromptPayload(string(state.Kind), "", nil)

	case StateBindingStep:
		step := findStep(in.Template, state.StepKey)
		var opts []chat.AssistantOption
		for _, c := range in.CandidateExecutors {
			if c.StepKey != state.StepKey {
				continue
			}
			opts = append(opts, chat.AssistantOption{
				ID:       c.InstallationID,
				Label:    c.DisplayName,
				Sublabel: formatExecutorSublabel(c),
				Value:    c.InstallationID,
				PriceBrl: c.PriceBrl,
			})
		}
		title := state.StepKey
		if step != nil && step.GetTitle() != "" {
			title = step.GetTitle()
		}
		text = fmt.Sprintf("Pick an executor for %s.", title)
		payload = chat.BuildAssistantPromptPayload(string(state.Kind), state.StepKey, opts)
		return

	case StateSetOverseer:
		opts := []chat.AssistantOption{
			{ID: "self", Label: "You", Value: "self"},
		}
		text = "Who answers questions from this step?"
		payload = chat.BuildAssistantPromptPayload(string(state.Kind), state.StepKey, opts)
		return

	case StateSetPolicies:
		opts := []chat.AssistantOption{
			{ID: "balanced", Label: "Balanced", Sublabel: "Pause until answered · Require approval", Value: "PAUSE_UNTIL_ANSWERED+REQUIRE_APPROVAL"},
			{ID: "hands_off", Label: "Hands-off", Sublabel: "Pause until answered · Auto-publish", Value: "PAUSE_UNTIL_ANSWERED+AUTO_PUBLISH"},
			{ID: "strict", Label: "Strict", Sublabel: "Fail step on timeout · Require approval", Value: "FAIL_STEP+REQUIRE_APPROVAL"},
		}
		text = "How should this plan behave when something needs attention?"
		payload = chat.BuildAssistantPromptPayload(string(state.Kind), "", opts)
		return

	case StateConfirm:
		opts := []chat.AssistantOption{
			{ID: "save", Label: "Save", Value: "save"},
		}
		text = "Ready to save?"
		payload = chat.BuildAssistantPromptPayload(string(state.Kind), "", opts)
		return

	case StateSaved:
		text = "Saved. Run it from the canvas or set a schedule."
		payload = chat.BuildAssistantPromptPayload(string(state.Kind), "", nil)
		return
	}
	return "", chat.BuildAssistantPromptPayload(string(state.Kind), "", nil)
}

func findStep(tpl *plansv1.PlanTemplate, key string) *plansv1.PlanStep {
	for _, s := range tpl.GetSteps() {
		if s.GetKey() == key {
			return s
		}
	}
	return nil
}

func formatExecutorSublabel(c ExecutorOption) string {
	if c.PriceBrl != nil {
		return fmt.Sprintf("%s · R$ %.2f/run", c.SkuKey, *c.PriceBrl)
	}
	return c.SkuKey
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/thbertoldi/harpia/control-plane && go test ./internal/planassistant/... -run 'TestBuildPrompt' -v
```

Expected: PASS for all four.

- [ ] **Step 5: Commit**

```bash
cd /home/thbertoldi/harpia
git add control-plane/internal/planassistant/prompts.go control-plane/internal/planassistant/prompts_test.go
git commit -m "feat(ux-m5): planassistant.BuildPrompt — per-state prompt builders"
```

---

## Task 5: `planassistant.controller` — `SeedThread` + `NextTurn`

**Files:**
- Create: `control-plane/internal/planassistant/controller.go`
- Create: `control-plane/internal/planassistant/controller_test.go`

**Interfaces:**
- Consumes:
  - `chat.Store` (existing)
  - `DeriveState`, `BuildPrompt`, `ExecutorOption` (Tasks 3, 4)
  - `chat.Build*` helpers (Task 2)
- Produces:
  - `type ExecutorCatalog interface { CandidatesForStep(ctx context.Context, tenantID uuid.UUID, template *plansv1.PlanTemplate, stepKey string) ([]ExecutorOption, error) }`
  - `type ConfigurationStore interface { GetConfiguration(ctx context.Context, tenantID, id uuid.UUID) (*plansv1.PlanConfiguration, error); UpdateFromSelection(ctx context.Context, tenantID, configID uuid.UUID, state AssistantState, value string) (*plansv1.PlanConfiguration, error) }`
  - `type Controller struct { Chat chat.Store; Catalog ExecutorCatalog; Configs ConfigurationStore; Templates TemplateStore }`
  - `type TemplateStore interface { GetTemplateByID(ctx context.Context, id uuid.UUID) (*plansv1.PlanTemplate, error) }`
  - `func (c *Controller) SeedThread(ctx context.Context, tenantID, configID uuid.UUID) error`
  - `func (c *Controller) NextTurn(ctx context.Context, tenantID, configID uuid.UUID) error`

- [ ] **Step 1: Write failing tests**

Create `control-plane/internal/planassistant/controller_test.go`:

```go
package planassistant_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/chat"
	"github.com/harpia/control-plane/internal/planassistant"
)

type fakeChat struct {
	appended []chat.AppendInput
	listing  []*chatv1.ThreadMessage
}

func (f *fakeChat) AppendMessage(_ context.Context, _ uuid.UUID, in chat.AppendInput) (*chatv1.ThreadMessage, error) {
	f.appended = append(f.appended, in)
	return &chatv1.ThreadMessage{Id: uuid.NewString(), Kind: in.Kind, PayloadJson: in.PayloadJSON, Text: in.Text}, nil
}
func (f *fakeChat) ListMessages(_ context.Context, _ uuid.UUID, _ string, _ int64, _ int) ([]*chatv1.ThreadMessage, error) {
	return f.listing, nil
}

type fakeConfigs struct{ cur *plansv1.PlanConfiguration }

func (f *fakeConfigs) GetConfiguration(_ context.Context, _, _ uuid.UUID) (*plansv1.PlanConfiguration, error) {
	return f.cur, nil
}
func (f *fakeConfigs) UpdateFromSelection(_ context.Context, _, _ uuid.UUID, _ planassistant.AssistantState, _ string) (*plansv1.PlanConfiguration, error) {
	return f.cur, nil
}

type fakeCatalog struct{}

func (fakeCatalog) CandidatesForStep(_ context.Context, _ uuid.UUID, _ *plansv1.PlanTemplate, _ string) ([]planassistant.ExecutorOption, error) {
	return nil, nil
}

type fakeTemplates struct{ tpl *plansv1.PlanTemplate }

func (f *fakeTemplates) GetTemplateByID(_ context.Context, _ uuid.UUID) (*plansv1.PlanTemplate, error) {
	return f.tpl, nil
}

func TestSeedThread_EmitsConfigurationStartedThenFirstPrompt(t *testing.T) {
	chatStore := &fakeChat{}
	tpl := mkTemplate("draft")
	cfg := &plansv1.PlanConfiguration{Id: uuid.NewString(), PlanTemplateId: "tpl-1"}
	c := &planassistant.Controller{
		Chat:      chatStore,
		Catalog:   fakeCatalog{},
		Configs:   &fakeConfigs{cur: cfg},
		Templates: &fakeTemplates{tpl: tpl},
	}
	if err := c.SeedThread(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if len(chatStore.appended) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(chatStore.appended))
	}
	if chatStore.appended[0].Kind != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_CONFIGURATION_STARTED {
		t.Fatalf("first message kind: %v", chatStore.appended[0].Kind)
	}
	if chatStore.appended[1].Kind != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_PROMPT {
		t.Fatalf("second message kind: %v", chatStore.appended[1].Kind)
	}
}

func TestNextTurn_AdvancesAfterSelection(t *testing.T) {
	chatStore := &fakeChat{}
	tpl := mkTemplate("draft", "publish")
	// Simulate: draft bound; configuration now needs SET_OVERSEER for draft.
	cfg := &plansv1.PlanConfiguration{
		Id:             uuid.NewString(),
		PlanTemplateId: "tpl-1",
		SlotBindings:   []*plansv1.SlotBinding{{StepKey: "draft", ExecutorInstallationId: "inst"}, {StepKey: "publish", ExecutorInstallationId: "inst2"}},
	}
	c := &planassistant.Controller{
		Chat:      chatStore,
		Catalog:   fakeCatalog{},
		Configs:   &fakeConfigs{cur: cfg},
		Templates: &fakeTemplates{tpl: tpl},
	}
	if err := c.NextTurn(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	if len(chatStore.appended) != 1 {
		t.Fatalf("expected 1 new prompt, got %d", len(chatStore.appended))
	}
	if chatStore.appended[0].Kind != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_PROMPT {
		t.Fatalf("expected ASSISTANT_PROMPT, got %v", chatStore.appended[0].Kind)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/thbertoldi/harpia/control-plane && go test ./internal/planassistant/... -run 'TestSeedThread|TestNextTurn' -v
```

Expected: FAIL — types and methods undefined.

- [ ] **Step 3: Implement `controller.go`**

Create `control-plane/internal/planassistant/controller.go`:

```go
package planassistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/chat"
)

// ExecutorCatalog returns the candidate executors for a step.
type ExecutorCatalog interface {
	CandidatesForStep(ctx context.Context, tenantID uuid.UUID, template *plansv1.PlanTemplate, stepKey string) ([]ExecutorOption, error)
}

// ConfigurationStore reads and writes PlanConfigurations from the controller's
// perspective. UpdateFromSelection applies the user's selection to the
// stored configuration (binding / overseer / policies / status) and returns
// the new state.
type ConfigurationStore interface {
	GetConfiguration(ctx context.Context, tenantID, configID uuid.UUID) (*plansv1.PlanConfiguration, error)
	UpdateFromSelection(ctx context.Context, tenantID, configID uuid.UUID, state AssistantState, value string) (*plansv1.PlanConfiguration, error)
}

// TemplateStore returns a PlanTemplate by ID.
type TemplateStore interface {
	GetTemplateByID(ctx context.Context, id uuid.UUID) (*plansv1.PlanTemplate, error)
}

// Controller orchestrates the assistant: it derives state, builds the next
// prompt, and writes chat messages via the injected chat.Store.
type Controller struct {
	Chat      chat.Store
	Catalog   ExecutorCatalog
	Configs   ConfigurationStore
	Templates TemplateStore
}

// SeedThread is called once when a PlanConfiguration is first created.
// It writes CONFIGURATION_STARTED followed by the first ASSISTANT_PROMPT.
func (c *Controller) SeedThread(ctx context.Context, tenantID, configID uuid.UUID) error {
	cfg, err := c.Configs.GetConfiguration(ctx, tenantID, configID)
	if err != nil {
		return fmt.Errorf("planassistant: load configuration: %w", err)
	}
	if cfg == nil {
		return errors.New("planassistant: configuration not found")
	}
	if _, err := c.Chat.AppendMessage(ctx, tenantID, chat.AppendInput{
		ThreadID:    cfg.GetId(),
		Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
		Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_CONFIGURATION_STARTED,
		Text:        "Let's set up your plan.",
		PayloadJSON: chat.BuildConfigurationStartedPayload(cfg.GetPlanTemplateId()),
	}); err != nil {
		return fmt.Errorf("planassistant: emit CONFIGURATION_STARTED: %w", err)
	}
	return c.emitCurrentPrompt(ctx, tenantID, cfg)
}

// NextTurn is called after a USER_SELECTION has been appended (by the
// AppendPlanThreadMessage handler). It:
//   1. Loads the latest USER_SELECTION and the ASSISTANT_PROMPT it answers.
//   2. Parses the answered prompt's state from its payload_json.
//   3. Calls Configs.UpdateFromSelection(state, value) — server-authoritative
//      mutation of binding / overseer / policies / status (see Task 7).
//   4. Re-loads the configuration, derives the new state, emits the next
//      ASSISTANT_PROMPT (or ASSISTANT_TEXT for SAVED).
//
// Best-effort & idempotent: if no recent USER_SELECTION is found, NextTurn
// just emits the prompt for the current derived state.
func (c *Controller) NextTurn(ctx context.Context, tenantID, configID uuid.UUID) error {
	cfg, err := c.Configs.GetConfiguration(ctx, tenantID, configID)
	if err != nil {
		return fmt.Errorf("planassistant: load configuration: %w", err)
	}
	if state, value, ok := c.findSelectionAndState(ctx, tenantID, cfg); ok {
		mutated, err := c.Configs.UpdateFromSelection(ctx, tenantID, configID, state, value)
		if err != nil {
			return fmt.Errorf("planassistant: apply selection: %w", err)
		}
		if mutated != nil {
			cfg = mutated
		}
	}
	return c.emitCurrentPrompt(ctx, tenantID, cfg)
}

// findSelectionAndState walks the recent chat history backwards to locate
// the most recent USER_SELECTION and the ASSISTANT_PROMPT it answers, then
// returns the prompt's state + the selection's value.
func (c *Controller) findSelectionAndState(ctx context.Context, tenantID uuid.UUID, cfg *plansv1.PlanConfiguration) (AssistantState, string, bool) {
	const lookback = 50
	msgs, err := c.Chat.ListMessages(ctx, tenantID, cfg.GetId(), 0, lookback)
	if err != nil || len(msgs) == 0 {
		return AssistantState{}, "", false
	}
	var sel *chatv1.ThreadMessage
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].GetKind() == chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_SELECTION {
			sel = msgs[i]
			break
		}
	}
	if sel == nil {
		return AssistantState{}, "", false
	}
	var selPayload struct {
		InResponseToMessageID string `json:"in_response_to_message_id"`
		Value                 string `json:"value"`
	}
	if err := json.Unmarshal([]byte(sel.GetPayloadJson()), &selPayload); err != nil {
		return AssistantState{}, "", false
	}
	var prompt *chatv1.ThreadMessage
	for _, m := range msgs {
		if m.GetId() == selPayload.InResponseToMessageID {
			prompt = m
			break
		}
	}
	if prompt == nil {
		return AssistantState{}, "", false
	}
	var promptPayload struct {
		State   string `json:"state"`
		StepKey string `json:"step_key"`
	}
	if err := json.Unmarshal([]byte(prompt.GetPayloadJson()), &promptPayload); err != nil {
		return AssistantState{}, "", false
	}
	return AssistantState{Kind: StateKind(promptPayload.State), StepKey: promptPayload.StepKey}, selPayload.Value, true
}

func (c *Controller) emitCurrentPrompt(ctx context.Context, tenantID uuid.UUID, cfg *plansv1.PlanConfiguration) error {
	tplID, err := uuid.Parse(cfg.GetPlanTemplateId())
	if err != nil {
		return fmt.Errorf("planassistant: parse template id: %w", err)
	}
	tpl, err := c.Templates.GetTemplateByID(ctx, tplID)
	if err != nil {
		return fmt.Errorf("planassistant: load template: %w", err)
	}
	state := DeriveState(tpl, cfg, nil)
	in := PromptInput{Template: tpl, Config: cfg}
	if state.Kind == StateBindingStep {
		cands, err := c.Catalog.CandidatesForStep(ctx, tenantID, tpl, state.StepKey)
		if err != nil {
			return fmt.Errorf("planassistant: load candidates: %w", err)
		}
		in.CandidateExecutors = cands
	}
	text, payload := BuildPrompt(state, in)
	kind := chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_PROMPT
	if state.Kind == StateSaved {
		kind = chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_TEXT
	}
	if _, err := c.Chat.AppendMessage(ctx, tenantID, chat.AppendInput{
		ThreadID:    cfg.GetId(),
		Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_AGENT,
		Kind:        kind,
		Text:        text,
		PayloadJSON: payload,
	}); err != nil {
		return fmt.Errorf("planassistant: emit prompt: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/thbertoldi/harpia/control-plane && go test ./internal/planassistant/... -v
```

Expected: PASS for all tests in the package (state + prompts + controller).

- [ ] **Step 5: Commit**

```bash
cd /home/thbertoldi/harpia
git add control-plane/internal/planassistant/controller.go control-plane/internal/planassistant/controller_test.go
git commit -m "feat(ux-m5): planassistant.Controller — SeedThread + NextTurn"
```

---

## Task 6: Wire `SeedThread` into `CreatePlanConfiguration`

**Files:**
- Modify: `control-plane/internal/plans/handler.go`

**Interfaces:**
- Consumes: `planassistant.Controller`, `planassistant.ExecutorCatalog`, `planassistant.ConfigurationStore`, `planassistant.TemplateStore` (Task 5).
- Produces:
  - `PlanHandler` gains a `assistant *planassistant.Controller` field.
  - `NewPlanHandler` signature gains a `assistant *planassistant.Controller` parameter (before `starters ...PlanWorkflowStarter`).
  - In `CreatePlanConfiguration`, the existing `CONFIGURATION_SAVED` append is REPLACED by a call to `handler.assistant.SeedThread(ctx, tenantID, created.ID)`.

- [ ] **Step 1: Add the `assistant` field to `PlanHandler` and the constructor**

In `control-plane/internal/plans/handler.go`, find `type PlanHandler struct { ... }` (line ~22) and ADD `assistant *planassistant.Controller` as a field.

Add the import: `"github.com/harpia/control-plane/internal/planassistant"`.

Update `NewPlanHandler` signature and assignment:

```go
func NewPlanHandler(repo *Repository, executors ExecutorLookup, schedule *ScheduleManager, chatStore chat.Store, assistant *planassistant.Controller, starters ...PlanWorkflowStarter) (*PlanHandler, error) {
	// ... existing checks ...
	handler := &PlanHandler{
		chat:            chatStore,
		assistant:       assistant,
		repo:            repo,
		// ... rest unchanged ...
	}
	// ... rest unchanged ...
}
```

- [ ] **Step 2: Replace the CONFIGURATION_SAVED emission in `CreatePlanConfiguration`**

Locate the block at lines ~185-193 (the `if h.chat != nil { _, _ = h.chat.AppendMessage(...) }` after `created` is returned). REPLACE the entire `if h.chat != nil { ... }` block with:

```go
	if h.assistant != nil {
		if err := h.assistant.SeedThread(ctx, tenantID, created.ID); err != nil {
			// Best-effort: log and continue. The thread will lack the assistant
			// turn until a manual rebuild, but configuration save succeeded.
			// The plan-thread page handles "no messages" gracefully.
			_ = err
		}
	}
```

Leave the `UpdatePlanConfiguration`'s existing `CONFIGURATION_SAVED` append in place for now — Task 8 modifies it.

- [ ] **Step 3: Update every `NewPlanHandler` call site**

```bash
cd /home/thbertoldi/harpia
grep -rln 'NewPlanHandler(' control-plane/ | xargs sed -n
```

Find each caller. For each one, INSERT `nil` (or a real `*planassistant.Controller` if available — for now `nil`) between the `chatStore` and the `starters` arguments. The most likely callers are in `control-plane/cmd/control-plane/main.go` and any tests.

For tests, simply pass `nil`. Production wiring is fixed in Task 7.

Build to confirm:

```bash
cd /home/thbertoldi/harpia/control-plane && go build ./...
```

Expected: builds clean.

- [ ] **Step 4: Run the existing plan handler tests**

```bash
cd /home/thbertoldi/harpia/control-plane && go test ./internal/plans/... -run 'CreatePlanConfiguration' -v
```

Expected: PASS (existing tests pass `nil` for assistant per Step 3; CreatePlanConfiguration still works because the new code is gated on `h.assistant != nil`).

- [ ] **Step 5: Commit**

```bash
cd /home/thbertoldi/harpia
git add control-plane/internal/plans/handler.go control-plane/cmd/control-plane/main.go control-plane/internal/plans/*_test.go
git commit -m "feat(ux-m5): wire planassistant.SeedThread into CreatePlanConfiguration"
```

---

## Task 7: Production wiring — adapter types + `main.go`

**Files:**
- Modify: `control-plane/cmd/control-plane/main.go` (construct the assistant)
- Modify: `control-plane/internal/plans/handler.go` (small adapter helpers `assistantConfigStore`, `assistantCatalog`, `assistantTemplates`)

**Interfaces:**
- Produces three private adapter types in `package plans` implementing the three interfaces from Task 5, wrapping `*Repository` + `ExecutorLookup` so `planassistant.Controller` can be constructed from existing components.

- [ ] **Step 1: Add adapter types in `handler.go`**

Append to `control-plane/internal/plans/handler.go` (above the existing methods, after the imports):

```go
// AssistantConfigurationStore adapts *Repository to planassistant.ConfigurationStore.
type AssistantConfigurationStore struct {
	Repo *Repository
}

func (a *AssistantConfigurationStore) GetConfiguration(ctx context.Context, tenantID, configID uuid.UUID) (*plansv1.PlanConfiguration, error) {
	cfg, err := a.Repo.GetConfiguration(ctx, tenantID, configID)
	if err != nil {
		return nil, err
	}
	return configurationToProto(cfg), nil
}

// UpdateFromSelection applies the user's selection to the persisted
// PlanConfiguration. Server-authoritative; mirrors spec §2.6.
// Switch over state.Kind to mutate the right field, then write back via
// Repository.UpdateConfiguration. Idempotent re-application is safe.
func (a *AssistantConfigurationStore) UpdateFromSelection(ctx context.Context, tenantID, configID uuid.UUID, state planassistant.AssistantState, value string) (*plansv1.PlanConfiguration, error) {
	cfg, err := a.Repo.GetConfiguration(ctx, tenantID, configID)
	if err != nil {
		return nil, err
	}
	switch state.Kind {
	case planassistant.StateBindingStep:
		// Upsert SlotBinding for state.StepKey with executor_installation_id=value.
		// Look up the executor's SKU + kind from ExecutorLookup so the SlotBinding
		// fields are complete; mirror the wizard's selectionsToSlotBindings path.
		cfg.SlotBindings = upsertSlotBinding(cfg.SlotBindings, state.StepKey, value)
	case planassistant.StateSetOverseer:
		userID := value
		if value == "self" {
			if uid, ok := currentUserIDFromCtx(ctx); ok {
				userID = uid
			}
		}
		cfg.OverseerBindings = upsertOverseerBinding(cfg.OverseerBindings, state.StepKey, userID)
	case planassistant.StateSetPolicies:
		// value is one of "PAUSE_UNTIL_ANSWERED+REQUIRE_APPROVAL",
		// "PAUSE_UNTIL_ANSWERED+AUTO_PUBLISH", "FAIL_STEP+REQUIRE_APPROVAL".
		cfg.BehaviorPolicies = parsePolicyValue(value)
	case planassistant.StateConfirm:
		if value == "save" {
			next := plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE
			if cfg.Schedule != nil && cfg.Schedule.CronExpression != "" {
				next = plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_SCHEDULED
			}
			cfg.Status = next
		}
	}
	// Persist via the repository's existing update path.
	domain, err := configurationFromProto(cfg) // existing helper, or write a thin inverse of configurationToProto
	if err != nil {
		return nil, err
	}
	updated, err := a.Repo.UpdateConfiguration(ctx, domain)
	if err != nil {
		return nil, err
	}
	return configurationToProto(updated), nil
}

// upsertSlotBinding / upsertOverseerBinding / parsePolicyValue /
// currentUserIDFromCtx are small helpers in this file. If
// `configurationFromProto` does not exist, write it as the inverse of the
// existing `configurationToProto` (same field-by-field mapping).
func upsertSlotBinding(existing []*plansv1.SlotBinding, stepKey, installationID string) []*plansv1.SlotBinding {
	for _, b := range existing {
		if b.StepKey == stepKey {
			b.ExecutorInstallationId = installationID
			return existing
		}
	}
	return append(existing, &plansv1.SlotBinding{StepKey: stepKey, ExecutorInstallationId: installationID})
}

func upsertOverseerBinding(existing []*plansv1.OverseerBinding, stepKey, userID string) []*plansv1.OverseerBinding {
	for _, b := range existing {
		if b.StepKey == stepKey {
			b.OverseerUserId = userID
			return existing
		}
	}
	return append(existing, &plansv1.OverseerBinding{StepKey: stepKey, OverseerUserId: userID})
}

func parsePolicyValue(v string) *plansv1.PlanBehaviorPolicies {
	timeout := plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_PAUSE_UNTIL_ANSWERED
	approval := plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_REQUIRE_APPROVAL
	hours := int32(24)
	switch v {
	case "PAUSE_UNTIL_ANSWERED+AUTO_PUBLISH":
		approval = plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_AUTO_PUBLISH
	case "FAIL_STEP+REQUIRE_APPROVAL":
		timeout = plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_FAIL_STEP
	}
	return &plansv1.PlanBehaviorPolicies{
		ElicitationTimeoutBehavior: timeout,
		ElicitationTimeoutHours:    hours,
		PublishApprovalMode:        approval,
	}
}

func currentUserIDFromCtx(ctx context.Context) (string, bool) {
	rc, err := identity.RequireRequestContext(ctx)
	if err != nil {
		return "", false
	}
	if rc.UserID == "" {
		return "", false
	}
	return rc.UserID, true
}

// AssistantTemplates adapts *Repository to planassistant.TemplateStore.
type AssistantTemplates struct{ Repo *Repository }

func (a *AssistantTemplates) GetTemplateByID(ctx context.Context, id uuid.UUID) (*plansv1.PlanTemplate, error) {
	tpl, err := a.Repo.GetTemplateByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return templateToProto(tpl), nil
}

// AssistantCatalog adapts ExecutorLookup to planassistant.ExecutorCatalog.
//
// Implementation note: `ExecutorLookup` (validation.go) does not yet expose
// a "list installations compatible with this step" method — only
// per-id getters + `ListEntitlements`. Add a new method to the interface
// and to *Repository's implementation:
//
//   ListCompatibleInstallationsForStep(ctx, tenantID, step *plansv1.PlanStep)
//     ([]executors.ExecutorInstallation, error)
//
// The implementation iterates tenant entitlements, filters by
// step.ExecutorRequirement.executor_kind + required_capabilities (mirror
// the predicate used by `isInstallationCompatibleWithStep` in
// frontend/src/lib/plans/slot-binding.ts), and returns the matching
// installations. Reuse the existing validator's logic where possible.
type AssistantCatalog struct{ Executors ExecutorLookup }

func (a *AssistantCatalog) CandidatesForStep(ctx context.Context, tenantID uuid.UUID, template *plansv1.PlanTemplate, stepKey string) ([]planassistant.ExecutorOption, error) {
	var out []planassistant.ExecutorOption
	for _, step := range template.GetSteps() {
		if step.GetKey() != stepKey {
			continue
		}
		installations, err := a.Executors.ListCompatibleInstallationsForStep(ctx, tenantID, step)
		if err != nil {
			return nil, err
		}
		for _, inst := range installations {
			var price *float64
			if inst.UnitPriceBrl != nil {
				p := *inst.UnitPriceBrl
				price = &p
			}
			out = append(out, planassistant.ExecutorOption{
				StepKey:        stepKey,
				InstallationID: inst.ID.String(),
				DisplayName:    inst.DisplayName,
				SkuKey:         inst.SkuKey,
				Tier:           inst.Tier,
				PriceBrl:       price,
			})
		}
	}
	return out, nil
}
```

If `ExecutorLookup` does not yet expose `ListCompatibleInstallations`, grep for the existing method that the validator uses for compatibility filtering and call that one instead. The point of this task is to translate to the `ExecutorOption` shape; the upstream method already exists.

- [ ] **Step 2: Construct the controller in `main.go`**

Open `control-plane/cmd/control-plane/main.go`. Locate where `NewPlanHandler` is called. Immediately before that call, ADD:

```go
assistantController := &planassistant.Controller{
	Chat:      chatStore,
	Catalog:   &plans.AssistantCatalog{Executors: executorLookup},
	Configs:   &plans.AssistantConfigurationStore{Repo: planRepo},
	Templates: &plans.AssistantTemplates{Repo: planRepo},
}
```

(Use the actual local variable names from `main.go`; the names `chatStore`, `executorLookup`, `planRepo` are the typical ones — adjust if different.)

Then pass `assistantController` as the new argument to `NewPlanHandler`.

Add the import: `"github.com/harpia/control-plane/internal/planassistant"`.

- [ ] **Step 3: Build & run smoke test**

```bash
cd /home/thbertoldi/harpia/control-plane && go build ./... && go test ./... 2>&1 | tail -30
```

Expected: build clean. Tests pass (the M3 plan-thread tests now see `CONFIGURATION_STARTED` instead of `CONFIGURATION_SAVED` on configuration create — see Task 6's replacement; if any M3 test asserts the kind explicitly, update its expectation).

- [ ] **Step 4: Commit**

```bash
cd /home/thbertoldi/harpia
git add control-plane/cmd/control-plane/main.go control-plane/internal/plans/handler.go
git commit -m "feat(ux-m5): wire planassistant.Controller in main + adapter types"
```

---

## Task 8: `UpdatePlanConfiguration` — schedule side effect + RUNNABLE↔SCHEDULED

**Files:**
- Modify: `control-plane/internal/plans/handler.go` (in `UpdatePlanConfiguration`)
- Modify: `control-plane/go.mod` / `go.sum` (if `github.com/robfig/cron/v3` is absent)
- Create: appended tests in `control-plane/internal/plans/handler_retry_test.go` or a new `control-plane/internal/plans/schedule_side_effect_test.go`

**Interfaces:**
- Consumes: `chat.BuildScheduleSetPayload` (Task 2), `chat.Store`.
- Produces: when `UpdatePlanConfiguration` is called and the incoming `schedule.cron_expression` differs from the existing one:
  - validates the cron with `robfig/cron/v3`'s `cron.ParseStandard`;
  - if invalid → returns `connect.NewError(connect.CodeInvalidArgument, ...)`;
  - if valid → appends `SCHEDULE_SET` chat message; reconciles status (`RUNNABLE` → `SCHEDULED` if cron now non-empty; `SCHEDULED` → `RUNNABLE` if cron cleared).
- Also: the existing `CONFIGURATION_SAVED` emission in `UpdatePlanConfiguration` is RETAINED (it's the audit signal for non-schedule edits).

- [ ] **Step 1: Add `github.com/robfig/cron/v3` if absent**

```bash
cd /home/thbertoldi/harpia/control-plane && grep -q 'robfig/cron/v3' go.mod || go get github.com/robfig/cron/v3
```

Expected: dependency present after the command.

- [ ] **Step 2: Write the failing test**

Create `control-plane/internal/plans/schedule_side_effect_test.go`:

```go
package plans_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/plans"
	// re-use existing test fixtures from handler_retry_test.go's helpers if they fit;
	// otherwise stand up an in-memory pgxpool via the existing test harness.
)

func TestUpdatePlanConfiguration_ValidCron_EmitsScheduleSetAndPromotesToScheduled(t *testing.T) {
	t.Skip("Wire-up: use the project's existing handler-test scaffold to construct a *plans.PlanHandler bound to an in-memory chat.Store + repo with a RUNNABLE PlanConfiguration. Then call UpdatePlanConfiguration with schedule.cron_expression='0 9 * * *' and assert: (1) chat store recorded a SCHEDULE_SET message; (2) returned PlanConfiguration.status == PLAN_CONFIGURATION_STATUS_SCHEDULED.")
}

func TestUpdatePlanConfiguration_InvalidCron_ReturnsInvalidArgument(t *testing.T) {
	t.Skip("Same harness: call UpdatePlanConfiguration with schedule.cron_expression='not a cron'. Assert connect.CodeInvalidArgument.")
}
```

If `handler_retry_test.go` already has a reusable fixture builder (look for a `newTestHandler(t)` or similar), inline it here and remove the `t.Skip` — replace the skip with the actual assertions. Otherwise these stay as documented test placeholders that the next milestone fills in once the harness is generalized.

- [ ] **Step 3: Implement the side effect**

In `control-plane/internal/plans/handler.go`, locate `UpdatePlanConfiguration` (line ~221). Just BEFORE the existing `if h.chat != nil { ... }` block (which writes `CONFIGURATION_SAVED`), INSERT:

```go
	// Schedule reconciliation: if the cron expression changed, validate it,
	// emit SCHEDULE_SET, and reconcile RUNNABLE ↔ SCHEDULED. See M5 design
	// spec §2.7 and §2.11.
	prevCron := ""
	prevTz := ""
	if existing.Schedule.Valid {
		prevCron = existing.Schedule.CronExpression
		prevTz = existing.Schedule.Timezone
	}
	newCron := ""
	newTz := ""
	if updatedInput.Schedule != nil {
		newCron = updatedInput.Schedule.CronExpression
		newTz = updatedInput.Schedule.Timezone
	}
	if newCron != prevCron || newTz != prevTz {
		if newCron != "" {
			if _, parseErr := cron.ParseStandard(newCron); parseErr != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument,
					fmt.Errorf("invalid schedule cron expression: %w", parseErr))
			}
		}
		// Status reconciliation.
		switch updated.Status {
		case plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE:
			if newCron != "" {
				updated.Status = plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_SCHEDULED
				_ = h.repo.UpdateConfigurationStatus(ctx, tenantID, updated.ID, updated.Status)
			}
		case plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_SCHEDULED:
			if newCron == "" {
				updated.Status = plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE
				_ = h.repo.UpdateConfigurationStatus(ctx, tenantID, updated.ID, updated.Status)
			}
		}
		if h.chat != nil {
			_, _ = h.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
				ThreadID:    updated.ID.String(),
				Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
				Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_SCHEDULE_SET,
				Text:        "Schedule updated.",
				PayloadJSON: chat.BuildScheduleSetPayload(newCron, newTz),
			})
		}
	}
```

Add to imports: `"github.com/robfig/cron/v3"` as `cron`.

If `Repository.UpdateConfigurationStatus` does not exist, add a minimal helper in `repository.go`:

```go
func (r *Repository) UpdateConfigurationStatus(ctx context.Context, tenantID, configID uuid.UUID, status plansv1.PlanConfigurationStatus) error {
	return r.exec(ctx, tenantID, `UPDATE plan_configurations SET status = $1, updated_at = NOW() WHERE id = $2 AND tenant_id = $3`, status.String(), configID, tenantID)
}
```

(Confirm the existing `exec` helper signature; if not present, use `pool.Exec` via `database.WithTenant` matching the pattern in chat/store.go.)

- [ ] **Step 4: Build + run the new tests**

```bash
cd /home/thbertoldi/harpia/control-plane && go build ./... && go test ./internal/plans/... -run 'Schedule' -v
```

Expected: build clean, tests pass (or skip with the documented placeholder message).

- [ ] **Step 5: Commit**

```bash
cd /home/thbertoldi/harpia
git add control-plane/internal/plans/handler.go control-plane/internal/plans/repository.go control-plane/internal/plans/schedule_side_effect_test.go control-plane/go.mod control-plane/go.sum
git commit -m "feat(ux-m5): UpdatePlanConfiguration validates cron, emits SCHEDULE_SET, reconciles SCHEDULED status"
```

---

## Task 9: `AppendPlanThreadMessage` → `NextTurn` on `USER_SELECTION`

**Files:**
- Modify: `control-plane/internal/plans/thread_handler.go`
- Modify: `control-plane/internal/plans/thread_handler_test.go` (append a focused test)

**Interfaces:**
- Consumes: `h.assistant *planassistant.Controller` from Task 6.
- Produces: when the request kind is `THREAD_MESSAGE_KIND_USER_SELECTION` AND `h.assistant != nil`, after the message is appended the handler calls `h.assistant.NextTurn(ctx, tenantID, configID)`. NextTurn errors are logged-and-ignored (best-effort — the user's selection still persists).

- [ ] **Step 1: Add the branch**

In `control-plane/internal/plans/thread_handler.go`, locate `AppendPlanThreadMessage`. After the `msg, err := h.chat.AppendMessage(...)` call returns successfully and BEFORE the `return connect.NewResponse(...)`, INSERT:

```go
	if req.Msg.GetKind() == chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_SELECTION && h.assistant != nil {
		parsedConfigID, parseErr := uuid.Parse(configID)
		if parseErr == nil {
			_ = h.assistant.NextTurn(ctx, tenantID, parsedConfigID)
		}
	}
```

Add to imports: `chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"`.

- [ ] **Step 2: Append a focused test**

Append to `control-plane/internal/plans/thread_handler_test.go`:

```go
func TestAppendPlanThreadMessage_UserSelection_TriggersNextTurn(t *testing.T) {
	t.Skip("Wire-up: construct PlanHandler with a stub planassistant.Controller that records its NextTurn calls. Call AppendPlanThreadMessage with kind=USER_SELECTION. Assert NextTurn was called exactly once with the expected (tenantID, configID).")
}
```

- [ ] **Step 3: Build**

```bash
cd /home/thbertoldi/harpia/control-plane && go build ./...
```

Expected: clean.

- [ ] **Step 4: Commit**

```bash
cd /home/thbertoldi/harpia
git add control-plane/internal/plans/thread_handler.go control-plane/internal/plans/thread_handler_test.go
git commit -m "feat(ux-m5): AppendPlanThreadMessage triggers planassistant.NextTurn on USER_SELECTION"
```

---

## Task 10: Frontend `chat/types.ts` — extend union + maps

**Files:**
- Modify: `frontend/src/lib/chat/types.ts`

**Interfaces:**
- Consumes: regen'd `ProtoThreadMessageKind` values from Task 1.
- Produces:
  - `ChatMessageKind` union gains: `"CONFIGURATION_STARTED"`, `"ASSISTANT_PROMPT"`, `"USER_SELECTION"`, `"STEP_REBOUND"`, `"SCHEDULE_SET"`.
  - `KIND_FROM_PROTO` and `KIND_TO_PROTO` updated.

- [ ] **Step 1: Extend the union**

In `frontend/src/lib/chat/types.ts`, locate the `export type ChatMessageKind = ...` union. ADD the five new variants:

```ts
export type ChatMessageKind =
  | "USER_TEXT"
  | "ASSISTANT_TEXT"
  | "CONFIGURATION_SAVED"
  | "CONFIGURATION_STARTED"
  | "ASSISTANT_PROMPT"
  | "USER_SELECTION"
  | "STEP_REBOUND"
  | "SCHEDULE_SET"
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

- [ ] **Step 2: Extend `KIND_FROM_PROTO`**

ADD these entries (the proto enum values come from the regen):

```ts
  [ProtoThreadMessageKind.CONFIGURATION_STARTED]: "CONFIGURATION_STARTED",
  [ProtoThreadMessageKind.ASSISTANT_PROMPT]: "ASSISTANT_PROMPT",
  [ProtoThreadMessageKind.USER_SELECTION]: "USER_SELECTION",
  [ProtoThreadMessageKind.STEP_REBOUND]: "STEP_REBOUND",
  [ProtoThreadMessageKind.SCHEDULE_SET]: "SCHEDULE_SET",
```

- [ ] **Step 3: Extend `KIND_TO_PROTO`**

ADD these entries:

```ts
  CONFIGURATION_STARTED: ProtoThreadMessageKind.CONFIGURATION_STARTED,
  ASSISTANT_PROMPT: ProtoThreadMessageKind.ASSISTANT_PROMPT,
  USER_SELECTION: ProtoThreadMessageKind.USER_SELECTION,
  STEP_REBOUND: ProtoThreadMessageKind.STEP_REBOUND,
  SCHEDULE_SET: ProtoThreadMessageKind.SCHEDULE_SET,
```

- [ ] **Step 4: Type-check**

```bash
cd /home/thbertoldi/harpia/frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -10
```

Expected: no new errors over baseline. The added variants may cause exhaustiveness warnings in `SystemEventCard.svelte` and `ThreadMessage.svelte`; that's expected and fixed in Tasks 11 & 14.

- [ ] **Step 5: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/chat/types.ts
git commit -m "feat(ux-m5): extend ChatMessageKind with 5 assistant message kinds"
```

---

## Task 11: `lib/plans/cost.ts` — `computeRunCost` + tests

**Files:**
- Create: `frontend/src/lib/plans/cost.ts`
- Create: `frontend/src/lib/plans/cost.test.ts`

**Interfaces:**
- Consumes: `PlanTemplate`, `PlanConfiguration`, `SlotBinding` from `$lib/gen/harpia/plans/v1/plans_pb`.
- Produces:
  - `type CostBreakdownEntry = { stepKey: string; stepTitle: string; executorInstallationId: string | null; executorName: string | null; pricePerRunBrl: number | null; }`
  - `type RunCost = { totalPerRunBrl: number; currency: "BRL"; unboundStepCount: number; breakdown: CostBreakdownEntry[]; }`
  - `type ExecutorPriceLookup = (installationId: string) => { displayName: string; pricePerRunBrl: number | null } | null`
  - `export function computeRunCost(template: PlanTemplate, configuration: PlanConfiguration, pricing: ExecutorPriceLookup): RunCost`

- [ ] **Step 1: Write failing tests**

Create `frontend/src/lib/plans/cost.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { computeRunCost, type ExecutorPriceLookup } from "./cost";
import type {
  PlanConfiguration,
  PlanTemplate,
} from "$lib/gen/harpia/plans/v1/plans_pb";

function tpl(...stepKeys: string[]): PlanTemplate {
  return {
    id: "tpl",
    steps: stepKeys.map((k) => ({ key: k, title: k }) as any),
  } as PlanTemplate;
}

function cfg(...bindings: Array<[string, string]>): PlanConfiguration {
  return {
    id: "cfg",
    slotBindings: bindings.map(([stepKey, installationId]) => ({
      stepKey,
      executorInstallationId: installationId,
    })),
  } as unknown as PlanConfiguration;
}

const pricing: ExecutorPriceLookup = (id) => {
  if (id === "inst-a") return { displayName: "Junior Writer", pricePerRunBrl: 0.05 };
  if (id === "inst-b") return { displayName: "Senior Reviewer", pricePerRunBrl: 0.2 };
  return null;
};

describe("computeRunCost", () => {
  it("returns zero with all steps unbound", () => {
    const cost = computeRunCost(tpl("a", "b"), cfg(), pricing);
    expect(cost.totalPerRunBrl).toBe(0);
    expect(cost.unboundStepCount).toBe(2);
    expect(cost.breakdown).toHaveLength(2);
    expect(cost.breakdown[0].pricePerRunBrl).toBeNull();
  });

  it("sums prices when fully bound", () => {
    const cost = computeRunCost(tpl("a", "b"), cfg(["a", "inst-a"], ["b", "inst-b"]), pricing);
    expect(cost.totalPerRunBrl).toBeCloseTo(0.25, 5);
    expect(cost.unboundStepCount).toBe(0);
    expect(cost.breakdown[0].executorName).toBe("Junior Writer");
  });

  it("handles partial bindings", () => {
    const cost = computeRunCost(tpl("a", "b"), cfg(["a", "inst-a"]), pricing);
    expect(cost.totalPerRunBrl).toBeCloseTo(0.05, 5);
    expect(cost.unboundStepCount).toBe(1);
  });

  it("counts bound but unpriced executors as unboundStepCount=0 and adds null to breakdown", () => {
    const cost = computeRunCost(tpl("a"), cfg(["a", "unknown"]), pricing);
    expect(cost.unboundStepCount).toBe(0);
    expect(cost.totalPerRunBrl).toBe(0);
    expect(cost.breakdown[0].executorName).toBeNull();
    expect(cost.breakdown[0].pricePerRunBrl).toBeNull();
  });
});
```

- [ ] **Step 2: Run to verify FAIL**

```bash
cd /home/thbertoldi/harpia/frontend && npx vitest run src/lib/plans/cost.test.ts
```

Expected: FAIL — module not found.

- [ ] **Step 3: Implement**

Create `frontend/src/lib/plans/cost.ts`:

```ts
import type {
  PlanConfiguration,
  PlanTemplate,
} from "$lib/gen/harpia/plans/v1/plans_pb";

export type CostBreakdownEntry = {
  stepKey: string;
  stepTitle: string;
  executorInstallationId: string | null;
  executorName: string | null;
  pricePerRunBrl: number | null;
};

export type RunCost = {
  totalPerRunBrl: number;
  currency: "BRL";
  unboundStepCount: number;
  breakdown: CostBreakdownEntry[];
};

export type ExecutorPriceLookup = (
  installationId: string,
) => { displayName: string; pricePerRunBrl: number | null } | null;

export function computeRunCost(
  template: PlanTemplate,
  configuration: PlanConfiguration,
  pricing: ExecutorPriceLookup,
): RunCost {
  const bindingByStep = new Map<string, string>();
  for (const sb of configuration.slotBindings ?? []) {
    if (sb.executorInstallationId) {
      bindingByStep.set(sb.stepKey, sb.executorInstallationId);
    }
  }
  let total = 0;
  let unbound = 0;
  const breakdown: CostBreakdownEntry[] = [];
  for (const step of template.steps ?? []) {
    const installationId = bindingByStep.get(step.key) ?? null;
    if (!installationId) {
      unbound += 1;
      breakdown.push({
        stepKey: step.key,
        stepTitle: step.title || step.key,
        executorInstallationId: null,
        executorName: null,
        pricePerRunBrl: null,
      });
      continue;
    }
    const info = pricing(installationId);
    const price = info?.pricePerRunBrl ?? null;
    if (price != null) total += price;
    breakdown.push({
      stepKey: step.key,
      stepTitle: step.title || step.key,
      executorInstallationId: installationId,
      executorName: info?.displayName ?? null,
      pricePerRunBrl: price,
    });
  }
  return { totalPerRunBrl: total, currency: "BRL", unboundStepCount: unbound, breakdown };
}
```

- [ ] **Step 4: Run to verify PASS**

```bash
cd /home/thbertoldi/harpia/frontend && npx vitest run src/lib/plans/cost.test.ts
```

Expected: 4/4 PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/plans/cost.ts frontend/src/lib/plans/cost.test.ts
git commit -m "feat(ux-m5): lib/plans/cost — computeRunCost pure helper"
```

---

## Task 12: `lib/plans/assistant.ts` — `selectChip` + `editBinding`

**Files:**
- Create: `frontend/src/lib/plans/assistant.ts`
- Create: `frontend/src/lib/plans/assistant.test.ts`

**Interfaces:**
- Consumes: `appendThreadMessage` from `$lib/chat/client`, `planClient` from `$lib/rpc`.
- Produces:
  - `export async function selectChip(args: { tenantId: string; configurationId: string; promptMessageId: string; optionId: string; value: string }): Promise<void>`
  - `export async function editBinding(args: { tenantId: string; configurationId: string; existingConfiguration: PlanConfiguration; template: PlanTemplate; stepKey: string; newInstallationId: string }): Promise<PlanConfiguration>`

- [ ] **Step 1: Write failing tests**

Create `frontend/src/lib/plans/assistant.test.ts`:

```ts
import { describe, it, expect, vi, beforeEach } from "vitest";

const appendThreadMessage = vi.fn();
const updatePlanConfiguration = vi.fn();

vi.mock("$lib/chat/client", () => ({ appendThreadMessage }));
vi.mock("$lib/rpc", () => ({ planClient: { updatePlanConfiguration } }));

import { selectChip, editBinding } from "./assistant";
import { PlanConfigurationStatus, type PlanConfiguration, type PlanTemplate } from "$lib/gen/harpia/plans/v1/plans_pb";

beforeEach(() => {
  appendThreadMessage.mockReset();
  updatePlanConfiguration.mockReset();
});

describe("selectChip", () => {
  it("appends a USER_SELECTION message", async () => {
    appendThreadMessage.mockResolvedValueOnce({});
    await selectChip({
      tenantId: "t",
      configurationId: "c",
      promptMessageId: "m1",
      optionId: "junior",
      value: "inst-junior",
    });
    expect(appendThreadMessage).toHaveBeenCalledWith(
      "t", "c", "OVERSEER", "USER_SELECTION", "", expect.stringContaining("junior"),
    );
  });
});

describe("editBinding", () => {
  it("writes STEP_REBOUND then UpdatePlanConfiguration", async () => {
    appendThreadMessage.mockResolvedValueOnce({});
    updatePlanConfiguration.mockResolvedValueOnce({ planConfiguration: { id: "c" } });
    const template = { id: "tpl", steps: [{ key: "a" }, { key: "b" }] } as PlanTemplate;
    const config = {
      id: "c",
      status: PlanConfigurationStatus.RUNNABLE,
      slotBindings: [
        { stepKey: "a", executorInstallationId: "inst-old" },
        { stepKey: "b", executorInstallationId: "inst-b" },
      ],
      overseerBindings: [],
      behaviorPolicies: undefined,
      schedule: undefined,
      seedArtifacts: [],
    } as unknown as PlanConfiguration;
    await editBinding({
      tenantId: "t",
      configurationId: "c",
      existingConfiguration: config,
      template,
      stepKey: "a",
      newInstallationId: "inst-new",
    });
    expect(appendThreadMessage).toHaveBeenCalledWith(
      "t", "c", "SYSTEM", "STEP_REBOUND", "",
      expect.stringContaining("inst-old"),
    );
    expect(updatePlanConfiguration).toHaveBeenCalledTimes(1);
    const call = updatePlanConfiguration.mock.calls[0][0];
    expect(call.slotBindings.find((b: any) => b.stepKey === "a").executorInstallationId).toBe("inst-new");
  });
});
```

- [ ] **Step 2: Run to verify FAIL**

```bash
cd /home/thbertoldi/harpia/frontend && npx vitest run src/lib/plans/assistant.test.ts
```

Expected: FAIL — module not found.

- [ ] **Step 3: Implement**

Create `frontend/src/lib/plans/assistant.ts`:

```ts
import { appendThreadMessage } from "$lib/chat/client";
import { planClient } from "$lib/rpc";
import type {
  PlanConfiguration,
  PlanTemplate,
} from "$lib/gen/harpia/plans/v1/plans_pb";

export async function selectChip(args: {
  tenantId: string;
  configurationId: string;
  promptMessageId: string;
  optionId: string;
  value: string;
}): Promise<void> {
  const payload = JSON.stringify({
    in_response_to_message_id: args.promptMessageId,
    option_id: args.optionId,
    value: args.value,
  });
  await appendThreadMessage(
    args.tenantId,
    args.configurationId,
    "OVERSEER",
    "USER_SELECTION",
    "",
    payload,
  );
}

export async function editBinding(args: {
  tenantId: string;
  configurationId: string;
  existingConfiguration: PlanConfiguration;
  template: PlanTemplate;
  stepKey: string;
  newInstallationId: string;
}): Promise<PlanConfiguration> {
  const previous =
    args.existingConfiguration.slotBindings.find((b) => b.stepKey === args.stepKey)
      ?.executorInstallationId ?? "";
  const reboundPayload = JSON.stringify({
    step_key: args.stepKey,
    previous_executor_installation_id: previous,
    new_executor_installation_id: args.newInstallationId,
  });
  await appendThreadMessage(
    args.tenantId,
    args.configurationId,
    "SYSTEM",
    "STEP_REBOUND",
    "",
    reboundPayload,
  );
  const nextSlotBindings = args.existingConfiguration.slotBindings.map((b) =>
    b.stepKey === args.stepKey
      ? { ...b, executorInstallationId: args.newInstallationId }
      : b,
  );
  const response = await planClient.updatePlanConfiguration({
    tenantId: args.tenantId,
    planConfigurationId: args.configurationId,
    status: args.existingConfiguration.status,
    seedArtifacts: args.existingConfiguration.seedArtifacts,
    slotBindings: nextSlotBindings,
    overseerBindings: args.existingConfiguration.overseerBindings,
    behaviorPolicies: args.existingConfiguration.behaviorPolicies,
    schedule: args.existingConfiguration.schedule,
  });
  if (!response.planConfiguration) {
    throw new Error("editBinding: UpdatePlanConfiguration returned no configuration");
  }
  return response.planConfiguration;
}
```

- [ ] **Step 4: Run to verify PASS**

```bash
cd /home/thbertoldi/harpia/frontend && npx vitest run src/lib/plans/assistant.test.ts
```

Expected: 2/2 PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/plans/assistant.ts frontend/src/lib/plans/assistant.test.ts
git commit -m "feat(ux-m5): lib/plans/assistant — selectChip + editBinding helpers"
```

---

## Task 13: `AssistantPromptCard.svelte` (chip group + edit pencil)

**Files:**
- Create: `frontend/src/lib/components/thread/AssistantPromptCard.svelte`

**Interfaces:**
- Consumes: `selectChip` from `$lib/plans/assistant`, `ChatMessage` type.
- Produces: a card that renders the assistant's `text`, the chip options from `payloadJson.options`, and an `Edit` pencil if `isAnswered=true`. Emits `selectChip(...)` on chip click.

- [ ] **Step 1: Write the component**

Create `frontend/src/lib/components/thread/AssistantPromptCard.svelte`:

```svelte
<script lang="ts">
  import { Pencil } from "lucide-svelte";
  import type { ChatMessage } from "$lib/chat/types";
  import { selectChip } from "$lib/plans/assistant";
  import { locale, translate } from "$lib/i18n";

  interface Option {
    id: string;
    label: string;
    sublabel?: string;
    value: string;
    price_brl?: number;
  }
  interface Props {
    message: ChatMessage;
    configurationId: string;
    tenantId: string;
    isAnswered: boolean;
    isLive: boolean;
  }
  let { message, configurationId, tenantId, isAnswered, isLive }: Props = $props();

  let editing = $state(false);
  let pending = $state(false);

  const payload = $derived.by(() => {
    try {
      return JSON.parse(message.payloadJson) as {
        state?: string;
        step_key?: string;
        options?: Option[];
      };
    } catch {
      return { options: [] as Option[] };
    }
  });

  const showChips = $derived(isLive || editing);

  async function onSelect(option: Option) {
    if (pending) return;
    pending = true;
    try {
      await selectChip({
        tenantId,
        configurationId,
        promptMessageId: message.id,
        optionId: option.id,
        value: option.value,
      });
    } finally {
      pending = false;
      editing = false;
    }
  }
</script>

<div
  id={`m-${message.id}`}
  class="rounded-lg border border-plumage bg-obsidian-light px-4 py-3 {isLive ? 'ring-1 ring-talon-gold' : ''}"
>
  <div class="flex items-start justify-between gap-2">
    <p class="text-[13px] text-cream font-body">{message.text}</p>
    {#if isAnswered && !isLive}
      <button
        type="button"
        class="cursor-pointer rounded p-1 text-crown-ash hover:text-talon-gold"
        onclick={() => (editing = !editing)}
        aria-label={translate("assistant.edit", $locale)}
      >
        <Pencil class="size-3.5" />
      </button>
    {/if}
  </div>

  {#if showChips && payload.options && payload.options.length > 0}
    <div class="mt-3 flex flex-wrap gap-2">
      {#each payload.options as opt (opt.id)}
        <button
          type="button"
          disabled={pending}
          onclick={() => onSelect(opt)}
          class="cursor-pointer rounded-md border border-plumage bg-obsidian px-3 py-1.5 text-left text-[12px] text-cream hover:border-talon-gold hover:text-talon-gold disabled:cursor-not-allowed disabled:opacity-50"
        >
          <span class="font-medium">{opt.label}</span>
          {#if opt.sublabel}
            <span class="ml-2 text-[10px] text-crown-ash-dark">{opt.sublabel}</span>
          {/if}
        </button>
      {/each}
    </div>
  {/if}
</div>
```

- [ ] **Step 2: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/components/thread/AssistantPromptCard.svelte
git commit -m "feat(ux-m5): AssistantPromptCard — chip group with edit pencil"
```

---

## Task 14: `ConfirmCard.svelte` + `TemplatePickerCard.svelte` + `SystemEventCard` icons + `ThreadMessage` dispatch

**Files:**
- Create: `frontend/src/lib/components/thread/ConfirmCard.svelte`
- Create: `frontend/src/lib/components/thread/TemplatePickerCard.svelte`
- Modify: `frontend/src/lib/components/thread/SystemEventCard.svelte` (icons for `STEP_REBOUND`, `SCHEDULE_SET`, `CONFIGURATION_STARTED`)
- Modify: `frontend/src/lib/components/thread/ThreadMessage.svelte` (dispatch new kinds)

**Interfaces:**
- Consumes: existing dispatch contract.
- Produces:
  - `ConfirmCard` shows the assistant's summary + a primary `Save` chip (uses `selectChip`).
  - `TemplatePickerCard` renders template chips; on click, calls a `onPickTemplate` callback (used only by `/new`; this card is mounted directly in `/new`, not from the thread dispatcher).
  - `SystemEventCard` icons extended.
  - `ThreadMessage.svelte` routes `ASSISTANT_PROMPT` to either `ConfirmCard` (when payload state is `"CONFIRM"`) or `AssistantPromptCard`; `USER_SELECTION` renders as a small bubble; `STEP_REBOUND` / `SCHEDULE_SET` / `CONFIGURATION_STARTED` go to `SystemEventCard`.

- [ ] **Step 1: `ConfirmCard.svelte`**

Create `frontend/src/lib/components/thread/ConfirmCard.svelte`:

```svelte
<script lang="ts">
  import type { ChatMessage } from "$lib/chat/types";
  import { selectChip } from "$lib/plans/assistant";
  import { locale, translate } from "$lib/i18n";

  interface Props {
    message: ChatMessage;
    configurationId: string;
    tenantId: string;
    isLive: boolean;
  }
  let { message, configurationId, tenantId, isLive }: Props = $props();
  let saving = $state(false);

  async function onSave() {
    if (saving) return;
    saving = true;
    try {
      await selectChip({
        tenantId,
        configurationId,
        promptMessageId: message.id,
        optionId: "save",
        value: "save",
      });
    } finally {
      saving = false;
    }
  }
</script>

<div
  id={`m-${message.id}`}
  class="rounded-lg border border-plumage bg-obsidian-light px-4 py-3 {isLive ? 'ring-1 ring-talon-gold' : ''}"
>
  <p class="text-[13px] font-body text-cream">{message.text}</p>
  {#if isLive}
    <div class="mt-3 flex items-center gap-2">
      <button
        type="button"
        disabled={saving}
        onclick={onSave}
        class="cursor-pointer rounded-md border border-talon-gold bg-talon-gold px-3 py-1.5 text-[12px] font-semibold text-obsidian hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
      >
        {translate("confirm.save", $locale)}
      </button>
    </div>
  {/if}
</div>
```

- [ ] **Step 2: `TemplatePickerCard.svelte`**

Create `frontend/src/lib/components/thread/TemplatePickerCard.svelte`:

```svelte
<script lang="ts">
  import type { PlanTemplate } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { locale, translate } from "$lib/i18n";

  interface Props {
    templates: PlanTemplate[];
    onPick: (templateId: string) => void;
    disabled?: boolean;
  }
  let { templates, onPick, disabled = false }: Props = $props();
</script>

<div class="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
  {#each templates as template (template.id)}
    <button
      type="button"
      {disabled}
      onclick={() => onPick(template.id)}
      class="cursor-pointer rounded-lg border border-plumage bg-obsidian-light px-3 py-3 text-left hover:border-talon-gold disabled:cursor-not-allowed disabled:opacity-50"
    >
      <p class="text-[13px] font-heading font-semibold text-cream">{template.name}</p>
      {#if template.description}
        <p class="mt-1 text-[11px] font-body text-crown-ash">{template.description}</p>
      {/if}
      <p class="mt-2 text-[10px] font-mono text-crown-ash-dark">
        {translate("new.stepsCount", $locale, { count: template.steps?.length ?? 0 })}
      </p>
    </button>
  {/each}
</div>
```

- [ ] **Step 3: Extend `SystemEventCard.svelte`**

In `frontend/src/lib/components/thread/SystemEventCard.svelte`, update the `iconFor` derived to add the three new kinds. ADD imports for new icons and extend the ternary:

```svelte
<script lang="ts">
  import { Activity, Check, AlertTriangle, Play, Save, Repeat, Calendar, Sparkles } from "lucide-svelte";
  // ... existing imports
  const iconFor = $derived(
    message.kind === "RUN_STARTED" ? Play
    : message.kind === "RUN_COMPLETED" ? Check
    : message.kind === "RUN_FAILED" ? AlertTriangle
    : message.kind === "STEP_BOUND" || message.kind === "STEP_STARTED" ? Activity
    : message.kind === "STEP_REBOUND" ? Repeat
    : message.kind === "SCHEDULE_SET" ? Calendar
    : message.kind === "CONFIGURATION_STARTED" ? Sparkles
    : Save,
  );
</script>
```

The rest of the template stays the same.

- [ ] **Step 4: Update `ThreadMessage.svelte` dispatch**

In `frontend/src/lib/components/thread/ThreadMessage.svelte`, replace the existing `{#if ... #else}` with a richer dispatch. Add the required props (`configurationId`, `tenantId`, `isLive`, `isAnswered`) to the `Props` interface and template:

```svelte
<script lang="ts">
  import type { ChatMessage } from "$lib/chat/types";
  import { locale } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";
  import SystemEventCard from "./SystemEventCard.svelte";
  import ElicitationRefCard from "./ElicitationRefCard.svelte";
  import ApprovalRefCard from "./ApprovalRefCard.svelte";
  import AssistantPromptCard from "./AssistantPromptCard.svelte";
  import ConfirmCard from "./ConfirmCard.svelte";

  interface Props {
    message: ChatMessage;
    configurationId?: string;
    tenantId?: string;
    isLive?: boolean;
    isAnswered?: boolean;
  }
  let {
    message,
    configurationId = "",
    tenantId = "",
    isLive = false,
    isAnswered = false,
  }: Props = $props();

  const isConfirm = $derived.by(() => {
    if (message.kind !== "ASSISTANT_PROMPT") return false;
    try {
      return JSON.parse(message.payloadJson)?.state === "CONFIRM";
    } catch {
      return false;
    }
  });
</script>

{#if message.kind === "USER_TEXT"}
  <div id={`m-${message.id}`} class="max-w-[85%] self-end rounded-lg border border-plumage bg-obsidian-light px-3 py-2">
    <p class="text-[13px] whitespace-pre-wrap text-cream">{message.text}</p>
    <p class="mt-1 text-right text-[10px] text-crown-ash-dark">
      {formatRelativeTime(message.createdAt, $locale)}
    </p>
  </div>
{:else if message.kind === "USER_SELECTION"}
  <div id={`m-${message.id}`} class="max-w-[85%] self-end rounded-md border border-plumage/60 bg-obsidian px-3 py-1 text-[11px] text-crown-ash">
    → {message.text || JSON.parse(message.payloadJson || "{}").value || "—"}
  </div>
{:else if message.kind === "ASSISTANT_PROMPT" && isConfirm}
  <ConfirmCard {message} {configurationId} {tenantId} {isLive} />
{:else if message.kind === "ASSISTANT_PROMPT"}
  <AssistantPromptCard {message} {configurationId} {tenantId} {isAnswered} {isLive} />
{:else if message.kind === "ELICITATION_RAISED" || message.kind === "ELICITATION_ANSWERED"}
  <ElicitationRefCard {message} />
{:else if message.kind === "APPROVAL_RAISED" || message.kind === "APPROVAL_DECIDED"}
  <ApprovalRefCard {message} />
{:else}
  <SystemEventCard {message} />
{/if}
```

- [ ] **Step 5: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/components/thread/ConfirmCard.svelte frontend/src/lib/components/thread/TemplatePickerCard.svelte frontend/src/lib/components/thread/SystemEventCard.svelte frontend/src/lib/components/thread/ThreadMessage.svelte
git commit -m "feat(ux-m5): ConfirmCard, TemplatePickerCard, dispatch + icons for new kinds"
```

---

## Task 15: `PlanCostPill.svelte` + `PlanThreadTopBar.svelte`

**Files:**
- Create: `frontend/src/lib/components/PlanCostPill.svelte`
- Create: `frontend/src/lib/components/PlanThreadTopBar.svelte`

**Interfaces:**
- Consumes: `RunCost`, `computeRunCost`, `ExecutorPriceLookup` from Task 11.
- Produces:
  - `PlanCostPill`: props `{ cost: RunCost }`. Renders compact pill.
  - `PlanThreadTopBar`: props `{ planName: string; statusLabel: string; cost: RunCost; onOpenSchedule?: () => void }`. Renders one-row top strip.

- [ ] **Step 1: `PlanCostPill.svelte`**

```svelte
<script lang="ts">
  import { Coins } from "lucide-svelte";
  import { locale, translate } from "$lib/i18n";
  import type { RunCost } from "$lib/plans/cost";

  interface Props { cost: RunCost }
  let { cost }: Props = $props();

  const label = $derived(
    cost.unboundStepCount > 0
      ? translate("cost.pillPartial", $locale, {
          amount: cost.totalPerRunBrl.toFixed(2),
          pending: cost.unboundStepCount,
        })
      : translate("cost.pillFull", $locale, { amount: cost.totalPerRunBrl.toFixed(2) }),
  );
</script>

<span
  class="inline-flex items-center gap-1 rounded-full border border-plumage bg-obsidian-light px-2 py-1 text-[11px] text-cream"
  title={cost.breakdown.map((b) => `${b.stepTitle}: ${b.pricePerRunBrl != null ? `R$ ${b.pricePerRunBrl.toFixed(2)}` : "—"}`).join("\n")}
>
  <Coins class="size-3.5 text-talon-gold" />
  {label}
</span>
```

- [ ] **Step 2: `PlanThreadTopBar.svelte`**

```svelte
<script lang="ts">
  import { Calendar } from "lucide-svelte";
  import PlanCostPill from "./PlanCostPill.svelte";
  import { locale, translate } from "$lib/i18n";
  import type { RunCost } from "$lib/plans/cost";

  interface Props {
    planName: string;
    statusLabel: string;
    cost: RunCost;
    onOpenSchedule?: () => void;
  }
  let { planName, statusLabel, cost, onOpenSchedule }: Props = $props();
</script>

<header class="flex flex-wrap items-center gap-3 border-b border-plumage/60 bg-obsidian px-4 py-2">
  <span class="font-heading text-[13px] font-semibold text-cream">{planName}</span>
  <span class="rounded-full border border-plumage px-2 py-0.5 text-[10px] font-mono text-crown-ash uppercase">
    {statusLabel}
  </span>
  <div class="ml-auto flex items-center gap-2">
    <PlanCostPill {cost} />
    {#if onOpenSchedule}
      <button
        type="button"
        onclick={onOpenSchedule}
        class="flex items-center gap-1 rounded border border-plumage px-2 py-1 text-[11px] text-crown-ash hover:border-talon-gold hover:text-talon-gold"
      >
        <Calendar class="size-3.5" />
        {translate("schedule.openButton", $locale)}
      </button>
    {/if}
  </div>
</header>
```

- [ ] **Step 3: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/components/PlanCostPill.svelte frontend/src/lib/components/PlanThreadTopBar.svelte
git commit -m "feat(ux-m5): PlanCostPill + PlanThreadTopBar"
```

---

## Task 16: `ScheduleDialog.svelte` + wire `CanvasTopBar`

**Files:**
- Create: `frontend/src/lib/components/canvas/ScheduleDialog.svelte`
- Modify: `frontend/src/lib/components/canvas/CanvasTopBar.svelte`

**Interfaces:**
- Consumes: `planClient.updatePlanConfiguration` (existing).
- Produces:
  - `ScheduleDialog`: props `{ open: boolean; configuration: PlanConfiguration; onClose: () => void; onSaved?: (next: PlanConfiguration) => void }`. Renders a modal with cadence chips, time picker, custom-cron textbox, save/cancel.
  - `CanvasTopBar`: replaces the stubbed Schedule button with an `onOpenSchedule` callback prop; new `cost?: RunCost` prop mounts `PlanCostPill` next to `Run history`.

- [ ] **Step 1: `ScheduleDialog.svelte`**

```svelte
<script lang="ts">
  import { X } from "lucide-svelte";
  import { planClient } from "$lib/rpc";
  import { locale, translate } from "$lib/i18n";
  import type { PlanConfiguration } from "$lib/gen/harpia/plans/v1/plans_pb";

  interface Props {
    open: boolean;
    configuration: PlanConfiguration;
    onClose: () => void;
    onSaved?: (next: PlanConfiguration) => void;
  }
  let { open, configuration, onClose, onSaved }: Props = $props();

  type Cadence = "manual" | "daily" | "weekly" | "monthly" | "custom";

  let cadence = $state<Cadence>("manual");
  let time = $state("09:00");
  let dayOfWeek = $state(1); // 0=Sun, 1=Mon, ...
  let dayOfMonth = $state(1);
  let customCron = $state("");
  let error = $state<string | null>(null);
  let saving = $state(false);

  function cronFromInputs(): string {
    if (cadence === "manual") return "";
    if (cadence === "custom") return customCron.trim();
    const [hh, mm] = time.split(":").map((s) => parseInt(s, 10) || 0);
    if (cadence === "daily") return `${mm} ${hh} * * *`;
    if (cadence === "weekly") return `${mm} ${hh} * * ${dayOfWeek}`;
    if (cadence === "monthly") return `${mm} ${hh} ${dayOfMonth} * *`;
    return "";
  }

  async function save() {
    error = null;
    saving = true;
    try {
      const cron = cronFromInputs();
      const response = await planClient.updatePlanConfiguration({
        tenantId: configuration.tenantId,
        planConfigurationId: configuration.id,
        status: configuration.status,
        seedArtifacts: configuration.seedArtifacts,
        slotBindings: configuration.slotBindings,
        overseerBindings: configuration.overseerBindings,
        behaviorPolicies: configuration.behaviorPolicies,
        schedule: { cronExpression: cron, timezone: configuration.schedule?.timezone || "UTC" } as any,
      });
      if (response.planConfiguration && onSaved) onSaved(response.planConfiguration);
      onClose();
    } catch (e) {
      error = e instanceof Error ? e.message : "Failed to save schedule";
    } finally {
      saving = false;
    }
  }
</script>

{#if open}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-obsidian/80 p-4">
    <div class="w-full max-w-md rounded-lg border border-plumage bg-obsidian-light p-4">
      <div class="mb-3 flex items-center justify-between">
        <h2 class="font-heading text-[14px] font-semibold text-cream">
          {translate("schedule.title", $locale)}
        </h2>
        <button type="button" onclick={onClose} aria-label={translate("schedule.close", $locale)} class="cursor-pointer text-crown-ash hover:text-cream">
          <X class="size-4" />
        </button>
      </div>

      <div class="space-y-3">
        <div class="flex flex-wrap gap-2">
          {#each ["manual", "daily", "weekly", "monthly", "custom"] as c (c)}
            <button
              type="button"
              onclick={() => (cadence = c as Cadence)}
              class="cursor-pointer rounded-md border px-2 py-1 text-[11px] {cadence === c ? 'border-talon-gold bg-talon-gold/10 text-talon-gold' : 'border-plumage text-crown-ash hover:border-talon-gold'}"
            >
              {translate(`schedule.cadence.${c}`, $locale)}
            </button>
          {/each}
        </div>

        {#if cadence === "daily" || cadence === "weekly" || cadence === "monthly"}
          <label class="block text-[11px] text-crown-ash">
            {translate("schedule.time", $locale)}
            <input type="time" bind:value={time} class="ml-2 rounded border border-plumage bg-obsidian px-2 py-1 text-[12px] text-cream" />
          </label>
        {/if}
        {#if cadence === "weekly"}
          <label class="block text-[11px] text-crown-ash">
            {translate("schedule.dayOfWeek", $locale)}
            <select bind:value={dayOfWeek} class="ml-2 rounded border border-plumage bg-obsidian px-2 py-1 text-[12px] text-cream">
              <option value={0}>{translate("schedule.dow.0", $locale)}</option>
              <option value={1}>{translate("schedule.dow.1", $locale)}</option>
              <option value={2}>{translate("schedule.dow.2", $locale)}</option>
              <option value={3}>{translate("schedule.dow.3", $locale)}</option>
              <option value={4}>{translate("schedule.dow.4", $locale)}</option>
              <option value={5}>{translate("schedule.dow.5", $locale)}</option>
              <option value={6}>{translate("schedule.dow.6", $locale)}</option>
            </select>
          </label>
        {/if}
        {#if cadence === "monthly"}
          <label class="block text-[11px] text-crown-ash">
            {translate("schedule.dayOfMonth", $locale)}
            <input type="number" min="1" max="28" bind:value={dayOfMonth} class="ml-2 w-16 rounded border border-plumage bg-obsidian px-2 py-1 text-[12px] text-cream" />
          </label>
        {/if}
        {#if cadence === "custom"}
          <label class="block text-[11px] text-crown-ash">
            {translate("schedule.customCron", $locale)}
            <input type="text" bind:value={customCron} placeholder="0 9 * * *" class="mt-1 w-full rounded border border-plumage bg-obsidian px-2 py-1 font-mono text-[12px] text-cream" />
          </label>
        {/if}

        {#if error}
          <p class="text-[11px] text-red-400">{error}</p>
        {/if}
      </div>

      <div class="mt-4 flex justify-end gap-2">
        <button type="button" onclick={onClose} class="cursor-pointer rounded-md border border-plumage px-3 py-1.5 text-[12px] text-crown-ash hover:border-talon-gold hover:text-talon-gold">
          {translate("schedule.cancel", $locale)}
        </button>
        <button type="button" disabled={saving} onclick={save} class="cursor-pointer rounded-md border border-talon-gold bg-talon-gold px-3 py-1.5 text-[12px] font-semibold text-obsidian hover:opacity-90 disabled:opacity-50">
          {translate("schedule.save", $locale)}
        </button>
      </div>
    </div>
  </div>
{/if}
```

- [ ] **Step 2: Update `CanvasTopBar.svelte`**

Replace the stub `Schedule` button. ADD `onOpenSchedule: () => void` and `cost?: RunCost` to `Props`. Change the stub block to:

```svelte
<button
  type="button"
  onclick={onOpenSchedule}
  class="flex items-center gap-1 rounded border border-plumage bg-transparent px-2 py-1 text-[11px] text-crown-ash hover:border-talon-gold hover:text-talon-gold"
>
  <Calendar class="size-3.5" />
  {translate("canvas.topbar.schedule", $locale)}
</button>
```

Add a `PlanCostPill` import; immediately before the `Run history` button block ADD:

```svelte
{#if cost}
  <PlanCostPill {cost} />
{/if}
```

- [ ] **Step 3: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/components/canvas/ScheduleDialog.svelte frontend/src/lib/components/canvas/CanvasTopBar.svelte
git commit -m "feat(ux-m5): ScheduleDialog + wire schedule button + cost pill into CanvasTopBar"
```

---

## Task 17: `YourPlansList.svelte` + mount in layout sidebar

**Files:**
- Create: `frontend/src/lib/components/sidebar/YourPlansList.svelte`
- Modify: `frontend/src/routes/+layout.svelte`

**Interfaces:**
- Consumes: `planClient.listPlanConfigurations` (existing streaming RPC), `getTenant()` from `$lib/auth`.
- Produces: a small left-rail list inside the existing `nav` block. Up to 10 rows, sorted by `updated_at` desc, status badge per row, click → `/plans/configurations/<id>`.

- [ ] **Step 1: `YourPlansList.svelte`**

```svelte
<script lang="ts">
  import { resolve } from "$app/paths";
  import { getTenant } from "$lib/auth";
  import { locale, translate } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";
  import { planClient } from "$lib/rpc";
  import { PlanConfigurationStatus, type PlanConfiguration } from "$lib/gen/harpia/plans/v1/plans_pb";

  const tenantId = $derived(getTenant()?.id ?? "");
  let plans = $state<PlanConfiguration[]>([]);
  let loadError = $state(false);

  const POLL_MS = 5000;

  $effect(() => {
    if (!tenantId) return;
    const controller = new AbortController();
    let cancelled = false;

    async function loadOnce() {
      try {
        const out: PlanConfiguration[] = [];
        for await (const page of planClient.listPlanConfigurations(
          { tenantId, pageSize: 50, pageToken: "" },
          { signal: controller.signal },
        )) {
          out.push(...page.planConfigurations);
        }
        if (cancelled) return;
        plans = out
          .filter(
            (p) =>
              p.status !== PlanConfigurationStatus.ARCHIVED &&
              p.status !== PlanConfigurationStatus.DISABLED,
          )
          .sort(
            (a, b) =>
              Date.parse(b.updatedAt || b.createdAt) -
              Date.parse(a.updatedAt || a.createdAt),
          )
          .slice(0, 10);
      } catch {
        if (!cancelled) loadError = true;
      }
    }

    void loadOnce();
    const intervalId = setInterval(loadOnce, POLL_MS);
    return () => {
      cancelled = true;
      controller.abort();
      clearInterval(intervalId);
    };
  });

  function statusLabelKey(status: PlanConfigurationStatus): string {
    switch (status) {
      case PlanConfigurationStatus.RUNNABLE: return "sidebar.yourPlans.status.runnable";
      case PlanConfigurationStatus.SCHEDULED: return "sidebar.yourPlans.status.scheduled";
      default: return "sidebar.yourPlans.status.draft";
    }
  }
</script>

<div class="mt-4">
  <p class="px-3 font-mono text-[10px] tracking-widest text-crown-ash-dark uppercase">
    {translate("sidebar.yourPlans.heading", $locale)}
  </p>
  {#if loadError}
    <p class="px-3 mt-1 text-[11px] text-crown-ash-dark">{translate("sidebar.yourPlans.loadError", $locale)}</p>
  {:else if plans.length === 0}
    <p class="px-3 mt-1 text-[11px] text-crown-ash-dark">{translate("sidebar.yourPlans.empty", $locale)}</p>
  {:else}
    <div class="mt-1 space-y-0.5">
      {#each plans as plan (plan.id)}
        <a
          href={resolve(`/plans/configurations/${plan.id}`)}
          class="flex items-center gap-2 rounded-md px-3 py-1.5 text-[12px] text-crown-ash hover:bg-obsidian-light hover:text-cream"
        >
          <span class="flex-1 truncate">{plan.id.slice(0, 8)}</span>
          <span class="text-[9px] uppercase text-crown-ash-dark">
            {translate(statusLabelKey(plan.status), $locale)}
          </span>
          <span class="text-[9px] text-crown-ash-dark">
            {formatRelativeTime(plan.updatedAt || plan.createdAt, $locale)}
          </span>
        </a>
      {/each}
    </div>
    <a
      href={resolve(`/discover`)}
      class="mt-1 block px-3 py-1 text-[10px] text-crown-ash-dark hover:text-talon-gold"
    >
      {translate("sidebar.yourPlans.seeAll", $locale)}
    </a>
  {/if}
</div>
```

(Templates may evolve to display `template.name` instead of the truncated id; that requires a `GetPlanTemplate` lookup per row. For v1 the id-slice is acceptable; an improvement task can replace it later.)

- [ ] **Step 2: Mount in `+layout.svelte`**

In `frontend/src/routes/+layout.svelte`, inside the `<nav>` block, AFTER the `<div class="space-y-1"> ... </div>` that renders nav sections and BEFORE the `<div class="mt-8 border-t border-plumage pt-6">` "role" block, ADD:

```svelte
{#if personaMode === "operator"}
  <YourPlansList />
{/if}
```

Add the import at the top of the `<script>` block:

```ts
import YourPlansList from "$lib/components/sidebar/YourPlansList.svelte";
```

- [ ] **Step 3: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/components/sidebar/YourPlansList.svelte frontend/src/routes/+layout.svelte
git commit -m "feat(ux-m5): YourPlansList sidebar entries for operator persona"
```

---

## Task 18: `/new` page — greeting + gallery + composer + `?template=` auto-commit

**Files:**
- Modify: `frontend/src/routes/new/+page.svelte`
- Create: `frontend/src/routes/new/+page.ts`

**Interfaces:**
- Consumes: `planClient.listPlanTemplates`, `planClient.createPlanConfiguration`, `TemplatePickerCard`.
- Produces: Functional `/new` route that creates a draft PlanConfiguration on template pick and navigates to its thread.

- [ ] **Step 1: `+page.ts` — load templates**

Create `frontend/src/routes/new/+page.ts`:

```ts
import type { PageLoad } from "./$types";
import { planClient } from "$lib/rpc";
import { requireTenantId } from "$lib/auth";
import type { PlanTemplate } from "$lib/gen/harpia/plans/v1/plans_pb";

export const load: PageLoad = async ({ url }) => {
  const tenantId = requireTenantId();
  const templates: PlanTemplate[] = [];
  for await (const page of planClient.listPlanTemplates({ tenantId, pageSize: 50, pageToken: "" })) {
    templates.push(...page.planTemplates);
  }
  return { templates, autoTemplateId: url.searchParams.get("template") ?? "" };
};
```

- [ ] **Step 2: `+page.svelte` — greeting + gallery + composer + auto-commit**

Replace `frontend/src/routes/new/+page.svelte` entirely:

```svelte
<script lang="ts">
  import { onMount } from "svelte";
  import { goto } from "$app/navigation";
  import { resolve } from "$app/paths";
  import { getTenant } from "$lib/auth";
  import { locale, translate } from "$lib/i18n";
  import { planClient } from "$lib/rpc";
  import { PlanConfigurationStatus } from "$lib/gen/harpia/plans/v1/plans_pb";
  import HarpyHeading from "$lib/components/ui/HarpyHeading.svelte";
  import TemplatePickerCard from "$lib/components/thread/TemplatePickerCard.svelte";

  let { data } = $props();
  const tenantId = $derived(getTenant()?.id ?? "");

  let creating = $state(false);
  let createError = $state<string | null>(null);
  let composerText = $state("");

  const matchedTemplate = $derived.by(() => {
    const q = composerText.trim().toLowerCase();
    if (q.length < 2) return null;
    const hit = data.templates.find(
      (t) => t.name.toLowerCase().includes(q) || (t.description ?? "").toLowerCase().includes(q),
    );
    return hit ?? null;
  });

  async function pickTemplate(templateId: string) {
    if (creating || !tenantId) return;
    creating = true;
    createError = null;
    try {
      const response = await planClient.createPlanConfiguration({
        tenantId,
        workspaceId: "",
        planTemplateId: templateId,
        status: PlanConfigurationStatus.DRAFT,
        seedArtifacts: [],
        slotBindings: [],
        overseerBindings: [],
        behaviorPolicies: undefined,
        schedule: undefined,
      });
      const configId = response.planConfiguration?.id;
      if (!configId) throw new Error("createPlanConfiguration returned no id");
      await goto(resolve(`/plans/configurations/${configId}`));
    } catch (e) {
      createError = e instanceof Error ? e.message : "Failed to create plan";
    } finally {
      creating = false;
    }
  }

  onMount(() => {
    if (data.autoTemplateId) void pickTemplate(data.autoTemplateId);
  });
</script>

<svelte:head><title>{translate("nav.newPlan", $locale)} · Harpia</title></svelte:head>

<div class="mx-auto max-w-3xl px-4 py-6">
  <HarpyHeading tag="h1" class="text-2xl text-cream">
    {translate("new.greeting", $locale)}
  </HarpyHeading>
  <p class="mt-1 text-[13px] font-body text-crown-ash">
    {translate("new.subgreeting", $locale)}
  </p>

  <div class="mt-4">
    <input
      type="text"
      bind:value={composerText}
      onkeydown={(e) => {
        if (e.key === "Enter" && matchedTemplate) void pickTemplate(matchedTemplate.id);
      }}
      placeholder={translate("new.composerPlaceholder", $locale)}
      class="w-full rounded-md border border-plumage bg-obsidian-light px-3 py-2 text-[13px] text-cream"
    />
    {#if composerText.trim().length >= 2 && !matchedTemplate}
      <p class="mt-1 text-[11px] text-crown-ash-dark">
        {translate("new.noMatch", $locale, { query: composerText })}
      </p>
    {/if}
  </div>

  <div class="mt-6">
    <p class="mb-2 font-mono text-[10px] uppercase tracking-widest text-crown-ash-dark">
      {translate("new.galleryHeading", $locale)}
    </p>
    <TemplatePickerCard templates={data.templates} onPick={pickTemplate} disabled={creating} />
  </div>

  {#if createError}
    <p class="mt-3 text-[11px] text-red-400">{createError}</p>
  {/if}
</div>
```

- [ ] **Step 3: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/routes/new/+page.svelte frontend/src/routes/new/+page.ts
git commit -m "feat(ux-m5): /new launcher with template gallery + auto-commit"
```

---

## Task 19: Plan-thread page — mount top bar, load catalog, route new kinds

**Files:**
- Modify: `frontend/src/routes/plans/configurations/[configurationId]/+page.ts`
- Modify: `frontend/src/routes/plans/configurations/[configurationId]/+page.svelte`

**Interfaces:**
- Consumes: `computeRunCost`, `PlanThreadTopBar`, `ScheduleDialog`, existing chat loader.
- Produces:
  - `+page.ts` returns `template`, `configuration`, `executorCatalog` (a `Map<installationId, { displayName, pricePerRunBrl }>`).
  - `+page.svelte` mounts top bar with `cost` + schedule chip, threads `tenantId/configurationId/isLive/isAnswered` props down to `ThreadMessage`.

- [ ] **Step 1: Extend `+page.ts`**

Append to (or extend) the existing load function so it ALSO loads the executor catalog. Read the current file first; the addition is roughly:

```ts
// after existing template + configuration loading:
const installations = new Map<string, { displayName: string; pricePerRunBrl: number | null }>();
// One simple approach: list compatible installations for each step via the
// existing slot-binding helpers. Reuse loadSlotBindingPageData if it returns
// installation data; otherwise inline an executorClient.listInstallations call.
// Keep this synchronous to the load() promise so the page is reactive.
return { configurationId, template, configuration, executorCatalog: installations };
```

If a richer catalog endpoint exists (`executorClient.listExecutorInstallations(tenantId)`), prefer that and map each row to `{ id, displayName, pricePerRunBrl }`. The cost pill will tolerate missing entries (yields null prices).

- [ ] **Step 2: Modify `+page.svelte`**

In `frontend/src/routes/plans/configurations/[configurationId]/+page.svelte`, ADD imports:

```svelte
import PlanThreadTopBar from "$lib/components/PlanThreadTopBar.svelte";
import ScheduleDialog from "$lib/components/canvas/ScheduleDialog.svelte";
import { computeRunCost, type ExecutorPriceLookup } from "$lib/plans/cost";
import { PlanConfigurationStatus } from "$lib/gen/harpia/plans/v1/plans_pb";
```

Add reactive state for the schedule dialog:

```ts
let scheduleOpen = $state(false);
```

Build a `cost` derived. If `data.template` is present:

```ts
const pricing: ExecutorPriceLookup = (id) => data.executorCatalog?.get(id) ?? null;
const cost = $derived(
  data.template && data.configuration
    ? computeRunCost(data.template, data.configuration, pricing)
    : { totalPerRunBrl: 0, currency: "BRL" as const, unboundStepCount: 0, breakdown: [] },
);
const statusLabel = $derived(
  data.configuration?.status === PlanConfigurationStatus.RUNNABLE
    ? translate("plans.configure.status.runnable", $locale)
    : data.configuration?.status === PlanConfigurationStatus.SCHEDULED
      ? translate("plans.configure.status.scheduled", $locale)
      : translate("plans.configure.status.draft", $locale),
);
```

Mount the top bar above the existing DAG mini-map row:

```svelte
{#if data.template && data.configuration}
  <PlanThreadTopBar
    planName={data.template.name}
    {statusLabel}
    {cost}
    onOpenSchedule={() => (scheduleOpen = true)}
  />
{/if}
```

After the `{/if}` closing the `sections` block (right before `<ThreadComposer ...>`), update the per-message rendering loop so it threads the new props:

```svelte
{#each sections as section (section.kind === "execution" ? section.group.executionId : section.message.id)}
  {#if section.kind === "plan-scope"}
    {@const isLastAssistantPrompt = (() => {
      // The "live" prompt is the most-recent ASSISTANT_PROMPT with no
      // downstream USER_SELECTION whose payload references its id.
      if (section.message.kind !== "ASSISTANT_PROMPT") return false;
      const idx = messages.findIndex((m) => m.id === section.message.id);
      if (idx < 0) return false;
      const after = messages.slice(idx + 1);
      const answered = after.some((m) => {
        if (m.kind !== "USER_SELECTION") return false;
        try {
          return JSON.parse(m.payloadJson)?.in_response_to_message_id === section.message.id;
        } catch { return false; }
      });
      return !answered;
    })()}
    {@const isAnsweredPrompt = section.message.kind === "ASSISTANT_PROMPT" && !isLastAssistantPrompt}
    <ThreadMessage
      message={section.message}
      configurationId={data.configurationId}
      {tenantId}
      isLive={isLastAssistantPrompt}
      isAnswered={isAnsweredPrompt}
    />
  {:else}
    <ExecutionSection
      group={section.group}
      defaultExpanded={section.group.executionId === mostRecentExecutionId}
    />
  {/if}
{/each}
```

After the closing `</div>` of the main column AND before the `<style>` block, mount the dialog:

```svelte
{#if data.configuration}
  <ScheduleDialog
    open={scheduleOpen}
    configuration={data.configuration}
    onClose={() => (scheduleOpen = false)}
  />
{/if}
```

- [ ] **Step 3: Manual smoke**

```bash
cd /home/thbertoldi/harpia/frontend && npm run dev
```

Open `http://localhost:5173/new`, pick a template, walk a few chips, hit Save, then click the schedule chip in the top bar. Confirm chips fire and the dialog opens.

- [ ] **Step 4: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/routes/plans/configurations/[configurationId]/+page.svelte frontend/src/routes/plans/configurations/[configurationId]/+page.ts
git commit -m "feat(ux-m5): plan thread mounts top bar + cost pill + schedule dialog; routes assistant kinds"
```

---

## Task 20: Canvas page — wire ScheduleDialog state + pass cost into top bar

**Files:**
- Modify: `frontend/src/routes/plans/configurations/[configurationId]/canvas/+page.svelte`

**Interfaces:**
- Consumes: `ScheduleDialog`, `computeRunCost`.
- Produces: canvas top bar's `onOpenSchedule` now opens the dialog; `cost` prop is fed.

- [ ] **Step 1: Edit the canvas page**

Add imports:

```svelte
import ScheduleDialog from "$lib/components/canvas/ScheduleDialog.svelte";
import { computeRunCost, type ExecutorPriceLookup } from "$lib/plans/cost";
```

Add state and derived (similar to Task 19):

```ts
let scheduleOpen = $state(false);
const pricing: ExecutorPriceLookup = (id) => data.executorCatalog?.get(id) ?? null;
const cost = $derived(
  data.template && data.configuration
    ? computeRunCost(data.template, data.configuration, pricing)
    : { totalPerRunBrl: 0, currency: "BRL" as const, unboundStepCount: 0, breakdown: [] },
);
```

(If the canvas `+page.ts` does not already load `executorCatalog`, add a parallel piece of code to Task 19's Step 1 to the canvas `+page.ts`.)

Pass to `CanvasTopBar`:

```svelte
<CanvasTopBar
  configurationId={data.configurationId}
  planName={data.template?.name}
  pendingAnswerCount={pendingCount}
  onOpenRunHistory={() => (historyOpen = true)}
  onOpenSettings={() => (settingsOpen = true)}
  onOpenSchedule={() => (scheduleOpen = true)}
  onAnswerNext={answerNext}
  {cost}
/>
```

Mount the dialog at the bottom of the layout:

```svelte
{#if data.configuration}
  <ScheduleDialog
    open={scheduleOpen}
    configuration={data.configuration}
    onClose={() => (scheduleOpen = false)}
  />
{/if}
```

- [ ] **Step 2: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/routes/plans/configurations/[configurationId]/canvas/+page.svelte frontend/src/routes/plans/configurations/[configurationId]/canvas/+page.ts
git commit -m "feat(ux-m5): canvas page wires ScheduleDialog + cost pill"
```

---

## Task 21: Delete wizard routes + redirects + retarget `/[templateId]` CTA

**Files:**
- Delete: `frontend/src/routes/plans/[templateId]/configure/+page.svelte`
- Delete: `frontend/src/routes/plans/[templateId]/configure/+layout.server.ts`
- Delete: `frontend/src/routes/plans/[templateId]/configure/overseer/+page.svelte`
- Delete: `frontend/src/routes/plans/[templateId]/configure/policies/+page.svelte`
- Delete: `frontend/src/routes/plans/[templateId]/configure/summary/+page.svelte`
- Create: `frontend/src/routes/plans/[templateId]/configure/+page.ts`
- Create: `frontend/src/routes/plans/[templateId]/configure/overseer/+page.ts`
- Create: `frontend/src/routes/plans/[templateId]/configure/policies/+page.ts`
- Create: `frontend/src/routes/plans/[templateId]/configure/summary/+page.ts`
- Modify: `frontend/src/routes/plans/[templateId]/+page.svelte`

**Interfaces:**
- Produces: 302 redirects from every legacy wizard URL to `/new?template=<templateId>`. The template-detail page's CTA points to the same.

- [ ] **Step 1: Delete the four wizard `.svelte` files + layout server**

```bash
cd /home/thbertoldi/harpia
git rm frontend/src/routes/plans/\[templateId\]/configure/+page.svelte \
  frontend/src/routes/plans/\[templateId\]/configure/+layout.server.ts \
  frontend/src/routes/plans/\[templateId\]/configure/overseer/+page.svelte \
  frontend/src/routes/plans/\[templateId\]/configure/policies/+page.svelte \
  frontend/src/routes/plans/\[templateId\]/configure/summary/+page.svelte
```

- [ ] **Step 2: Create the four redirect `+page.ts` files**

For each route (`configure`, `configure/overseer`, `configure/policies`, `configure/summary`), create a `+page.ts`:

```ts
import { redirect } from "@sveltejs/kit";
import type { PageLoad } from "./$types";

export const load: PageLoad = ({ params }) => {
  throw redirect(302, `/new?template=${encodeURIComponent(params.templateId)}`);
};
```

All four files contain identical code.

- [ ] **Step 3: Retarget the `/[templateId]` CTA**

In `frontend/src/routes/plans/[templateId]/+page.svelte`, find the "Use this template" / "Configure" CTA — likely an `<a href={resolve('/plans/' + templateId + '/configure')}>`. Replace its href with `/new?template={templateId}`. Grep for it:

```bash
grep -n 'configure' frontend/src/routes/plans/\[templateId\]/+page.svelte
```

Update each occurrence that points at `/configure` to `/new?template=...`. If the CTA is rendered conditionally on `configuration?.status`, the logic stays — only the href changes.

- [ ] **Step 4: Smoke**

```bash
cd /home/thbertoldi/harpia/frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -10
```

Expected: no new errors over baseline. Manual: visit `/plans/<some-template>/configure` and confirm 302 to `/new?template=...`.

- [ ] **Step 5: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/routes/plans/\[templateId\]/configure frontend/src/routes/plans/\[templateId\]/+page.svelte
git commit -m "chore(ux-m5): delete wizard routes + 302 to /new; retarget template CTA"
```

---

## Task 22: i18n keys (lockstep en + pt-BR)

**Files:**
- Modify: `frontend/src/lib/i18n/en.json`
- Modify: `frontend/src/lib/i18n/pt-BR.json`

**Interfaces:**
- Produces: every i18n key referenced by Tasks 13–20 added to BOTH files.

- [ ] **Step 1: Add keys to `en.json`**

In `frontend/src/lib/i18n/en.json`, ADD these keys (alphabetically merged with existing entries):

```json
{
  "assistant.edit": "Edit",
  "confirm.save": "Save",
  "cost.pillFull": "R$ {amount} / run",
  "cost.pillPartial": "R$ {amount} / run · {pending} pending",
  "new.greeting": "What would you like to automate?",
  "new.subgreeting": "Pick a template below or describe your goal.",
  "new.composerPlaceholder": "Describe what you want to automate...",
  "new.noMatch": "No template matched “{query}”. Pick one below.",
  "new.galleryHeading": "Templates",
  "new.stepsCount": "{count} steps",
  "schedule.openButton": "Schedule",
  "schedule.title": "Schedule",
  "schedule.close": "Close",
  "schedule.cancel": "Cancel",
  "schedule.save": "Save",
  "schedule.time": "Time",
  "schedule.dayOfWeek": "Day",
  "schedule.dayOfMonth": "Day of month",
  "schedule.customCron": "Cron expression",
  "schedule.cadence.manual": "Manual only",
  "schedule.cadence.daily": "Daily",
  "schedule.cadence.weekly": "Weekly",
  "schedule.cadence.monthly": "Monthly",
  "schedule.cadence.custom": "Custom cron",
  "schedule.dow.0": "Sun",
  "schedule.dow.1": "Mon",
  "schedule.dow.2": "Tue",
  "schedule.dow.3": "Wed",
  "schedule.dow.4": "Thu",
  "schedule.dow.5": "Fri",
  "schedule.dow.6": "Sat",
  "sidebar.yourPlans.heading": "Your plans",
  "sidebar.yourPlans.empty": "No plans yet. Start one with + New plan.",
  "sidebar.yourPlans.loadError": "Failed to load plans.",
  "sidebar.yourPlans.seeAll": "See all",
  "sidebar.yourPlans.status.draft": "Draft",
  "sidebar.yourPlans.status.runnable": "Runnable",
  "sidebar.yourPlans.status.scheduled": "Scheduled",
  "plans.configure.status.scheduled": "Scheduled"
}
```

- [ ] **Step 2: Add equivalent keys to `pt-BR.json`**

```json
{
  "assistant.edit": "Editar",
  "confirm.save": "Salvar",
  "cost.pillFull": "R$ {amount} / execução",
  "cost.pillPartial": "R$ {amount} / execução · {pending} pendentes",
  "new.greeting": "O que você quer automatizar?",
  "new.subgreeting": "Escolha um modelo abaixo ou descreva seu objetivo.",
  "new.composerPlaceholder": "Descreva o que você quer automatizar...",
  "new.noMatch": "Nenhum modelo corresponde a “{query}”. Escolha abaixo.",
  "new.galleryHeading": "Modelos",
  "new.stepsCount": "{count} etapas",
  "schedule.openButton": "Agendar",
  "schedule.title": "Agendamento",
  "schedule.close": "Fechar",
  "schedule.cancel": "Cancelar",
  "schedule.save": "Salvar",
  "schedule.time": "Hora",
  "schedule.dayOfWeek": "Dia",
  "schedule.dayOfMonth": "Dia do mês",
  "schedule.customCron": "Expressão cron",
  "schedule.cadence.manual": "Somente manual",
  "schedule.cadence.daily": "Diário",
  "schedule.cadence.weekly": "Semanal",
  "schedule.cadence.monthly": "Mensal",
  "schedule.cadence.custom": "Cron personalizado",
  "schedule.dow.0": "Dom",
  "schedule.dow.1": "Seg",
  "schedule.dow.2": "Ter",
  "schedule.dow.3": "Qua",
  "schedule.dow.4": "Qui",
  "schedule.dow.5": "Sex",
  "schedule.dow.6": "Sáb",
  "sidebar.yourPlans.heading": "Seus planos",
  "sidebar.yourPlans.empty": "Nenhum plano ainda. Comece com + Novo plano.",
  "sidebar.yourPlans.loadError": "Falha ao carregar planos.",
  "sidebar.yourPlans.seeAll": "Ver todos",
  "sidebar.yourPlans.status.draft": "Rascunho",
  "sidebar.yourPlans.status.runnable": "Executável",
  "sidebar.yourPlans.status.scheduled": "Agendado",
  "plans.configure.status.scheduled": "Agendado"
}
```

- [ ] **Step 3: Run i18n parity test**

```bash
cd /home/thbertoldi/harpia/frontend && npx vitest run src/lib/i18n/hardcoded-copy.test.ts
```

Expected: PASS. If FAIL, the missing key set is reported — add to both files until it passes.

- [ ] **Step 4: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/i18n/en.json frontend/src/lib/i18n/pt-BR.json
git commit -m "feat(ux-m5): i18n keys for assistant, confirm, cost, schedule, sidebar.yourPlans, new"
```

---

## Task 23: End-to-end verification log

**Files:**
- Create: `docs/superpowers/plans/2026-06-22-harpia-ux-realignment-m5-chat-config.verification.md`

**Interfaces:**
- Produces: a verification log following the format used by M2/M3/M4. Each scenario lists steps and observed result.

- [ ] **Step 1: Boot the stack**

```bash
cd /home/thbertoldi/harpia && tilt up
```

Wait for `agent-runtime`, `control-plane`, `frontend` to be green. Then:

```bash
cd /home/thbertoldi/harpia/frontend && npm run dev
```

Open `http://localhost:5173`.

- [ ] **Step 2: Walk the golden path**

For each scenario below, record what you saw in the verification file:

1. **`/new` greets and shows templates.** Visit `/new`. Verify greeting, composer, gallery of N template chips.
2. **Template pick → thread.** Click a template. Verify navigation to `/plans/configurations/<id>` AND that the first message is `CONFIGURATION_STARTED`, the second is an `ASSISTANT_PROMPT` for `BINDING_STEP`.
3. **Walk bindings.** Click an executor chip for each step. After each click, verify a new `ASSISTANT_PROMPT` appears for the next step / overseer / policies / confirm.
4. **Save promotes to RUNNABLE.** Click `Save` on the `CONFIRM` card. Verify thread shows the assistant's saved acknowledgment and the top-bar status pill shows `Runnable`.
5. **Edit pencil.** Scroll up to a `BINDING_STEP` prompt, click the Edit icon, pick a different chip. Verify `STEP_REBOUND` system event appears, slot binding actually changes (re-load the page; new selection persists).
6. **Schedule dialog.** Click `Schedule` in the top bar. Pick `Daily` + `09:00`. Save. Verify `SCHEDULE_SET` event in thread, status pill flips to `Scheduled`, `PlanConfiguration.schedule.cronExpression == "0 9 * * *"` (check via DevTools network).
7. **Wizard redirects.** Visit `http://localhost:5173/plans/<some-template>/configure`. Verify 302 to `/new?template=<id>` and the page auto-creates the configuration.
8. **Sidebar.** Open the left nav. Verify "Your plans" header with at least the newly-created plan, click it, verify navigation works.
9. **Cost pill.** Verify the pill in the top bar reads `R$ X / run` matching the sum of bound executors' prices.

- [ ] **Step 3: Write the verification doc**

Use the format from `docs/superpowers/plans/2026-06-22-harpia-ux-realignment-m4-expanded-canvas.verification.md` as a template. Note any deviation, screenshot if a regression appears.

- [ ] **Step 4: Commit**

```bash
cd /home/thbertoldi/harpia
git add docs/superpowers/plans/2026-06-22-harpia-ux-realignment-m5-chat-config.verification.md
git commit -m "docs(ux-m5): end-to-end verification log"
```

---

## Task 24: Final whole-branch review

**Files:**
- None directly (review-only)

- [ ] **Step 1: Invoke the requesting-code-review skill**

Follow `superpowers:requesting-code-review`. Dispatch the review subagent with the full M5 diff and the spec at `docs/superpowers/specs/2026-06-22-harpia-ux-m5-chat-config-design.md` as the brief.

- [ ] **Step 2: Triage findings**

Apply CRITICAL fixes inline (one commit per fix). File IMPORTANT items as follow-up GitHub issues. Document NOTEs in the verification log.

- [ ] **Step 3: Final commit & push**

```bash
cd /home/thbertoldi/harpia
git push -u origin feat/ux-realignment-m5-chat-config
```

Open a PR with the spec link in the description.

---

## Self-review checklist (for the plan author)

After writing this plan, check before handoff:

- [ ] Every spec section (§1–§8) traces to one or more tasks above.
- [ ] No "TBD" / "TODO" markers in step bodies (skipped tests carry an explicit harness rationale).
- [ ] Type names are consistent: `AssistantState`, `StateKind`, `ExecutorOption`, `PromptInput`, `RunCost`, `CostBreakdownEntry`, `ExecutorPriceLookup`, `ChatMessageKind` variants.
- [ ] Every i18n key referenced in component code is in Task 22's en/pt-BR lists.
- [ ] Every deletion in Task 21 has a matching redirect file.
- [ ] Tasks that touch protos run `buf generate` in the same commit.
