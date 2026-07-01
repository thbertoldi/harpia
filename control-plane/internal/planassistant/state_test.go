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

func validPolicies() *plansv1.PlanBehaviorPolicies {
	return &plansv1.PlanBehaviorPolicies{
		ElicitationTimeoutBehavior: plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_PAUSE_UNTIL_ANSWERED,
		PublishApprovalMode:        plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_AUTO_PUBLISH,
	}
}

func TestDeriveState_AwaitingTemplate_WhenTemplateIdEmpty(t *testing.T) {
	got := planassistant.DeriveState(mkTemplate("a"), &plansv1.PlanConfiguration{}, nil)
	if got.Kind != planassistant.StateAwaitingTemplate {
		t.Fatalf("got %v, want AWAITING_TEMPLATE", got.Kind)
	}
}

func TestDeriveState_BindingStep_WhenStepUnbound(t *testing.T) {
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId: "tpl-1",
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		SlotBindings:   []*plansv1.SlotBinding{{StepKey: "a", ExecutorInstallationId: "inst-a"}},
		// step "b" unbound
		BehaviorPolicies: validPolicies(),
	}
	got := planassistant.DeriveState(mkTemplate("a", "b"), cfg, nil)
	if got.Kind != planassistant.StateBindingStep {
		t.Fatalf("got %+v, want BINDING_STEP", got)
	}
	if got.StepKey != "b" {
		t.Fatalf("StepKey = %q, want b", got.StepKey)
	}
}

func TestDeriveState_BindingMatrix_WhenPoliciesUnset(t *testing.T) {
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId: "tpl-1",
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		SlotBindings:   []*plansv1.SlotBinding{{StepKey: "a", ExecutorInstallationId: "inst-a"}},
		// policies unset
	}
	got := planassistant.DeriveState(mkTemplate("a"), cfg, nil)
	if got.Kind != planassistant.StateBindingMatrix {
		t.Fatalf("got %+v, want BINDING_MATRIX (policies unset)", got)
	}
}

func TestDeriveState_BindingMatrix_WhenAllStepsBoundAndStatusStillDraft(t *testing.T) {
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId:   "tpl-1",
		Status:           plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		SlotBindings:     []*plansv1.SlotBinding{{StepKey: "a", ExecutorInstallationId: "inst-a"}},
		BehaviorPolicies: validPolicies(),
	}
	got := planassistant.DeriveState(mkTemplate("a"), cfg, nil)
	if got.Kind != planassistant.StateBindingMatrix {
		t.Fatalf("got %+v, want BINDING_MATRIX (still DRAFT)", got)
	}
}

func TestDeriveState_Saved_WhenStatusRunnable(t *testing.T) {
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId: "tpl-1",
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
		SlotBindings:   []*plansv1.SlotBinding{{StepKey: "a", ExecutorInstallationId: "inst-a"}},
	}
	got := planassistant.DeriveState(mkTemplate("a"), cfg, []*chatv1.ThreadMessage{})
	if got.Kind != planassistant.StateSaved {
		t.Fatalf("got %+v, want SAVED (status leaves DRAFT)", got)
	}
}

func TestDeriveState_Saved_WhenStatusScheduled(t *testing.T) {
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId: "tpl-1",
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_SCHEDULED,
		SlotBindings:   []*plansv1.SlotBinding{{StepKey: "a", ExecutorInstallationId: "inst-a"}},
	}
	got := planassistant.DeriveState(mkTemplate("a"), cfg, nil)
	if got.Kind != planassistant.StateSaved {
		t.Fatalf("got %+v, want SAVED (status leaves DRAFT)", got)
	}
}
