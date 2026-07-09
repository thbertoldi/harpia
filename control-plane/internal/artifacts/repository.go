package artifacts

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
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
	ID                  uuid.UUID
	TenantID            uuid.UUID
	ArtifactTypeID      uuid.UUID
	ArtifactTypeKey     string
	StorageURI          string
	ContentHash         string
	CurrentVersionID    uuid.NullUUID
	PlanConfigurationID uuid.NullUUID
	PlanExecutionID     uuid.NullUUID
	StepExecutionID     uuid.NullUUID
	Status              artifactsv1.ArtifactStatus
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type ArtifactVersion struct {
	ID                    uuid.UUID
	ArtifactID            uuid.UUID
	TenantID              uuid.UUID
	VersionNumber         int32
	StorageURI            string
	ContentHash           string
	SourcePlanExecutionID uuid.NullUUID
	SourceStepExecutionID uuid.NullUUID
	SourceVersionID       uuid.NullUUID
	CreatedByUserID       uuid.NullUUID
	CreatedByKind         artifactsv1.ArtifactVersionCreatedByKind
	EditSummary           string
	CreatedAt             time.Time
}

type ArtifactListFilter struct {
	PlanConfigurationID *uuid.UUID
	PlanExecutionID     *uuid.UUID
	ArtifactTypeKey     string
	Limit               int
	Offset              int
}

var ErrContentHashConflict = errors.New("artifact content hash changed")

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

