package executors_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
	"github.com/harpia/control-plane/internal/artifacts"
	"github.com/harpia/control-plane/internal/executors"
)

type memoryArtifactRepo struct {
	types     map[string]*artifacts.ArtifactType
	artifacts map[uuid.UUID]*artifacts.Artifact
}

func (m *memoryArtifactRepo) GetTypeByID(_ context.Context, typeID uuid.UUID) (*artifacts.ArtifactType, error) {
	for _, artifactType := range m.types {
		if artifactType.ID == typeID {
			return artifactType, nil
		}
	}
	return nil, context.Canceled
}

func (m *memoryArtifactRepo) GetTypeByKey(_ context.Context, key string) (*artifacts.ArtifactType, error) {
	artifactType, ok := m.types[key]
	if !ok {
		return nil, context.Canceled
	}
	return artifactType, nil
}

func (m *memoryArtifactRepo) CreateArtifact(_ context.Context, artifact *artifacts.Artifact) (*artifacts.Artifact, error) {
	if m.artifacts == nil {
		m.artifacts = make(map[uuid.UUID]*artifacts.Artifact)
	}
	created := *artifact
	if created.ID == uuid.Nil {
		created.ID = uuid.New()
	}
	if created.CreatedAt.IsZero() {
		created.CreatedAt = time.Now().UTC()
	}
	m.artifacts[created.ID] = &created
	return &created, nil
}

func (m *memoryArtifactRepo) GetArtifact(_ context.Context, tenantID, artifactID uuid.UUID) (*artifacts.Artifact, error) {
	artifact, ok := m.artifacts[artifactID]
	if !ok || artifact.TenantID != tenantID {
		return nil, context.Canceled
	}
	return artifact, nil
}

type memoryPayloadStore struct {
	objects map[string][]byte
}

func (m *memoryPayloadStore) Put(_ context.Context, objectPath string, payload []byte) (string, error) {
	if m.objects == nil {
		m.objects = make(map[string][]byte)
	}
	uri := "s3://harpia/tenant/test/" + objectPath
	m.objects[uri] = append([]byte(nil), payload...)
	return uri, nil
}

func (m *memoryPayloadStore) Get(_ context.Context, storageURI string) ([]byte, error) {
	payload, ok := m.objects[storageURI]
	if !ok {
		return nil, context.Canceled
	}
	return append([]byte(nil), payload...), nil
}

func TestExecutorArtifactStoreCreateValidatedPayloadAcceptsTypeKeyRef(t *testing.T) {
	typeID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	repo := &memoryArtifactRepo{
		types: map[string]*artifacts.ArtifactType{
			artifacts.TypeKeyNewsList: {
				ID:  typeID,
				Key: artifacts.TypeKeyNewsList,
			},
		},
	}
	store := &memoryPayloadStore{}
	artifactStore := executors.NewExecutorArtifactStore(repo, store)

	newsList := &artifactsv1.NewsList{
		Articles: []*artifactsv1.NewsArticle{{
			Title: "Story",
			Url:   "https://example.com/story",
		}},
	}
	payload, err := protojson.Marshal(newsList)
	if err != nil {
		t.Fatalf("marshal news list: %v", err)
	}

	artifactID, err := artifactStore.CreateValidatedPayload(context.Background(), executors.CreateArtifactRequest{
		TenantID:              uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		OutputArtifactTypeRef: artifacts.TypeKeyNewsList,
		StepExecutionID:       "step-fetch-news",
		Payload:               payload,
	})
	if err != nil {
		t.Fatalf("CreateValidatedPayload() error = %v", err)
	}
	if artifactID == "" {
		t.Fatal("expected artifact id")
	}
	if len(store.objects) != 1 {
		t.Fatalf("stored payloads = %d, want 1", len(store.objects))
	}
	for _, stored := range store.objects {
		if err := artifacts.ValidatePayload(artifacts.TypeKeyNewsList, stored); err != nil {
			t.Fatalf("stored payload failed schema validation: %v", err)
		}
	}
}
