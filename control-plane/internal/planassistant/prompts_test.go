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
