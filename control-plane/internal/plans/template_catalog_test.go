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
