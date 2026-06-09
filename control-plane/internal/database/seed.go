package database

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureDevData(ctx context.Context, pool *pgxpool.Pool) (tenantID uuid.UUID, userID uuid.UUID, err error) {
	err = pool.QueryRow(ctx,
		`INSERT INTO tenants (name, slug) VALUES ('Dev', 'dev') ON CONFLICT (slug) DO NOTHING RETURNING id`,
	).Scan(&tenantID)
	if err != nil {
		err = pool.QueryRow(ctx, `SELECT id FROM tenants WHERE slug = 'dev'`).Scan(&tenantID)
		if err != nil {
			return uuid.Nil, uuid.Nil, fmt.Errorf("ensure dev tenant: %w", err)
		}
	}

	err = WithUserExternalID(ctx, pool, "dev-user", func(q Querier) error {
		err := q.QueryRow(ctx,
			`INSERT INTO users (tenant_id, external_id, email, name, role)
			 VALUES ($1, 'dev-user', 'dev@harpia.local', 'Dev User', 'admin')
			 ON CONFLICT (tenant_id, external_id) DO NOTHING RETURNING id`,
			tenantID,
		).Scan(&userID)
		if err != nil {
			err = q.QueryRow(ctx,
				`SELECT id FROM users WHERE tenant_id = $1 AND external_id = 'dev-user'`, tenantID,
			).Scan(&userID)
			if err != nil {
				return fmt.Errorf("ensure dev user: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}

	return tenantID, userID, nil
}
