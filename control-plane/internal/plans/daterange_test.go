package plans

import (
	"testing"
	"time"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
)

func TestResolveDateRangePresetLast7Days(t *testing.T) {
	now := time.Date(2026, 7, 1, 15, 30, 0, 0, time.UTC)
	got, ok := ResolveDateRangePreset("last_7_days", now)
	if !ok {
		t.Fatal("ResolveDateRangePreset ok = false, want true")
	}
	if got.GetStartDate() != "2026-06-24" || got.GetEndDate() != "2026-06-30" {
		t.Fatalf("date range = %s..%s, want 2026-06-24..2026-06-30", got.GetStartDate(), got.GetEndDate())
	}
}

func TestResolveDateRangePresetUnknown(t *testing.T) {
	got, ok := ResolveDateRangePreset("next_week", time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))
	if ok || got != nil {
		t.Fatalf("ResolveDateRangePreset unknown = (%v,%v), want (nil,false)", got, ok)
	}
}

func TestResolveDateRangePresetForWeeklyScheduleWindow(t *testing.T) {
	got, ok := ResolveDateRangePresetForSchedule("schedule_window", &plansv1.PlanSchedule{
		CronExpression: "0 9 * * MON",
		Timezone:       "UTC",
	}, time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC))
	if !ok {
		t.Fatal("ResolveDateRangePresetForSchedule ok = false, want true")
	}
	if got.GetStartDate() != "2026-07-01" || got.GetEndDate() != "2026-07-07" {
		t.Fatalf("weekly schedule window = %#v", got)
	}
}

func TestResolveDateRangePresetForDailyScheduleWindow(t *testing.T) {
	got, ok := ResolveDateRangePresetForSchedule("schedule_window", &plansv1.PlanSchedule{
		CronExpression: "0 9 * * *",
		Timezone:       "UTC",
	}, time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC))
	if !ok {
		t.Fatal("ResolveDateRangePresetForSchedule ok = false, want true")
	}
	if got.GetStartDate() != "2026-07-07" || got.GetEndDate() != "2026-07-07" {
		t.Fatalf("daily schedule window = %#v", got)
	}
}
