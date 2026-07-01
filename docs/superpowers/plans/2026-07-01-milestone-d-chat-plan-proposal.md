# Milestone D: Chat-Driven Plan Proposal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. This plan is written executor-cold: assume no prior context beyond this document and the referenced files.

**Goal:** Let a user turn a natural-language message in a thread into an attached, ready-to-configure plan, using an LLM to pick the template and extract inputs.

**Architecture:** A new `ThreadService.ProposePlan` RPC reads a thread's latest user message, runs it through a `PlanClassifier` port (LLM adapter using the tenant's configured provider), and appends a `PLAN_PROPOSED` message. A new frontend card renders the proposal with an editable, generic inputs form; confirming creates the `PlanConfiguration` (thread-bound, already supported) and emits `PLAN_ATTACHED`, after which the existing deterministic binding-matrix flow takes over unchanged.

**Tech Stack:** Go + ConnectRPC + protobuf/buf codegen; PostgreSQL (no schema change in D); Svelte 5 + TypeScript + Vitest.

**Spec:** `docs/superpowers/specs/2026-07-01-milestone-d-chat-plan-proposal-design.md`

## Global Constraints

- Work from `trunk` (trunk-based development). Keep each task a small commit. Never add a `Co-Authored-By: Claude` trailer.
- After proto edits: `cd /home/thbertoldi/harpia/proto && buf generate && buf lint` (exit 0). Generated code lands in `control-plane/gen/**`, `frontend/src/lib/gen/**`, `agent-runtime/src/harpia_agents/gen/**` — commit it.
- Backend focused tests run from `control-plane/`. Frontend focused tests run from `frontend/` via `bunx vitest run <file>` (NOT `bun test` — the project uses Vitest; `bun test` lacks `vi.hoisted`).
- Frontend type check: `cd frontend && bun run check`. There is a known baseline of 12 pre-existing errors in files NOT touched by this plan (`auth-roles.test.ts`, `plans/artifact-flow.ts`, `canvas/ScheduleDialog.svelte`, `+layout.svelte`, `plans/[templateId]/+page.svelte`). A task passes if it introduces no NEW errors in files it modifies.
- Provider constraint: use the tenant's configured LLM provider via `llm_config`. Do NOT use Anthropic/Claude. Default provider is `deepseek` (OpenAI-compatible API).
- No `PlanConfiguration` schema change and no new migration in this milestone.
- Do not modify `internal/planassistant` — the deterministic binding-matrix flow is reused as-is.

---

## File Structure

### Protobuf
- Modify: `proto/harpia/chat/v1/chat.proto` — add `ProposePlan` rpc + `ProposePlanRequest`/`ProposePlanResponse`.

### Backend (control-plane)
- Modify: `control-plane/internal/chat/messages.go` — `BuildPlanProposedPayload`, `BuildPlanAttachedPayload`, payload + candidate structs.
- Modify: `control-plane/internal/chat/messages_test.go` — payload tests (create if absent).
- Create: `control-plane/internal/copilot/classifier.go` — `PlanClassifier` port + shared types.
- Create: `control-plane/internal/copilot/llm_classifier.go` — LLM adapter (OpenAI-compatible).
- Create: `control-plane/internal/copilot/llm_classifier_test.go` — adapter unit tests.
- Create: `control-plane/internal/plans/template_catalog.go` — `CopilotCatalog` adapter over `Repository`.
- Create: `control-plane/internal/plans/template_catalog_test.go` — mapping test.
- Modify: `control-plane/internal/threads/handler.go` — deps + `ProposePlan`.
- Modify: `control-plane/internal/threads/handler_test.go` — update `NewHandler` calls + `ProposePlan` tests.
- Modify: `control-plane/internal/plans/handler.go` — emit `PLAN_ATTACHED` on thread-bound create.
- Modify: `control-plane/cmd/api/main.go` — construct + wire classifier/catalog.

### Frontend
- Modify: `frontend/src/lib/chat/types.ts` — 8 new message kinds.
- Modify: `frontend/src/lib/plans/template-inputs.ts` — generic parse/serialize helpers.
- Create: `frontend/src/lib/plans/template-inputs.test.ts` — helper tests.
- Create: `frontend/src/lib/components/thread/TemplateInputsForm.svelte` — generic inputs form.
- Create: `frontend/src/lib/components/thread/PlanProposalCard.svelte` — proposal card.
- Modify: `frontend/src/lib/components/thread/ThreadMessage.svelte` — dispatch `PLAN_PROPOSED`.
- Modify: `frontend/src/routes/chat/[threadId]/+page.svelte` — always-on composer + auto-propose.
- Modify: `frontend/src/routes/new/+page.svelte` — prompt-first.

### Docs
- Create: `docs/superpowers/plans/2026-07-01-milestone-d-chat-plan-proposal.verification.md`.

---

### Task 1: Add ProposePlan proto + stub handler

**Files:**
- Modify: `proto/harpia/chat/v1/chat.proto`
- Modify: `control-plane/internal/threads/handler.go`
- Generated: `control-plane/gen/**`, `frontend/src/lib/gen/**`, `agent-runtime/src/harpia_agents/gen/**`

**Interfaces:**
- Produces: `chatv1.ProposePlanRequest{TenantId, ThreadId string}`, `chatv1.ProposePlanResponse{Message *ThreadMessage}`, and `chatv1connect.ThreadServiceHandler` gains `ProposePlan`.

- [ ] **Step 1: Add the RPC to the service block**

In `proto/harpia/chat/v1/chat.proto`, inside `service ThreadService { ... }`, add after the `AppendThreadMessage` line:

```proto
  rpc ProposePlan(ProposePlanRequest) returns (ProposePlanResponse);
```

- [ ] **Step 2: Add request/response messages**

In the same file, after the `AppendThreadMessageResponse` message, add:

```proto
message ProposePlanRequest {
  string tenant_id = 1;
  string thread_id = 2;
}

message ProposePlanResponse {
  // The appended PLAN_PROPOSED message (empty candidates when the router
  // could not confidently match a template).
  ThreadMessage message = 1;
}
```

- [ ] **Step 3: Generate clients**

Run:

```bash
cd /home/thbertoldi/harpia/proto && buf generate && buf lint
```

Expected: exit 0; `control-plane/gen/harpia/chat/v1/chatv1connect/chat.connect.go` now references `ProposePlan`.

- [ ] **Step 4: Add a stub handler so the build stays green**

The regenerated `ThreadServiceHandler` interface now requires `ProposePlan`. Add a temporary stub to `control-plane/internal/threads/handler.go` (it is replaced with the real implementation in Task 5). Append this method at the end of the file:

```go
// ProposePlan is implemented in Task 5. Stub keeps the generated
// ThreadServiceHandler interface satisfied until then.
func (h *Handler) ProposePlan(ctx context.Context, req *connect.Request[chatv1.ProposePlanRequest]) (*connect.Response[chatv1.ProposePlanResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("ProposePlan not implemented"))
}
```

If `errors` is not already imported in `handler.go`, it is (the file already uses `errors`). Verify the import block includes `"errors"`.

- [ ] **Step 5: Verify build**

Run:

```bash
cd /home/thbertoldi/harpia/control-plane && go build ./... 2>&1 | head
```

Expected: no output (build succeeds).

- [ ] **Step 6: Commit**

```bash
cd /home/thbertoldi/harpia
git add proto control-plane/gen frontend/src/lib/gen agent-runtime/src/harpia_agents/gen control-plane/internal/threads/handler.go
git commit -m "feat: add ProposePlan rpc and stub handler"
```

---

### Task 2: Chat payload builders for PLAN_PROPOSED / PLAN_ATTACHED

**Files:**
- Modify: `control-plane/internal/chat/messages.go`
- Modify/Create: `control-plane/internal/chat/messages_test.go`

**Interfaces:**
- Produces:
  - `chat.PlanProposalCandidate{TemplateID, TemplateKey, TemplateName string; Confidence float64; InputValuesJSON string}`
  - `chat.BuildPlanProposedPayload(sourceMessageID string, candidates []PlanProposalCandidate) string`
  - `chat.BuildPlanAttachedPayload(planConfigurationID, templateID string) string`

- [ ] **Step 1: Write failing tests**

Add to `control-plane/internal/chat/messages_test.go` (create the file with `package chat` if it does not exist):

```go
package chat

import (
	"encoding/json"
	"testing"
)

func TestBuildPlanProposedPayload(t *testing.T) {
	got := BuildPlanProposedPayload("msg-1", []PlanProposalCandidate{
		{TemplateID: "tpl-1", TemplateKey: "linkedin", TemplateName: "LinkedIn Post", Confidence: 0.9, InputValuesJSON: `{"theme":"retail"}`},
	})
	var decoded struct {
		SourceMessageID string `json:"source_message_id"`
		Candidates      []struct {
			TemplateID      string  `json:"template_id"`
			TemplateKey     string  `json:"template_key"`
			TemplateName    string  `json:"template_name"`
			Confidence      float64 `json:"confidence"`
			InputValuesJSON string  `json:"input_values_json"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.SourceMessageID != "msg-1" {
		t.Fatalf("source_message_id = %q", decoded.SourceMessageID)
	}
	if len(decoded.Candidates) != 1 || decoded.Candidates[0].TemplateKey != "linkedin" {
		t.Fatalf("candidates = %+v", decoded.Candidates)
	}
	if decoded.Candidates[0].InputValuesJSON != `{"theme":"retail"}` {
		t.Fatalf("input_values_json = %q", decoded.Candidates[0].InputValuesJSON)
	}
}

