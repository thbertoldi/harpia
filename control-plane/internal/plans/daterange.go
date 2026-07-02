package plans

import (
	"time"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
)

func ResolveDateRangePreset(preset string, now time.Time) (*artifactsv1.DateRange, bool) {
	switch preset {
	case "last_7_days":
		end := now.UTC().AddDate(0, 0, -1)
		start := now.UTC().AddDate(0, 0, -7)
		return &artifactsv1.DateRange{
			StartDate: start.Format("2006-01-02"),
			EndDate:   end.Format("2006-01-02"),
		}, true
	default:
		return nil, false
	}
}
