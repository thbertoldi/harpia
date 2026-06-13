package executors

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/harpia/control-plane/internal/database"
)

const (
	KindIntegration = "integration"
	KindAgent       = "agent"
)

type CompatibilityMetadata struct {
	InputArtifactTypeKeys  []string `json:"input_artifact_type_keys,omitempty"`
	OutputArtifactTypeKeys []string `json:"output_artifact_type_keys,omitempty"`
	ConnectionType         string   `json:"connection_type,omitempty"`
	ManifestID             string   `json:"manifest_id,omitempty"`
	ManifestVersion        string   `json:"manifest_version,omitempty"`
}

type ExecutorSKU struct {
	ID            uuid.UUID
	Key           string
	DisplayName   string
	Description   string
	Kind          string
	PriceCents    int64
	Currency      string
	Compatibility CompatibilityMetadata
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ExecutorEntitlement struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	ExecutorSKUID uuid.UUID
	GrantedAt     time.Time
	GrantedBy     *uuid.UUID
}

type ExecutorInstallation struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	ExecutorSKUID    uuid.UUID
	Kind             string
	DisplayName      string
	Enabled          bool
	ConnectionStatus *string
	ConfigJSON       json.RawMessage
	ManifestID       *string
	ManifestVersion  *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) ListSKUs(ctx context.Context, kindFilter string, limit, offset int) ([]ExecutorSKU, error) {
	query := `SELECT id, key, display_name, description, kind, price_cents, currency, compatibility, created_at, updated_at
	          FROM executor_skus`
	args := []any{}
	argPos := 1

	if kindFilter != "" {
		query += fmt.Sprintf(" WHERE kind = $%d", argPos)
		args = append(args, kindFilter)
		argPos++
	}

	query += fmt.Sprintf(" ORDER BY key ASC LIMIT $%d OFFSET $%d", argPos, argPos+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list executor skus: %w", err)
	}
	defer rows.Close()

	return scanSKUs(rows)
}

func (r *Repository) GetSKUByID(ctx context.Context, id uuid.UUID) (*ExecutorSKU, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, key, display_name, description, kind, price_cents, currency, compatibility, created_at, updated_at
		 FROM executor_skus WHERE id = $1`, id,
	)
	return scanSKU(row)
}

func (r *Repository) GetSKUByKey(ctx context.Context, key string) (*ExecutorSKU, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, key, display_name, description, kind, price_cents, currency, compatibility, created_at, updated_at
		 FROM executor_skus WHERE key = $1`, key,
	)
	return scanSKU(row)
}

func (r *Repository) UpsertSKU(ctx context.Context, sku *ExecutorSKU) (*ExecutorSKU, error) {
	compatibility, err := json.Marshal(sku.Compatibility)
	if err != nil {
		return nil, fmt.Errorf("marshal compatibility: %w", err)
	}

	row := r.pool.QueryRow(ctx,
		`INSERT INTO executor_skus (key, display_name, description, kind, price_cents, currency, compatibility)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (key) DO UPDATE SET
		   display_name = EXCLUDED.display_name,
		   description = EXCLUDED.description,
		   kind = EXCLUDED.kind,
		   price_cents = EXCLUDED.price_cents,
		   currency = EXCLUDED.currency,
		   compatibility = EXCLUDED.compatibility,
		   updated_at = now()
		 RETURNING id, key, display_name, description, kind, price_cents, currency, compatibility, created_at, updated_at`,
		sku.Key, sku.DisplayName, sku.Description, sku.Kind, sku.PriceCents, sku.Currency, compatibility,
	)
	return scanSKU(row)
}

func (r *Repository) ListEntitlements(ctx context.Context, tenantID uuid.UUID, skuID *uuid.UUID, limit, offset int) ([]ExecutorEntitlement, error) {
	var entitlements []ExecutorEntitlement
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		query := `SELECT id, tenant_id, executor_sku_id, granted_at, granted_by
		          FROM executor_entitlements WHERE tenant_id = $1`
		args := []any{tenantID}
		argPos := 2

		if skuID != nil {
			query += fmt.Sprintf(" AND executor_sku_id = $%d", argPos)
			args = append(args, *skuID)
			argPos++
		}

		query += fmt.Sprintf(" ORDER BY granted_at DESC LIMIT $%d OFFSET $%d", argPos, argPos+1)
		args = append(args, limit, offset)

		rows, err := q.Query(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("list executor entitlements: %w", err)
		}
		defer rows.Close()

		entitlements, err = scanEntitlements(rows)
		return err
	})
	if err != nil {
		return nil, err
	}
	return entitlements, nil
}