// UpsertType inserts an artifact type, or updates the mutable fields when the
// key already exists. It is the idempotent counterpart to RegisterType used by
// the startup seeder (catalog data lives in Go, not in migrations — see
// Platform Constitution §13).
func (r *Repository) UpsertType(ctx context.Context, artifactType *ArtifactType) (*ArtifactType, error) {
	var upserted ArtifactType
	err := r.pool.QueryRow(ctx,
		`INSERT INTO artifact_types (key, schema_ref, version, description)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (key) DO UPDATE SET
		     schema_ref = EXCLUDED.schema_ref,
		     version = EXCLUDED.version,
		     description = EXCLUDED.description
		 RETURNING id, key, schema_ref, version, description`,
		artifactType.Key, artifactType.SchemaRef, artifactType.Version, artifactType.Description,
	).Scan(&upserted.ID, &upserted.Key, &upserted.SchemaRef, &upserted.Version, &upserted.Description)
	if err != nil {
		return nil, fmt.Errorf("upsert artifact type: %w", err)
	}
	return &upserted, nil
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
	if artifact.ID == uuid.Nil {
		artifact.ID = uuid.New()
	}
	status := artifact.Status
	if status == artifactsv1.ArtifactStatus_ARTIFACT_STATUS_UNSPECIFIED {
		status = artifactsv1.ArtifactStatus_ARTIFACT_STATUS_GENERATED
	}

	var created *Artifact
	err := database.WithTenant(ctx, r.pool, artifact.TenantID, func(q database.Querier) error {
		if _, err := q.Exec(ctx,
			`INSERT INTO artifacts (
				id,
				tenant_id,
				artifact_type_id,
				storage_uri,
				content_hash,
				plan_configuration_id,
				plan_execution_id,
				step_execution_id,
				status
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			artifact.ID,
			artifact.TenantID,
			artifact.ArtifactTypeID,
			artifact.StorageURI,
			artifact.ContentHash,
			nullUUIDValue(artifact.PlanConfigurationID),
			nullUUIDValue(artifact.PlanExecutionID),
			nullUUIDValue(artifact.StepExecutionID),
			status.String(),
		); err != nil {
			return err
		}

		versionID := uuid.New()
		if _, err := q.Exec(ctx,
			`INSERT INTO artifact_versions (
				id,
				artifact_id,
				tenant_id,
				version_number,
				storage_uri,
				content_hash,
				source_plan_execution_id,
				source_step_execution_id,
				created_by_kind,
				edit_summary
			)
			VALUES ($1, $2, $3, 1, $4, $5, $6, $7, $8, $9)`,
			versionID,
			artifact.ID,
			artifact.TenantID,
			artifact.StorageURI,
			artifact.ContentHash,
			nullUUIDValue(artifact.PlanExecutionID),
			nullUUIDValue(artifact.StepExecutionID),
			artifactsv1.ArtifactVersionCreatedByKind_ARTIFACT_VERSION_CREATED_BY_KIND_SYSTEM.String(),
			"Initial artifact payload",
		); err != nil {
			return err
		}

		if _, err := q.Exec(ctx,
			`UPDATE artifacts
			 SET current_version_id = $1,
			     updated_at = created_at
			 WHERE id = $2 AND tenant_id = $3`,
			versionID,
			artifact.ID,
			artifact.TenantID,
		); err != nil {
			return err
		}

		loaded, err := selectArtifactByID(ctx, q, artifact.TenantID, artifact.ID, false)
		if err != nil {
			return err
		}
		created = loaded
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create artifact: %w", err)
	}
	return created, nil
}

func (r *Repository) GetArtifact(ctx context.Context, tenantID, artifactID uuid.UUID) (*Artifact, error) {
	var artifact *Artifact
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		loaded, err := selectArtifactByID(ctx, q, tenantID, artifactID, false)
		if err != nil {
			return err
		}
		artifact = loaded
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get artifact: %w", err)
	}
	return artifact, nil
}

func (r *Repository) ListArtifacts(ctx context.Context, tenantID uuid.UUID, filter ArtifactListFilter) ([]Artifact, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	var artifacts []Artifact
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		args := []any{tenantID}
		query := artifactSelectColumns + `
			FROM artifacts a
			JOIN artifact_types t ON t.id = a.artifact_type_id
			WHERE a.tenant_id = $1`
		if filter.PlanConfigurationID != nil {
			args = append(args, *filter.PlanConfigurationID)
			query += fmt.Sprintf(" AND a.plan_configuration_id = $%d", len(args))
		}
		if filter.PlanExecutionID != nil {
			args = append(args, *filter.PlanExecutionID)
			query += fmt.Sprintf(" AND a.plan_execution_id = $%d", len(args))
		}
		if filter.ArtifactTypeKey != "" {
			args = append(args, filter.ArtifactTypeKey)
			query += fmt.Sprintf(" AND t.key = $%d", len(args))
		}
		args = append(args, filter.Limit, filter.Offset)
		query += fmt.Sprintf(" ORDER BY a.updated_at DESC, a.created_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

		rows, err := q.Query(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			artifact, err := scanArtifact(rows)
			if err != nil {
				return err
			}
			artifacts = append(artifacts, *artifact)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("list artifacts: %w", err)
	}
	return artifacts, nil
}

func (r *Repository) CreateArtifactVersion(ctx context.Context, artifact *Artifact, version *ArtifactVersion) (*ArtifactVersion, *Artifact, error) {
	if version.ID == uuid.Nil {
		version.ID = uuid.New()
	}
	if version.CreatedByKind == artifactsv1.ArtifactVersionCreatedByKind_ARTIFACT_VERSION_CREATED_BY_KIND_UNSPECIFIED {
		version.CreatedByKind = artifactsv1.ArtifactVersionCreatedByKind_ARTIFACT_VERSION_CREATED_BY_KIND_SYSTEM
	}

	var (
		created *ArtifactVersion
		updated *Artifact
	)
	err := database.WithTenant(ctx, r.pool, artifact.TenantID, func(q database.Querier) error {
		current, err := selectArtifactByID(ctx, q, artifact.TenantID, artifact.ID, true)
		if err != nil {
			return err
		}
		if current.ContentHash != artifact.ContentHash {
			return ErrContentHashConflict
		}

		var versionNumber int32
		if err := q.QueryRow(ctx,
			`SELECT COALESCE(MAX(version_number), 0) + 1
			 FROM artifact_versions
			 WHERE artifact_id = $1 AND tenant_id = $2`,
			current.ID,
			current.TenantID,
		).Scan(&versionNumber); err != nil {
			return err
		}

		row := q.QueryRow(ctx,
			`INSERT INTO artifact_versions (
				id,
				artifact_id,
				tenant_id,
				version_number,
				storage_uri,
				content_hash,
				source_plan_execution_id,
				source_step_execution_id,
				source_version_id,
				created_by_user_id,
				created_by_kind,
				edit_summary
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			RETURNING id, artifact_id, tenant_id, version_number, storage_uri, content_hash,
				source_plan_execution_id, source_step_execution_id, source_version_id,
				created_by_user_id, created_by_kind, edit_summary, created_at`,
			version.ID,
			current.ID,
			current.TenantID,
			versionNumber,
			version.StorageURI,
			version.ContentHash,
			nullUUIDValue(version.SourcePlanExecutionID),
			nullUUIDValue(version.SourceStepExecutionID),
			nullUUIDValue(version.SourceVersionID),
			nullUUIDValue(version.CreatedByUserID),
			version.CreatedByKind.String(),
			version.EditSummary,
		)
		inserted, err := scanArtifactVersion(row)
		if err != nil {
			return err
		}
		created = inserted

		if _, err := q.Exec(ctx,
			`UPDATE artifacts
			 SET storage_uri = $1,
			     content_hash = $2,
			     current_version_id = $3,
			     status = $4,
			     updated_at = now()
			 WHERE id = $5 AND tenant_id = $6`,
			created.StorageURI,
			created.ContentHash,
			created.ID,
			artifactsv1.ArtifactStatus_ARTIFACT_STATUS_EDITED.String(),
			current.ID,
			current.TenantID,
		); err != nil {
			return err
		}

		loaded, err := selectArtifactByID(ctx, q, current.TenantID, current.ID, false)
		if err != nil {
			return err
		}
		updated = loaded
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create artifact version: %w", err)
	}
	return created, updated, nil
}

func (r *Repository) ListArtifactVersions(ctx context.Context, tenantID, artifactID uuid.UUID, limit, offset int) ([]ArtifactVersion, string, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	var versions []ArtifactVersion
	nextToken := ""
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		if _, err := selectArtifactByID(ctx, q, tenantID, artifactID, false); err != nil {
			return err
		}

		rows, err := q.Query(ctx,
			`SELECT id, artifact_id, tenant_id, version_number, storage_uri, content_hash,
				source_plan_execution_id, source_step_execution_id, source_version_id,
				created_by_user_id, created_by_kind, edit_summary, created_at
			 FROM artifact_versions
			 WHERE artifact_id = $1 AND tenant_id = $2
			 ORDER BY version_number DESC
			 LIMIT $3 OFFSET $4`,
			artifactID,
			tenantID,
			limit+1,
			offset,
		)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			version, err := scanArtifactVersion(rows)
			if err != nil {
				return err
			}
			versions = append(versions, *version)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if len(versions) > limit {
			versions = versions[:limit]
			nextToken = strconv.Itoa(offset + limit)
		}
		return nil
	})
	if err != nil {
		return nil, "", fmt.Errorf("list artifact versions: %w", err)
	}
	return versions, nextToken, nil
}

func (r *Repository) GetArtifactVersion(ctx context.Context, tenantID, artifactID, versionID uuid.UUID) (*ArtifactVersion, error) {
	var version *ArtifactVersion
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`SELECT id, artifact_id, tenant_id, version_number, storage_uri, content_hash,
				source_plan_execution_id, source_step_execution_id, source_version_id,
				created_by_user_id, created_by_kind, edit_summary, created_at
			 FROM artifact_versions
			 WHERE id = $1 AND artifact_id = $2 AND tenant_id = $3`,
			versionID,
			artifactID,
			tenantID,
		)
		loaded, err := scanArtifactVersion(row)
		if err != nil {
			return err
		}
		version = loaded
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get artifact version: %w", err)
	}
	return version, nil
}

func IsNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

const artifactSelectColumns = `
	SELECT a.id,
	       a.tenant_id,
	       a.artifact_type_id,
	       t.key,
	       a.storage_uri,
	       a.content_hash,
	       a.current_version_id,
	       a.plan_configuration_id,
	       a.plan_execution_id,
	       a.step_execution_id,
	       a.status,
	       a.created_at,
	       a.updated_at
`

type rowScanner interface {
	Scan(dest ...any) error
}

func selectArtifactByID(ctx context.Context, q database.Querier, tenantID, artifactID uuid.UUID, forUpdate bool) (*Artifact, error) {
	query := artifactSelectColumns + `
		FROM artifacts a
		JOIN artifact_types t ON t.id = a.artifact_type_id
		WHERE a.id = $1 AND a.tenant_id = $2`
	if forUpdate {
		query += " FOR UPDATE OF a"
	}
	return scanArtifact(q.QueryRow(ctx, query, artifactID, tenantID))
}

func scanArtifact(row rowScanner) (*Artifact, error) {
	var (
		artifact   Artifact
		statusText string
	)
	if err := row.Scan(
		&artifact.ID,
		&artifact.TenantID,
		&artifact.ArtifactTypeID,
		&artifact.ArtifactTypeKey,
		&artifact.StorageURI,
		&artifact.ContentHash,
		&artifact.CurrentVersionID,
		&artifact.PlanConfigurationID,
		&artifact.PlanExecutionID,
		&artifact.StepExecutionID,
		&statusText,
		&artifact.CreatedAt,
		&artifact.UpdatedAt,
	); err != nil {
		return nil, err
	}
	artifact.Status = artifactStatusFromDB(statusText)
	return &artifact, nil
}

func scanArtifactVersion(row rowScanner) (*ArtifactVersion, error) {
	var (
		version  ArtifactVersion
		kindText string
	)
	if err := row.Scan(
		&version.ID,
		&version.ArtifactID,
		&version.TenantID,
		&version.VersionNumber,
		&version.StorageURI,
		&version.ContentHash,
		&version.SourcePlanExecutionID,
		&version.SourceStepExecutionID,
		&version.SourceVersionID,
		&version.CreatedByUserID,
		&kindText,
		&version.EditSummary,
		&version.CreatedAt,
	); err != nil {
		return nil, err
	}
	version.CreatedByKind = artifactVersionCreatedByKindFromDB(kindText)
	return &version, nil
}

func artifactStatusFromDB(value string) artifactsv1.ArtifactStatus {
	if number, ok := artifactsv1.ArtifactStatus_value[value]; ok {
		return artifactsv1.ArtifactStatus(number)
	}
	return artifactsv1.ArtifactStatus_ARTIFACT_STATUS_UNSPECIFIED
}

func artifactVersionCreatedByKindFromDB(value string) artifactsv1.ArtifactVersionCreatedByKind {
	if number, ok := artifactsv1.ArtifactVersionCreatedByKind_value[value]; ok {
		return artifactsv1.ArtifactVersionCreatedByKind(number)
	}
	return artifactsv1.ArtifactVersionCreatedByKind_ARTIFACT_VERSION_CREATED_BY_KIND_UNSPECIFIED
}

func nullUUIDValue(value uuid.NullUUID) any {
	if !value.Valid {
		return nil
	}
	return value.UUID
}
