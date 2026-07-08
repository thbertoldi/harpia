package plans

import (
	"strings"
	"time"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
)

func ResolveDateRangePreset(preset string, now time.Time) (*artifactsv1.DateRange, bool) {
	return resolveDateRange(preset, nil, now)
}

func ResolveDateRangePresetForSchedule(preset string, schedule *plansv1.PlanSchedule, now time.Time) (*artifactsv1.DateRange, bool) {
	return resolveDateRange(preset, schedule, now)
}

func resolveDateRange(preset string, schedule *plansv1.PlanSchedule, now time.Time) (*artifactsv1.DateRange, bool) {
	preset = strings.TrimSpace(preset)
	var days int
	switch preset {
	case "schedule_window":
		days = scheduleWindowDays(schedule)
	case "last_7_days":
		days = 7
		if schedule != nil && strings.TrimSpace(schedule.GetCronExpression()) != "" {
			days = scheduleWindowDays(schedule)
		}
	default:
		return nil, false
	}
	if days <= 0 {
		days = 7
	}
	loc := time.UTC
	if schedule != nil && strings.TrimSpace(schedule.GetTimezone()) != "" {
		if loaded, err := time.LoadLocation(strings.TrimSpace(schedule.GetTimezone())); err == nil {
			loc = loaded
		}
	}
	end := now.In(loc).AddDate(0, 0, -1)
	start := end.AddDate(0, 0, -(days - 1))
	return &artifactsv1.DateRange{
		StartDate: start.Format("2006-01-02"),
		EndDate:   end.Format("2006-01-02"),
	}, true
}

func scheduleWindowDays(schedule *plansv1.PlanSchedule) int {
	if schedule == nil {
		return 7
	}
	fields := strings.Fields(schedule.GetCronExpression())
	if len(fields) != 5 {
		return 7
	}
	dayOfMonth := fields[2]
	dayOfWeek := fields[4]
	if !cronFieldEvery(dayOfWeek) {
		return 7
	}
	if !cronFieldEvery(dayOfMonth) {
		return 31
	}
	return 1
}

func cronFieldEvery(field string) bool {
	field = strings.TrimSpace(field)
	return field == "" || field == "*" || field == "?"
}
