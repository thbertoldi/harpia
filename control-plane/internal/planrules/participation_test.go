package planrules

import (
	"testing"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
)

func TestStepWillRun(t *testing.T) {
	step := &plansv1.PlanStep{ExecutorRequirement: &plansv1.ExecutorRequirement{
		OptionalCapabilities: []string{"image-generation", "carousel-authoring"},
	}}

	tests := []struct {
		name     string
		step     *plansv1.PlanStep
		included []string
		want     bool
	}{
		{name: "nil step", step: nil, want: true},
		{name: "step without optional capabilities", step: &plansv1.PlanStep{}, want: true},
		{name: "capabilities unspecified", step: step, want: false},
		{name: "one capability omitted", step: step, included: []string{"image-generation"}, want: false},
		{name: "all capabilities included", step: step, included: []string{"image-generation", "carousel-authoring"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StepWillRun(tt.step, tt.included); got != tt.want {
				t.Fatalf("StepWillRun() = %t, want %t", got, tt.want)
			}
		})
	}
}
