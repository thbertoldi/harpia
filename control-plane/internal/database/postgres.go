package database

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

func SetTenantContext(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) (*pgxpool.Conn, error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire connection: %w", err)
	}

	_, err = conn.Exec(ctx, "SET LOCAL harpia.tenant_id = $1", tenantID.String())
	if err != nil {
		conn.Release()
		return nil, fmt.Errorf("set tenant context: %w", err)
	}

	return conn, nil
}
