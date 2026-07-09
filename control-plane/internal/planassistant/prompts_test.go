package planassistant_test

import (
	"encoding/json"
	"strings"
	"testing"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/planassistant"
)

func TestBuildPrompt_BindingMatrix_RendersAllRows(t *testing.T) {
	tpl := mkTemplate("draft", "publish")
	tpl.Steps[0].Title = "Draft post"
	tpl.Steps[0].InputArtifactTypeId = "Brief"
	tpl.Steps[0].OutputArtifactTypeId = "Text"
	price := 0.05
	in := planassistant.PromptInput{
		Template: tpl,
		Config: &plansv1.PlanConfiguration{
			PlanTemplateId: "tpl-1",
			SlotBindings:   []*plansv1.SlotBinding{{StepKey: "draft", ExecutorInstallationId: "inst-1"}},
		},
		CandidatesByStep: map[string][]planassistant.ExecutorOption{
			"draft":   {{StepKey: "draft", InstallationID: "inst-1", DisplayName: "Junior Writer", SkuKey: "writer-junior", Tier: "junior", PriceBrl: &price}},
			"publish": {{StepKey: "publish", InstallationID: "inst-2", DisplayName: "Mailchimp", SkuKey: "mailchimp"}},
		},
		CurrentUserLabel: "Ana",
	}
	state := planassistant.AssistantState{Kind: planassistant.StateBindingMatrix}
	text, payload := planassistant.BuildPrompt(state, in)
	if !strings.Contains(text, "Ana") {
		t.Fatalf("intro should mention overseer default user: %q", text)
	}

	var parsed struct {
		State string `json:"state"`
		Rows  []struct {
			StepKey           string `json:"step_key"`
			StepTitle         string `json:"step_title"`
			CurrentExecutorID string `json:"current_executor_id"`
			CurrentOverseerLb string `json:"current_overseer_label"`
			Contracts         struct {
				Input  string `json:"input"`
				Output string `json:"output"`
			} `json:"contracts"`
			Options []struct {
				ID string `json:"id"`
			} `json:"options"`
		} `json:"rows"`
	}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		t.Fatalf("payload not valid JSON: %v\n%s", err, payload)
	}
	if parsed.State != "BINDING_MATRIX" {
		t.Fatalf("want state BINDING_MATRIX, got %q", parsed.State)
	}
	if len(parsed.Rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(parsed.Rows))
	}
	if parsed.Rows[0].StepTitle != "Draft post" {
		t.Fatalf("row 0 title: %q", parsed.Rows[0].StepTitle)
	}
	if parsed.Rows[0].Contracts.Input != "Brief" || parsed.Rows[0].Contracts.Output != "Text" {
		t.Fatalf("row 0 contracts: %+v", parsed.Rows[0].Contracts)
	}
	if parsed.Rows[0].CurrentExecutorID != "inst-1" {
		t.Fatalf("row 0 should reflect existing binding, got %q", parsed.Rows[0].CurrentExecutorID)
	}
	if parsed.Rows[1].CurrentExecutorID != "" {
		t.Fatalf("row 1 should be unbound, got %q", parsed.Rows[1].CurrentExecutorID)
	}
	for _, r := range parsed.Rows {
		if r.CurrentOverseerLb != "Ana" {
			t.Fatalf("overseer label should default to current user 'Ana', got %q", r.CurrentOverseerLb)
		}
	}
}

