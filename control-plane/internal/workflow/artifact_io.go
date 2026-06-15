package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
	"github.com/harpia/control-plane/internal/artifacts"
)

type ArtifactRepository interface {
	GetTypeByID(ctx context.Context, typeID uuid.UUID) (*artifacts.ArtifactType, error)
	GetTypeByKey(ctx context.Context, key string) (*artifacts.ArtifactType, error)
	CreateArtifact(ctx context.Context, artifact *artifacts.Artifact) (*artifacts.Artifact, error)
	GetArtifact(ctx context.Context, tenantID, artifactID uuid.UUID) (*artifacts.Artifact, error)
}

type ArtifactIO struct {
	repo  ArtifactRepository
	store artifacts.PayloadStore
}

func NewArtifactIO(repo ArtifactRepository, store artifacts.PayloadStore) *ArtifactIO {
	return &ArtifactIO{repo: repo, store: store}
}

func (a *ArtifactIO) LoadDateRangeInput(ctx context.Context, tenantID uuid.UUID, refs []ArtifactRef) (*artifactsv1.DateRange, error) {
	if a == nil || a.repo == nil || a.store == nil {
		return nil, fmt.Errorf("artifact io is not configured")
	}

	payload, err := a.loadInputPayload(ctx, tenantID, refs, artifacts.TypeKeyDateRange)
	if err != nil {
		return nil, err
	}

	dateRange := &artifactsv1.DateRange{}
	if err := protojson.Unmarshal(payload, dateRange); err != nil {
		return nil, fmt.Errorf("parse date range artifact: %w", err)
	}
	if err := artifacts.ValidatePayload(artifacts.TypeKeyDateRange, payload); err != nil {
		return nil, err
	}
	return dateRange, nil
}

func (a *ArtifactIO) CreateNewsListArtifact(
	ctx context.Context,
	tenantID uuid.UUID,
	artifactTypeID string,
	stepExecutionID string,
	newsList *artifactsv1.NewsList,
) (string, error) {
	if a == nil || a.repo == nil || a.store == nil {
		return "", fmt.Errorf("artifact io is not configured")
	}
	if newsList == nil {
		return "", fmt.Errorf("news list is required")
	}

	payload, err := protojson.Marshal(newsList)
	if err != nil {
		return "", fmt.Errorf("marshal news list: %w", err)
	}
	if err := artifacts.ValidatePayload(artifacts.TypeKeyNewsList, payload); err != nil {
		return "", err
	}

	artifactType, err := a.resolveArtifactType(ctx, artifactTypeID, artifacts.TypeKeyNewsList)
	if err != nil {
		return "", err
	}

	artifactID := uuid.New()
	objectPath := artifacts.ArtifactObjectPath(artifactID, stepExecutionID)
	storageURI, err := a.store.Put(ctx, objectPath, payload)
	if err != nil {
		return "", fmt.Errorf("store news list payload: %w", err)
	}

	created, err := a.repo.CreateArtifact(ctx, &artifacts.Artifact{
		ID:             artifactID,
		TenantID:       tenantID,
		ArtifactTypeID: artifactType.ID,
		StorageURI:     storageURI,
		ContentHash:    artifacts.ContentHash(payload),
	})
	if err != nil {
		return "", fmt.Errorf("create news list artifact: %w", err)
	}
	return created.ID.String(), nil
}

func (a *ArtifactIO) loadInputPayload(ctx context.Context, tenantID uuid.UUID, refs []ArtifactRef, expectedType string) ([]byte, error) {
	for _, ref := range refs {
		actualType := strings.TrimSpace(ref.ArtifactType)
		if actualType != "" && actualType != expectedType {
			continue
		}
		if literal := strings.TrimSpace(ref.LiteralJSON); literal != "" {
			payload := []byte(literal)
			if !json.Valid(payload) {
				return nil, fmt.Errorf("literal input for %q is not valid JSON", expectedType)
			}
			return payload, nil
		}
		if artifactID := strings.TrimSpace(ref.ArtifactID); artifactID != "" {
			return a.loadArtifactPayload(ctx, tenantID, artifactID)
		}
	}
	return nil, fmt.Errorf("missing input artifact for %q", expectedType)
}

func (a *ArtifactIO) loadArtifactPayload(ctx context.Context, tenantID uuid.UUID, artifactIDRaw string) ([]byte, error) {
	artifactID, err := uuid.Parse(artifactIDRaw)
	if err != nil {
		return nil, fmt.Errorf("parse artifact id: %w", err)
	}
	artifact, err := a.repo.GetArtifact(ctx, tenantID, artifactID)
	if err != nil {
		return nil, fmt.Errorf("load artifact %q: %w", artifactIDRaw, err)
	}
	payload, err := a.store.Get(ctx, artifact.StorageURI)
	if err != nil {
		return nil, fmt.Errorf("load artifact payload %q: %w", artifactIDRaw, err)
	}
	return payload, nil
}

func (a *ArtifactIO) resolveArtifactType(ctx context.Context, artifactTypeID, artifactTypeKey string) (*artifacts.ArtifactType, error) {
	if trimmed := strings.TrimSpace(artifactTypeID); trimmed != "" {
		typeID, err := uuid.Parse(trimmed)
		if err != nil {
			return nil, fmt.Errorf("parse artifact type id: %w", err)
		}
		artifactType, err := a.repo.GetTypeByID(ctx, typeID)
		if err != nil {
			return nil, fmt.Errorf("load artifact type %q: %w", trimmed, err)
		}
		return artifactType, nil
	}
	if trimmed := strings.TrimSpace(artifactTypeKey); trimmed != "" {
		artifactType, err := a.repo.GetTypeByKey(ctx, trimmed)
		if err != nil {
			return nil, fmt.Errorf("load artifact type key %q: %w", trimmed, err)
		}
		return artifactType, nil
	}
	return nil, fmt.Errorf("artifact type id or key is required")
}
