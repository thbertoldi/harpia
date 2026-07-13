package chat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
)

func TestBuildExecutionPromptPayloadRoundTripsStableIdentity(t *testing.T) {
	payload := &chatv1.ExecutionPromptPayload{
		ConfigurationId:    "configuration-1",
		PlanExecutionId:    "execution-1",
		State:              chatv1.ExecutionPromptState_EXECUTION_PROMPT_STATE_EXECUTION_AWAITING_REVIEW,
		StepExecutionId:    "step-1",
		PlanStepKey:        "publish",
		CompletedStepCount: 2,
		ActiveStepCount:    3,
		PendingInteraction: &chatv1.ExecutionInteractionPointer{
			Kind:            chatv1.ExecutionInteractionKind_EXECUTION_INTERACTION_KIND_REVIEW,
			RequestId:       "review-1",
			StepExecutionId: "step-1",
			PlanStepKey:     "publish",
			SubjectArtifactRef: &artifactsv1.ArtifactRef{
				ArtifactId: "artifact-1", ArtifactVersionId: "version-1", ArtifactTypeKey: "harpia.artifacts.v1.LinkedInPost", ContentHash: "hash-1",
			},
		},
		Actions: []*chatv1.ExecutionPromptAction{{ActionId: "review.accept", LabelKey: "execution.review.accept"}},
	}
	encoded := BuildExecutionPromptPayload(payload)
	if !strings.Contains(encoded, `"plan_execution_id":"execution-1"`) || strings.Contains(encoded, `"label":`) {
		t.Fatalf("execution prompt payload must use stable proto identities only: %s", encoded)
	}
	decoded, ok := ParseExecutionPromptPayload(encoded)
	if !ok || decoded.GetPendingInteraction().GetSubjectArtifactRef().GetArtifactVersionId() != "version-1" {
		t.Fatalf("execution prompt payload did not round trip: %+v", decoded)
	}
}

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

