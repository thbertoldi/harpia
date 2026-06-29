package plans

import (
	"encoding/json"
	"testing"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
)

func TestConfigurationToProtoIncludesParameterValuesJSON(t *testing.T) {
	config := &PlanConfiguration{
		ParameterValues: json.RawMessage(`{"theme":"retail","language":"pt-BR"}`),
	}

	got := configurationToProto(config)

	if got.ParameterValuesJson != `{"theme":"retail","language":"pt-BR"}` {
		t.Fatalf("ParameterValuesJson = %q", got.ParameterValuesJson)
	}
}

func TestTemplateToProtoIncludesInputParameters(t *testing.T) {
	template := &PlanTemplate{
		InputParameters: json.RawMessage(`[
			{"key":"theme","label":"Theme","type":"TEMPLATE_INPUT_PARAMETER_TYPE_TEXT","required":true}
		]`),
	}

	got := templateToProto(template)

	if len(got.InputParameters) != 1 {
		t.Fatalf("input parameter count = %d, want 1", len(got.InputParameters))
	}
	if got.InputParameters[0].Key != "theme" {
		t.Fatalf("key = %q, want theme", got.InputParameters[0].Key)
	}
	if got.InputParameters[0].Type != plansv1.TemplateInputParameterType_TEMPLATE_INPUT_PARAMETER_TYPE_TEXT {
		t.Fatalf("type = %v, want TEXT", got.InputParameters[0].Type)
	}
}
