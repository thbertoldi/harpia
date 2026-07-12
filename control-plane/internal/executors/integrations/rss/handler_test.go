package rss_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mmcdole/gofeed"

	"github.com/harpia/control-plane/internal/artifacts"
	"github.com/harpia/control-plane/internal/executors"
	"github.com/harpia/control-plane/internal/executors/integrations/rss"
	"github.com/harpia/control-plane/internal/executors/runtime"
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

func (m *memoryArtifactRepo) GetArtifactVersion(_ context.Context, _, _, _ uuid.UUID) (*artifacts.ArtifactVersion, error) {
	return nil, context.Canceled
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

func newTestArtifactStore() runtime.ExecutorArtifactStore {
	typeID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	return runtime.NewExecutorArtifactStore(&memoryArtifactRepo{
		types: map[string]*artifacts.ArtifactType{
			artifacts.TypeKeyNewsList: {ID: typeID, Key: artifacts.TypeKeyNewsList},
		},
	}, &memoryPayloadStore{})
}

type stubFeedFetcher struct {
	feed *gofeed.Feed
	err  error
}

func (s stubFeedFetcher) Fetch(_ context.Context, _ string) (*gofeed.Feed, error) {
	if s.err != nil {
		return nil, rss.NewFeedFetchError("https://example.com/rss", s.err)
	}
	return s.feed, nil
}

func TestHandlerExecuteSuccessWithPlanTemplateTypeKey(t *testing.T) {
	store := newTestArtifactStore()
	handler := rss.NewHandler(store, stubFeedFetcher{feed: &gofeed.Feed{
		Title: "Example News",
		Items: []*gofeed.Item{{
			Title:           "Story",
			Link:            "https://example.com/story",
			Description:     "Summary",
			PublishedParsed: timePtr(time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)),
		}},
	}})

	result, err := handler.Execute(context.Background(), runtime.IntegrationExecutionRequest{
		TenantID:              uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		StepExecutionID:       "step-fetch-news",
		OutputArtifactTypeKey: artifacts.TypeKeyNewsList,
		InputArtifacts: []runtime.InputArtifactRef{{
			ArtifactTypeKey: artifacts.TypeKeyDateRange,
			LiteralJSON:     `{"startDate":"2026-01-01","endDate":"2026-01-07"}`,
		}},
		Installation: runtime.InstallationSnapshot{
			ExecutorSKUKey: executors.SKURSSNewsFeed,
			ConfigJSON:     json.RawMessage(`{"feeds":["https://example.com/rss"]}`),
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Status != runtime.IntegrationStatusCompleted {
		t.Fatalf("Status = %q, want completed (%s)", result.Status, result.Error)
	}
	if result.OutputArtifactID == "" {
		t.Fatal("expected output artifact id")
	}
}

func TestHandlerExecuteInvalidConfig(t *testing.T) {
	handler := rss.NewHandler(newTestArtifactStore(), stubFeedFetcher{})
	result, err := handler.Execute(context.Background(), runtime.IntegrationExecutionRequest{
		TenantID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		InputArtifacts: []runtime.InputArtifactRef{{
			ArtifactTypeKey: artifacts.TypeKeyDateRange,
			LiteralJSON:     `{"startDate":"2026-01-01","endDate":"2026-01-07"}`,
		}},
		Installation: runtime.InstallationSnapshot{
			ConfigJSON: json.RawMessage(`{}`),
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Status != runtime.IntegrationStatusFailed {
		t.Fatalf("Status = %q, want failed", result.Status)
	}
}

func TestHandlerExecuteDeadFeedReturnsRetryableError(t *testing.T) {
	handler := rss.NewHandler(newTestArtifactStore(), stubFeedFetcher{err: errors.New("connection refused")})
	_, err := handler.Execute(context.Background(), executors.IntegrationExecutionRequest{
		TenantID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		InputArtifacts: []runtime.InputArtifactRef{{
			ArtifactTypeKey: artifacts.TypeKeyDateRange,
			LiteralJSON:     `{"startDate":"2026-01-01","endDate":"2026-01-07"}`,
		}},
		Installation: runtime.InstallationSnapshot{
			ConfigJSON: json.RawMessage(`{"feeds":["https://example.com/rss"]}`),
		},
	})
	if err == nil {
		t.Fatal("expected retryable error")
	}
	var retryable *runtime.RetryableError
	if !errors.As(err, &retryable) {
		t.Fatalf("error = %T(%v), want *runtime.RetryableError", err, err)
	}
}

func TestIntegrationRegistryRoutesBySKUKey(t *testing.T) {
	registry := runtime.NewIntegrationRegistry(rss.NewHandler(newTestArtifactStore(), stubFeedFetcher{}))
	result, err := registry.Run(context.Background(), runtime.IntegrationExecutionRequest{
		Installation: runtime.InstallationSnapshot{ExecutorSKUKey: executors.SKULinkedInPublish},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != runtime.IntegrationStatusFailed {
		t.Fatalf("Status = %q, want failed", result.Status)
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}
