package plans

import (
	"testing"

	"github.com/google/uuid"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
)

// --- MergeParameterValue ---

func TestMergeParameterValue_IntoEmpty(t *testing.T) {
	got := MergeParameterValue("", "theme", "AI operations")
	assertJSONEqual(t, got, `{"theme":"AI operations"}`)
}

func TestMergeParameterValue_OverwriteExisting(t *testing.T) {
	current := `{"theme":"old","language":"pt-BR"}`
	got := MergeParameterValue(current, "theme", "new")
	assertJSONEqual(t, got, `{"theme":"new","language":"pt-BR"}`)
}

func TestMergeParameterValue_PreserveSiblings(t *testing.T) {
	current := `{"theme":"AI","language":"pt-BR","tone":"analytical"}`
	got := MergeParameterValue(current, "language", "en")
	assertJSONEqual(t, got, `{"theme":"AI","language":"en","tone":"analytical"}`)
}

func TestMergeParameterValue_NonStringValue(t *testing.T) {
	got := MergeParameterValue(`{}`, "date_range", map[string]any{"preset": "last_7_days"})
	assertJSONEqual(t, got, `{"date_range":{"preset":"last_7_days"}}`)
}

// --- FindSlotParameterForStep ---

func TestFindSlotParameterForStep(t *testing.T) {
	template := handlerMaterializeTemplate(t, uuid.New())

	// source_group → SLOT_BINDING/fetch-news in the weekly-newsletter fixture.
	key, ok := FindSlotParameterForStep(template, "fetch-news")
	if !ok {
		t.Fatal("FindSlotParameterForStep(fetch-news) = not found, want found")
	}
	if key != "source_group" {
		t.Fatalf("FindSlotParameterForStep(fetch-news) = %q, want source_group", key)
	}
}

func TestFindSlotParameterForStep_NotFound(t *testing.T) {
	template := handlerMaterializeTemplate(t, uuid.New())

	// write-draft is driven by seed inputs, not a slot parameter.
	if _, ok := FindSlotParameterForStep(template, "write-draft"); ok {
		t.Fatal("FindSlotParameterForStep(write-draft) = found, want not found")
	}
	if _, ok := FindSlotParameterForStep(template, "does-not-exist"); ok {
		t.Fatal("FindSlotParameterForStep(unknown step) = found, want not found")
	}
}

func TestFindSlotParameterForStep_NilTemplate(t *testing.T) {
	if _, ok := FindSlotParameterForStep(nil, "fetch-news"); ok {
		t.Fatal("FindSlotParameterForStep(nil template) = found, want not found")
	}
}

// --- FindPolicyParameter ---

func TestFindPolicyParameter(t *testing.T) {
	template := handlerMaterializeTemplate(t, uuid.New())

	key, ok := FindPolicyParameter(template, "publish_approval_mode")
	if !ok {
		t.Fatal("FindPolicyParameter(publish_approval_mode) = not found, want found")
	}
	if key != "approval_mode" {
		t.Fatalf("FindPolicyParameter(publish_approval_mode) = %q, want approval_mode", key)
	}

	timeoutKey, ok := FindPolicyParameter(template, "elicitation_timeout_behavior")
	if !ok || timeoutKey != "elicitation_timeout_behavior" {
		t.Fatalf("FindPolicyParameter(elicitation_timeout_behavior) = %q,%v, want elicitation_timeout_behavior,true", timeoutKey, ok)
	}
}

func TestFindPolicyParameter_NotFound(t *testing.T) {
	template := handlerMaterializeTemplate(t, uuid.New())
	if _, ok := FindPolicyParameter(template, "unknown_policy"); ok {
		t.Fatal("FindPolicyParameter(unknown) = found, want not found")
	}
	if _, ok := FindPolicyParameter(nil, "publish_approval_mode"); ok {
		t.Fatal("FindPolicyParameter(nil template) = found, want not found")
	}
}

// --- MergeOverseerBinding ---

func TestMergeOverseerBinding_Insert(t *testing.T) {
	current := []*plansv1.OverseerBinding{
		{StepKey: "write-draft", OverseerUserId: "user-1"},
	}
	got := MergeOverseerBinding(current, "publish-linkedin", "user-2")

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (caller binding + inserted)", len(got))
	}
	inserted := findOverseer(got, "publish-linkedin")
	if inserted == nil {
		t.Fatal("missing inserted publish-linkedin binding")
	}
	if inserted.OverseerUserId != "user-2" {
		t.Fatalf("inserted overseer = %q, want user-2", inserted.OverseerUserId)
	}
	// Caller slice must not be mutated.
	if len(current) != 1 {
		t.Fatalf("caller slice mutated: len = %d, want 1", len(current))
	}
	// Sibling binding preserved.
	if wd := findOverseer(got, "write-draft"); wd == nil || wd.OverseerUserId != "user-1" {
		t.Fatalf("write-draft sibling not preserved: %#v", wd)
	}
}

func TestMergeOverseerBinding_Update(t *testing.T) {
	current := []*plansv1.OverseerBinding{
		{StepKey: "write-draft", OverseerUserId: "user-1"},
	}
	got := MergeOverseerBinding(current, "write-draft", "user-2")

	if len(got) != 1 {
		t.Fatalf("len = %d, want 1 (in-place update)", len(got))
	}
	if got[0].OverseerUserId != "user-2" {
		t.Fatalf("overseer = %q, want user-2", got[0].OverseerUserId)
	}
	// The caller's binding pointer must not be mutated.
	if current[0].OverseerUserId != "user-1" {
		t.Fatalf("caller binding mutated: %q, want user-1", current[0].OverseerUserId)
	}
}

// --- PromoteStatus ---

func TestPromoteStatus(t *testing.T) {
	cronSchedule := &plansv1.PlanSchedule{CronExpression: "0 9 * * Mon", Timezone: "UTC"}

	if got := PromoteStatus(ConfigurationStatusDraft, nil); got != ConfigurationStatusRunnable {
		t.Fatalf("Draft + nil schedule = %q, want runnable", got)
	}
	if got := PromoteStatus(ConfigurationStatusDraft, cronSchedule); got != ConfigurationStatusScheduled {
		t.Fatalf("Draft + cron = %q, want scheduled", got)
	}
	// A blank cron expression must not promote to SCHEDULED.
	blankCron := &plansv1.PlanSchedule{CronExpression: "  "}
	if got := PromoteStatus(ConfigurationStatusDraft, blankCron); got != ConfigurationStatusRunnable {
		t.Fatalf("Draft + blank cron = %q, want runnable", got)
	}
	// Non-DRAFT statuses are returned unchanged (no demotion/re-promotion).
	if got := PromoteStatus(ConfigurationStatusRunnable, cronSchedule); got != ConfigurationStatusRunnable {
		t.Fatalf("Runnable must not change = %q, want runnable", got)
	}
	if got := PromoteStatus(ConfigurationStatusScheduled, nil); got != ConfigurationStatusScheduled {
		t.Fatalf("Scheduled must not change = %q, want scheduled", got)
	}
	if got := PromoteStatus(ConfigurationStatusDisabled, cronSchedule); got != ConfigurationStatusDisabled {
		t.Fatalf("Disabled must not change = %q, want disabled", got)
	}
}
