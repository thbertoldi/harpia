package budget

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	budgetv1 "github.com/harpia/control-plane/gen/harpia/budget/v1"
	"github.com/harpia/control-plane/internal/identity"
)

type quotaRepository interface {
	UpsertQuota(context.Context, TenantQuota) (*TenantQuota, error)
	GetQuotaSnapshots(context.Context, uuid.UUID, string, time.Time) ([]QuotaSnapshot, error)
	ReserveBudget(context.Context, Reservation) (*ReservationResult, error)
	RecordUsage(context.Context, UsageEvent) (*UsageRecordResult, error)
	RebuildUsageSummary(context.Context, uuid.UUID, time.Time, time.Time) error
}

type Handler struct {
	repo quotaRepository
	now  func() time.Time
}

type HandlerOptions struct {
	Repo quotaRepository
	Now  func() time.Time
}

func NewHandler(opts HandlerOptions) (*Handler, error) {
	if opts.Repo == nil {
		return nil, errors.New("budget: repository is required")
	}
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now().UTC() }
	}
	return &Handler{repo: opts.Repo, now: opts.Now}, nil
}

func (h *Handler) SetTenantQuota(ctx context.Context, req *connect.Request[budgetv1.SetTenantQuotaRequest]) (*connect.Response[budgetv1.SetTenantQuotaResponse], error) {
	tenantID, err := requireTenantAdmin(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}
	period, err := periodFromProto(req.Msg.Period)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	budgetMicros, err := moneyMicros(req.Msg.Budget)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	threshold, err := normalizeWarningThreshold(req.Msg.WarningThreshold)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	quota, err := h.repo.UpsertQuota(ctx, TenantQuota{
		TenantID:         tenantID,
		Provider:         normalizeProvider(req.Msg.Provider),
		Period:           period,
		BudgetMicros:     budgetMicros,
		HardCap:          req.Msg.HardCap,
		WarningThreshold: threshold,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&budgetv1.SetTenantQuotaResponse{Quota: quotaToProto(quota)}), nil
}

func (h *Handler) GetQuota(ctx context.Context, req *connect.Request[budgetv1.GetQuotaRequest]) (*connect.Response[budgetv1.GetQuotaResponse], error) {
	tenantID, err := identity.RequireTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}
	snapshots, err := h.repo.GetQuotaSnapshots(ctx, tenantID, normalizeProvider(req.Msg.Provider), h.now())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*budgetv1.ProviderQuota, 0, len(snapshots))
	for i := range snapshots {
		out = append(out, snapshotToProto(&snapshots[i]))
	}
	return connect.NewResponse(&budgetv1.GetQuotaResponse{Quotas: out}), nil
}

func (h *Handler) ReserveBudget(ctx context.Context, req *connect.Request[budgetv1.ReserveBudgetRequest]) (*connect.Response[budgetv1.ReserveBudgetResponse], error) {
	tenantID, err := requireInternalTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}
	estimatedMicros, err := moneyMicros(req.Msg.EstimatedCost)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	reservation, err := reservationFromProto(tenantID, req.Msg, estimatedMicros)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.repo.ReserveBudget(ctx, reservation)
	if err != nil {
		if errors.Is(err, ErrQuotaExceeded) {
			return nil, connect.NewError(connect.CodeResourceExhausted, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&budgetv1.ReserveBudgetResponse{
		ReservationId: result.Reservation.ID.String(),
		Remaining:     moneyProto(result.RemainingMicros),
		QuotaWarning:  result.Warning,
		ExpiresAt:     timestamppb.New(result.Reservation.ExpiresAt),
	}), nil
}

func (h *Handler) RecordUsage(ctx context.Context, req *connect.Request[budgetv1.RecordUsageRequest]) (*connect.Response[budgetv1.RecordUsageResponse], error) {
	tenantID, err := requireInternalTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}
	event, err := usageEventFromProto(tenantID, req.Msg, h.now())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	result, err := h.repo.RecordUsage(ctx, event)
	if err != nil {
		if errors.Is(err, ErrQuotaExceeded) {
			return nil, connect.NewError(connect.CodeResourceExhausted, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&budgetv1.RecordUsageResponse{
		UsageEventId: result.UsageEventID.String(),
		Remaining:    moneyProto(result.RemainingMicros),
		QuotaWarning: result.Warning,
	}), nil
}

func (h *Handler) RebuildUsageSummary(ctx context.Context, req *connect.Request[budgetv1.RebuildUsageSummaryRequest]) (*connect.Response[budgetv1.RebuildUsageSummaryResponse], error) {
	tenantID, err := requireInternalTenant(ctx, req.Msg.TenantId)
	if err != nil {
		return nil, err
	}
	from := timestampOrZero(req.Msg.From)
	to := timestampOrZero(req.Msg.To)
	if from.IsZero() || to.IsZero() || !from.Before(to) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("valid from/to range is required"))
	}
	if err := h.repo.RebuildUsageSummary(ctx, tenantID, from, to); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&budgetv1.RebuildUsageSummaryResponse{}), nil
}

