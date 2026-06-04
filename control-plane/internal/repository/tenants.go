package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Tenant struct {
	ID        uuid.UUID       `json:"id"`
	Name      string          `json:"name"`
	Slug      string          `json:"slug"`
	Settings  json.RawMessage `json:"settings"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type TenantRepository struct {
	pool *pgxpool.Pool
}

func NewTenantRepository(pool *pgxpool.Pool) *TenantRepository {
	return &TenantRepository{pool: pool}
}

func (r *TenantRepository) Create(ctx context.Context, tenant *Tenant) (*Tenant, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO tenants (name, slug, settings)
		 VALUES ($1, $2, $3)
		 RETURNING id, name, slug, settings, created_at, updated_at`,
		tenant.Name, tenant.Slug, tenant.Settings,
	)

	var created Tenant
	err := row.Scan(
		&created.ID, &created.Name, &created.Slug, &created.Settings,
		&created.CreatedAt, &created.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert tenant: %w", err)
	}

	return &created, nil
}

func (r *TenantRepository) GetBySlug(ctx context.Context, slug string) (*Tenant, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, name, slug, settings, created_at, updated_at
		 FROM tenants
		 WHERE slug = $1`,
		slug,
	)

	var tenant Tenant
	err := row.Scan(
		&tenant.ID, &tenant.Name, &tenant.Slug, &tenant.Settings,
		&tenant.CreatedAt, &tenant.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get tenant by slug: %w", err)
	}

	return &tenant, nil
}