func TestBuildApprovalRaisedPayloadWithContext(t *testing.T) {
	got := BuildApprovalRaisedPayload("abc-123", ApprovalRaisedContext{
		PlanConfigurationID: "config-1",
		PlanExecutionID:     "exec-1",
		StepExecutionID:     "step-exec-1",
		PlanStepKey:         "publish-linkedin",
		InputArtifactID:     "artifact-linkedin-draft",
	})

	var decoded map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if decoded["approval_request_id"] != "abc-123" {
		t.Fatalf("expected approval_request_id=abc-123, got %s", decoded["approval_request_id"])
	}
	if decoded["plan_step_key"] != "publish-linkedin" {
		t.Fatalf("expected plan_step_key=publish-linkedin, got %s", decoded["plan_step_key"])
	}
	if decoded["input_artifact_id"] != "artifact-linkedin-draft" {
		t.Fatalf("expected input_artifact_id=artifact-linkedin-draft, got %s", decoded["input_artifact_id"])
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

func TestBuildAssistantPoliciesStepPayload(t *testing.T) {
	got := BuildAssistantPoliciesStepPayload("publish_approval_mode", []AssistantPolicyField{
		{
			Key:          "publish_approval_mode",
			ParameterKey: "approval_mode",
			CurrentValue: "require_approval",
			Options: []AssistantOption{
				{ID: "require_approval", Label: "Require approval", Value: "require_approval"},
				{ID: "auto_publish", Label: "Auto publish", Value: "auto_publish"},
			},
		},
		{
			Key:          "elicitation_timeout_behavior",
			ParameterKey: "elicitation_timeout_behavior",
			Options: []AssistantOption{
				{ID: "pause_until_answered", Label: "Pause until answered", Value: "pause_until_answered"},
			},
		},
	}, false)
	want := `{"state":"POLICIES_STEP","policy_key":"publish_approval_mode","fields":[{"key":"publish_approval_mode","parameter_key":"approval_mode","current_value":"require_approval","options":[{"id":"require_approval","label":"Require approval","value":"require_approval"},{"id":"auto_publish","label":"Auto publish","value":"auto_publish"}]},{"key":"elicitation_timeout_behavior","parameter_key":"elicitation_timeout_behavior","current_value":"","options":[{"id":"pause_until_answered","label":"Pause until answered","value":"pause_until_answered"}]}],"policies_set":false}`
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestBuildAssistantPoliciesStepPayloadDefaultsNilFields(t *testing.T) {
	got := BuildAssistantPoliciesStepPayload("", nil, false)
	want := `{"state":"POLICIES_STEP","fields":[],"policies_set":false}`
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
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

func TestBuildPlanProposedPayload(t *testing.T) {
	got := BuildPlanProposedPayload("msg-1", "Create a LinkedIn post about retail", []PlanProposalCandidate{
		{TemplateID: "tpl-1", TemplateKey: "linkedin", TemplateName: "LinkedIn Post", Confidence: 0.9, InputValuesJSON: `{"theme":"retail"}`},
	})
	var decoded struct {
		SourceMessageID string `json:"source_message_id"`
		Summary         string `json:"summary"`
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
	if decoded.Summary != "Create a LinkedIn post about retail" {
		t.Fatalf("summary = %q", decoded.Summary)
	}
	if len(decoded.Candidates) != 1 || decoded.Candidates[0].TemplateKey != "linkedin" {
		t.Fatalf("candidates = %+v", decoded.Candidates)
	}
	if decoded.Candidates[0].InputValuesJSON != `{"theme":"retail"}` {
		t.Fatalf("input_values_json = %q", decoded.Candidates[0].InputValuesJSON)
	}
}

func TestBuildPlanProposedPayloadIncludesRefinementMetadata(t *testing.T) {
	got := BuildPlanProposedPayload("msg-1", "Create a LinkedIn post about AI", []PlanProposalCandidate{
		{
			TemplateID:           "tpl-1",
			TemplateKey:          "linkedin",
			TemplateName:         "LinkedIn Post",
			Confidence:           0.72,
			InputValuesJSON:      `{"theme":"AI"}`,
			RecommendationReason: "Good fit for social publishing",
			CompatibilityLabel:   "Good match",
		},
		{
			TemplateID:           "tpl-2",
			TemplateKey:          "newsletter",
			TemplateName:         "Newsletter",
			Confidence:           0.91,
			InputValuesJSON:      `{"theme":"AI"}`,
			RecommendationReason: "Best fit for recurring digest",
			CompatibilityLabel:   "Best match",
		},
	}, PlanRefinementDefaults{
		Audience:       "startup founders",
		Themes:         []string{"AI", "B2B"},
		TopicsToAvoid:  []string{"rumors"},
		SourceGroups:   []string{"tech", "business"},
		Language:       "pt-BR",
		Tone:           "practical",
		DateRangeStart: "2026-06-01",
		DateRangeEnd:   "2026-07-01",
	})
	var decoded struct {
		BestCandidateID    string `json:"best_candidate_id"`
		RefinementDefaults struct {
			Audience       string   `json:"audience"`
			Themes         []string `json:"themes"`
			TopicsToAvoid  []string `json:"topics_to_avoid"`
			SourceGroups   []string `json:"source_groups"`
			Language       string   `json:"language"`
			Tone           string   `json:"tone"`
			DateRangeStart string   `json:"date_range_start"`
			DateRangeEnd   string   `json:"date_range_end"`
		} `json:"refinement_defaults"`
		Candidates []struct {
			TemplateID           string `json:"template_id"`
			RecommendationReason string `json:"recommendation_reason"`
			CompatibilityLabel   string `json:"compatibility_label"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.BestCandidateID != "tpl-2" {
		t.Fatalf("best_candidate_id = %q", decoded.BestCandidateID)
	}
	if decoded.RefinementDefaults.Audience != "startup founders" {
		t.Fatalf("refinement_defaults = %+v", decoded.RefinementDefaults)
	}
	if len(decoded.RefinementDefaults.Themes) != 2 || decoded.RefinementDefaults.Themes[0] != "AI" {
		t.Fatalf("themes = %+v", decoded.RefinementDefaults.Themes)
	}
	if decoded.Candidates[1].RecommendationReason != "Best fit for recurring digest" {
		t.Fatalf("candidate metadata = %+v", decoded.Candidates)
	}
}

func TestBuildPlanProposedPayloadEmptyCandidates(t *testing.T) {
	got := BuildPlanProposedPayload("msg-1", "", nil)
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
