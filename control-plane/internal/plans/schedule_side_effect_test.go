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
