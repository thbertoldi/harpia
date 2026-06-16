package executors

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/harpia/control-plane/internal/artifacts"
)

// ExecutorArtifactStore is the executors BC port for runtime artifact persistence.
// OutputArtifactTypeRef values from plan steps are stable artifact type keys
// (for example harpia.artifacts.v1.NewsList), not database UUIDs.
type ExecutorArtifactStore interface {
	LoadPayloadForType(ctx context.Context, tenantID uuid.UUID, refs []InputArtifactRef, typeKey string) ([]byte, error)
	CreateValidatedPayload(ctx context.Context, req CreateArtifactRequest) (string, error)
}

type CreateArtifactRequest struct {
	TenantID              uuid.UUID
	OutputArtifactTypeRef string
	StepExecutionID       string
	Payload               []byte
}

type artifactStore interface {
	GetTypeByID(ctx context.Context, typeID uuid.UUID) (*artifacts.ArtifactType, error)
	GetTypeByKey(ctx context.Context, key string) (*artifacts.ArtifactType, error)
	CreateArtifact(ctx context.Context, artifact *artifacts.Artifact) (*artifacts.Artifact, error)
	GetArtifact(ctx context.Context, tenantID, artifactID uuid.UUID) (*artifacts.Artifact, error)
}

type ExecutorArtifactStoreAdapter struct {
	repo  artifactStore
	store artifacts.PayloadStore
}

func NewExecutorArtifactStore(repo artifactStore, store artifacts.PayloadStore) *ExecutorArtifactStoreAdapter {
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
		actualType := strings.TrimSpace(ref.ArtifactType)
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
			return g.loadArtifactPayload(ctx, tenantID, artifactID)
		}
	}
	return nil, fmt.Errorf("missing input artifact for %q", typeKey)
}

func (g *ExecutorArtifactStoreAdapter) CreateValidatedPayload(ctx context.Context, req CreateArtifactRequest) (string, error) {
	if g == nil || g.repo == nil || g.store == nil {
		return "", fmt.Errorf("executor artifact store is not configured")
	}
	if len(req.Payload) == 0 {
		return "", fmt.Errorf("payload is required")
	}

	artifactType, err := g.resolveArtifactType(ctx, req.OutputArtifactTypeRef)
	if err != nil {
		return "", err
	}
	if err := artifacts.ValidatePayload(artifactType.Key, req.Payload); err != nil {
		return "", err
	}

	artifactID := uuid.New()
	objectPath := artifacts.ArtifactObjectPath(artifactID, req.StepExecutionID)
	storageURI, err := g.store.Put(ctx, objectPath, req.Payload)
	if err != nil {
		return "", fmt.Errorf("store artifact payload: %w", err)
	}

	created, err := g.repo.CreateArtifact(ctx, &artifacts.Artifact{
		ID:             artifactID,
		TenantID:       req.TenantID,
		ArtifactTypeID: artifactType.ID,
		StorageURI:     storageURI,
		ContentHash:    artifacts.ContentHash(req.Payload),
	})
	if err != nil {
		return "", fmt.Errorf("create artifact: %w", err)
	}
	return created.ID.String(), nil
}

func (g *ExecutorArtifactStoreAdapter) loadArtifactPayload(ctx context.Context, tenantID uuid.UUID, artifactIDRaw string) ([]byte, error) {
	artifactID, err := uuid.Parse(artifactIDRaw)
	if err != nil {
		return nil, fmt.Errorf("parse artifact id: %w", err)
	}
	artifact, err := g.repo.GetArtifact(ctx, tenantID, artifactID)
	if err != nil {
		return nil, fmt.Errorf("load artifact %q: %w", artifactIDRaw, err)
	}
	payload, err := g.store.Get(ctx, artifact.StorageURI)
	if err != nil {
		return nil, fmt.Errorf("load artifact payload %q: %w", artifactIDRaw, err)
	}
	return payload, nil
}

func (g *ExecutorArtifactStoreAdapter) resolveArtifactType(ctx context.Context, artifactTypeRef string) (*artifacts.ArtifactType, error) {
	ref := strings.TrimSpace(artifactTypeRef)
	if ref == "" {
		return nil, fmt.Errorf("output artifact type ref is required")
	}

	if typeID, err := uuid.Parse(ref); err == nil {
		artifactType, lookupErr := g.repo.GetTypeByID(ctx, typeID)
		if lookupErr != nil {
			return nil, fmt.Errorf("load artifact type id %q: %w", ref, lookupErr)
		}
		return artifactType, nil
	}

	artifactType, err := g.repo.GetTypeByKey(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("load artifact type key %q: %w", ref, err)
	}
	return artifactType, nil
}
