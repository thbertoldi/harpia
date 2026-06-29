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

func TestConfigurationToProtoDefaultsInvalidParameterValuesJSON(t *testing.T) {
	tests := []struct {
		name string
		raw  json.RawMessage
	}{
		{name: "empty", raw: nil},
		{name: "invalid", raw: json.RawMessage(`{`)},
		{name: "null", raw: json.RawMessage(`null`)},
		{name: "array", raw: json.RawMessage(`[]`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := configurationToProto(&PlanConfiguration{ParameterValues: tt.raw})

			if got.ParameterValuesJson != `{}` {
				t.Fatalf("ParameterValuesJson = %q, want {}", got.ParameterValuesJson)
			}
		})
	}
}

func TestNormalizeParameterValuesJSONDefaultsAndCompactsObject(t *testing.T) {
	got, err := normalizeParameterValuesJSON(` { "theme" : "retail" , "language" : "pt-BR" } `)
	if err != nil {
		t.Fatalf("normalize object: %v", err)
	}
	if string(got) != `{"theme":"retail","language":"pt-BR"}` {
		t.Fatalf("normalized object = %q", got)
	}

	got, err = normalizeParameterValuesJSON("")
	if err != nil {
		t.Fatalf("normalize empty: %v", err)
	}
	if string(got) != `{}` {
		t.Fatalf("normalized empty = %q, want {}", got)
	}
}

func TestNormalizeParameterValuesJSONRejectsNonObjectAndNull(t *testing.T) {
	for _, raw := range []string{`[]`, `"x"`, `42`, `null`} {
		t.Run(raw, func(t *testing.T) {
			if got, err := normalizeParameterValuesJSON(raw); err == nil {
				t.Fatalf("normalizeParameterValuesJSON(%s) = %q, want error", raw, got)
			}
		})
	}
}

func TestParameterValuesForUpdatePreservesExistingOnBlankInput(t *testing.T) {
	existing := json.RawMessage(`{"theme":"retail","language":"pt-BR"}`)

	got, err := parameterValuesForUpdate(" \t\n", existing)
	if err != nil {
		t.Fatalf("preserve existing: %v", err)
	}
	if string(got) != string(existing) {
		t.Fatalf("blank update = %q, want existing %q", got, existing)
	}

	got, err = parameterValuesForUpdate(`{}`, existing)
	if err != nil {
		t.Fatalf("clear values: %v", err)
	}
	if string(got) != `{}` {
		t.Fatalf("explicit clear = %q, want {}", got)
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