func (r *Repository) GetEntitlementByID(ctx context.Context, tenantID, id uuid.UUID) (*ExecutorEntitlement, error) {
	var entitlement *ExecutorEntitlement
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`SELECT id, tenant_id, executor_sku_id, granted_at, granted_by
			 FROM executor_entitlements WHERE tenant_id = $1 AND id = $2`,
			tenantID, id,
		)
		var scanErr error
		entitlement, scanErr = scanEntitlement(row)
		return scanErr
	})
	if err != nil {
		return nil, err
	}
	return entitlement, nil
}

func (r *Repository) CreateEntitlement(ctx context.Context, tenantID, skuID uuid.UUID, grantedBy *uuid.UUID) (*ExecutorEntitlement, error) {
	var entitlement *ExecutorEntitlement
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`INSERT INTO executor_entitlements (tenant_id, executor_sku_id, granted_by)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (tenant_id, executor_sku_id) DO UPDATE SET granted_at = now()
			 RETURNING id, tenant_id, executor_sku_id, granted_at, granted_by`,
			tenantID, skuID, grantedBy,
		)
		var scanErr error
		entitlement, scanErr = scanEntitlement(row)
		return scanErr
	})
	if err != nil {
		return nil, err
	}
	return entitlement, nil
}

func (r *Repository) ListInstallations(ctx context.Context, tenantID uuid.UUID, kindFilter string, skuID *uuid.UUID, limit, offset int) ([]ExecutorInstallation, error) {
	var installations []ExecutorInstallation
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		query := `SELECT id, tenant_id, executor_sku_id, kind, display_name, enabled,
		                 connection_status, config_json, manifest_id, manifest_version, created_at, updated_at
		          FROM executor_installations WHERE tenant_id = $1`
		args := []any{tenantID}
		argPos := 2

		if kindFilter != "" {
			query += fmt.Sprintf(" AND kind = $%d", argPos)
			args = append(args, kindFilter)
			argPos++
		}
		if skuID != nil {
			query += fmt.Sprintf(" AND executor_sku_id = $%d", argPos)
			args = append(args, *skuID)
			argPos++
		}

		query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argPos, argPos+1)
		args = append(args, limit, offset)

		rows, err := q.Query(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("list executor installations: %w", err)
		}
		defer rows.Close()

		installations, err = scanInstallations(rows)
		return err
	})
	if err != nil {
		return nil, err
	}
	return installations, nil
}

func (r *Repository) GetInstallationByID(ctx context.Context, tenantID, id uuid.UUID) (*ExecutorInstallation, error) {
	var installation *ExecutorInstallation
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`SELECT id, tenant_id, executor_sku_id, kind, display_name, enabled,
			        connection_status, config_json, manifest_id, manifest_version, created_at, updated_at
			 FROM executor_installations WHERE tenant_id = $1 AND id = $2`,
			tenantID, id,
		)
		var scanErr error
		installation, scanErr = scanInstallation(row)
		return scanErr
	})
	if err != nil {
		return nil, err
	}
	return installation, nil
}

