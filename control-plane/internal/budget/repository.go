package budget

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/harpia/control-plane/internal/database"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) UpsertQuota(ctx context.Context, quota TenantQuota) (*TenantQuota, error) {
	var out TenantQuota
	err := database.WithTenant(ctx, r.pool, quota.TenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`INSERT INTO tenant_quotas (
				tenant_id, provider, period, budget_usd, hard_cap, warning_threshold
			) VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (tenant_id, provider, period) DO UPDATE SET
				budget_usd = EXCLUDED.budget_usd,
				hard_cap = EXCLUDED.hard_cap,
				warning_threshold = EXCLUDED.warning_threshold
			RETURNING tenant_id, provider, period, budget_usd, hard_cap, warning_threshold, updated_at`,
			quota.TenantID,
			quota.Provider,
			quota.Period,
			microsToUSD(quota.BudgetMicros),
			quota.HardCap,
			quota.WarningThreshold,
		)
		return scanQuota(row, &out)
	})
	if err != nil {
		return nil, fmt.Errorf("upsert tenant quota: %w", err)
	}
	return &out, nil
}

func (r *Repository) GetQuotaSnapshots(ctx context.Context, tenantID uuid.UUID, provider string, now time.Time) ([]QuotaSnapshot, error) {
	var out []QuotaSnapshot
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		quotas, err := r.listQuotasForProvider(ctx, q, tenantID, provider, false)
		if err != nil {
			return err
		}
		out = make([]QuotaSnapshot, 0, len(quotas))
		for _, quota := range quotas {
			snapshot, err := r.snapshotQuota(ctx, q, quota, now)
			if err != nil {
				return err
			}
			out = append(out, snapshot)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get quota snapshots: %w", err)
	}
	return out, nil
}

