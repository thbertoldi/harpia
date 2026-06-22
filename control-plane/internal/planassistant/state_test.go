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

func TestDeriveState_AwaitingTemplate_WhenTemplateIdEmpty(t *testing.T) {
	got := planassistant.DeriveState(mkTemplate("a"), &plansv1.PlanConfiguration{}, nil)
	if got.Kind != planassistant.StateAwaitingTemplate {
		t.Fatalf("got %v, want AWAITING_TEMPLATE", got.Kind)
	}
}

func TestDeriveState_BindingFirstStep_WhenNoSlotBindings(t *testing.T) {
	cfg := &plansv1.PlanConfiguration{PlanTemplateId: "tpl-1"}
	got := planassistant.DeriveState(mkTemplate("a", "b"), cfg, nil)
	if got.Kind != planassistant.StateBindingStep || got.StepKey != "a" {
		t.Fatalf("got %+v, want BINDING_STEP(a)", got)
	}
}

func TestDeriveState_BindingNextUnboundStep(t *testing.T) {
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId: "tpl-1",
		SlotBindings: []*plansv1.SlotBinding{
			{StepKey: "a", ExecutorInstallationId: "inst-a"},
		},
	}
	got := planassistant.DeriveState(mkTemplate("a", "b"), cfg, nil)
	if got.Kind != planassistant.StateBindingStep || got.StepKey != "b" {
		t.Fatalf("got %+v, want BINDING_STEP(b)", got)
	}
}

func TestDeriveState_OverseerAfterAllBindings(t *testing.T) {
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId: "tpl-1",
		SlotBindings: []*plansv1.SlotBinding{
			{StepKey: "a", ExecutorInstallationId: "inst-a"},
			{StepKey: "b", ExecutorInstallationId: "inst-b"},
		},
	}
	got := planassistant.DeriveState(mkTemplate("a", "b"), cfg, nil)
	if got.Kind != planassistant.StateSetOverseer || got.StepKey != "a" {
		t.Fatalf("got %+v, want SET_OVERSEER(a)", got)
	}
}

func TestDeriveState_PoliciesAfterAllOverseers(t *testing.T) {
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId: "tpl-1",
		SlotBindings: []*plansv1.SlotBinding{
			{StepKey: "a", ExecutorInstallationId: "inst-a"},
		},
		OverseerBindings: []*plansv1.OverseerBinding{
			{StepKey: "a", OverseerUserId: "user-1"},
		},
	}
	got := planassistant.DeriveState(mkTemplate("a"), cfg, nil)
	if got.Kind != planassistant.StateSetPolicies {
		t.Fatalf("got %+v, want SET_POLICIES", got)
	}
}

func TestDeriveState_ConfirmAfterPolicies(t *testing.T) {
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId: "tpl-1",
		SlotBindings:   []*plansv1.SlotBinding{{StepKey: "a", ExecutorInstallationId: "inst-a"}},
		OverseerBindings: []*plansv1.OverseerBinding{{StepKey: "a", OverseerUserId: "u"}},
		BehaviorPolicies: &plansv1.PlanBehaviorPolicies{
			ElicitationTimeoutBehavior: plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_PAUSE_UNTIL_ANSWERED,
			PublishApprovalMode:        plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_AUTO_PUBLISH,
		},
	}
	got := planassistant.DeriveState(mkTemplate("a"), cfg, nil)
	if got.Kind != planassistant.StateConfirm {
		t.Fatalf("got %+v, want CONFIRM", got)
	}
}

func TestDeriveState_SavedWhenStatusRunnable(t *testing.T) {
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId:   "tpl-1",
		Status:           plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_RUNNABLE,
		SlotBindings:     []*plansv1.SlotBinding{{StepKey: "a", ExecutorInstallationId: "inst-a"}},
		OverseerBindings: []*plansv1.OverseerBinding{{StepKey: "a", OverseerUserId: "u"}},
		BehaviorPolicies: &plansv1.PlanBehaviorPolicies{
			ElicitationTimeoutBehavior: plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_PAUSE_UNTIL_ANSWERED,
			PublishApprovalMode:        plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_AUTO_PUBLISH,
		},
	}
	// One SAVED-signalling SCHEDULE_SET or no further prompt — but easier:
	// once status is RUNNABLE+SCHEDULED-eligible, treat as SAVED regardless of msgs.
	got := planassistant.DeriveState(mkTemplate("a"), cfg, []*chatv1.ThreadMessage{})
	if got.Kind != planassistant.StateSaved {
		t.Fatalf("got %+v, want SAVED", got)
	}
}
