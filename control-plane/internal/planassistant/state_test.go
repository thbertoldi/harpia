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

func markStepAgent(tpl *plansv1.PlanTemplate, stepKey string) {
	for _, step := range tpl.GetSteps() {
		if step.GetKey() == stepKey {
			step.ExecutorRequirement = &plansv1.ExecutorRequirement{
				ExecutorKind: plansv1.ExecutorKind_EXECUTOR_KIND_AGENT,
			}
			return
		}
	}
}

func markStepIntegration(tpl *plansv1.PlanTemplate, stepKey string) {
	for _, step := range tpl.GetSteps() {
		if step.GetKey() == stepKey {
			step.ExecutorRequirement = &plansv1.ExecutorRequirement{
				ExecutorKind: plansv1.ExecutorKind_EXECUTOR_KIND_INTEGRATION,
			}
			return
		}
	}
}

func validPolicies() *plansv1.PlanBehaviorPolicies {
	return &plansv1.PlanBehaviorPolicies{
		ElicitationTimeoutBehavior: plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_PAUSE_UNTIL_ANSWERED,
		PublishApprovalMode:        plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_AUTO_PUBLISH,
	}
}

// withPolicyParams declares behavior-policy input parameters on a template so
// DeriveState will prompt for them. Templates that declare no such parameters
// (e.g. draft-only plans that never publish) must skip the policies step.
func withPolicyParams(tpl *plansv1.PlanTemplate, policyKeys ...string) *plansv1.PlanTemplate {
	for _, pk := range policyKeys {
		tpl.InputParameters = append(tpl.InputParameters, &plansv1.TemplateInputParameter{
			Key: pk,
			RuntimeMappings: []*plansv1.TemplateInputRuntimeMapping{{
				Target:    plansv1.TemplateInputRuntimeTarget_TEMPLATE_INPUT_RUNTIME_TARGET_BEHAVIOR_POLICY,
				PolicyKey: pk,
			}},
		})
	}
	return tpl
}

// A draft-only template declares no behavior-policy parameters, so a fully
// bound configuration must advance straight to the review/save matrix rather
// than dead-ending on a policies prompt whose selection can never materialize.
// Regression for the news-digest-draft journey stall.
func TestDeriveState_BindingMatrix_WhenTemplateDeclaresNoPolicies(t *testing.T) {
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId: "tpl-1",
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		SlotBindings:   []*plansv1.SlotBinding{{StepKey: "a", ExecutorInstallationId: "inst-a"}},
		// no behavior policies set, and the template declares none
	}
	got := planassistant.DeriveState(mkTemplate("a"), cfg, nil)
	if got.Kind != planassistant.StateBindingMatrix {
		t.Fatalf("got %+v, want BINDING_MATRIX (template declares no policies)", got)
	}
}

// A template that declares only the publish policy must not prompt for the
// elicitation policy it never declared.
func TestDeriveState_BindingMatrix_WhenOnlyDeclaredPolicySet(t *testing.T) {
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId: "tpl-1",
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		SlotBindings:   []*plansv1.SlotBinding{{StepKey: "a", ExecutorInstallationId: "inst-a"}},
		BehaviorPolicies: &plansv1.PlanBehaviorPolicies{
			PublishApprovalMode: plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_REQUIRE_APPROVAL,
		},
	}
	tpl := withPolicyParams(mkTemplate("a"), "publish_approval_mode")
	got := planassistant.DeriveState(tpl, cfg, nil)
	if got.Kind != planassistant.StateBindingMatrix {
		t.Fatalf("got %+v, want BINDING_MATRIX (only declared policy is set)", got)
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

func TestDeriveState_PoliciesStep_WhenPoliciesUnset(t *testing.T) {
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId: "tpl-1",
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		SlotBindings:   []*plansv1.SlotBinding{{StepKey: "a", ExecutorInstallationId: "inst-a"}},
		// policies unset
	}
	tpl := withPolicyParams(mkTemplate("a"), "publish_approval_mode", "elicitation_timeout_behavior")
	got := planassistant.DeriveState(tpl, cfg, nil)
	if got.Kind != planassistant.StateKind("POLICIES_STEP") {
		t.Fatalf("got %+v, want POLICIES_STEP (policies unset)", got)
	}
}