func TestBuildPrompt_BindingStep_RendersFocusedOptionsAndAllRows(t *testing.T) {
	tpl := mkTemplate("fetch-news", "write-draft")
	tpl.Steps[0].Title = "Fetch news"
	tpl.Steps[0].InputArtifactTypeId = "NewsQuery"
	tpl.Steps[0].OutputArtifactTypeId = "NewsItems"
	price := 0.12
	in := planassistant.PromptInput{
		Template: tpl,
		Config: &plansv1.PlanConfiguration{
			PlanTemplateId: "tpl-1",
			SlotBindings:   []*plansv1.SlotBinding{{StepKey: "write-draft", ExecutorInstallationId: "writer-1"}},
		},
		CandidatesByStep: map[string][]planassistant.ExecutorOption{
			"fetch-news": {
				{StepKey: "fetch-news", InstallationID: "rss-tech", DisplayName: "Tech RSS preset", SkuKey: "rss-news-feed", PriceBrl: &price},
				{StepKey: "fetch-news", InstallationID: "rss-br", DisplayName: "Brazil RSS preset", SkuKey: "rss-news-feed"},
			},
			"write-draft": {{StepKey: "write-draft", InstallationID: "writer-1", DisplayName: "Senior Writer", SkuKey: "newsletter-writer-senior"}},
		},
		CurrentUserLabel: "Ana",
	}
	state := planassistant.AssistantState{Kind: planassistant.StateBindingStep, StepKey: "fetch-news"}
	text, payload := planassistant.BuildPrompt(state, in)
	if !strings.Contains(text, "fetch-news") && !strings.Contains(text, "Fetch news") {
		t.Fatalf("text should mention the focused step, got %q", text)
	}

	var parsed struct {
		State   string `json:"state"`
		StepKey string `json:"step_key"`
		Options []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"options"`
		Rows []struct {
			StepKey           string `json:"step_key"`
			CurrentExecutorID string `json:"current_executor_id"`
			Options           []struct {
				ID string `json:"id"`
			} `json:"options"`
		} `json:"rows"`
	}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		t.Fatalf("payload not valid JSON: %v\n%s", err, payload)
	}
	if parsed.State != "BINDING_STEP" {
		t.Fatalf("want state BINDING_STEP, got %q", parsed.State)
	}
	if parsed.StepKey != "fetch-news" {
		t.Fatalf("step_key = %q, want fetch-news", parsed.StepKey)
	}
	if len(parsed.Options) != 2 || parsed.Options[0].ID != "rss-tech" {
		t.Fatalf("focused options not populated from fetch-news candidates: %+v", parsed.Options)
	}
	if len(parsed.Rows) != 2 {
		t.Fatalf("want all 2 rows, got %d", len(parsed.Rows))
	}
	if parsed.Rows[1].StepKey != "write-draft" || parsed.Rows[1].CurrentExecutorID != "writer-1" {
		t.Fatalf("bound row not reflected in shared rows: %+v", parsed.Rows[1])
	}
}

func TestBuildPrompt_OverseerStep_RendersCurrentUserOptionAndRows(t *testing.T) {
	tpl := mkTemplate("fetch-news", "write-draft", "adapt")
	markStepIntegration(tpl, "fetch-news")
	markStepAgent(tpl, "write-draft")
	markStepAgent(tpl, "adapt")
	tpl.Steps[1].Title = "Write draft"
	tpl.Steps[1].InputArtifactTypeId = "NewsItems"
	tpl.Steps[1].OutputArtifactTypeId = "TextDraft"
	in := planassistant.PromptInput{
		Template: tpl,
		Config: &plansv1.PlanConfiguration{
			PlanTemplateId: "tpl-1",
			SlotBindings: []*plansv1.SlotBinding{
				{StepKey: "fetch-news", ExecutorInstallationId: "inst-rss"},
				{StepKey: "write-draft", ExecutorInstallationId: "inst-writer"},
				{StepKey: "adapt", ExecutorInstallationId: "inst-adapt"},
			},
			OverseerBindings: []*plansv1.OverseerBinding{
				{StepKey: "adapt", OverseerUserId: "user-paula"},
			},
		},
		CandidatesByStep: map[string][]planassistant.ExecutorOption{
			"fetch-news":  {{StepKey: "fetch-news", InstallationID: "inst-rss", DisplayName: "Tech RSS", SkuKey: "rss-news-feed"}},
			"write-draft": {{StepKey: "write-draft", InstallationID: "inst-writer", DisplayName: "Writer", SkuKey: "newsletter-writer-senior"}},
			"adapt":       {{StepKey: "adapt", InstallationID: "inst-adapt", DisplayName: "Voice", SkuKey: "linkedin-voice-senior"}},
		},
		CurrentUserID:    "user-ana",
		CurrentUserLabel: "Ana Operator",
	}
	state := planassistant.AssistantState{Kind: planassistant.StateKind("OVERSEER_STEP"), StepKey: "write-draft"}
	text, payload := planassistant.BuildPrompt(state, in)
	if !strings.Contains(text, "Write draft") {
		t.Fatalf("text should mention the focused step, got %q", text)
	}

	var parsed struct {
		State            string   `json:"state"`
		StepKey          string   `json:"step_key"`
		RequiredStepKeys []string `json:"required_step_keys"`
		Options          []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"options"`
		Rows []struct {
			StepKey              string `json:"step_key"`
			CurrentExecutorID    string `json:"current_executor_id"`
			CurrentOverseerID    string `json:"current_overseer_id"`
			CurrentOverseerLabel string `json:"current_overseer_label"`
		} `json:"rows"`
	}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		t.Fatalf("payload not valid JSON: %v\n%s", err, payload)
	}
	if parsed.State != "OVERSEER_STEP" {
		t.Fatalf("want state OVERSEER_STEP, got %q", parsed.State)
	}
	if parsed.StepKey != "write-draft" {
		t.Fatalf("step_key = %q, want write-draft", parsed.StepKey)
	}
	if len(parsed.RequiredStepKeys) != 2 || parsed.RequiredStepKeys[0] != "write-draft" || parsed.RequiredStepKeys[1] != "adapt" {
		t.Fatalf("required_step_keys not populated from agent steps: %+v", parsed.RequiredStepKeys)
	}
	if len(parsed.Options) != 1 || parsed.Options[0].Value != "user-ana" || parsed.Options[0].Label != "Ana Operator" {
		t.Fatalf("current-user overseer option not populated: %+v", parsed.Options)
	}
	if len(parsed.Rows) != 3 {
		t.Fatalf("want all 3 rows, got %d", len(parsed.Rows))
	}
	if parsed.Rows[1].CurrentOverseerID != "" {
		t.Fatalf("focused row should be missing overseer, got %+v", parsed.Rows[1])
	}
	if parsed.Rows[2].CurrentOverseerID != "user-paula" || parsed.Rows[2].CurrentOverseerLabel != "user-paula" {
		t.Fatalf("existing overseer binding not reflected in rows: %+v", parsed.Rows[2])
	}
}