func TestBuildPlanProposedPayloadEmptyCandidates(t *testing.T) {
	got := BuildPlanProposedPayload("msg-1", nil)
	var decoded struct {
		Candidates []any `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.Candidates) != 0 {
		t.Fatalf("expected empty candidates, got %+v", decoded.Candidates)
	}
}

func TestBuildPlanAttachedPayload(t *testing.T) {
	got := BuildPlanAttachedPayload("cfg-1", "tpl-1")
	var decoded struct {
		PlanConfigurationID string `json:"plan_configuration_id"`
		TemplateID          string `json:"template_id"`
	}
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.PlanConfigurationID != "cfg-1" || decoded.TemplateID != "tpl-1" {
		t.Fatalf("decoded = %+v", decoded)
	}
}
```

- [ ] **Step 2: Run tests (expect FAIL)**

```bash
cd /home/thbertoldi/harpia/control-plane && go test ./internal/chat -run 'TestBuildPlan' -count=1
```

Expected: compile failure / undefined `BuildPlanProposedPayload`.

- [ ] **Step 3: Implement the builders**

Append to `control-plane/internal/chat/messages.go` (it already defines `mustEncodeJSON`; reuse it):

```go
// PlanProposalCandidate is one template the router proposes for a thread's
// latest user message, with inferred input values.
type PlanProposalCandidate struct {
	TemplateID      string  `json:"template_id"`
	TemplateKey     string  `json:"template_key"`
	TemplateName    string  `json:"template_name"`
	Confidence      float64 `json:"confidence"`
	InputValuesJSON string  `json:"input_values_json"`
}

type planProposedPayload struct {
	SourceMessageID string                  `json:"source_message_id"`
	Candidates      []PlanProposalCandidate `json:"candidates"`
}

// BuildPlanProposedPayload returns the JSON payload for a PLAN_PROPOSED message.
// candidates may be empty when the router found no confident match.
func BuildPlanProposedPayload(sourceMessageID string, candidates []PlanProposalCandidate) string {
	if candidates == nil {
		candidates = []PlanProposalCandidate{}
	}
	return mustEncodeJSON(planProposedPayload{SourceMessageID: sourceMessageID, Candidates: candidates})
}

type planAttachedPayload struct {
	PlanConfigurationID string `json:"plan_configuration_id"`
	TemplateID          string `json:"template_id"`
}

// BuildPlanAttachedPayload returns the JSON payload for a PLAN_ATTACHED message,
// emitted when a plan is created from a thread.
func BuildPlanAttachedPayload(planConfigurationID, templateID string) string {
	return mustEncodeJSON(planAttachedPayload{PlanConfigurationID: planConfigurationID, TemplateID: templateID})
}
```

- [ ] **Step 4: Run tests (expect PASS)**

```bash
cd /home/thbertoldi/harpia/control-plane && go test ./internal/chat -count=1
```

Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
cd /home/thbertoldi/harpia
git add control-plane/internal/chat
git commit -m "feat: add plan proposal chat payload builders"
```

---

### Task 3: PlanClassifier port + LLM adapter

**Files:**
- Create: `control-plane/internal/copilot/classifier.go`
- Create: `control-plane/internal/copilot/llm_classifier.go`
- Create: `control-plane/internal/copilot/llm_classifier_test.go`

**Interfaces:**
- Produces:
  - `copilot.TemplateSummary{ID uuid.UUID; Key, Name, Description string; Inputs []InputParamSummary}`
  - `copilot.InputParamSummary{Key, Label, Type string; Required bool; OptionsJSON string}`
  - `copilot.Candidate{TemplateID uuid.UUID; Confidence float64; InputValuesJSON string}`
  - `copilot.ClassifyInput{TenantID uuid.UUID; Text string; Templates []TemplateSummary}`
  - `copilot.PlanClassifier` interface with `Classify(ctx, ClassifyInput) ([]Candidate, error)`
  - `copilot.CredentialResolver` interface + `copilot.NewLLMClassifier(resolver CredentialResolver, provider string, doer HTTPDoer) *LLMClassifier`

- [ ] **Step 1: Create the port + shared types**

Create `control-plane/internal/copilot/classifier.go`:

```go
// Package copilot routes a thread's natural-language message to a plan template
// and extracts template input values. The classifier is a port so the thread
// handler can be tested without a live model; the LLM adapter is the production
// implementation.
package copilot

import (
	"context"

	"github.com/google/uuid"
)

// InputParamSummary describes one template input parameter for the router prompt.
type InputParamSummary struct {
	Key         string
	Label       string
	Type        string // TemplateInputParameterType enum name, e.g. "TEMPLATE_INPUT_PARAMETER_TYPE_TEXT"
	Required    bool
	OptionsJSON string
}

// TemplateSummary is the catalog entry the router reasons over.
type TemplateSummary struct {
	ID          uuid.UUID
	Key         string
	Name        string
	Description string
	Inputs      []InputParamSummary
}

// Candidate is one proposed template with a confidence and extracted inputs.
type Candidate struct {
	TemplateID      uuid.UUID
	Confidence      float64
	InputValuesJSON string // JSON object keyed by input parameter key
}

// ClassifyInput is the request to the router.
type ClassifyInput struct {
	TenantID  uuid.UUID
	Text      string
	Templates []TemplateSummary
}

// PlanClassifier maps a user message to ranked template candidates.
type PlanClassifier interface {
	Classify(ctx context.Context, in ClassifyInput) ([]Candidate, error)
}
```

- [ ] **Step 2: Write failing adapter tests**

Create `control-plane/internal/copilot/llm_classifier_test.go`:

```go
package copilot

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/harpia/control-plane/internal/llm_config"
	"github.com/harpia/control-plane/internal/llm_config/secret"
)

type fakeResolver struct {
	cred *llm_config.ResolvedCredential
	err  error
}

func (f fakeResolver) Resolve(ctx context.Context, tenantID uuid.UUID, provider, model string) (*llm_config.ResolvedCredential, error) {
	return f.cred, f.err
}

type fakeDoer struct {
	resp *http.Response
	err  error
}

func (f fakeDoer) Do(*http.Request) (*http.Response, error) { return f.resp, f.err }

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{},
	}
}

func okResolver() fakeResolver {
	return fakeResolver{cred: &llm_config.ResolvedCredential{
		Provider:     "deepseek",
		DefaultModel: "deepseek-chat",
		APIKey:       secret.NewRedacted("sk-test"),
	}}
}

func templates() []TemplateSummary {
	return []TemplateSummary{{
		ID:   uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		Key:  "linkedin",
		Name: "LinkedIn Post",
		Inputs: []InputParamSummary{
			{Key: "theme", Type: "TEMPLATE_INPUT_PARAMETER_TYPE_TEXT"},
			{Key: "language", Type: "TEMPLATE_INPUT_PARAMETER_TYPE_LANGUAGE"},
		},
	}}
}

func TestClassifyParsesCandidatesAndFiltersInputs(t *testing.T) {
	// The model returns a known template key, plus an unknown input key that
	// must be dropped.
	content := `{"candidates":[{"template_key":"linkedin","confidence":0.92,"inputs":{"theme":"retail","language":"pt-BR","bogus":"x"}}]}`
	body := `{"choices":[{"message":{"content":` + quote(content) + `}}]}`
	c := NewLLMClassifier(okResolver(), "deepseek", fakeDoer{resp: jsonResponse(body)})

	got, err := c.Classify(context.Background(), ClassifyInput{Text: "post about retail", Templates: templates()})
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("candidates = %d, want 1", len(got))
	}
	if got[0].TemplateID != templates()[0].ID {
		t.Fatalf("template id = %s", got[0].TemplateID)
	}
	if got[0].Confidence != 0.92 {
		t.Fatalf("confidence = %v", got[0].Confidence)
	}
	if strings.Contains(got[0].InputValuesJSON, "bogus") {
		t.Fatalf("unknown input key not dropped: %s", got[0].InputValuesJSON)
	}
	if !strings.Contains(got[0].InputValuesJSON, "retail") {
		t.Fatalf("expected theme in inputs: %s", got[0].InputValuesJSON)
	}
}