func requireTenantAdmin(ctx context.Context, requestedTenantID string) (uuid.UUID, error) {
	tenantID, err := identity.RequireTenant(ctx, requestedTenantID)
	if err != nil {
		return uuid.Nil, err
	}
	rc, ok := identity.RequestContextFrom(ctx)
	if !ok {
		return uuid.Nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing request context"))
	}
	for _, role := range rc.Roles {
		if role == "admin" {
			return tenantID, nil
		}
	}
	return uuid.Nil, connect.NewError(connect.CodePermissionDenied, errors.New("tenant.admin permission required"))
}

func requireInternalTenant(ctx context.Context, requestedTenantID string) (uuid.UUID, error) {
	if !identity.IsInternalServiceCaller(ctx) {
		return uuid.Nil, connect.NewError(connect.CodePermissionDenied, errors.New("internal service caller required"))
	}
	return identity.RequireTenant(ctx, requestedTenantID)
}

func periodFromProto(period budgetv1.QuotaPeriod) (string, error) {
	switch period {
	case budgetv1.QuotaPeriod_QUOTA_PERIOD_DAILY:
		return PeriodDaily, nil
	case budgetv1.QuotaPeriod_QUOTA_PERIOD_MONTHLY:
		return PeriodMonthly, nil
	default:
		return "", errors.New("period is required")
	}
}

func periodToProto(period string) budgetv1.QuotaPeriod {
	switch period {
	case PeriodDaily:
		return budgetv1.QuotaPeriod_QUOTA_PERIOD_DAILY
	case PeriodMonthly:
		return budgetv1.QuotaPeriod_QUOTA_PERIOD_MONTHLY
	default:
		return budgetv1.QuotaPeriod_QUOTA_PERIOD_UNSPECIFIED
	}
}

func moneyMicros(m *budgetv1.Money) (int64, error) {
	if m == nil {
		return 0, nil
	}
	currency := m.Currency
	if currency == "" {
		currency = "USD"
	}
	if currency != "USD" {
		return 0, fmt.Errorf("unsupported currency %q", currency)
	}
	if m.AmountMicros < 0 {
		return 0, errors.New("money amount_micros must be non-negative")
	}
	return m.AmountMicros, nil
}

func moneyProto(micros int64) *budgetv1.Money {
	if micros < 0 {
		micros = 0
	}
	return &budgetv1.Money{Currency: "USD", AmountMicros: micros}
}