func TestBuildPrompt_PoliciesStep_RendersPolicyFields(t *testing.T) {
	tpl := mkTemplate("fetch-news", "publish")
	tpl.InputParameters = []*plansv1.TemplateInputParameter{
		{
			Key: "approval_mode",
			RuntimeMappings: []*plansv1.TemplateInputRuntimeMapping{{
				Target:    plansv1.TemplateInputRuntimeTarget_TEMPLATE_INPUT_RUNTIME_TARGET_BEHAVIOR_POLICY,
				PolicyKey: "publish_approval_mode",
			}},
		},
		{
			Key: "elicitation_timeout",
			RuntimeMappings: []*plansv1.TemplateInputRuntimeMapping{{
				Target:    plansv1.TemplateInputRuntimeTarget_TEMPLATE_INPUT_RUNTIME_TARGET_BEHAVIOR_POLICY,
				PolicyKey: "elicitation_timeout_behavior",
			}},
		},
	}
	in := planassistant.PromptInput{
		Template: tpl,
		Config: &plansv1.PlanConfiguration{
			PlanTemplateId: "tpl-1",
			BehaviorPolicies: &plansv1.PlanBehaviorPolicies{
				PublishApprovalMode: plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_REQUIRE_APPROVAL,
			},
		},
	}
	state := planassistant.AssistantState{Kind: planassistant.StatePoliciesStep}
	text, payload := planassistant.BuildPrompt(state, in)
	if !strings.Contains(strings.ToLower(text), "runtime") {
		t.Fatalf("text should mention runtime behavior, got %q", text)
	}

	var parsed struct {
		State     string `json:"state"`
		PolicyKey string `json:"policy_key"`
		Fields    []struct {
			Key          string `json:"key"`
			ParameterKey string `json:"parameter_key"`
			CurrentValue string `json:"current_value"`
			Options      []struct {
				ID    string `json:"id"`
				Label string `json:"label"`
				Value string `json:"value"`
			} `json:"options"`
		} `json:"fields"`
		PoliciesSet bool `json:"policies_set"`
	}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		t.Fatalf("payload not valid JSON: %v\n%s", err, payload)
	}
	if parsed.State != "POLICIES_STEP" {
		t.Fatalf("want state POLICIES_STEP, got %q", parsed.State)
	}
	if parsed.PoliciesSet {
		t.Fatalf("partial policies should not be complete")
	}
	if parsed.PolicyKey != "elicitation_timeout_behavior" {
		t.Fatalf("want missing policy_key elicitation_timeout_behavior, got %q", parsed.PolicyKey)
	}
	if len(parsed.Fields) != 1 {
		t.Fatalf("want 1 focused policy field, got %d", len(parsed.Fields))
	}
	if parsed.Fields[0].Key != "elicitation_timeout_behavior" ||
		parsed.Fields[0].ParameterKey != "elicitation_timeout" ||
		parsed.Fields[0].CurrentValue != "" ||
		len(parsed.Fields[0].Options) != 3 {
		t.Fatalf("elicitation field not populated from template/current config: %+v", parsed.Fields[0])
	}
	// Backward-compat: a template that does NOT declare content_output_format
	// must not surface it in the payload even though orderedPolicyKeys now
	// lists it first.
	if strings.Contains(payload, "content_output_format") {
		t.Fatalf("payload should not include content_output_format for a template that does not declare it: %s", payload)
	}
}