func TestClassifyDropsUnknownTemplateKeys(t *testing.T) {
	content := `{"candidates":[{"template_key":"nope","confidence":0.9,"inputs":{}}]}`
	body := `{"choices":[{"message":{"content":` + quote(content) + `}}]}`
	c := NewLLMClassifier(okResolver(), "deepseek", fakeDoer{resp: jsonResponse(body)})
	got, err := c.Classify(context.Background(), ClassifyInput{Text: "x", Templates: templates()})
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected unknown template dropped, got %+v", got)
	}
}

func TestClassifyDegradesToEmptyOnResolverError(t *testing.T) {
	c := NewLLMClassifier(fakeResolver{err: errors.New("no llm configured")}, "deepseek", fakeDoer{})
	got, err := c.Classify(context.Background(), ClassifyInput{Text: "x", Templates: templates()})
	if err != nil {
		t.Fatalf("expected graceful degradation, got err: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty candidates, got %+v", got)
	}
}

func TestClassifyDegradesToEmptyOnHTTPError(t *testing.T) {
	c := NewLLMClassifier(okResolver(), "deepseek", fakeDoer{err: errors.New("boom")})
	got, err := c.Classify(context.Background(), ClassifyInput{Text: "x", Templates: templates()})
	if err != nil {
		t.Fatalf("expected graceful degradation, got err: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty candidates, got %+v", got)
	}
}

// quote JSON-encodes a string (adds surrounding quotes and escapes) for
// embedding model content inside a response body literal.
func quote(s string) string {
	b, _ := jsonMarshal(s)
	return string(b)
}
```

Note: `jsonMarshal` is a tiny helper you add in the adapter file (Step 3) so tests and adapter share one JSON entrypoint.

- [ ] **Step 3: Run tests (expect FAIL)**

```bash
cd /home/thbertoldi/harpia/control-plane && go test ./internal/copilot -count=1
```

Expected: compile failure / undefined `NewLLMClassifier`.

- [ ] **Step 4: Implement the LLM adapter**

Create `control-plane/internal/copilot/llm_classifier.go`:

```go
package copilot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/harpia/control-plane/internal/llm_config"
)

// HTTPDoer is the subset of *http.Client the adapter needs (for testability).
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// CredentialResolver resolves a tenant's LLM credential. *llm_config.Resolver
// satisfies this.
type CredentialResolver interface {
	Resolve(ctx context.Context, tenantID uuid.UUID, provider, model string) (*llm_config.ResolvedCredential, error)
}

// providerBaseURLs maps a provider to its OpenAI-compatible chat completions
// base URL. DeepSeek is OpenAI-compatible.
var providerBaseURLs = map[string]string{
	"deepseek": "https://api.deepseek.com",
	"openai":   "https://api.openai.com",
}

const classifyTimeout = 20 * time.Second

// LLMClassifier is the production PlanClassifier. It calls the tenant's
// configured OpenAI-compatible provider once and degrades to an empty result on
// any failure so the chat flow never hard-fails on the model.
type LLMClassifier struct {
	resolver CredentialResolver
	provider string
	http     HTTPDoer
}

func NewLLMClassifier(resolver CredentialResolver, provider string, doer HTTPDoer) *LLMClassifier {
	if provider == "" {
		provider = "deepseek"
	}
	if doer == nil {
		doer = &http.Client{Timeout: classifyTimeout}
	}
	return &LLMClassifier{resolver: resolver, provider: provider, http: doer}
}

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string        `json:"model"`
	Messages       []chatMessage `json:"messages"`
	Temperature    float64       `json:"temperature"`
	ResponseFormat struct {
		Type string `json:"type"`
	} `json:"response_format"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type modelCandidates struct {
	Candidates []struct {
		TemplateKey string         `json:"template_key"`
		Confidence  float64        `json:"confidence"`
		Inputs      map[string]any `json:"inputs"`
	} `json:"candidates"`
}

// Classify returns ranked candidates, or an empty slice (nil error) on any soft
// failure (no credential, HTTP error, unparseable response).
func (c *LLMClassifier) Classify(ctx context.Context, in ClassifyInput) ([]Candidate, error) {
	cred, err := c.resolver.Resolve(ctx, in.TenantID, c.provider, "")
	if err != nil || cred == nil || cred.APIKey.IsEmpty() {
		return []Candidate{}, nil
	}
	baseURL, ok := providerBaseURLs[c.provider]
	if !ok {
		return []Candidate{}, nil
	}
	model := cred.DefaultModel
	if model == "" {
		return []Candidate{}, nil
	}

	reqBody := chatRequest{
		Model:       model,
		Temperature: 0,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt(in.Templates)},
			{Role: "user", Content: in.Text},
		},
	}
	reqBody.ResponseFormat.Type = "json_object"
	raw, err := jsonMarshal(reqBody)
	if err != nil {
		return []Candidate{}, nil
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return []Candidate{}, nil
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cred.APIKey.Reveal())

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return []Candidate{}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return []Candidate{}, nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return []Candidate{}, nil
	}

	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil || len(parsed.Choices) == 0 {
		return []Candidate{}, nil
	}
	var mc modelCandidates
	if err := json.Unmarshal([]byte(parsed.Choices[0].Message.Content), &mc); err != nil {
		return []Candidate{}, nil
	}

	byKey := map[string]TemplateSummary{}
	for _, t := range in.Templates {
		byKey[t.Key] = t
	}

	out := make([]Candidate, 0, len(mc.Candidates))
	for _, cand := range mc.Candidates {
		tpl, ok := byKey[cand.TemplateKey]
		if !ok {
			continue // drop unknown template keys
		}
		allowed := map[string]struct{}{}
		for _, p := range tpl.Inputs {
			allowed[p.Key] = struct{}{}
		}
		filtered := map[string]any{}
		for k, v := range cand.Inputs {
			if _, ok := allowed[k]; ok {
				filtered[k] = v
			}
		}
		inputsJSON, err := jsonMarshal(filtered)
		if err != nil {
			inputsJSON = []byte("{}")
		}
		out = append(out, Candidate{
			TemplateID:      tpl.ID,
			Confidence:      cand.Confidence,
			InputValuesJSON: string(inputsJSON),
		})
	}
	return out, nil
}

func systemPrompt(templates []TemplateSummary) string {
	type promptParam struct {
		Key      string `json:"key"`
		Label    string `json:"label"`
		Type     string `json:"type"`
		Required bool   `json:"required"`
	}
	type promptTemplate struct {
		Key         string        `json:"key"`
		Name        string        `json:"name"`
		Description string        `json:"description"`
		Inputs      []promptParam `json:"inputs"`
	}
	catalog := make([]promptTemplate, 0, len(templates))
	for _, t := range templates {
		pt := promptTemplate{Key: t.Key, Name: t.Name, Description: t.Description}
		for _, p := range t.Inputs {
			pt.Inputs = append(pt.Inputs, promptParam{Key: p.Key, Label: p.Label, Type: p.Type, Required: p.Required})
		}
		catalog = append(catalog, pt)
	}
	catalogJSON, _ := jsonMarshal(catalog)
	return fmt.Sprintf(`You route a user's request to plan templates and extract input values.
Template catalog (JSON): %s

Respond ONLY with a JSON object of this exact shape:
{"candidates":[{"template_key":"<key from catalog>","confidence":<0.0-1.0>,"inputs":{"<input key>":"<value>"}}]}

Rules:
- Include at most 3 candidates, ranked by confidence (highest first).
- Only use template_key values that appear in the catalog.
- Only use input keys declared for that template. Omit inputs you cannot infer.
- If nothing in the catalog fits, return {"candidates":[]}.`, string(catalogJSON))
}
```

- [ ] **Step 5: Run tests (expect PASS)**

```bash
cd /home/thbertoldi/harpia/control-plane && go test ./internal/copilot -count=1
```

Expected: `ok`.

- [ ] **Step 6: Commit**

```bash
cd /home/thbertoldi/harpia
git add control-plane/internal/copilot
git commit -m "feat: add plan classifier port and llm adapter"
```

---

### Task 4: Template catalog adapter

**Files:**
- Create: `control-plane/internal/plans/template_catalog.go`
- Create: `control-plane/internal/plans/template_catalog_test.go`

**Interfaces:**
- Consumes: `copilot.TemplateSummary`, `copilot.InputParamSummary`; `plans.Repository`; existing `templateToProto` (in `plans/handler.go`).
- Produces: `plans.NewCopilotCatalog(repo *Repository) *CopilotCatalog` with `ListTemplateSummaries(ctx) ([]copilot.TemplateSummary, error)`; and pure mapper `templateSummaryFromProto(p *plansv1.PlanTemplate, id uuid.UUID) copilot.TemplateSummary`.

- [ ] **Step 1: Write failing mapper test**

Create `control-plane/internal/plans/template_catalog_test.go`:

```go
package plans

import (
	"testing"

	"github.com/google/uuid"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
)

func TestTemplateSummaryFromProto(t *testing.T) {
	id := uuid.New()
	p := &plansv1.PlanTemplate{
		Key:         "linkedin",
		Name:        "LinkedIn Post",
		Description: "Generate a post",
		InputParameters: []*plansv1.TemplateInputParameter{
			{Key: "theme", Label: "Theme", Type: plansv1.TemplateInputParameterType_TEMPLATE_INPUT_PARAMETER_TYPE_TEXT, Required: true},
			{Key: "language", Label: "Language", Type: plansv1.TemplateInputParameterType_TEMPLATE_INPUT_PARAMETER_TYPE_LANGUAGE, OptionsJson: `["pt-BR","en-US"]`},
		},
	}
	got := templateSummaryFromProto(p, id)
	if got.ID != id || got.Key != "linkedin" || got.Name != "LinkedIn Post" {
		t.Fatalf("summary header = %+v", got)
	}
	if len(got.Inputs) != 2 {
		t.Fatalf("inputs = %d, want 2", len(got.Inputs))
	}
	if got.Inputs[0].Key != "theme" || !got.Inputs[0].Required {
		t.Fatalf("input[0] = %+v", got.Inputs[0])
	}
	if got.Inputs[1].Type != "TEMPLATE_INPUT_PARAMETER_TYPE_LANGUAGE" {
		t.Fatalf("input[1].Type = %q", got.Inputs[1].Type)
	}
	if got.Inputs[1].OptionsJSON != `["pt-BR","en-US"]` {
		t.Fatalf("input[1].OptionsJSON = %q", got.Inputs[1].OptionsJSON)
	}
}
```

- [ ] **Step 2: Run test (expect FAIL)**

```bash
cd /home/thbertoldi/harpia/control-plane && go test ./internal/plans -run TestTemplateSummaryFromProto -count=1
```

Expected: undefined `templateSummaryFromProto`.

- [ ] **Step 3: Implement the catalog adapter**

Create `control-plane/internal/plans/template_catalog.go`:

```go
package plans

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/copilot"
)

// CopilotCatalog adapts the plan template repository to the copilot router's
// TemplateSummary view. Templates are global (not tenant-scoped) in the current
// schema, matching Repository.ListTemplates.
type CopilotCatalog struct {
	repo *Repository
}

func NewCopilotCatalog(repo *Repository) *CopilotCatalog {
	return &CopilotCatalog{repo: repo}
}

// ListTemplateSummaries returns all templates as router catalog entries.
func (c *CopilotCatalog) ListTemplateSummaries(ctx context.Context) ([]copilot.TemplateSummary, error) {
	tpls, err := c.repo.ListTemplates(ctx, "", 100, 0)
	if err != nil {
		return nil, fmt.Errorf("list template summaries: %w", err)
	}
	out := make([]copilot.TemplateSummary, 0, len(tpls))
	for i := range tpls {
		proto := templateToProto(&tpls[i])
		out = append(out, templateSummaryFromProto(proto, tpls[i].ID))
	}
	return out, nil
}

// templateSummaryFromProto maps a proto template to a router summary. It reuses
// templateToProto's parsing of input_parameters (see plans/handler.go).
func templateSummaryFromProto(p *plansv1.PlanTemplate, id uuid.UUID) copilot.TemplateSummary {
	summary := copilot.TemplateSummary{
		ID:          id,
		Key:         p.GetKey(),
		Name:        p.GetName(),
		Description: p.GetDescription(),
	}
	for _, ip := range p.GetInputParameters() {
		summary.Inputs = append(summary.Inputs, copilot.InputParamSummary{
			Key:         ip.GetKey(),
			Label:       ip.GetLabel(),
			Type:        ip.GetType().String(),
			Required:    ip.GetRequired(),
			OptionsJSON: ip.GetOptionsJson(),
		})
	}
	return summary
}
```

- [ ] **Step 4: Run test (expect PASS)**

```bash
cd /home/thbertoldi/harpia/control-plane && go test ./internal/plans -run TestTemplateSummaryFromProto -count=1
```

Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
cd /home/thbertoldi/harpia
git add control-plane/internal/plans/template_catalog.go control-plane/internal/plans/template_catalog_test.go
git commit -m "feat: add copilot template catalog adapter"
```

---

### Task 5: ProposePlan handler + wiring

**Files:**
- Modify: `control-plane/internal/threads/handler.go`
- Modify: `control-plane/internal/threads/handler_test.go`
- Modify: `control-plane/cmd/api/main.go`

**Interfaces:**
- Consumes: `copilot.PlanClassifier`, `copilot.ClassifyInput`, `copilot.Candidate`, `copilot.TemplateSummary`; `chat.BuildPlanProposedPayload`, `chat.PlanProposalCandidate`; `plans.NewCopilotCatalog`.
- Produces: `threads.TemplateCatalog` interface; extended `threads.NewHandler(repo RepositoryAPI, chatStore chat.Store, catalog TemplateCatalog, classifier copilot.PlanClassifier) *Handler`; real `Handler.ProposePlan`.

- [ ] **Step 1: Write failing handler tests**

Add to `control-plane/internal/threads/handler_test.go` (append; keep existing tests):

```go
type fakeCatalog struct {
	summaries []copilot.TemplateSummary
}

func (f fakeCatalog) ListTemplateSummaries(ctx context.Context) ([]copilot.TemplateSummary, error) {
	return f.summaries, nil
}

type fakeClassifier struct {
	candidates []copilot.Candidate
}

func (f fakeClassifier) Classify(ctx context.Context, in copilot.ClassifyInput) ([]copilot.Candidate, error) {
	return f.candidates, nil
}

func TestProposePlanEmitsPlanProposed(t *testing.T) {
	tenantID := uuid.New()
	threadID := uuid.New()
	tplID := uuid.New()
	messages := &fakeMessageStore{}
	// Seed a latest USER_TEXT in the thread.
	_, _ = messages.AppendMessage(context.Background(), tenantID, chat.AppendInput{
		ThreadID: threadID.String(),
		Role:     chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_OVERSEER,
		Kind:     chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_TEXT,
		Text:     "Create a LinkedIn post about retail",
	})
	catalog := fakeCatalog{summaries: []copilot.TemplateSummary{{ID: tplID, Key: "linkedin", Name: "LinkedIn Post"}}}
	classifier := fakeClassifier{candidates: []copilot.Candidate{{TemplateID: tplID, Confidence: 0.9, InputValuesJSON: `{"theme":"retail"}`}}}
	h := NewHandler(&fakeThreadRepo{}, messages, catalog, classifier)
	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		UserID: uuid.NewString(), TenantID: tenantID, Roles: []string{"Overseer"},
	})

	resp, err := h.ProposePlan(ctx, connect.NewRequest(&chatv1.ProposePlanRequest{
		TenantId: tenantID.String(), ThreadId: threadID.String(),
	}))
	if err != nil {
		t.Fatalf("ProposePlan: %v", err)
	}
	if resp.Msg.Message.GetKind() != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_PLAN_PROPOSED {
		t.Fatalf("kind = %v", resp.Msg.Message.GetKind())
	}
	if !strings.Contains(resp.Msg.Message.GetPayloadJson(), "linkedin") {
		t.Fatalf("payload = %s", resp.Msg.Message.GetPayloadJson())
	}
	if !strings.Contains(resp.Msg.Message.GetPayloadJson(), "retail") {
		t.Fatalf("payload missing inferred inputs: %s", resp.Msg.Message.GetPayloadJson())
	}
}

func TestProposePlanRequiresUserMessage(t *testing.T) {
	tenantID := uuid.New()
	threadID := uuid.New()
	h := NewHandler(&fakeThreadRepo{}, &fakeMessageStore{}, fakeCatalog{}, fakeClassifier{})
	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		UserID: uuid.NewString(), TenantID: tenantID, Roles: []string{"Overseer"},
	})
	_, err := h.ProposePlan(ctx, connect.NewRequest(&chatv1.ProposePlanRequest{
		TenantId: tenantID.String(), ThreadId: threadID.String(),
	}))
	if err == nil {
		t.Fatal("expected error when no user message exists")
	}
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("code = %v, want FailedPrecondition", connect.CodeOf(err))
	}
}

