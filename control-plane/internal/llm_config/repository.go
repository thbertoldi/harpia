// Package llm_config implements tenant-scoped LLM provider configuration
// storage, envelope encryption, and the internal resolver used by the agent
// runtime to obtain credentials for one execution context.
//
// See docs/notes/2026-06-16-tenant-llm-config-design.md for the design.
package llm_config

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/harpia/control-plane/internal/database"
	cryptoenv "github.com/harpia/control-plane/internal/llm_config/crypto"
)

// Config is the persisted row shape (metadata only — never includes
// plaintext key material).
type Config struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	Provider      string
	KEKVersion    string
	EncryptedDEK  []byte
	EncryptedKey  []byte
	DefaultModel  string
	AllowedModels []string
	ManagedBy     string
	LastRotatedAt time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ErrNotFound is returned when a (tenant, provider) row does not exist.
var ErrNotFound = errors.New("tenant llm config not found")

// Repository persists tenant_llm_configs rows.
//
// The repository never sees plaintext keys: encryption is performed by the
// handler before calling Upsert. RLS context is established for every
// operation via database.WithTenant.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a repository bound to the given pgx pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Upsert creates or replaces the (tenant, provider) configuration row.
//
// The caller is expected to have already envelope-encrypted the API key.
// The function refreshes `last_rotated_at` whenever ciphertext changes.
func (r *Repository) Upsert(ctx context.Context, tenantID uuid.UUID, cfg *Config) (*Config, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}
	if cfg.Provider == "" {
		return nil, errors.New("provider is required")
	}
	if len(cfg.EncryptedDEK) == 0 || len(cfg.EncryptedKey) == 0 {
		return nil, errors.New("encrypted dek/key payloads are required")
	}
	if cfg.ManagedBy == "" {
		cfg.ManagedBy = "tenant_self"
	}
	if cfg.AllowedModels == nil {
		cfg.AllowedModels = []string{}
	}

	var out Config
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`INSERT INTO tenant_llm_configs (
				tenant_id, provider, kek_version, encrypted_dek, encrypted_api_key,
				default_model, allowed_models, managed_by, last_rotated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
			ON CONFLICT (tenant_id, provider) DO UPDATE SET
				kek_version = EXCLUDED.kek_version,
				encrypted_dek = EXCLUDED.encrypted_dek,
				encrypted_api_key = EXCLUDED.encrypted_api_key,
				default_model = EXCLUDED.default_model,
				allowed_models = EXCLUDED.allowed_models,
				managed_by = EXCLUDED.managed_by,
				last_rotated_at = now()
			RETURNING id, tenant_id, provider, kek_version, encrypted_dek, encrypted_api_key,
				default_model, allowed_models, managed_by, last_rotated_at, created_at, updated_at`,
			tenantID, cfg.Provider, cfg.KEKVersion, cfg.EncryptedDEK, cfg.EncryptedKey,
			cfg.DefaultModel, cfg.AllowedModels, cfg.ManagedBy,
		)
		return scanConfig(row, &out)
	})
	if err != nil {
		return nil, fmt.Errorf("upsert tenant_llm_configs: %w", err)
	}
	return &out, nil
}

// UpdateMetadata refreshes only non-ciphertext fields for an existing row
// (used when the caller did not include a new api_key in Set).
func (r *Repository) UpdateMetadata(ctx context.Context, tenantID uuid.UUID, provider, defaultModel string, allowedModels []string, managedBy string) (*Config, error) {
	if allowedModels == nil {
		allowedModels = []string{}
	}
	if managedBy == "" {
		managedBy = "tenant_self"
	}
	var out Config
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`UPDATE tenant_llm_configs SET
				default_model = $3,
				allowed_models = $4,
				managed_by = $5
			WHERE tenant_id = $1 AND provider = $2
			RETURNING id, tenant_id, provider, kek_version, encrypted_dek, encrypted_api_key,
				default_model, allowed_models, managed_by, last_rotated_at, created_at, updated_at`,
			tenantID, provider, defaultModel, allowedModels, managedBy,
		)
		if err := scanConfig(row, &out); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// GetByProvider returns the row for (tenant, provider) or ErrNotFound.
func (r *Repository) GetByProvider(ctx context.Context, tenantID uuid.UUID, provider string) (*Config, error) {
	var out Config
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`SELECT id, tenant_id, provider, kek_version, encrypted_dek, encrypted_api_key,
				default_model, allowed_models, managed_by, last_rotated_at, created_at, updated_at
			FROM tenant_llm_configs
			WHERE tenant_id = $1 AND provider = $2`,
			tenantID, provider,
		)
		if err := scanConfig(row, &out); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ListByTenant returns all rows for a tenant.
func (r *Repository) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]Config, error) {
	var out []Config
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		rows, err := q.Query(ctx,
			`SELECT id, tenant_id, provider, kek_version, encrypted_dek, encrypted_api_key,
				default_model, allowed_models, managed_by, last_rotated_at, created_at, updated_at
			FROM tenant_llm_configs
			WHERE tenant_id = $1
			ORDER BY provider`,
			tenantID,
		)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c Config
			if err := scanConfig(rows, &c); err != nil {
				return err
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("list tenant_llm_configs: %w", err)
	}
	return out, nil
}

// Delete removes the (tenant, provider) row. Returns ErrNotFound when no row
// matched, so callers can surface a clean 404-like response.
func (r *Repository) Delete(ctx context.Context, tenantID uuid.UUID, provider string) error {
	return database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		tag, err := q.Exec(ctx,
			`DELETE FROM tenant_llm_configs WHERE tenant_id = $1 AND provider = $2`,
			tenantID, provider,
		)
		if err != nil {
			return fmt.Errorf("delete tenant_llm_configs: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// scanConfig is a small helper accepting either pgx.Row or pgx.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanConfig(s scanner, c *Config) error {
	return s.Scan(
		&c.ID, &c.TenantID, &c.Provider, &c.KEKVersion, &c.EncryptedDEK, &c.EncryptedKey,
		&c.DefaultModel, &c.AllowedModels, &c.ManagedBy, &c.LastRotatedAt, &c.CreatedAt, &c.UpdatedAt,
	)
}

// AsEncryptedRecord exposes the row in the shape the crypto envelope expects.
func (c *Config) AsEncryptedRecord() cryptoenv.EncryptedRecord {
	return cryptoenv.EncryptedRecord{
		KEKVersion:       c.KEKVersion,
		EncryptedDEK:     c.EncryptedDEK,
		EncryptedPayload: c.EncryptedKey,
	}
}
