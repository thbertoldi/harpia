package budget

import (
	"errors"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	PeriodDaily   = "daily"
	PeriodMonthly = "monthly"

	defaultWarningThreshold = 0.8
	defaultReservationTTL   = 15 * time.Minute
)

var (
	ErrQuotaExceeded = errors.New("LLM_QUOTA_EXHAUSTED")
	ErrQuotaNotFound = errors.New("tenant quota not found")
)

type TenantQuota struct {
	TenantID         uuid.UUID
	Provider         string
	Period           string
	BudgetMicros     int64
	HardCap          bool
	WarningThreshold float64
	UpdatedAt        time.Time
}

type QuotaSnapshot struct {
	TenantID         uuid.UUID
	Provider         string
	Period           string
	BudgetMicros     int64
	UsedMicros       int64
	ReservedMicros   int64
	RemainingMicros  int64
	InputTokens      int64
	OutputTokens     int64
	EventCount       int64
	HardCap          bool
	Warning          bool
	WarningThreshold float64
	PeriodStart      time.Time
	PeriodEnd        time.Time
}

type Reservation struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	Provider        string
	TaskID          uuid.NullUUID
	StepID          uuid.NullUUID
	PlanExecutionID uuid.NullUUID
	StepExecutionID uuid.NullUUID
	AgentType       string
	EstimatedMicros int64
	IdempotencyKey  string
	ExpiresAt       time.Time
}

type ReservationResult struct {
	Reservation     Reservation
	RemainingMicros int64
	Warning         bool
}

type UsageEvent struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	ReservationID   uuid.NullUUID
	TaskID          uuid.NullUUID
	StepID          uuid.NullUUID
	PlanExecutionID uuid.NullUUID
	StepExecutionID uuid.NullUUID
	AgentType       string
	Provider        string
	Model           string
	InputTokens     int64
	OutputTokens    int64
	CostMicros      int64
	IdempotencyKey  string
	Timestamp       time.Time
}

type UsageRecordResult struct {
	UsageEventID    uuid.UUID
	RemainingMicros int64
	Warning         bool
	Duplicate       bool
}

func normalizeProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

func normalizePeriod(period string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(period)) {
	case PeriodDaily:
		return PeriodDaily, nil
	case PeriodMonthly:
		return PeriodMonthly, nil
	default:
		return "", errors.New("period must be daily or monthly")
	}
}

func normalizeWarningThreshold(value float64) (float64, error) {
	if value == 0 {
		return defaultWarningThreshold, nil
	}
	if value <= 0 || value > 1 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, errors.New("warning_threshold must be in (0, 1]")
	}
	return value, nil
}

func periodBounds(period string, now time.Time) (time.Time, time.Time) {
	now = now.UTC()
	switch period {
	case PeriodDaily:
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		return start, start.AddDate(0, 0, 1)
	case PeriodMonthly:
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		return start, start.AddDate(0, 1, 0)
	default:
		return time.Time{}, time.Time{}
	}
}

func microsToUSD(micros int64) float64 {
	return float64(micros) / 1_000_000
}

func usdToMicros(usd float64) int64 {
	return int64(math.Round(usd * 1_000_000))
}

func remainingMicros(budgetMicros, usedMicros, reservedMicros int64) int64 {
	remaining := budgetMicros - usedMicros - reservedMicros
	if remaining < 0 {
		return 0
	}
	return remaining
}

func warningReached(budgetMicros, projectedMicros int64, threshold float64) bool {
	if budgetMicros <= 0 {
		return false
	}
	return float64(projectedMicros) >= float64(budgetMicros)*threshold
}
