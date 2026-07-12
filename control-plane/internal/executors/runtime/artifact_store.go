package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
	"github.com/harpia/control-plane/internal/artifacts"
)

// ExecutorArtifactStore is the executors BC port for runtime artifact persistence.
// OutputArtifactTypeKey values from plan steps are stable artifact type keys
// (for example harpia.artifacts.v1.NewsList), not database UUIDs.
type ExecutorArtifactStore interface {
	LoadPayloadForType(ctx context.Context, tenantID uuid.UUID, refs []InputArtifactRef, typeKey string) ([]byte, error)
	LoadPinnedPayload(ctx context.Context, tenantID uuid.UUID, ref VersionedArtifactRef) ([]byte, error)
	CreateValidatedPayload(ctx context.Context, req CreateArtifactRequest) (string, error)
	CreateValidatedPayloadRef(ctx context.Context, req CreateArtifactRequest) (VersionedArtifactRef, error)
	CreateValidatedVersionPayload(ctx context.Context, req CreateArtifactVersionRequest) (VersionedArtifactRef, error)
}

// VersionedArtifactRef is the executor-facing form of an immutable artifact
// reference. The adapter verifies tenant, type, version, and hash before it
// returns bytes to an integration executor.
type VersionedArtifactRef struct {
	ArtifactID        string
	ArtifactVersionID string
	ArtifactTypeKey   string
	ContentHash       string
}

type CreateArtifactRequest struct {
	TenantID              uuid.UUID
	OutputArtifactTypeKey string
	PlanExecutionID       string
	StepExecutionID       string
	Payload               []byte
}

// CreateArtifactVersionRequest appends a candidate version to an existing
// typed Artifact. The expected content hash prevents a stale enrichment from
// silently replacing a newer post version.
type CreateArtifactVersionRequest struct {
	TenantID                uuid.UUID
	ArtifactID              string
	ExpectedContentHash     string
	SourceArtifactVersionID string
	OutputArtifactTypeKey   string
	PlanExecutionID         string
	StepExecutionID         string
	EditSummary             string
	Payload                 []byte
}

// ArtifactRepository persists artifact metadata for executor runtime access.
type ArtifactRepository interface {
	GetTypeByID(ctx context.Context, typeID uuid.UUID) (*artifacts.ArtifactType, error)
	GetTypeByKey(ctx context.Context, key string) (*artifacts.ArtifactType, error)
	CreateArtifact(ctx context.Context, artifact *artifacts.Artifact) (*artifacts.Artifact, error)
	CreateArtifactVersion(ctx context.Context, artifact *artifacts.Artifact, version *artifacts.ArtifactVersion) (*artifacts.ArtifactVersion, *artifacts.Artifact, error)
	GetArtifact(ctx context.Context, tenantID, artifactID uuid.UUID) (*artifacts.Artifact, error)
	GetArtifactVersion(ctx context.Context, tenantID, artifactID, versionID uuid.UUID) (*artifacts.ArtifactVersion, error)
}

type ExecutorArtifactStoreAdapter struct {
	repo  ArtifactRepository
	store artifacts.PayloadStore
}

func NewExecutorArtifactStore(repo ArtifactRepository, store artifacts.PayloadStore) *ExecutorArtifactStoreAdapter {
	return &ExecutorArtifactStoreAdapter{repo: repo, store: store}
}

func (g *ExecutorArtifactStoreAdapter) LoadPayloadForType(
	ctx context.Context,
	tenantID uuid.UUID,
	refs []InputArtifactRef,
	typeKey string,
) ([]byte, error) {
	if g == nil || g.repo == nil || g.store == nil {
		return nil, fmt.Errorf("executor artifact store is not configured")
	}

	for _, ref := range refs {
		actualType := strings.TrimSpace(ref.ArtifactTypeKey)
		if actualType != "" && actualType != typeKey {
			continue
		}
		if literal := strings.TrimSpace(ref.LiteralJSON); literal != "" {
			payload := []byte(literal)
			if !json.Valid(payload) {
				return nil, fmt.Errorf("literal input for %q is not valid JSON", typeKey)
			}
			return payload, nil
		}
		if artifactID := strings.TrimSpace(ref.ArtifactID); artifactID != "" {
			return g.LoadPinnedPayload(ctx, tenantID, VersionedArtifactRef{
				ArtifactID:        artifactID,
				ArtifactVersionID: ref.ArtifactVersionID,
				ArtifactTypeKey:   firstNonEmpty(actualType, typeKey),
				ContentHash:       ref.ContentHash,
			})
		}
	}
	return nil, fmt.Errorf("missing input artifact for %q", typeKey)
}