func (r *Repository) ReserveBudget(ctx context.Context, reservation Reservation) (*ReservationResult, error) {
	now := time.Now().UTC()
	if reservation.ExpiresAt.IsZero() {
		reservation.ExpiresAt = now.Add(defaultReservationTTL)
	}
	if reservation.ID == uuid.Nil {
		reservation.ID = uuid.New()
	}

	var result ReservationResult
	err := database.WithTenant(ctx, r.pool, reservation.TenantID, func(q database.Querier) error {
		quotas, err := r.listQuotasForProvider(ctx, q, reservation.TenantID, reservation.Provider, true)
		if err != nil {
			return err
		}
		remaining := int64(0)
		remainingSet := false
		warning := false
		for _, quota := range quotas {
			snapshot, err := r.snapshotQuota(ctx, q, quota, now)
			if err != nil {
				return err
			}
			projected := snapshot.UsedMicros + snapshot.ReservedMicros + reservation.EstimatedMicros
			if quota.HardCap && projected > quota.BudgetMicros {
				return ErrQuotaExceeded
			}
			quotaRemaining := remainingMicros(quota.BudgetMicros, projected, 0)
			if !remainingSet || quotaRemaining < remaining {
				remaining = quotaRemaining
				remainingSet = true
			}
			warning = warning || warningReached(quota.BudgetMicros, projected, quota.WarningThreshold)
		}

		row := q.QueryRow(ctx,
			`INSERT INTO llm_budget_reservations (
				id, tenant_id, provider, task_id, step_id, plan_execution_id,
				step_execution_id, agent_type, estimated_cost_usd, idempotency_key, expires_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (tenant_id, idempotency_key) WHERE idempotency_key <> ''
			DO UPDATE SET expires_at = llm_budget_reservations.expires_at
			RETURNING id, tenant_id, provider, task_id, step_id, plan_execution_id,
				step_execution_id, agent_type, estimated_cost_usd, idempotency_key, expires_at`,
			reservation.ID,
			reservation.TenantID,
			reservation.Provider,
			nullUUIDValue(reservation.TaskID),
			nullUUIDValue(reservation.StepID),
			nullUUIDValue(reservation.PlanExecutionID),
			nullUUIDValue(reservation.StepExecutionID),
			reservation.AgentType,
			microsToUSD(reservation.EstimatedMicros),
			reservation.IdempotencyKey,
			reservation.ExpiresAt,
		)
		if err := scanReservation(row, &result.Reservation); err != nil {
			return err
		}
		result.RemainingMicros = remaining
		result.Warning = warning
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reserve budget: %w", err)
	}
	return &result, nil
}

func (r *Repository) RecordUsage(ctx context.Context, event UsageEvent) (*UsageRecordResult, error) {
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	var result UsageRecordResult
	err := database.WithTenant(ctx, r.pool, event.TenantID, func(q database.Querier) error {
		if event.IdempotencyKey != "" {
			tag, err := q.Exec(ctx,
				`INSERT INTO llm_usage_event_keys (
					tenant_id, idempotency_key, usage_event_id, usage_event_ts
				) VALUES ($1, $2, $3, $4)
				ON CONFLICT (tenant_id, idempotency_key) DO NOTHING`,
				event.TenantID,
				event.IdempotencyKey,
				event.ID,
				event.Timestamp,
			)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				result.Duplicate = true
				return r.fillPostUsageStatus(ctx, q, event.TenantID, event.Provider, event.Timestamp, &result)
			}
		}

		if _, err := q.Exec(ctx, `SELECT ensure_llm_usage_events_partition($1)`, event.Timestamp); err != nil {
			return fmt.Errorf("ensure usage partition: %w", err)
		}
		if _, err := q.Exec(ctx,
			`INSERT INTO llm_usage_events (
				id, tenant_id, reservation_id, task_id, step_id, plan_execution_id,
				step_execution_id, agent_type, provider, model, input_tokens,
				output_tokens, cost_usd, idempotency_key, ts
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
			event.ID,
			event.TenantID,
			nullUUIDValue(event.ReservationID),
			nullUUIDValue(event.TaskID),
			nullUUIDValue(event.StepID),
			nullUUIDValue(event.PlanExecutionID),
			nullUUIDValue(event.StepExecutionID),
			event.AgentType,
			event.Provider,
			event.Model,
			event.InputTokens,
			event.OutputTokens,
			microsToUSD(event.CostMicros),
			event.IdempotencyKey,
			event.Timestamp,
		); err != nil {
			return fmt.Errorf("insert usage event: %w", err)
		}

		if event.ReservationID.Valid {
			if _, err := q.Exec(ctx,
				`UPDATE llm_budget_reservations
				 SET status = 'consumed', released_at = now()
				 WHERE tenant_id = $1 AND id = $2 AND status = 'reserved'`,
				event.TenantID,
				event.ReservationID.UUID,
			); err != nil {
				return fmt.Errorf("consume reservation: %w", err)
			}
		}

		if err := r.incrementSummary(ctx, q, event, event.Provider, PeriodDaily); err != nil {
			return err
		}
		if err := r.incrementSummary(ctx, q, event, event.Provider, PeriodMonthly); err != nil {
			return err
		}
		if err := r.incrementSummary(ctx, q, event, "", PeriodDaily); err != nil {
			return err
		}
		if err := r.incrementSummary(ctx, q, event, "", PeriodMonthly); err != nil {
			return err
		}

		result.UsageEventID = event.ID
		return r.fillPostUsageStatus(ctx, q, event.TenantID, event.Provider, event.Timestamp, &result)
	})
	if err != nil {
		return nil, fmt.Errorf("record usage: %w", err)
	}
	if result.UsageEventID == uuid.Nil {
		result.UsageEventID = event.ID
	}
	return &result, nil
}

func (r *Repository) RebuildUsageSummary(ctx context.Context, tenantID uuid.UUID, from, to time.Time) error {
	if from.IsZero() || to.IsZero() || !from.Before(to) {
		return errors.New("valid from/to range is required")
	}
	return database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		if _, err := q.Exec(ctx,
			`DELETE FROM tenant_usage_summary
			 WHERE tenant_id = $1 AND period_start >= $2::date AND period_start < $3::date`,
			tenantID,
			from.UTC(),
			to.UTC(),
		); err != nil {
			return fmt.Errorf("clear usage summary: %w", err)
		}
		if _, err := q.Exec(ctx,
			`INSERT INTO tenant_usage_summary (
				tenant_id, provider, period, period_start, input_tokens,
				output_tokens, cost_usd, event_count
			)
			SELECT tenant_id, provider, 'daily', date_trunc('day', ts)::date,
				COALESCE(sum(input_tokens), 0), COALESCE(sum(output_tokens), 0),
				COALESCE(sum(cost_usd), 0), count(*)
			FROM llm_usage_events
			WHERE tenant_id = $1 AND ts >= $2 AND ts < $3
			GROUP BY tenant_id, provider, date_trunc('day', ts)::date
			UNION ALL
			SELECT tenant_id, '', 'daily', date_trunc('day', ts)::date,
				COALESCE(sum(input_tokens), 0), COALESCE(sum(output_tokens), 0),
				COALESCE(sum(cost_usd), 0), count(*)
			FROM llm_usage_events
			WHERE tenant_id = $1 AND ts >= $2 AND ts < $3
			GROUP BY tenant_id, date_trunc('day', ts)::date
			UNION ALL
			SELECT tenant_id, provider, 'monthly', date_trunc('month', ts)::date,
				COALESCE(sum(input_tokens), 0), COALESCE(sum(output_tokens), 0),
				COALESCE(sum(cost_usd), 0), count(*)
			FROM llm_usage_events
			WHERE tenant_id = $1 AND ts >= $2 AND ts < $3
			GROUP BY tenant_id, provider, date_trunc('month', ts)::date
			UNION ALL
			SELECT tenant_id, '', 'monthly', date_trunc('month', ts)::date,
				COALESCE(sum(input_tokens), 0), COALESCE(sum(output_tokens), 0),
				COALESCE(sum(cost_usd), 0), count(*)
			FROM llm_usage_events
			WHERE tenant_id = $1 AND ts >= $2 AND ts < $3
			GROUP BY tenant_id, date_trunc('month', ts)::date
			ON CONFLICT (tenant_id, provider, period, period_start) DO UPDATE SET
				input_tokens = EXCLUDED.input_tokens,
				output_tokens = EXCLUDED.output_tokens,
				cost_usd = EXCLUDED.cost_usd,
				event_count = EXCLUDED.event_count,
				updated_at = now()`,
			tenantID,
			from.UTC(),
			to.UTC(),
		); err != nil {
			return fmt.Errorf("rebuild usage summary: %w", err)
		}
		return nil
	})
}

type scanner interface {
	Scan(dest ...any) error
}

func scanQuota(s scanner, q *TenantQuota) error {
	var budgetUSD float64
	if err := s.Scan(
		&q.TenantID,
		&q.Provider,
		&q.Period,
		&budgetUSD,
		&q.HardCap,
		&q.WarningThreshold,
		&q.UpdatedAt,
	); err != nil {
		return err
	}
	q.BudgetMicros = usdToMicros(budgetUSD)
	return nil
}

func scanReservation(s scanner, r *Reservation) error {
	var estimatedUSD float64
	if err := s.Scan(
		&r.ID,
		&r.TenantID,
		&r.Provider,
		&r.TaskID,
		&r.StepID,
		&r.PlanExecutionID,
		&r.StepExecutionID,
		&r.AgentType,
		&estimatedUSD,
		&r.IdempotencyKey,
		&r.ExpiresAt,
	); err != nil {
		return err
	}
	r.EstimatedMicros = usdToMicros(estimatedUSD)
	return nil
}

func (r *Repository) listQuotasForProvider(ctx context.Context, q database.Querier, tenantID uuid.UUID, provider string, forUpdate bool) ([]TenantQuota, error) {
	sql := `SELECT tenant_id, provider, period, budget_usd, hard_cap, warning_threshold, updated_at
		FROM tenant_quotas
		WHERE tenant_id = $1 AND (provider = '' OR provider = $2)
		ORDER BY provider ASC, period ASC`
	if forUpdate {
		sql += ` FOR UPDATE`
	}
	rows, err := q.Query(ctx, sql, tenantID, provider)
	if err != nil {
		return nil, fmt.Errorf("list quotas: %w", err)
	}
	defer rows.Close()

	var quotas []TenantQuota
	for rows.Next() {
		var quota TenantQuota
		if err := scanQuota(rows, &quota); err != nil {
			return nil, fmt.Errorf("scan quota: %w", err)
		}
		quotas = append(quotas, quota)
	}
	return quotas, rows.Err()
}

func (r *Repository) snapshotQuota(ctx context.Context, q database.Querier, quota TenantQuota, now time.Time) (QuotaSnapshot, error) {
	start, end := periodBounds(quota.Period, now)
	snapshot := QuotaSnapshot{
		TenantID:         quota.TenantID,
		Provider:         quota.Provider,
		Period:           quota.Period,
		BudgetMicros:     quota.BudgetMicros,
		HardCap:          quota.HardCap,
		WarningThreshold: quota.WarningThreshold,
		PeriodStart:      start,
		PeriodEnd:        end,
	}
	var usedUSD float64
	err := q.QueryRow(ctx,
		`SELECT input_tokens, output_tokens, cost_usd, event_count
		 FROM tenant_usage_summary
		 WHERE tenant_id = $1 AND provider = $2 AND period = $3 AND period_start = $4`,
		quota.TenantID,
		quota.Provider,
		quota.Period,
		start,
	).Scan(&snapshot.InputTokens, &snapshot.OutputTokens, &usedUSD, &snapshot.EventCount)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return snapshot, fmt.Errorf("load usage summary: %w", err)
	}
	snapshot.UsedMicros = usdToMicros(usedUSD)

	var reservedUSD float64
	reservationProviderPredicate := `provider = $2`
	if quota.Provider == "" {
		reservationProviderPredicate = `$2 = $2`
	}
	err = q.QueryRow(ctx,
		fmt.Sprintf(`SELECT COALESCE(sum(estimated_cost_usd), 0)
			FROM llm_budget_reservations
			WHERE tenant_id = $1 AND %s AND status = 'reserved'
				AND expires_at > $3 AND created_at >= $4 AND created_at < $5`, reservationProviderPredicate),
		quota.TenantID,
		quota.Provider,
		now,
		start,
		end,
	).Scan(&reservedUSD)
	if err != nil {
		return snapshot, fmt.Errorf("load active reservations: %w", err)
	}
	snapshot.ReservedMicros = usdToMicros(reservedUSD)
	snapshot.RemainingMicros = remainingMicros(snapshot.BudgetMicros, snapshot.UsedMicros, snapshot.ReservedMicros)
	snapshot.Warning = warningReached(snapshot.BudgetMicros, snapshot.UsedMicros+snapshot.ReservedMicros, snapshot.WarningThreshold)
	return snapshot, nil
}

func (r *Repository) incrementSummary(ctx context.Context, q database.Querier, event UsageEvent, provider, period string) error {
	start, _ := periodBounds(period, event.Timestamp)
	_, err := q.Exec(ctx,
		`INSERT INTO tenant_usage_summary (
			tenant_id, provider, period, period_start, input_tokens,
			output_tokens, cost_usd, event_count
		) VALUES ($1, $2, $3, $4, $5, $6, $7, 1)
		ON CONFLICT (tenant_id, provider, period, period_start) DO UPDATE SET
			input_tokens = tenant_usage_summary.input_tokens + EXCLUDED.input_tokens,
			output_tokens = tenant_usage_summary.output_tokens + EXCLUDED.output_tokens,
			cost_usd = tenant_usage_summary.cost_usd + EXCLUDED.cost_usd,
			event_count = tenant_usage_summary.event_count + 1,
			updated_at = now()`,
		event.TenantID,
		provider,
		period,
		start,
		event.InputTokens,
		event.OutputTokens,
		microsToUSD(event.CostMicros),
	)
	if err != nil {
		return fmt.Errorf("increment %s usage summary: %w", period, err)
	}
	return nil
}

func (r *Repository) fillPostUsageStatus(ctx context.Context, q database.Querier, tenantID uuid.UUID, provider string, now time.Time, result *UsageRecordResult) error {
	quotas, err := r.listQuotasForProvider(ctx, q, tenantID, provider, false)
	if err != nil {
		return err
	}
	remaining := int64(0)
	remainingSet := false
	for _, quota := range quotas {
		snapshot, err := r.snapshotQuota(ctx, q, quota, now)
		if err != nil {
			return err
		}
		if !remainingSet || snapshot.RemainingMicros < remaining {
			remaining = snapshot.RemainingMicros
			remainingSet = true
		}
		result.Warning = result.Warning || snapshot.Warning
	}
	result.RemainingMicros = remaining
	return nil
}

func nullUUIDValue(value uuid.NullUUID) any {
	if !value.Valid {
		return nil
	}
	return value.UUID
}

func nullableUUID(raw string) (uuid.NullUUID, error) {
	if raw == "" {
		return uuid.NullUUID{}, nil
	}
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return uuid.NullUUID{}, err
	}
	return uuid.NullUUID{UUID: parsed, Valid: true}, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