func quotaToProto(quota *TenantQuota) *budgetv1.TenantQuota {
	if quota == nil {
		return nil
	}
	return &budgetv1.TenantQuota{
		TenantId:         quota.TenantID.String(),
		Provider:         quota.Provider,
		Period:           periodToProto(quota.Period),
		Budget:           moneyProto(quota.BudgetMicros),
		HardCap:          quota.HardCap,
		WarningThreshold: quota.WarningThreshold,
		UpdatedAt:        quota.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func snapshotToProto(snapshot *QuotaSnapshot) *budgetv1.ProviderQuota {
	return &budgetv1.ProviderQuota{
		TenantId:     snapshot.TenantID.String(),
		Provider:     snapshot.Provider,
		Period:       periodToProto(snapshot.Period),
		Budget:       moneyProto(snapshot.BudgetMicros),
		Used:         moneyProto(snapshot.UsedMicros),
		Reserved:     moneyProto(snapshot.ReservedMicros),
		Remaining:    moneyProto(snapshot.RemainingMicros),
		InputTokens:  snapshot.InputTokens,
		OutputTokens: snapshot.OutputTokens,
		EventCount:   snapshot.EventCount,
		HardCap:      snapshot.HardCap,
		Warning:      snapshot.Warning,
		PeriodStart:  timestamppb.New(snapshot.PeriodStart),
		PeriodEnd:    timestamppb.New(snapshot.PeriodEnd),
	}
}

func reservationFromProto(tenantID uuid.UUID, msg *budgetv1.ReserveBudgetRequest, estimatedMicros int64) (Reservation, error) {
	taskID, err := nullableUUID(msg.TaskId)
	if err != nil {
		return Reservation{}, fmt.Errorf("invalid task_id: %w", err)
	}
	stepID, err := nullableUUID(msg.StepId)
	if err != nil {
		return Reservation{}, fmt.Errorf("invalid step_id: %w", err)
	}
	planExecutionID, err := nullableUUID(msg.PlanExecutionId)
	if err != nil {
		return Reservation{}, fmt.Errorf("invalid plan_execution_id: %w", err)
	}
	stepExecutionID, err := nullableUUID(msg.StepExecutionId)
	if err != nil {
		return Reservation{}, fmt.Errorf("invalid step_execution_id: %w", err)
	}
	expiresAt := timestampOrZero(msg.ExpiresAt)
	return Reservation{
		TenantID:        tenantID,
		Provider:        normalizeProvider(msg.Provider),
		TaskID:          taskID,
		StepID:          stepID,
		PlanExecutionID: planExecutionID,
		StepExecutionID: stepExecutionID,
		AgentType:       msg.AgentType,
		EstimatedMicros: estimatedMicros,
		IdempotencyKey:  msg.IdempotencyKey,
		ExpiresAt:       expiresAt,
	}, nil
}

func usageEventFromProto(tenantID uuid.UUID, msg *budgetv1.RecordUsageRequest, now time.Time) (UsageEvent, error) {
	costMicros, err := moneyMicros(msg.Cost)
	if err != nil {
		return UsageEvent{}, err
	}
	taskID, err := nullableUUID(msg.TaskId)
	if err != nil {
		return UsageEvent{}, fmt.Errorf("invalid task_id: %w", err)
	}
	stepID, err := nullableUUID(msg.StepId)
	if err != nil {
		return UsageEvent{}, fmt.Errorf("invalid step_id: %w", err)
	}
	planExecutionID, err := nullableUUID(msg.PlanExecutionId)
	if err != nil {
		return UsageEvent{}, fmt.Errorf("invalid plan_execution_id: %w", err)
	}
	stepExecutionID, err := nullableUUID(msg.StepExecutionId)
	if err != nil {
		return UsageEvent{}, fmt.Errorf("invalid step_execution_id: %w", err)
	}
	reservationID, err := nullableUUID(msg.ReservationId)
	if err != nil {
		return UsageEvent{}, fmt.Errorf("invalid reservation_id: %w", err)
	}
	if msg.Provider == "" {
		return UsageEvent{}, errors.New("provider is required")
	}
	if msg.Model == "" {
		return UsageEvent{}, errors.New("model is required")
	}
	if msg.InputTokens < 0 || msg.OutputTokens < 0 {
		return UsageEvent{}, errors.New("token counts must be non-negative")
	}
	timestamp := timestampOrZero(msg.OccurredAt)
	if timestamp.IsZero() {
		timestamp = now
	}
	return UsageEvent{
		TenantID:        tenantID,
		ReservationID:   reservationID,
		TaskID:          taskID,
		StepID:          stepID,
		PlanExecutionID: planExecutionID,
		StepExecutionID: stepExecutionID,
		AgentType:       msg.AgentType,
		Provider:        normalizeProvider(msg.Provider),
		Model:           msg.Model,
		InputTokens:     msg.InputTokens,
		OutputTokens:    msg.OutputTokens,
		CostMicros:      costMicros,
		IdempotencyKey:  msg.IdempotencyKey,
		Timestamp:       timestamp.UTC(),
	}, nil
}

func timestampOrZero(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime().UTC()
}