func (g *ExecutorArtifactStoreAdapter) CreateValidatedPayload(ctx context.Context, req CreateArtifactRequest) (string, error) {
	ref, err := g.CreateValidatedPayloadRef(ctx, req)
	if err != nil {
		return "", err
	}
	return ref.ArtifactID, nil
}

func (g *ExecutorArtifactStoreAdapter) CreateValidatedPayloadRef(ctx context.Context, req CreateArtifactRequest) (VersionedArtifactRef, error) {
	if g == nil || g.repo == nil || g.store == nil {
		return VersionedArtifactRef{}, fmt.Errorf("executor artifact store is not configured")
	}
	if len(req.Payload) == 0 {
		return VersionedArtifactRef{}, fmt.Errorf("payload is required")
	}

	artifactType, err := g.resolveArtifactType(ctx, req.OutputArtifactTypeKey)
	if err != nil {
		return VersionedArtifactRef{}, err
	}
	if err := artifacts.ValidatePayload(artifactType.Key, req.Payload); err != nil {
		return VersionedArtifactRef{}, err
	}

	artifactID := uuid.New()
	objectPath := artifacts.ArtifactObjectPath(artifactID, req.StepExecutionID)
	storageURI, err := g.store.Put(ctx, objectPath, req.Payload)
	if err != nil {
		return VersionedArtifactRef{}, fmt.Errorf("store artifact payload: %w", err)
	}

	created, err := g.repo.CreateArtifact(ctx, &artifacts.Artifact{
		ID:              artifactID,
		TenantID:        req.TenantID,
		ArtifactTypeID:  artifactType.ID,
		ArtifactTypeKey: artifactType.Key,
		StorageURI:      storageURI,
		ContentHash:     artifacts.ContentHash(req.Payload),
		PlanExecutionID: nullableUUID(req.PlanExecutionID),
		StepExecutionID: nullableUUID(req.StepExecutionID),
	})
	if err != nil {
		return VersionedArtifactRef{}, fmt.Errorf("create artifact: %w", err)
	}
	ref := VersionedArtifactRef{
		ArtifactID:      created.ID.String(),
		ArtifactTypeKey: created.ArtifactTypeKey,
		ContentHash:     created.ContentHash,
	}
	if created.CurrentVersionID.Valid {
		ref.ArtifactVersionID = created.CurrentVersionID.UUID.String()
	}
	return ref, nil
}

func (g *ExecutorArtifactStoreAdapter) CreateValidatedVersionPayload(ctx context.Context, req CreateArtifactVersionRequest) (VersionedArtifactRef, error) {
	if g == nil || g.repo == nil || g.store == nil {
		return VersionedArtifactRef{}, fmt.Errorf("executor artifact store is not configured")
	}
	if len(req.Payload) == 0 {
		return VersionedArtifactRef{}, fmt.Errorf("payload is required")
	}
	artifactID, err := uuid.Parse(strings.TrimSpace(req.ArtifactID))
	if err != nil {
		return VersionedArtifactRef{}, fmt.Errorf("parse artifact id: %w", err)
	}
	artifact, err := g.repo.GetArtifact(ctx, req.TenantID, artifactID)
	if err != nil {
		return VersionedArtifactRef{}, fmt.Errorf("load artifact %q: %w", req.ArtifactID, err)
	}
	if expected := strings.TrimSpace(req.ExpectedContentHash); expected == "" || expected != artifact.ContentHash {
		return VersionedArtifactRef{}, fmt.Errorf("artifact %q content hash does not match expected current version", req.ArtifactID)
	}
	artifactType, err := g.repo.GetTypeByID(ctx, artifact.ArtifactTypeID)
	if err != nil {
		return VersionedArtifactRef{}, fmt.Errorf("load artifact type for %q: %w", req.ArtifactID, err)
	}
	if requestedType := strings.TrimSpace(req.OutputArtifactTypeKey); requestedType != "" && requestedType != artifactType.Key {
		return VersionedArtifactRef{}, fmt.Errorf("artifact %q type = %q, want %q", req.ArtifactID, artifactType.Key, requestedType)
	}
	if err := artifacts.ValidatePayload(artifactType.Key, req.Payload); err != nil {
		return VersionedArtifactRef{}, err
	}
	storageURI, err := g.store.Put(ctx, artifacts.ArtifactObjectPath(artifact.ID, req.StepExecutionID+"-"+uuid.NewString()), req.Payload)
	if err != nil {
		return VersionedArtifactRef{}, fmt.Errorf("store artifact version payload: %w", err)
	}
	version, updated, err := g.repo.CreateArtifactVersion(ctx, artifact, &artifacts.ArtifactVersion{
		TenantID:              req.TenantID,
		StorageURI:            storageURI,
		ContentHash:           artifacts.ContentHash(req.Payload),
		SourcePlanExecutionID: nullableUUID(req.PlanExecutionID),
		SourceStepExecutionID: nullableUUID(req.StepExecutionID),
		SourceVersionID:       nullableUUID(req.SourceArtifactVersionID),
		CreatedByKind:         artifactsv1.ArtifactVersionCreatedByKind_ARTIFACT_VERSION_CREATED_BY_KIND_INTEGRATION,
		EditSummary:           req.EditSummary,
	})
	if err != nil {
		return VersionedArtifactRef{}, fmt.Errorf("create artifact version: %w", err)
	}
	return VersionedArtifactRef{
		ArtifactID:        updated.ID.String(),
		ArtifactVersionID: version.ID.String(),
		ArtifactTypeKey:   updated.ArtifactTypeKey,
		ContentHash:       version.ContentHash,
	}, nil
}