func TestProposePlanEmptyCandidatesStillEmits(t *testing.T) {
	tenantID := uuid.New()
	threadID := uuid.New()
	messages := &fakeMessageStore{}
	_, _ = messages.AppendMessage(context.Background(), tenantID, chat.AppendInput{
		ThreadID: threadID.String(),
		Role:     chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_OVERSEER,
		Kind:     chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_TEXT,
		Text:     "something off-catalog",
	})
	h := NewHandler(&fakeThreadRepo{}, messages, fakeCatalog{}, fakeClassifier{candidates: nil})
	ctx := identity.WithRequestContext(context.Background(), identity.RequestContext{
		UserID: uuid.NewString(), TenantID: tenantID, Roles: []string{"Overseer"},
	})
	resp, err := h.ProposePlan(ctx, connect.NewRequest(&chatv1.ProposePlanRequest{
		TenantId: tenantID.String(), ThreadId: threadID.String(),
	}))
	if err != nil {
		t.Fatalf("ProposePlan: %v", err)
	}
	if resp.Msg.Message.GetKind() != chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_PLAN_PROPOSED {
		t.Fatalf("kind = %v", resp.Msg.Message.GetKind())
	}
}
```

Add these imports to the test file's import block if missing: `"strings"`, `"github.com/harpia/control-plane/internal/copilot"`. (`chat`, `chatv1`, `identity`, `uuid`, `connect`, `context`, `testing` are already imported.)

Also update the existing `TestCreateThreadPersistsInitialMessage` construction from:

```go
h := NewHandler(repo, messages)
```

to:

```go
h := NewHandler(repo, messages, fakeCatalog{}, fakeClassifier{})
```

- [ ] **Step 2: Run tests (expect FAIL)**

```bash
cd /home/thbertoldi/harpia/control-plane && go test ./internal/threads -count=1
```

Expected: compile failure (`NewHandler` arity, `ProposePlan` stub returns Unimplemented).

- [ ] **Step 3: Extend the handler struct, interface, and constructor**

In `control-plane/internal/threads/handler.go`:

Add imports `"github.com/harpia/control-plane/internal/copilot"` and confirm `"strings"` is present (it is).

Add the catalog interface near the top (after the existing `RepositoryAPI` interface):

```go
// TemplateCatalog lists plan templates as router catalog entries. Implemented
// by plans.CopilotCatalog.
type TemplateCatalog interface {
	ListTemplateSummaries(ctx context.Context) ([]copilot.TemplateSummary, error)
}
```

Extend the `Handler` struct:

```go
type Handler struct {
	repo       RepositoryAPI
	chat       chat.Store
	catalog    TemplateCatalog
	classifier copilot.PlanClassifier
}
```

Replace `NewHandler`:

```go
func NewHandler(repo RepositoryAPI, chatStore chat.Store, catalog TemplateCatalog, classifier copilot.PlanClassifier) *Handler {
	return &Handler{
		repo:       repo,
		chat:       chatStore,
		catalog:    catalog,
		classifier: classifier,
	}
}
```

- [ ] **Step 4: Replace the stub ProposePlan with the real implementation**

In `control-plane/internal/threads/handler.go`, replace the Task 1 stub `ProposePlan` with:

```go
const planProposalConfidenceThreshold = 0.6

// ProposePlan classifies the thread's latest user message against the template
// catalog and appends a PLAN_PROPOSED message. It always succeeds when a user
// message exists, even if the router returns no candidates (empty proposal).
func (h *Handler) ProposePlan(ctx context.Context, req *connect.Request[chatv1.ProposePlanRequest]) (*connect.Response[chatv1.ProposePlanResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.GetTenantId())
	if err != nil {
		return nil, err
	}
	threadID, err := parseRequiredUUID(req.Msg.GetThreadId(), "thread_id")
	if err != nil {
		return nil, err
	}

	// Find the latest USER_TEXT message in the thread.
	msgs, err := h.chat.ListMessages(ctx, tenantID, threadID.String(), 0, 0)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var latestUser *chatv1.ThreadMessage
	for _, m := range msgs {
		if m.GetKind() == chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_TEXT {
			latestUser = m
		}
	}
	if latestUser == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no user message to propose from"))
	}

	// Load the catalog and classify. Both degrade to an empty proposal.
	var summaries []copilot.TemplateSummary
	if h.catalog != nil {
		if s, catErr := h.catalog.ListTemplateSummaries(ctx); catErr == nil {
			summaries = s
		}
	}
	var candidates []copilot.Candidate
	if h.classifier != nil && len(summaries) > 0 {
		if c, clsErr := h.classifier.Classify(ctx, copilot.ClassifyInput{
			TenantID:  tenantID,
			Text:      latestUser.GetText(),
			Templates: summaries,
		}); clsErr == nil {
			candidates = c
		}
	}

	nameByID := map[string]string{}
	keyByID := map[string]string{}
	for _, s := range summaries {
		nameByID[s.ID.String()] = s.Name
		keyByID[s.ID.String()] = s.Key
	}

	payloadCandidates := make([]chat.PlanProposalCandidate, 0, len(candidates))
	for _, c := range candidates {
		payloadCandidates = append(payloadCandidates, chat.PlanProposalCandidate{
			TemplateID:      c.TemplateID.String(),
			TemplateKey:     keyByID[c.TemplateID.String()],
			TemplateName:    nameByID[c.TemplateID.String()],
			Confidence:      c.Confidence,
			InputValuesJSON: c.InputValuesJSON,
		})
	}

	msg, err := h.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
		ThreadID:    threadID.String(),
		Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_AGENT,
		Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_PLAN_PROPOSED,
		Text:        "Proposed a plan.",
		PayloadJSON: chat.BuildPlanProposedPayload(latestUser.GetId(), payloadCandidates),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&chatv1.ProposePlanResponse{Message: msg}), nil
}
```

Note: `planProposalConfidenceThreshold` is consumed by the frontend card for the single-vs-shortlist decision; it is defined here as the documented server-side constant and may be referenced in a future server-side ranking refinement. If Go reports it as unused, prefix with `var _ = planProposalConfidenceThreshold` is NOT needed because a `const` may be unused in Go. Leave as a `const`.

- [ ] **Step 5: Wire the classifier + catalog in main.go**

In `control-plane/cmd/api/main.go`:

Add import `"os"` (already present) and construct the catalog + classifier after `llmResolver` is created (around line 294-304) and before `threadHandler` is created (the foundation created `threadHandler := threadsvc.NewHandler(threadRepo, chatStore)` near line 229). Move/adjust so the classifier exists first. Concretely:

Replace the existing:

```go
	threadRepo := threadsvc.NewRepository(pool)
	threadHandler := threadsvc.NewHandler(threadRepo, chatStore)
```

with:

```go
	threadRepo := threadsvc.NewRepository(pool)
```

Then, after `llmResolver` is constructed (after the `llm_config.NewResolver(...)` block), add:

```go
	planProposalProvider := os.Getenv("HARPIA_LLM_PROPOSAL_PROVIDER")
	if planProposalProvider == "" {
		planProposalProvider = "deepseek"
	}
	planClassifier := copilot.NewLLMClassifier(llmResolver, planProposalProvider, nil)
	threadHandler := threadsvc.NewHandler(threadRepo, chatStore, plans.NewCopilotCatalog(planRepo), planClassifier)
```

Add import `"github.com/harpia/control-plane/internal/copilot"` to `main.go`.

- [ ] **Step 6: Run tests + build (expect PASS)**

```bash
cd /home/thbertoldi/harpia/control-plane && go build ./... && go test ./internal/threads ./cmd/api -count=1
```

Expected: build succeeds; `ok` for both packages.

- [ ] **Step 7: Commit**

```bash
cd /home/thbertoldi/harpia
git add control-plane/internal/threads control-plane/cmd/api/main.go
git commit -m "feat: implement ProposePlan handler and wire classifier"
```

---

### Task 6: Emit PLAN_ATTACHED on thread-bound create

**Files:**
- Modify: `control-plane/internal/plans/handler.go`

**Interfaces:**
- Consumes: `chat.BuildPlanAttachedPayload`.

- [ ] **Step 1: Emit PLAN_ATTACHED in CreatePlanConfiguration**

In `control-plane/internal/plans/handler.go`, inside `CreatePlanConfiguration`, after the successful `created, err := h.repo.CreateConfiguration(ctx, config)` block and its schedule sync, and BEFORE the existing `if h.assistant != nil { ... SeedThread ... }` block, add:

```go
	// Path B: announce that a plan was attached to the owning thread, before the
	// assistant seeds the binding-matrix prompt.
	if h.chat != nil && created.ThreadID != uuid.Nil {
		_, _ = h.chat.AppendMessage(ctx, tenantID, chat.AppendInput{
			ThreadID:    created.ThreadID.String(),
			Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
			Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_PLAN_ATTACHED,
			Text:        "Plan attached.",
			PayloadJSON: chat.BuildPlanAttachedPayload(created.ID.String(), created.PlanTemplateID.String()),
		})
	}
```

(`chat`, `chatv1`, and `uuid` are already imported in this file.)

- [ ] **Step 2: Build + run plans tests**

```bash
cd /home/thbertoldi/harpia/control-plane && go build ./... && go test ./internal/plans -count=1
```

Expected: build succeeds; `ok`. (This emission is exercised end-to-end in Task 11's smoke; a DB-free unit test is not feasible because `CreatePlanConfiguration` requires the repository.)

- [ ] **Step 3: Commit**

```bash
cd /home/thbertoldi/harpia
git add control-plane/internal/plans/handler.go
git commit -m "feat: emit PLAN_ATTACHED when a plan is created from a thread"
```

---

### Task 7: Frontend message-kind mappings

**Files:**
- Modify: `frontend/src/lib/chat/types.ts`

- [ ] **Step 1: Add the 8 kinds to the union**

In `frontend/src/lib/chat/types.ts`, extend the `ChatMessageKind` union (after `"APPROVAL_DECIDED"`):

```typescript
  | "PLAN_PROPOSED"
  | "PLAN_ATTACHED"
  | "PLAN_UPDATED"
  | "PLAN_RUN_REQUESTED"
  | "ARTIFACT_CREATED"
  | "ARTIFACT_UPDATED"
  | "ERROR_RAISED"
  | "ERROR_RECOVERED";