// TestBuildPrompt_PoliciesStep_RendersContentOutputFormat confirms the
// content_output_format behavior policy is prompted when a template (e.g.
// linkedin-content-studio) declares it. It should be the first policy
// prompted (orderedPolicyKeys puts it first) and surface all four formats.
func TestBuildPrompt_PoliciesStep_RendersContentOutputFormat(t *testing.T) {
	tpl := mkTemplate("draft", "publish")
	tpl.InputParameters = []*plansv1.TemplateInputParameter{
		{
			Key: "output_format",
			RuntimeMappings: []*plansv1.TemplateInputRuntimeMapping{{
				Target:    plansv1.TemplateInputRuntimeTarget_TEMPLATE_INPUT_RUNTIME_TARGET_BEHAVIOR_POLICY,
				PolicyKey: "content_output_format",
			}},
		},
		{
			Key: "approval_mode",
			RuntimeMappings: []*plansv1.TemplateInputRuntimeMapping{{
				Target:    plansv1.TemplateInputRuntimeTarget_TEMPLATE_INPUT_RUNTIME_TARGET_BEHAVIOR_POLICY,
				PolicyKey: "publish_approval_mode",
			}},
		},
		{
			Key: "elicitation_timeout",
			RuntimeMappings: []*plansv1.TemplateInputRuntimeMapping{{
				Target:    plansv1.TemplateInputRuntimeTarget_TEMPLATE_INPUT_RUNTIME_TARGET_BEHAVIOR_POLICY,
				PolicyKey: "elicitation_timeout_behavior",
			}},
		},
	}
	in := planassistant.PromptInput{
		Template: tpl,
		Config: &plansv1.PlanConfiguration{
			PlanTemplateId:   "tpl-1",
			BehaviorPolicies: &plansv1.PlanBehaviorPolicies{}, // content_output_format unset
		},
	}
	state := planassistant.AssistantState{Kind: planassistant.StatePoliciesStep}
	_, payload := planassistant.BuildPrompt(state, in)

	var parsed struct {
		PolicyKey string `json:"policy_key"`
		Fields    []struct {
			Key          string `json:"key"`
			ParameterKey string `json:"parameter_key"`
			CurrentValue string `json:"current_value"`
			Options      []struct {
				ID    string `json:"id"`
				Label string `json:"label"`
				Value string `json:"value"`
			} `json:"options"`
		} `json:"fields"`
	}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		t.Fatalf("payload not valid JSON: %v\n%s", err, payload)
	}
	// content_output_format is first in orderedPolicyKeys and unset, so it is
	// the policy the assistant focuses on.
	if parsed.PolicyKey != "content_output_format" {
		t.Fatalf("want policy_key content_output_format, got %q", parsed.PolicyKey)
	}
	if len(parsed.Fields) != 1 {
		t.Fatalf("want 1 focused policy field, got %d", len(parsed.Fields))
	}
	field := parsed.Fields[0]
	if field.Key != "content_output_format" {
		t.Fatalf("field key = %q, want content_output_format", field.Key)
	}
	if field.ParameterKey != "output_format" {
		t.Fatalf("parameter_key = %q, want output_format", field.ParameterKey)
	}
	if field.CurrentValue != "" {
		t.Fatalf("unset content_output_format should map to empty current_value, got %q", field.CurrentValue)
	}
	wantOptionValues := map[string]string{
		"text_post":         "Text post",
		"carousel":          "Carousel",
		"image_backed_post": "Image-backed post",
		"approval_only":     "Approval only (text)",
	}
	if len(field.Options) != len(wantOptionValues) {
		t.Fatalf("want %d options, got %d (%+v)", len(wantOptionValues), len(field.Options), field.Options)
	}
	for _, opt := range field.Options {
		wantLabel, ok := wantOptionValues[opt.Value]
		if !ok {
			t.Fatalf("unexpected option value %q in content_output_format field: %+v", opt.Value, field.Options)
		}
		if opt.ID != opt.Value {
			t.Fatalf("option id %q should equal value %q", opt.ID, opt.Value)
		}
		if opt.Label != wantLabel {
			t.Fatalf("option %q label = %q, want %q", opt.Value, opt.Label, wantLabel)
		}
	}
}

