package budget

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	budgetv1 "github.com/harpia/control-plane/gen/harpia/budget/v1"
	"github.com/harpia/control-plane/internal/identity"
)

type stubBudgetRepo struct {
	upsertQuotaFn         func(context.Context, TenantQuota) (*TenantQuota, error)
	getQuotaSnapshotsFn   func(context.Context, uuid.UUID, string, time.Time) ([]QuotaSnapshot, error)
	reserveBudgetFn       func(context.Context, Reservation) (*ReservationResult, error)
	recordUsageFn         func(context.Context, UsageEvent) (*UsageRecordResult, error)
	rebuildUsageSummaryFn func(context.Context, uuid.UUID, time.Time, time.Time) error
}

func (s *stubBudgetRepo) UpsertQuota(ctx context.Context, quota TenantQuota) (*TenantQuota, error) {
	if s.upsertQuotaFn != nil {
		return s.upsertQuotaFn(ctx, quota)
	}
	return &quota, nil
}

func (s *stubBudgetRepo) GetQuotaSnapshots(ctx context.Context, tenantID uuid.UUID, provider string, now time.Time) ([]QuotaSnapshot, error) {
	if s.getQuotaSnapshotsFn != nil {
		return s.getQuotaSnapshotsFn(ctx, tenantID, provider, now)
	}
	return nil, nil
}

func (s *stubBudgetRepo) ReserveBudget(ctx context.Context, reservation Reservation) (*ReservationResult, error) {
	if s.reserveBudgetFn != nil {
		return s.reserveBudgetFn(ctx, reservation)
	}
	reservation.ID = uuid.New()
	return &ReservationResult{Reservation: reservation}, nil
}

func (s *stubBudgetRepo) RecordUsage(ctx context.Context, event UsageEvent) (*UsageRecordResult, error) {
	if s.recordUsageFn != nil {
		return s.recordUsageFn(ctx, event)
	}
	return &UsageRecordResult{UsageEventID: uuid.New()}, nil
}

func (s *stubBudgetRepo) RebuildUsageSummary(ctx context.Context, tenantID uuid.UUID, from, to time.Time) error {
	if s.rebuildUsageSummaryFn != nil {
		return s.rebuildUsageSummaryFn(ctx, tenantID, from, to)
	}
	return nil
}

func budgetContext(tenantID uuid.UUID, role string) context.Context {
	return identity.WithRequestContext(context.Background(), identity.RequestContext{
		TenantID: tenantID,
		UserID:   "test-user",
		Roles:    []string{role},
		Tenants: []identity.TenantMembership{
			{TenantID: tenantID, Slug: "tenant-a", Role: role},
		},
	})
}

func internalBudgetContext(tenantID uuid.UUID) context.Context {
	return identity.WithInternalServiceCaller(identity.WithRequestContext(context.Background(), identity.RequestContext{
		TenantID: tenantID,
		UserID:   "internal-service",
	}))
}

func TestSetTenantQuotaRequiresAdmin(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	handler := &Handler{repo: &stubBudgetRepo{}, now: time.Now}
	_, err := handler.SetTenantQuota(
		budgetContext(tenantID, "engineer"),
		connect.NewRequest(&budgetv1.SetTenantQuotaRequest{
			TenantId: tenantID.String(),
			Provider: "openai",
			Period:   budgetv1.QuotaPeriod_QUOTA_PERIOD_MONTHLY,
			Budget:   &budgetv1.Money{Currency: "USD", AmountMicros: 10_000_000},
			HardCap:  true,
		}),
	)
	if err == nil {
		t.Fatal("expected permission denied")
	}
	if got := connect.CodeOf(err); got != connect.CodePermissionDenied {
		t.Fatalf("code = %v, want %v", got, connect.CodePermissionDenied)
	}
}

func TestReserveBudgetRequiresInternalCaller(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	handler := &Handler{repo: &stubBudgetRepo{}, now: time.Now}
	_, err := handler.ReserveBudget(
		budgetContext(tenantID, "admin"),
		connect.NewRequest(&budgetv1.ReserveBudgetRequest{
			TenantId:      tenantID.String(),
			Provider:      "openai",
			EstimatedCost: &budgetv1.Money{Currency: "USD", AmountMicros: 1},
		}),
	)
	if err == nil {
		t.Fatal("expected permission denied")
	}
	if got := connect.CodeOf(err); got != connect.CodePermissionDenied {
		t.Fatalf("code = %v, want %v", got, connect.CodePermissionDenied)
	}
}