func nullableUUID(raw string) uuid.NullUUID {
	parsed, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: parsed, Valid: true}
}

func (g *ExecutorArtifactStoreAdapter) LoadPinnedPayload(ctx context.Context, tenantID uuid.UUID, ref VersionedArtifactRef) ([]byte, error) {
	artifactIDRaw := strings.TrimSpace(ref.ArtifactID)
	artifactID, err := uuid.Parse(artifactIDRaw)
	if err != nil {
		return nil, fmt.Errorf("parse artifact id: %w", err)
	}
	artifact, err := g.repo.GetArtifact(ctx, tenantID, artifactID)
	if err != nil {
		return nil, fmt.Errorf("load artifact %q: %w", artifactIDRaw, err)
	}
	artifactType, err := g.repo.GetTypeByID(ctx, artifact.ArtifactTypeID)
	if err != nil {
		return nil, fmt.Errorf("load artifact type for %q: %w", artifactIDRaw, err)
	}
	if wantType := strings.TrimSpace(ref.ArtifactTypeKey); wantType != "" && artifactType.Key != wantType {
		return nil, fmt.Errorf("artifact %q type = %q, want %q", artifactIDRaw, artifactType.Key, wantType)
	}

	storageURI := artifact.StorageURI
	contentHash := artifact.ContentHash
	if versionRaw := strings.TrimSpace(ref.ArtifactVersionID); versionRaw != "" {
		versionID, parseErr := uuid.Parse(versionRaw)
		if parseErr != nil {
			return nil, fmt.Errorf("parse artifact version id: %w", parseErr)
		}
		version, loadErr := g.repo.GetArtifactVersion(ctx, tenantID, artifactID, versionID)
		if loadErr != nil {
			return nil, fmt.Errorf("load artifact version %q: %w", versionRaw, loadErr)
		}
		storageURI = version.StorageURI
		contentHash = version.ContentHash
	}
	if expectedHash := strings.TrimSpace(ref.ContentHash); expectedHash != "" && expectedHash != contentHash {
		return nil, fmt.Errorf("artifact %q content hash does not match pinned reference", artifactIDRaw)
	}
	payload, err := g.store.Get(ctx, storageURI)
	if err != nil {
		return nil, fmt.Errorf("load artifact payload %q: %w", artifactIDRaw, err)
	}
	return payload, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (g *ExecutorArtifactStoreAdapter) resolveArtifactType(ctx context.Context, artifactTypeKey string) (*artifacts.ArtifactType, error) {
	key := strings.TrimSpace(artifactTypeKey)
	if key == "" {
		return nil, fmt.Errorf("output artifact type key is required")
	}

	if typeID, err := uuid.Parse(key); err == nil {
		artifactType, lookupErr := g.repo.GetTypeByID(ctx, typeID)
		if lookupErr != nil {
			return nil, fmt.Errorf("load artifact type id %q: %w", key, lookupErr)
		}
		return artifactType, nil
	}

	artifactType, err := g.repo.GetTypeByKey(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("load artifact type key %q: %w", key, err)
	}
	return artifactType, nil
}