func TestDeriveState_PoliciesStep_WhenPoliciesPartial(t *testing.T) {
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId: "tpl-1",
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		SlotBindings:   []*plansv1.SlotBinding{{StepKey: "a", ExecutorInstallationId: "inst-a"}},
		BehaviorPolicies: &plansv1.PlanBehaviorPolicies{
			PublishApprovalMode: plansv1.PublishApprovalMode_PUBLISH_APPROVAL_MODE_REQUIRE_APPROVAL,
		},
	}
	tpl := withPolicyParams(mkTemplate("a"), "publish_approval_mode", "elicitation_timeout_behavior")
	got := planassistant.DeriveState(tpl, cfg, nil)
	if got.Kind != planassistant.StateKind("POLICIES_STEP") {
		t.Fatalf("got %+v, want POLICIES_STEP (policies partial)", got)
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

func TestDeriveState_OverseerStep_WhenAgentStepMissingOverseer(t *testing.T) {
	tpl := mkTemplate("fetch-news", "write-draft")
	markStepIntegration(tpl, "fetch-news")
	markStepAgent(tpl, "write-draft")
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId: "tpl-1",
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		SlotBindings: []*plansv1.SlotBinding{
			{StepKey: "fetch-news", ExecutorInstallationId: "inst-rss"},
			{StepKey: "write-draft", ExecutorInstallationId: "inst-agent"},
		},
		BehaviorPolicies: validPolicies(),
	}
	got := planassistant.DeriveState(tpl, cfg, nil)
	if got.Kind != planassistant.StateKind("OVERSEER_STEP") {
		t.Fatalf("got %+v, want OVERSEER_STEP", got)
	}
	if got.StepKey != "write-draft" {
		t.Fatalf("StepKey = %q, want write-draft", got.StepKey)
	}
}

func TestDeriveState_OverseerStep_SkipsBoundAndIntegrationSteps(t *testing.T) {
	tpl := mkTemplate("fetch-news", "write-draft", "adapt")
	markStepIntegration(tpl, "fetch-news")
	markStepAgent(tpl, "write-draft")
	markStepAgent(tpl, "adapt")
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId: "tpl-1",
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		SlotBindings: []*plansv1.SlotBinding{
			{StepKey: "fetch-news", ExecutorInstallationId: "inst-rss"},
			{StepKey: "write-draft", ExecutorInstallationId: "inst-writer"},
			{StepKey: "adapt", ExecutorInstallationId: "inst-adapt"},
		},
		OverseerBindings: []*plansv1.OverseerBinding{
			{StepKey: "write-draft", OverseerUserId: "user-ana"},
		},
		BehaviorPolicies: validPolicies(),
	}
	got := planassistant.DeriveState(tpl, cfg, nil)
	if got.Kind != planassistant.StateKind("OVERSEER_STEP") {
		t.Fatalf("got %+v, want OVERSEER_STEP", got)
	}
	if got.StepKey != "adapt" {
		t.Fatalf("StepKey = %q, want adapt", got.StepKey)
	}
}

func TestDeriveState_BindingMatrix_WhenRequiredOverseersBound(t *testing.T) {
	tpl := mkTemplate("fetch-news", "write-draft")
	markStepIntegration(tpl, "fetch-news")
	markStepAgent(tpl, "write-draft")
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId: "tpl-1",
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		SlotBindings: []*plansv1.SlotBinding{
			{StepKey: "fetch-news", ExecutorInstallationId: "inst-rss"},
			{StepKey: "write-draft", ExecutorInstallationId: "inst-agent"},
		},
		OverseerBindings: []*plansv1.OverseerBinding{
			{StepKey: "write-draft", OverseerUserId: "user-ana"},
		},
		BehaviorPolicies: validPolicies(),
	}
	got := planassistant.DeriveState(tpl, cfg, nil)
	if got.Kind != planassistant.StateBindingMatrix {
		t.Fatalf("got %+v, want BINDING_MATRIX", got)
	}
}