```

(Move the trailing `;` from the old last entry to the new last entry.)

- [ ] **Step 2: Add to KIND_FROM_PROTO**

In the same file, add to the `KIND_FROM_PROTO` object (before the closing `}`):

```typescript
  [ProtoThreadMessageKind.PLAN_PROPOSED]: "PLAN_PROPOSED",
  [ProtoThreadMessageKind.PLAN_ATTACHED]: "PLAN_ATTACHED",
  [ProtoThreadMessageKind.PLAN_UPDATED]: "PLAN_UPDATED",
  [ProtoThreadMessageKind.PLAN_RUN_REQUESTED]: "PLAN_RUN_REQUESTED",
  [ProtoThreadMessageKind.ARTIFACT_CREATED]: "ARTIFACT_CREATED",
  [ProtoThreadMessageKind.ARTIFACT_UPDATED]: "ARTIFACT_UPDATED",
  [ProtoThreadMessageKind.ERROR_RAISED]: "ERROR_RAISED",
  [ProtoThreadMessageKind.ERROR_RECOVERED]: "ERROR_RECOVERED",
```

- [ ] **Step 3: Add to KIND_TO_PROTO**

Add to the `KIND_TO_PROTO` object (before the closing `}`):

```typescript
  PLAN_PROPOSED: ProtoThreadMessageKind.PLAN_PROPOSED,
  PLAN_ATTACHED: ProtoThreadMessageKind.PLAN_ATTACHED,
  PLAN_UPDATED: ProtoThreadMessageKind.PLAN_UPDATED,
  PLAN_RUN_REQUESTED: ProtoThreadMessageKind.PLAN_RUN_REQUESTED,
  ARTIFACT_CREATED: ProtoThreadMessageKind.ARTIFACT_CREATED,
  ARTIFACT_UPDATED: ProtoThreadMessageKind.ARTIFACT_UPDATED,
  ERROR_RAISED: ProtoThreadMessageKind.ERROR_RAISED,
  ERROR_RECOVERED: ProtoThreadMessageKind.ERROR_RECOVERED,
```

- [ ] **Step 4: Type check (no new errors in this file)**

```bash
cd /home/thbertoldi/harpia/frontend && bun run check 2>&1 | grep "chat/types.ts" || echo "NO new errors in types.ts"
```

Expected: `NO new errors in types.ts`.

- [ ] **Step 5: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/chat/types.ts
git commit -m "feat: map new plan/artifact/error message kinds"
```

---

### Task 8: Generic template inputs helpers + form

**Files:**
- Modify: `frontend/src/lib/plans/template-inputs.ts`
- Create: `frontend/src/lib/plans/template-inputs.test.ts`
- Create: `frontend/src/lib/components/thread/TemplateInputsForm.svelte`

**Interfaces:**
- Produces:
  - `genericInputInitialValues(params: TemplateInputParameter[], extracted: Record<string, unknown>): Record<string, string>`
  - `genericParameterValuesJson(values: Record<string, string>): string`

- [ ] **Step 1: Write failing helper tests**

Create `frontend/src/lib/plans/template-inputs.test.ts`:

```typescript
import { describe, expect, it } from "vitest";
import {
  genericInputInitialValues,
  genericParameterValuesJson,
} from "./template-inputs";
import type { TemplateInputParameter } from "$lib/gen/harpia/plans/v1/plans_pb";
import { TemplateInputParameterType } from "$lib/gen/harpia/plans/v1/plans_pb";

function param(
  key: string,
  type: TemplateInputParameterType,
  defaultValueJson = "",
): TemplateInputParameter {
  return {
    $typeName: "harpia.plans.v1.TemplateInputParameter",
    key,
    label: key,
    description: "",
    type,
    required: false,
    defaultValueJson,
    optionsJson: "",
    runtimeMappings: [],
  } as TemplateInputParameter;
}

describe("genericInputInitialValues", () => {
  it("prefers extracted values, falls back to default_value_json, then empty", () => {
    const params = [
      param("theme", TemplateInputParameterType.TEXT),
      param("language", TemplateInputParameterType.LANGUAGE, '"en-US"'),
      param("tone", TemplateInputParameterType.TEXT),
    ];
    const got = genericInputInitialValues(params, { theme: "retail" });
    expect(got.theme).toBe("retail");
    expect(got.language).toBe("en-US");
    expect(got.tone).toBe("");
  });
});

describe("genericParameterValuesJson", () => {
  it("serializes a flat values object", () => {
    const json = genericParameterValuesJson({ theme: "retail", tone: "formal" });
    expect(JSON.parse(json)).toEqual({ theme: "retail", tone: "formal" });
  });
});
```

- [ ] **Step 2: Run test (expect FAIL)**

```bash
cd /home/thbertoldi/harpia/frontend && bunx vitest run src/lib/plans/template-inputs.test.ts
```

Expected: fails — helpers not exported.

- [ ] **Step 3: Add the generic helpers**

Append to `frontend/src/lib/plans/template-inputs.ts`:

```typescript
import type { TemplateInputParameter } from "$lib/gen/harpia/plans/v1/plans_pb";

// genericInputInitialValues builds a flat string map for a template's inputs.
// Priority per key: extracted value > default_value_json > "".
export function genericInputInitialValues(
  params: TemplateInputParameter[],
  extracted: Record<string, unknown>,
): Record<string, string> {
  const out: Record<string, string> = {};
  for (const p of params) {
    if (p.key in extracted && extracted[p.key] != null) {
      out[p.key] = String(extracted[p.key]);
      continue;
    }
    let fallback = "";
    if (p.defaultValueJson?.trim()) {
      try {
        const parsed = JSON.parse(p.defaultValueJson);
        fallback = parsed == null ? "" : String(parsed);
      } catch {
        fallback = "";
      }
    }
    out[p.key] = fallback;
  }
  return out;
}

// genericParameterValuesJson serializes the form's flat values to the
// parameter_values_json string CreatePlanConfiguration expects.
export function genericParameterValuesJson(
  values: Record<string, string>,
): string {
  return JSON.stringify(values);
}

// selectOptions parses a template parameter's options_json (a JSON string array)
// into option values; returns [] on any parse failure.
export function selectOptions(optionsJson: string): string[] {
  if (!optionsJson?.trim()) return [];
  try {
    const parsed = JSON.parse(optionsJson);
    return Array.isArray(parsed) ? parsed.map((v) => String(v)) : [];
  } catch {
    return [];
  }
}
```

- [ ] **Step 4: Run test (expect PASS)**

```bash
cd /home/thbertoldi/harpia/frontend && bunx vitest run src/lib/plans/template-inputs.test.ts
```

Expected: 2 passing.

- [ ] **Step 5: Create the generic form component**

Create `frontend/src/lib/components/thread/TemplateInputsForm.svelte`:

```svelte
<script lang="ts">
  import type { TemplateInputParameter } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { TemplateInputParameterType } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { selectOptions } from "$lib/plans/template-inputs";

  interface Props {
    params: TemplateInputParameter[];
    values: Record<string, string>;
  }

  // `values` is bindable so the parent reads edits back.
  let { params, values = $bindable() }: Props = $props();

  const T = TemplateInputParameterType;
</script>

<div class="flex flex-col gap-3">
  {#each params as p (p.key)}
    <label class="flex flex-col gap-1 text-[12px] text-crown-ash">
      <span class="font-medium text-cream">
        {p.label || p.key}{#if p.required}<span class="text-talon-gold"> *</span>{/if}
      </span>
      {#if p.description}
        <span class="text-[11px] text-crown-ash-dark">{p.description}</span>
      {/if}

      {#if p.type === T.TEXTAREA}
        <textarea
          bind:value={values[p.key]}
          rows="3"
          class="rounded border border-plumage bg-obsidian-light px-2 py-1 text-[12px] text-cream"
        ></textarea>
      {:else if p.type === T.SELECT || p.type === T.LANGUAGE}
        <select
          bind:value={values[p.key]}
          class="rounded border border-plumage bg-obsidian-light px-2 py-1 text-[12px] text-cream"
        >
          <option value="">—</option>
          {#each selectOptions(p.optionsJson) as opt (opt)}
            <option value={opt}>{opt}</option>
          {/each}
        </select>
      {:else}
        <!-- TEXT, DATE_RANGE, INTEGRATION_SELECTOR, UNSPECIFIED fall back to text input in D -->
        <input
          type="text"
          bind:value={values[p.key]}
          class="rounded border border-plumage bg-obsidian-light px-2 py-1 text-[12px] text-cream"
        />
      {/if}
    </label>
  {/each}
</div>
```

- [ ] **Step 6: Type check (no new errors)**

