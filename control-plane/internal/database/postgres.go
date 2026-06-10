package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	slog.Info("database connection pool created")
	return pool, nil
}

func WithTenant(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID, fn func(Querier) error) error {
	if tenantID == uuid.Nil {
		return errors.New("tenant id is required")
	}
	return withSessionSettings(ctx, pool, sessionSettings{tenantID: tenantID}, fn)
}

func WithUserExternalID(ctx context.Context, pool *pgxpool.Pool, userExternalID string, fn func(Querier) error) error {
	if userExternalID == "" {
		return errors.New("user external id is required")
	}
	return withSessionSettings(ctx, pool, sessionSettings{userExternalID: userExternalID}, fn)
}

type sessionSettings struct {
	tenantID       uuid.UUID
	userExternalID string
}

func withSessionSettings(ctx context.Context, pool *pgxpool.Pool, settings sessionSettings, fn func(Querier) error) error {
	if pool == nil {
		return errors.New("database pool is required")
	}
	if fn == nil {
		return errors.New("database callback is required")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin database transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	if settings.tenantID != uuid.Nil {
		if _, err := tx.Exec(ctx, "SELECT set_config('harpia.tenant_id', $1, true)", settings.tenantID.String()); err != nil {
			return fmt.Errorf("set tenant context: %w", err)
		}
	}

	if settings.userExternalID != "" {
		if _, err := tx.Exec(ctx, "SELECT set_config('harpia.user_external_id', $1, true)", settings.userExternalID); err != nil {
			return fmt.Errorf("set user context: %w", err)
		}
	}

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit database transaction: %w", err)
	}
	committed = true
	return nil
}