func TestDeriveState_PoliciesStep_WhenRequiredOverseersBoundButPoliciesMissing(t *testing.T) {
	tpl := mkTemplate("fetch-news", "write-draft")
	markStepIntegration(tpl, "fetch-news")
	markStepAgent(tpl, "write-draft")
	withPolicyParams(tpl, "publish_approval_mode", "elicitation_timeout_behavior")
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId: "tpl-1",
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		SlotBindings: []*plansv1.SlotBinding{
			{StepKey: "fetch-news", ExecutorInstallationId: "inst-rss"},
			{StepKey: "write-draft", ExecutorInstallationId: "inst-agent"},
		},
		OverseerBindings: []*plansv1.OverseerBinding{
			{StepKey: "write-draft", OverseerUserId: "user-ana"},
		},
	}
	got := planassistant.DeriveState(tpl, cfg, nil)
	if got.Kind != planassistant.StateKind("POLICIES_STEP") {
		t.Fatalf("got %+v, want POLICIES_STEP", got)
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

// markStepOptionalCapabilities attaches optional_capabilities to a step's
// ExecutorRequirement (ADR-018 D4). A step is opted out of the run when its
// optional capabilities are not all in the configuration's
// included_optional_capabilities.
func markStepOptionalCapabilities(tpl *plansv1.PlanTemplate, stepKey string, capabilities ...string) {
	for _, step := range tpl.GetSteps() {
		if step.GetKey() == stepKey {
			if step.ExecutorRequirement == nil {
				step.ExecutorRequirement = &plansv1.ExecutorRequirement{}
			}
			step.ExecutorRequirement.OptionalCapabilities = append(
				step.ExecutorRequirement.OptionalCapabilities, capabilities...,
			)
			return
		}
	}
}

// An opt-out step whose capability is not included must not require a binding,
// so a configuration that has bound only the always-on step reaches
// BINDING_MATRIX (configuration can become RUNNABLE without binding the
// excluded step).
func TestDeriveState_BindingMatrix_WhenOptionalCapabilityStepOptedOut(t *testing.T) {
	tpl := mkTemplate("write-draft", "generate-image")
	markStepOptionalCapabilities(tpl, "generate-image", "image-generation")
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId: "tpl-1",
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		SlotBindings:   []*plansv1.SlotBinding{{StepKey: "write-draft", ExecutorInstallationId: "inst-write"}},
		// generate-image not included → opted out, no binding needed.
		BehaviorPolicies: validPolicies(),
	}
	got := planassistant.DeriveState(tpl, cfg, nil)
	if got.Kind != planassistant.StateBindingMatrix {
		t.Fatalf("got %+v, want BINDING_MATRIX (opt-out step needs no binding)", got)
	}
}

// When the optional capability IS included, the step runs and still requires a
// binding, so the assistant must prompt for it (BINDING_STEP).
func TestDeriveState_BindingStep_WhenOptionalCapabilityIncluded(t *testing.T) {
	tpl := mkTemplate("write-draft", "generate-image")
	markStepOptionalCapabilities(tpl, "generate-image", "image-generation")
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId: "tpl-1",
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		SlotBindings:                 []*plansv1.SlotBinding{{StepKey: "write-draft", ExecutorInstallationId: "inst-write"}},
		IncludedOptionalCapabilities: []string{"image-generation"},
		BehaviorPolicies:             validPolicies(),
	}
	got := planassistant.DeriveState(tpl, cfg, nil)
	if got.Kind != planassistant.StateBindingStep {
		t.Fatalf("got %+v, want BINDING_STEP (included step still needs binding)", got)
	}
	if got.StepKey != "generate-image" {
		t.Fatalf("StepKey = %q, want generate-image", got.StepKey)
	}
}

// An opted-out agent step must not require an overseer: with all the always-on
// agent steps overseen, the assistant reaches BINDING_MATRIX rather than
// dead-ending on an overseer prompt for the excluded step.
func TestDeriveState_BindingMatrix_WhenOptedOutAgentStepNeedsNoOverseer(t *testing.T) {
	tpl := mkTemplate("write-draft", "generate-image")
	markStepAgent(tpl, "write-draft")
	markStepAgent(tpl, "generate-image")
	markStepOptionalCapabilities(tpl, "generate-image", "image-generation")
	cfg := &plansv1.PlanConfiguration{
		PlanTemplateId: "tpl-1",
		Status:         plansv1.PlanConfigurationStatus_PLAN_CONFIGURATION_STATUS_DRAFT,
		SlotBindings: []*plansv1.SlotBinding{
			{StepKey: "write-draft", ExecutorInstallationId: "inst-write"},
			{StepKey: "generate-image", ExecutorInstallationId: "inst-image"},
		},
		OverseerBindings: []*plansv1.OverseerBinding{
			{StepKey: "write-draft", OverseerUserId: "user-ana"},
		},
		// generate-image not included → opted out, no overseer needed.
		BehaviorPolicies: validPolicies(),
	}
	got := planassistant.DeriveState(tpl, cfg, nil)
	if got.Kind != planassistant.StateBindingMatrix {
		t.Fatalf("got %+v, want BINDING_MATRIX (opted-out agent step needs no overseer)", got)
	}
}