```bash
cd /home/thbertoldi/harpia/frontend && bun run check 2>&1 | grep -E "template-inputs|TemplateInputsForm" || echo "NO new errors"
```

Expected: `NO new errors`.

- [ ] **Step 7: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/plans/template-inputs.ts frontend/src/lib/plans/template-inputs.test.ts frontend/src/lib/components/thread/TemplateInputsForm.svelte
git commit -m "feat: add generic template inputs form and helpers"
```

---

### Task 9: PlanProposalCard + dispatch

**Files:**
- Create: `frontend/src/lib/components/thread/PlanProposalCard.svelte`
- Modify: `frontend/src/lib/components/thread/ThreadMessage.svelte`

**Interfaces:**
- Consumes: `planClient.getPlanTemplate`, the shared `savePlanConfigurationRecord` helper (`$lib/plans/plan-configuration`), `TemplateInputsForm`, `genericInputInitialValues`, `genericParameterValuesJson`.

- [ ] **Step 1: Create the proposal card**

Create `frontend/src/lib/components/thread/PlanProposalCard.svelte`:

```svelte
<script lang="ts">
  import { goto } from "$app/navigation";
  import { resolve } from "$app/paths";
  import type { ChatMessage } from "$lib/chat/types";
  import { planClient } from "$lib/rpc";
  import type {
    PlanTemplate,
    TemplateInputParameter,
  } from "$lib/gen/harpia/plans/v1/plans_pb";
  import { PlanConfigurationStatus } from "$lib/gen/harpia/plans/v1/plans_pb";
  import TemplateInputsForm from "./TemplateInputsForm.svelte";
  import {
    genericInputInitialValues,
    genericParameterValuesJson,
  } from "$lib/plans/template-inputs";

  interface Candidate {
    template_id: string;
    template_key: string;
    template_name: string;
    confidence: number;
    input_values_json: string;
  }

  interface Props {
    message: ChatMessage;
    tenantId: string;
    threadId: string;
  }

  let { message, tenantId, threadId }: Props = $props();

  const confidenceThreshold = 0.6;

  const candidates = $derived.by<Candidate[]>(() => {
    try {
      const parsed = JSON.parse(message.payloadJson || "{}");
      return Array.isArray(parsed.candidates) ? parsed.candidates : [];
    } catch {
      return [];
    }
  });

  // Auto-select the single confident candidate; otherwise wait for a pick.
  const autoSelected = $derived.by<Candidate | null>(() => {
    if (candidates.length === 1 && candidates[0].confidence >= confidenceThreshold) {
      return candidates[0];
    }
    return null;
  });

  let selected = $state<Candidate | null>(null);
  const active = $derived(selected ?? autoSelected);

  let template = $state<PlanTemplate | null>(null);
  let values = $state<Record<string, string>>({});
  let loading = $state(false);
  let creating = $state(false);
  let errorMsg = $state<string | null>(null);

  // Load the active candidate's template and seed the form values.
  $effect(() => {
    const cand = active;
    template = null;
    if (!cand) return;
    loading = true;
    (async () => {
      try {
        const resp = await planClient.getPlanTemplate({
          planTemplateId: cand.template_id,
        });
        const tpl = resp.planTemplate ?? null;
        template = tpl;
        const params: TemplateInputParameter[] = tpl?.inputParameters ?? [];
        let extracted: Record<string, unknown> = {};
        try {
          extracted = JSON.parse(cand.input_values_json || "{}");
        } catch {
          extracted = {};
        }
        values = genericInputInitialValues(params, extracted);
      } catch {
        errorMsg = "Failed to load template.";
      } finally {
        loading = false;
      }
    })();
  });

  async function confirm() {
    const cand = active;
    if (!cand || !template || creating) return;
    creating = true;
    errorMsg = null;
    try {
      const saved = await savePlan(cand);
      if (saved) await goto(resolve(`/chat/${threadId}`), { invalidateAll: true });
    } catch (e) {
      errorMsg = e instanceof Error ? e.message : "Failed to create plan.";
    } finally {
      creating = false;
    }
  }

  async function savePlan(cand: Candidate) {
    const { savePlanConfigurationRecord } = await import(
      "$lib/plans/plan-configuration"
    );
    if (!template) return null;
    return savePlanConfigurationRecord({
      tenantId,
      template,
      status: PlanConfigurationStatus.DRAFT,
      threadId,
      // parameter values flow through the shared helper below
      parameterValuesJson: genericParameterValuesJson(values),
    });
  }
</script>

<div
  id={`m-${message.id}`}
  class="max-w-[85%] self-start rounded-lg border border-plumage bg-obsidian-light px-3 py-3"