func (r *Repository) CreateInstallation(ctx context.Context, installation *ExecutorInstallation) (*ExecutorInstallation, error) {
	var created *ExecutorInstallation
	err := database.WithTenant(ctx, r.pool, installation.TenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`INSERT INTO executor_installations (
			   tenant_id, executor_sku_id, kind, display_name, enabled,
			   connection_status, config_json, manifest_id, manifest_version
			 ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			 RETURNING id, tenant_id, executor_sku_id, kind, display_name, enabled,
			           connection_status, config_json, manifest_id, manifest_version, created_at, updated_at`,
			installation.TenantID,
			installation.ExecutorSKUID,
			installation.Kind,
			installation.DisplayName,
			installation.Enabled,
			installation.ConnectionStatus,
			installation.ConfigJSON,
			installation.ManifestID,
			installation.ManifestVersion,
		)
		var scanErr error
		created, scanErr = scanInstallation(row)
		return scanErr
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func scanSKU(row pgx.Row) (*ExecutorSKU, error) {
	var sku ExecutorSKU
	var compatibilityJSON []byte
	err := row.Scan(
		&sku.ID, &sku.Key, &sku.DisplayName, &sku.Description, &sku.Kind,
		&sku.PriceCents, &sku.Currency, &compatibilityJSON, &sku.CreatedAt, &sku.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan executor sku: %w", err)
	}
	if len(compatibilityJSON) > 0 {
		if err := json.Unmarshal(compatibilityJSON, &sku.Compatibility); err != nil {
			return nil, fmt.Errorf("unmarshal compatibility: %w", err)
		}
	}
	return &sku, nil
}

func scanSKUs(rows pgx.Rows) ([]ExecutorSKU, error) {
	skus := make([]ExecutorSKU, 0)
	for rows.Next() {
		var sku ExecutorSKU
		var compatibilityJSON []byte
		if err := rows.Scan(
			&sku.ID, &sku.Key, &sku.DisplayName, &sku.Description, &sku.Kind,
			&sku.PriceCents, &sku.Currency, &compatibilityJSON, &sku.CreatedAt, &sku.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan executor sku row: %w", err)
		}
		if len(compatibilityJSON) > 0 {
			if err := json.Unmarshal(compatibilityJSON, &sku.Compatibility); err != nil {
				return nil, fmt.Errorf("unmarshal compatibility: %w", err)
			}
		}
		skus = append(skus, sku)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate executor skus: %w", err)
	}
	return skus, nil
}

func scanEntitlement(row pgx.Row) (*ExecutorEntitlement, error) {
	var entitlement ExecutorEntitlement
	err := row.Scan(
		&entitlement.ID, &entitlement.TenantID, &entitlement.ExecutorSKUID,
		&entitlement.GrantedAt, &entitlement.GrantedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("scan executor entitlement: %w", err)
	}
	return &entitlement, nil
}

func scanEntitlements(rows pgx.Rows) ([]ExecutorEntitlement, error) {
	entitlements := make([]ExecutorEntitlement, 0)
	for rows.Next() {
		var entitlement ExecutorEntitlement
		if err := rows.Scan(
			&entitlement.ID, &entitlement.TenantID, &entitlement.ExecutorSKUID,
			&entitlement.GrantedAt, &entitlement.GrantedBy,
		); err != nil {
			return nil, fmt.Errorf("scan executor entitlement row: %w", err)
		}
		entitlements = append(entitlements, entitlement)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate executor entitlements: %w", err)
	}
	return entitlements, nil
}

func scanInstallation(row pgx.Row) (*ExecutorInstallation, error) {
	var installation ExecutorInstallation
	err := row.Scan(
		&installation.ID, &installation.TenantID, &installation.ExecutorSKUID,
		&installation.Kind, &installation.DisplayName, &installation.Enabled,
		&installation.ConnectionStatus, &installation.ConfigJSON,
		&installation.ManifestID, &installation.ManifestVersion,
		&installation.CreatedAt, &installation.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan executor installation: %w", err)
	}
	return &installation, nil
}

func scanInstallations(rows pgx.Rows) ([]ExecutorInstallation, error) {
	installations := make([]ExecutorInstallation, 0)
	for rows.Next() {
		var installation ExecutorInstallation
		if err := rows.Scan(
			&installation.ID, &installation.TenantID, &installation.ExecutorSKUID,
			&installation.Kind, &installation.DisplayName, &installation.Enabled,
			&installation.ConnectionStatus, &installation.ConfigJSON,
			&installation.ManifestID, &installation.ManifestVersion,
			&installation.CreatedAt, &installation.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan executor installation row: %w", err)
		}
		installations = append(installations, installation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate executor installations: %w", err)
	}
	return installations, nil
}
