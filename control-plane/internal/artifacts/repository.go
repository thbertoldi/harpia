package artifacts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/harpia/control-plane/internal/database"
)

type ArtifactType struct {
	ID          uuid.UUID
	Key         string
	SchemaRef   string
	Version     int32
	Description string
}

type Artifact struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	ArtifactTypeID uuid.UUID
	StorageURI     string
	ContentHash    string
	CreatedAt      time.Time
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) RegisterType(ctx context.Context, artifactType *ArtifactType) (*ArtifactType, error) {
	var created ArtifactType
	err := r.pool.QueryRow(ctx,
		`INSERT INTO artifact_types (key, schema_ref, version, description)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, key, schema_ref, version, description`,
		artifactType.Key, artifactType.SchemaRef, artifactType.Version, artifactType.Description,
	).Scan(&created.ID, &created.Key, &created.SchemaRef, &created.Version, &created.Description)
	if err != nil {
		return nil, fmt.Errorf("register artifact type: %w", err)
	}
	return &created, nil
}

func (r *Repository) GetTypeByID(ctx context.Context, typeID uuid.UUID) (*ArtifactType, error) {
	var artifactType ArtifactType
	err := r.pool.QueryRow(ctx,
		`SELECT id, key, schema_ref, version, description
		 FROM artifact_types
		 WHERE id = $1`,
		typeID,
	).Scan(&artifactType.ID, &artifactType.Key, &artifactType.SchemaRef, &artifactType.Version, &artifactType.Description)
	if err != nil {
		return nil, fmt.Errorf("get artifact type: %w", err)
	}
	return &artifactType, nil
}

func (r *Repository) GetTypeByKey(ctx context.Context, key string) (*ArtifactType, error) {
	var artifactType ArtifactType
	err := r.pool.QueryRow(ctx,
		`SELECT id, key, schema_ref, version, description
		 FROM artifact_types
		 WHERE key = $1`,
		key,
	).Scan(&artifactType.ID, &artifactType.Key, &artifactType.SchemaRef, &artifactType.Version, &artifactType.Description)
	if err != nil {
		return nil, fmt.Errorf("get artifact type by key: %w", err)
	}
	return &artifactType, nil
}

func (r *Repository) CreateArtifact(ctx context.Context, artifact *Artifact) (*Artifact, error) {
	var created Artifact
	err := database.WithTenant(ctx, r.pool, artifact.TenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`INSERT INTO artifacts (id, tenant_id, artifact_type_id, storage_uri, content_hash)
			 VALUES ($1, $2, $3, $4, $5)
			 RETURNING id, tenant_id, artifact_type_id, storage_uri, content_hash, created_at`,
			artifact.ID, artifact.TenantID, artifact.ArtifactTypeID, artifact.StorageURI, artifact.ContentHash,
		)
		return row.Scan(
			&created.ID, &created.TenantID, &created.ArtifactTypeID,
			&created.StorageURI, &created.ContentHash, &created.CreatedAt,
		)
	})
	if err != nil {
		return nil, fmt.Errorf("create artifact: %w", err)
	}
	return &created, nil
}

func (r *Repository) GetArtifact(ctx context.Context, tenantID, artifactID uuid.UUID) (*Artifact, error) {
	var artifact Artifact
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`SELECT id, tenant_id, artifact_type_id, storage_uri, content_hash, created_at
			 FROM artifacts
			 WHERE id = $1 AND tenant_id = $2`,
			artifactID, tenantID,
		)
		return row.Scan(
			&artifact.ID, &artifact.TenantID, &artifact.ArtifactTypeID,
			&artifact.StorageURI, &artifact.ContentHash, &artifact.CreatedAt,
		)
	})
	if err != nil {
		return nil, fmt.Errorf("get artifact: %w", err)
	}
	return &artifact, nil
}

func IsNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