func TestReserveBudgetMapsQuotaExceededToResourceExhausted(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	handler := &Handler{
		repo: &stubBudgetRepo{
			reserveBudgetFn: func(context.Context, Reservation) (*ReservationResult, error) {
				return nil, ErrQuotaExceeded
			},
		},
		now: time.Now,
	}
	_, err := handler.ReserveBudget(
		internalBudgetContext(tenantID),
		connect.NewRequest(&budgetv1.ReserveBudgetRequest{
			TenantId:      tenantID.String(),
			Provider:      "openai",
			EstimatedCost: &budgetv1.Money{Currency: "USD", AmountMicros: 1_000_000},
		}),
	)
	if err == nil {
		t.Fatal("expected resource exhausted")
	}
	if got := connect.CodeOf(err); got != connect.CodeResourceExhausted {
		t.Fatalf("code = %v, want %v", got, connect.CodeResourceExhausted)
	}
}

func TestGetQuotaReturnsSnapshots(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	handler := &Handler{
		repo: &stubBudgetRepo{
			getQuotaSnapshotsFn: func(_ context.Context, gotTenantID uuid.UUID, provider string, gotNow time.Time) ([]QuotaSnapshot, error) {
				if gotTenantID != tenantID {
					t.Fatalf("tenant_id = %s, want %s", gotTenantID, tenantID)
				}
				if provider != "openai" {
					t.Fatalf("provider = %q, want openai", provider)
				}
				if !gotNow.Equal(now) {
					t.Fatalf("now = %s, want %s", gotNow, now)
				}
				return []QuotaSnapshot{{
					TenantID:        tenantID,
					Provider:        "openai",
					Period:          PeriodMonthly,
					BudgetMicros:    10_000_000,
					UsedMicros:      8_100_000,
					RemainingMicros: 1_900_000,
					Warning:         true,
					PeriodStart:     time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
					PeriodEnd:       time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
				}}, nil
			},
		},
		now: func() time.Time { return now },
	}
	resp, err := handler.GetQuota(
		budgetContext(tenantID, "engineer"),
		connect.NewRequest(&budgetv1.GetQuotaRequest{
			TenantId: tenantID.String(),
			Provider: "OpenAI",
		}),
	)
	if err != nil {
		t.Fatalf("GetQuota: %v", err)
	}
	if len(resp.Msg.Quotas) != 1 {
		t.Fatalf("quotas len = %d, want 1", len(resp.Msg.Quotas))
	}
	if !resp.Msg.Quotas[0].Warning {
		t.Fatal("expected warning")
	}
}

func TestRecordUsageValidatesProviderAndModel(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	handler := &Handler{repo: &stubBudgetRepo{}, now: time.Now}
	_, err := handler.RecordUsage(
		internalBudgetContext(tenantID),
		connect.NewRequest(&budgetv1.RecordUsageRequest{
			TenantId: tenantID.String(),
			Cost:     &budgetv1.Money{Currency: "USD", AmountMicros: 1},
		}),
	)
	if err == nil {
		t.Fatal("expected invalid argument")
	}
	if got := connect.CodeOf(err); got != connect.CodeInvalidArgument {
		t.Fatalf("code = %v, want %v", got, connect.CodeInvalidArgument)
	}
}

func TestRecordUsageMapsRepositoryErrors(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	handler := &Handler{
		repo: &stubBudgetRepo{
			recordUsageFn: func(context.Context, UsageEvent) (*UsageRecordResult, error) {
				return nil, errors.New("boom")
			},
		},
		now: time.Now,
	}
	_, err := handler.RecordUsage(
		internalBudgetContext(tenantID),
		connect.NewRequest(&budgetv1.RecordUsageRequest{
			TenantId: tenantID.String(),
			Provider: "openai",
			Model:    "gpt-4o-mini",
			Cost:     &budgetv1.Money{Currency: "USD", AmountMicros: 1},
		}),
	)
	if err == nil {
		t.Fatal("expected internal error")
	}
	if got := connect.CodeOf(err); got != connect.CodeInternal {
		t.Fatalf("code = %v, want %v", got, connect.CodeInternal)
	}
}
