package plans_test

import (
	"testing"
)

func TestUpdatePlanConfiguration_ValidCron_EmitsScheduleSetAndPromotesToScheduled(t *testing.T) {
	t.Skip("Wire-up: use the project's existing handler-test scaffold to construct a *plans.PlanHandler bound to an in-memory chat.Store + repo with a RUNNABLE PlanConfiguration. Then call UpdatePlanConfiguration with schedule.cron_expression='0 9 * * *' and assert: (1) chat store recorded a SCHEDULE_SET message; (2) returned PlanConfiguration.status == PLAN_CONFIGURATION_STATUS_SCHEDULED.")
}

func TestUpdatePlanConfiguration_InvalidCron_ReturnsInvalidArgument(t *testing.T) {
	t.Skip("Same harness: call UpdatePlanConfiguration with schedule.cron_expression='not a cron'. Assert connect.CodeInvalidArgument.")
}

func TestUpdatePlanConfiguration_StatusPromotionFromDraft_TriggersAssistantNextTurn(t *testing.T) {
	t.Skip("Wire-up (M6 §2.1): same handler scaffold as above, plus a stub planassistant.Controller recording NextTurn calls. Seed a DRAFT PlanConfiguration with all bindings + policies set. Call UpdatePlanConfiguration with status=PLAN_CONFIGURATION_STATUS_RUNNABLE and assert NextTurn was invoked exactly once. Then call again with an unchanged (still-RUNNABLE) status and assert NextTurn is NOT invoked a second time by the promotion guard (existing.Status==draft is the gate). The landing-emission behavior of NextTurn itself is unit-covered in internal/planassistant/controller_test.go::TestNextTurn_EmitsLandingOnPromotion.")
}
