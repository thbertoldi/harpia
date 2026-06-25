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