// TestBuildPrompt_PoliciesStep_ContentOutputFormat_ReflectsCurrentValue
// confirms the content_output_format field carries the config's current
// selection (so re-prompting after a partial selection shows the prior pick).
func TestBuildPrompt_PoliciesStep_ContentOutputFormat_ReflectsCurrentValue(t *testing.T) {
	tpl := mkTemplate("draft", "publish")
	tpl.InputParameters = []*plansv1.TemplateInputParameter{
		{
			Key: "output_format",
			RuntimeMappings: []*plansv1.TemplateInputRuntimeMapping{{
				Target:    plansv1.TemplateInputRuntimeTarget_TEMPLATE_INPUT_RUNTIME_TARGET_BEHAVIOR_POLICY,
				PolicyKey: "content_output_format",
			}},
		},
	}
	in := planassistant.PromptInput{
		Template: tpl,
		Config: &plansv1.PlanConfiguration{
			PlanTemplateId: "tpl-1",
			BehaviorPolicies: &plansv1.PlanBehaviorPolicies{
				ContentOutputFormat: plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_CAROUSEL,
			},
		},
	}
	state := planassistant.AssistantState{Kind: planassistant.StatePoliciesStep, PolicyKey: "content_output_format"}
	_, payload := planassistant.BuildPrompt(state, in)

	var parsed struct {
		Fields []struct {
			Key          string `json:"key"`
			CurrentValue string `json:"current_value"`
		} `json:"fields"`
	}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		t.Fatalf("payload not valid JSON: %v\n%s", err, payload)
	}
	if len(parsed.Fields) != 1 || parsed.Fields[0].Key != "content_output_format" {
		t.Fatalf("want single content_output_format field, got %+v", parsed.Fields)
	}
	if parsed.Fields[0].CurrentValue != "carousel" {
		t.Fatalf("current_value = %q, want carousel", parsed.Fields[0].CurrentValue)
	}
}

func TestBuildPrompt_Landing_HasThreeActions(t *testing.T) {
	state := planassistant.AssistantState{Kind: planassistant.StateSaved}
	in := planassistant.PromptInput{Config: &plansv1.PlanConfiguration{
		Status: plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
	}}
	text, payload := planassistant.BuildPrompt(state, in)
	if !strings.Contains(strings.ToLower(text), "saved") {
		t.Fatalf("landing text should narrate save, got %q", text)
	}
	var parsed struct {
		State   string `json:"state"`
		Actions []struct {
			ID string `json:"id"`
		} `json:"actions"`
	}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		t.Fatalf("payload not valid JSON: %v\n%s", err, payload)
	}
	if parsed.State != "landing" {
		t.Fatalf("want state landing, got %q", parsed.State)
	}
	want := map[string]bool{"run-now": false, "schedule": false, "walk-away": false}
	for _, a := range parsed.Actions {
		if _, ok := want[a.ID]; ok {
			want[a.ID] = true
		}
	}
	for id, found := range want {
		if !found {
			t.Fatalf("missing landing action %q in %s", id, payload)
		}
	}
}