>
  {#if candidates.length === 0}
    <p class="text-[13px] text-crown-ash">
      I couldn't match your request to a plan.
      <a class="text-talon-gold underline" href={resolve("/new")}>Browse templates</a>.
    </p>
  {:else if !active}
    <p class="mb-2 text-[13px] text-cream">Which plan did you mean?</p>
    <div class="flex flex-col gap-2">
      {#each candidates as c (c.template_id)}
        <button
          class="rounded border border-plumage px-2 py-1 text-left text-[12px] text-cream hover:border-talon-gold"
          onclick={() => (selected = c)}
        >
          {c.template_name || c.template_key}
        </button>
      {/each}
    </div>
  {:else}
    <p class="mb-2 text-[13px] font-medium text-cream">
      {active.template_name || active.template_key}
    </p>
    {#if loading}
      <p class="text-[12px] text-crown-ash-dark">Loading template…</p>
    {:else if template}
      <TemplateInputsForm
        params={template.inputParameters}
        bind:values
      />
      <button
        class="mt-3 rounded bg-talon-gold px-3 py-1 text-[12px] font-medium text-obsidian disabled:opacity-50"
        onclick={confirm}
        disabled={creating}
      >
        {creating ? "Creating…" : "Create this plan"}
      </button>
    {/if}
  {/if}
  {#if errorMsg}
    <p class="mt-2 text-[11px] text-red-400">{errorMsg}</p>
  {/if}
</div>
```

Note: `savePlanConfigurationRecord` must accept `parameterValuesJson`. Verify the helper's `PlanConfigurationSaveInput` — from the foundation it has `threadId?`. If it does NOT already pass `parameterValuesJson` to `createPlanConfiguration`, add that pass-through in Step 2.

- [ ] **Step 2: Ensure the save helper forwards parameterValuesJson + threadId**

In `frontend/src/lib/plans/plan-configuration.ts`, confirm `PlanConfigurationSaveInput` includes:

```typescript
  threadId?: string;
  parameterValuesJson?: string;
```

If `parameterValuesJson` is missing, add it to the interface, destructure it in `savePlanConfigurationRecord`, and pass `parameterValuesJson: parameterValuesJson ?? existing?.parameterValuesJson ?? ""` to BOTH the `updatePlanConfiguration` and `createPlanConfiguration` calls. (`threadId` pass-through already exists from the foundation.)

Note: `PLAN_PROPOSED` only ever appears in a config-less thread (before a plan is attached). It is therefore rendered directly by the chat page's config-less branch (Task 10), NOT through `ThreadMessage.svelte`/`buildThreadSections`. This keeps the render path self-contained and avoids depending on section classification. Wiring `PLAN_PROPOSED` into the canonical `ThreadMessage` dispatch is deferred to Milestone E (if proposals ever surface in configured threads).

- [ ] **Step 3: Type check (no new errors in touched files)**

```bash
cd /home/thbertoldi/harpia/frontend && bun run check 2>&1 | grep -E "PlanProposalCard|plan-configuration.ts" || echo "NO new errors"
```

Expected: `NO new errors`.

- [ ] **Step 4: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/components/thread/PlanProposalCard.svelte frontend/src/lib/plans/plan-configuration.ts
git commit -m "feat: add plan proposal card"
```

---

### Task 10: Chat composer always-on + auto-propose; /new prompt-first

**Files:**
- Modify: `frontend/src/routes/chat/[threadId]/+page.svelte`
- Modify: `frontend/src/routes/new/+page.svelte`

**Interfaces:**
- Consumes: `threadClient.proposePlan` (generated), `ThreadComposer`, `PlanProposalCard`.

- [ ] **Step 1: Always render the composer + auto-propose on the chat page**

In `frontend/src/routes/chat/[threadId]/+page.svelte`:

Add imports at the top of the script (alongside existing imports):

```typescript
  import { threadClient } from "$lib/rpc";
  import PlanProposalCard from "$lib/components/thread/PlanProposalCard.svelte";
```

`ThreadComposer` is already imported by the page. Add proposal state + trigger logic (after the existing `messages` state declaration):

```typescript
  let proposing = $state(false);

  // True when a PLAN_PROPOSED already follows the most recent USER_TEXT.
  const hasProposalForLatestUser = $derived.by(() => {
    let sawUser = false;
    for (const m of messages) {
      if (m.kind === "USER_TEXT") {
        sawUser = true;
        continue;
      }
      if (sawUser && m.kind === "PLAN_PROPOSED") return true;
    }
    return false;
  });

  const latestIsUnansweredUser = $derived.by(() => {
    for (let i = messages.length - 1; i >= 0; i--) {
      if (messages[i].kind === "USER_TEXT") return !hasProposalForLatestUser;
      if (messages[i].kind === "PLAN_PROPOSED") return false;
    }
    return false;
  });

  async function triggerProposal() {
    if (proposing || !tenantId || !routeThreadId) return;
    proposing = true;
    try {
      // Reads the thread's latest user message server-side and appends
      // PLAN_PROPOSED, which arrives back via the existing watch stream.
      await threadClient.proposePlan({ tenantId, threadId: routeThreadId });
    } catch {
      // best-effort; the user can retry by sending another message
    } finally {
      proposing = false;
    }
  }

  // Auto-propose when a config-less thread has an unanswered opening message.
  $effect(() => {
    if (routeConfigurationId) return;
    if (!latestIsUnansweredUser) return;
    if (proposing) return;
    void triggerProposal();
  });
```

Now render a self-contained config-less view: the message list (USER_TEXT bubbles + `PlanProposalCard`), a pending hint, and an always-on composer. `PLAN_PROPOSED` is rendered directly here — no `ConversationalWorkspace`/`buildThreadSections` dependency. Replace the current `{#if !routeConfigurationId}` branch (the placeholder that has no composer) with:

```svelte
  {#if !routeConfigurationId}
    <div class="flex flex-col gap-3">
      {#if messages.length === 0}
        <div class="rounded border border-plumage bg-obsidian-light px-4 py-3 text-sm text-crown-ash">
          Tell Aiuna what you want to create, then a plan will appear here.
        </div>
      {/if}
      {#each messages as m (m.id)}
        {#if m.kind === "PLAN_PROPOSED"}
          <PlanProposalCard message={m} {tenantId} threadId={routeThreadId} />
        {:else if m.kind === "USER_TEXT"}
          <div
            class="max-w-[85%] self-end rounded-lg border border-plumage bg-obsidian-light px-3 py-2 text-[13px] whitespace-pre-wrap text-cream"
          >
            {m.text}
          </div>
        {/if}
      {/each}
      {#if proposing}
        <div class="text-[12px] text-crown-ash-dark">Thinking about a plan…</div>
      {/if}
      <ThreadComposer
        {tenantId}
        configurationId={routeThreadId}
        onSent={triggerProposal}
      />
    </div>
  {:else}
    <!-- existing configured-thread markup below is unchanged -->
```

Notes:
- The existing `ThreadComposer` appends the `USER_TEXT` itself using the id passed as `configurationId` (here `routeThreadId`, correct because `appendThreadMessage` now targets `threadId`). Its `onSent` callback fires after the append; we point it at `triggerProposal`. The auto-propose `$effect` is a backstop and is idempotent via `hasProposalForLatestUser`.
- Verify `ThreadComposer.svelte` exposes an optional `onSent?: () => void` prop. It does (props: `{ tenantId, configurationId, onSent? }`). If the callback signature differs, wrap accordingly: `onSent={() => void triggerProposal()}`.
- Keep the existing `{:else}` configured-thread markup (top bar, DAG minimap, HintBanner, ConversationalWorkspace, ThreadComposer, ScheduleDialog) exactly as-is.

- [ ] **Step 2: Make /new prompt-first**

In `frontend/src/routes/new/+page.svelte`, the foundation already creates a thread and routes to `/chat/[threadId]` via `pickTemplate`. Add a prompt-submit path that creates a thread WITHOUT a template and lets the chat page propose. Add a handler:

```typescript
  async function startFromPrompt() {
    if (creating || !tenantId) return;
    const text = composerText.trim();
    if (text.length === 0) return;
    creating = true;
    createError = null;
    try {
      const threadResponse = await threadClient.createThread({
        tenantId,
        title: text.slice(0, 60),
        initialMessageText: text,
      });
      const threadId = threadResponse.thread?.id;
      if (!threadId) throw new Error("createThread returned no id");
      await goto(resolve(`/chat/${threadId}`));
    } catch (e) {
      createError = e instanceof Error ? e.message : "Failed to start";
    } finally {
      creating = false;
    }
  }
```

Change the composer's Enter handler so that, when there is no matched template, submitting starts a prompt-first thread instead of doing nothing:

Replace the existing input `onkeydown`:

```svelte
      onkeydown={(e) => {
        if (e.key === "Enter" && matchedTemplate)
          void pickTemplate(matchedTemplate.id);
      }}
```

with:

```svelte
      onkeydown={(e) => {
        if (e.key !== "Enter") return;
        if (matchedTemplate) void pickTemplate(matchedTemplate.id);
        else void startFromPrompt();
      }}
```

Ensure `threadClient` is imported in `/new` (the foundation added `import { planClient, threadClient } from "$lib/rpc";`). The template gallery below remains unchanged as the fallback.

- [ ] **Step 3: Type check (no new errors in touched files)**

```bash
cd /home/thbertoldi/harpia/frontend && bun run check 2>&1 | grep -E "routes/chat/\[threadId\]|routes/new" || echo "NO new errors"
```

Expected: `NO new errors`.

- [ ] **Step 4: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/routes/chat/[threadId]/+page.svelte frontend/src/routes/new/+page.svelte
git commit -m "feat: chat composer auto-proposes plans; /new prompt-first"
```

---

### Task 11: Verification

**Files:**
- Create: `docs/superpowers/plans/2026-07-01-milestone-d-chat-plan-proposal.verification.md`

- [ ] **Step 1: Backend checks**

```bash
cd /home/thbertoldi/harpia/control-plane && go test ./internal/chat ./internal/copilot ./internal/plans ./internal/threads ./cmd/api -count=1
```

Expected: all `ok`.

- [ ] **Step 2: Proto lint**

```bash
cd /home/thbertoldi/harpia/proto && buf lint
```

Expected: exit 0.

- [ ] **Step 3: Frontend focused tests**

```bash
cd /home/thbertoldi/harpia/frontend && bunx vitest run src/lib/plans/template-inputs.test.ts src/lib/chat/client.test.ts
```

Expected: all passing.

- [ ] **Step 4: Frontend type check (record baseline)**

```bash
cd /home/thbertoldi/harpia/frontend && bun run check 2>&1 | tail -3
```

Expected: no NEW errors in files this plan created/modified (the 12 baseline errors listed in Global Constraints may remain).

- [ ] **Step 5: Manual smoke (record results)**

Against a running dev stack with an LLM provider configured for the tenant:

1. Go to `/new`, type "Create a LinkedIn post about retail in Portuguese", press Enter.
2. Confirm you land on `/chat/[threadId]` and a `PLAN_PROPOSED` card appears with the LinkedIn template and inferred theme/language.
3. Adjust an input, click "Create this plan".
4. Confirm the thread shows `PLAN_ATTACHED` and the binding-matrix prompt, and the composer remains available.
5. With no LLM configured, repeat step 1–2 and confirm the card shows the "couldn't match — browse templates" state (graceful degradation), not an error.

- [ ] **Step 6: Write the verification note**

Create `docs/superpowers/plans/2026-07-01-milestone-d-chat-plan-proposal.verification.md` recording the exact command outputs from Steps 1–4 and the manual smoke results from Step 5, plus the commit range.

- [ ] **Step 7: Commit**

```bash
cd /home/thbertoldi/harpia
git add docs/superpowers/plans/2026-07-01-milestone-d-chat-plan-proposal.verification.md
git commit -m "docs: record milestone D verification"
```

---

## Follow-Ups (out of scope for D)

- Tenant LLM **budget enforcement** for the classifier call: it currently bypasses agent-runtime's per-execution budget controls. Wire budget/quota before heavy production use, or relocate the classifier behind agent-runtime.
- Generalize `BindingMatrixCard`'s LinkedIn-specific input form to reuse `TemplateInputsForm`.
- `PLAN_UPDATED`, `PLAN_RUN_REQUESTED`, `ARTIFACT_*`, `ERROR_*` rendering (Milestone E).
- Automated e2e for the chat-first journey with a stubbed model at the server boundary.

## Acceptance Criteria (from spec §11)

1. `ThreadService.ProposePlan` classifies the latest user message and emits `PLAN_PROPOSED` — Tasks 1, 5.
2. Routing runs through a `PlanClassifier` port + LLM adapter with graceful degradation — Task 3.
3. `PLAN_PROPOSED` renders with single / shortlist / no-match modes — Task 9.
4. Editable generic inputs form pre-filled with extracted values — Tasks 8, 9.
5. Confirm creates a thread-bound `PlanConfiguration` and emits `PLAN_ATTACHED`; matrix flow follows — Tasks 6, 9.
6. `/chat/[threadId]` shows a composer without a plan and auto-proposes — Task 10.
7. `/new` prompt-first, gallery retained — Task 10.
8. Backend + frontend + manual smoke covered with model fakes — Tasks 3, 5, 8, 11.
