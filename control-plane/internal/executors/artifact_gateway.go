package executors

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/harpia/control-plane/internal/artifacts"
)

// ArtifactGateway is the executors BC port for typed artifact persistence.
type ArtifactGateway interface {
	LoadPayloadForType(ctx context.Context, tenantID uuid.UUID, refs []InputArtifactRef, typeKey string) ([]byte, error)
	CreateValidatedPayload(ctx context.Context, req CreateArtifactRequest) (string, error)
}

type CreateArtifactRequest struct {
	TenantID        uuid.UUID
	ArtifactTypeID  string
	ArtifactTypeKey string
	StepExecutionID string
	Payload         []byte
}

type artifactStore interface {
	GetTypeByID(ctx context.Context, typeID uuid.UUID) (*artifacts.ArtifactType, error)
	GetTypeByKey(ctx context.Context, key string) (*artifacts.ArtifactType, error)
	CreateArtifact(ctx context.Context, artifact *artifacts.Artifact) (*artifacts.Artifact, error)
	GetArtifact(ctx context.Context, tenantID, artifactID uuid.UUID) (*artifacts.Artifact, error)
}

type ArtifactGatewayAdapter struct {
	repo  artifactStore
	store artifacts.PayloadStore
}

func NewArtifactGateway(repo artifactStore, store artifacts.PayloadStore) *ArtifactGatewayAdapter {
	return &ArtifactGatewayAdapter{repo: repo, store: store}
}

func (g *ArtifactGatewayAdapter) LoadPayloadForType(
	ctx context.Context,
	tenantID uuid.UUID,
	refs []InputArtifactRef,
	typeKey string,
) ([]byte, error) {
	if g == nil || g.repo == nil || g.store == nil {
		return nil, fmt.Errorf("artifact gateway is not configured")
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

func (g *ArtifactGatewayAdapter) CreateValidatedPayload(ctx context.Context, req CreateArtifactRequest) (string, error) {
	if g == nil || g.repo == nil || g.store == nil {
		return "", fmt.Errorf("artifact gateway is not configured")
	}
	if len(req.Payload) == 0 {
		return "", fmt.Errorf("payload is required")
	}
	if err := artifacts.ValidatePayload(req.ArtifactTypeKey, req.Payload); err != nil {
		return "", err
	}

	artifactType, err := g.resolveArtifactType(ctx, req.ArtifactTypeID, req.ArtifactTypeKey)
	if err != nil {
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

func (g *ArtifactGatewayAdapter) loadArtifactPayload(ctx context.Context, tenantID uuid.UUID, artifactIDRaw string) ([]byte, error) {
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

func (g *ArtifactGatewayAdapter) resolveArtifactType(ctx context.Context, artifactTypeID, artifactTypeKey string) (*artifacts.ArtifactType, error) {
	if trimmed := strings.TrimSpace(artifactTypeID); trimmed != "" {
		typeID, err := uuid.Parse(trimmed)
		if err != nil {
			return nil, fmt.Errorf("parse artifact type id: %w", err)
		}
		artifactType, err := g.repo.GetTypeByID(ctx, typeID)
		if err != nil {
			return nil, fmt.Errorf("load artifact type %q: %w", trimmed, err)
		}
		return artifactType, nil
	}
	if trimmed := strings.TrimSpace(artifactTypeKey); trimmed != "" {
		artifactType, err := g.repo.GetTypeByKey(ctx, trimmed)
		if err != nil {
			return nil, fmt.Errorf("load artifact type key %q: %w", trimmed, err)
		}
		return artifactType, nil
	}
	return nil, fmt.Errorf("artifact type id or key is required")
}
