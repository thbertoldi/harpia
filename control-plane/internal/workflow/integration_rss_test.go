package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mmcdole/gofeed"
	"go.temporal.io/sdk/temporal"

	"github.com/harpia/control-plane/internal/artifacts"
	"github.com/harpia/control-plane/internal/integrations/rss"
)

type integrationArtifactRepo struct {
	types     map[string]*artifacts.ArtifactType
	artifacts map[uuid.UUID]*artifacts.Artifact
}

func (m *integrationArtifactRepo) GetTypeByID(_ context.Context, typeID uuid.UUID) (*artifacts.ArtifactType, error) {
	for _, artifactType := range m.types {
		if artifactType.ID == typeID {
			return artifactType, nil
		}
	}
	return nil, context.Canceled
}

func (m *integrationArtifactRepo) GetTypeByKey(_ context.Context, key string) (*artifacts.ArtifactType, error) {
	artifactType, ok := m.types[key]
	if !ok {
		return nil, context.Canceled
	}
	return artifactType, nil
}

func (m *integrationArtifactRepo) CreateArtifact(_ context.Context, artifact *artifacts.Artifact) (*artifacts.Artifact, error) {
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

func (m *integrationArtifactRepo) GetArtifact(_ context.Context, tenantID, artifactID uuid.UUID) (*artifacts.Artifact, error) {
	artifact, ok := m.artifacts[artifactID]
	if !ok || artifact.TenantID != tenantID {
		return nil, context.Canceled
	}
	return artifact, nil
}

type integrationPayloadStore struct {
	objects map[string][]byte
}

func (m *integrationPayloadStore) Put(_ context.Context, objectPath string, payload []byte) (string, error) {
	if m.objects == nil {
		m.objects = make(map[string][]byte)
	}
	uri := "s3://harpia/tenant/test/" + objectPath
	m.objects[uri] = append([]byte(nil), payload...)
	return uri, nil
}

func (m *integrationPayloadStore) Get(_ context.Context, storageURI string) ([]byte, error) {
	payload, ok := m.objects[storageURI]
	if !ok {
		return nil, context.Canceled
	}
	return append([]byte(nil), payload...), nil
}

type stubFeedFetcher struct {
	feed *gofeed.Feed
	err  error
}

func (s stubFeedFetcher) Fetch(_ context.Context, _ string) (*gofeed.Feed, error) {
	if s.err != nil {
		return s.errFeed()
	}
	return s.feed, nil
}

func (s stubFeedFetcher) errFeed() (*gofeed.Feed, error) {
	return nil, rss.NewFeedFetchError("https://example.com/rss", s.err)
}

func TestRunIntegrationActivityFetchNewsSuccess(t *testing.T) {
	tenantID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	typeID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	store := &integrationPayloadStore{}
	repo := &integrationArtifactRepo{
		types: map[string]*artifacts.ArtifactType{
			artifacts.TypeKeyNewsList: {
				ID:  typeID,
				Key: artifacts.TypeKeyNewsList,
			},
		},
	}
	activities := &PlanActivities{
		Artifacts: NewArtifactIO(repo, store),
		FeedFetcher: stubFeedFetcher{feed: &gofeed.Feed{
			Title: "Example News",
			Items: []*gofeed.Item{{
				Title:           "Story",
				Link:            "https://example.com/story",
				Description:     "Summary",
				PublishedParsed: timePtr(time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)),
			}},
		}},
	}

	result, err := activities.RunIntegrationActivity(context.Background(), ExecutorActivityInput{
		TenantID:             tenantID.String(),
		StepExecutionID:      "step-fetch-news",
		PlanStepKey:          "fetch-news",
		OutputArtifactTypeID: typeID.String(),
		InputArtifacts: []ArtifactRef{{
			ArtifactType: artifacts.TypeKeyDateRange,
			LiteralJSON:  `{"startDate":"2026-01-01","endDate":"2026-01-07"}`,
		}},
		ExecutorInstallationSnapshot: ExecutorInstallationSnapshot{
			ConfigJSON:     `{"feeds":["https://example.com/rss"]}`,
			ExecutorSKUKey: "rss-news-feed",
		},
	})
	if err != nil {
		t.Fatalf("RunIntegrationActivity() error = %v", err)
	}
	if result.Status != ExecutorResultStatusCompleted {
		t.Fatalf("Status = %q, want completed (%s)", result.Status, result.Error)
	}
	if result.OutputArtifactID == "" {
		t.Fatal("expected output artifact id")
	}
	if len(store.objects) != 1 {
		t.Fatalf("stored payloads = %d, want 1", len(store.objects))
	}
	for _, payload := range store.objects {
		if err := artifacts.ValidatePayload(artifacts.TypeKeyNewsList, payload); err != nil {
			t.Fatalf("stored payload failed schema validation: %v", err)
		}
	}
}

func TestRunIntegrationActivityFetchNewsInvalidConfig(t *testing.T) {
	activities := &PlanActivities{
		Artifacts:   NewArtifactIO(&integrationArtifactRepo{}, &integrationPayloadStore{}),
		FeedFetcher: stubFeedFetcher{},
	}
	result, err := activities.RunIntegrationActivity(context.Background(), ExecutorActivityInput{
		TenantID:        "22222222-2222-2222-2222-222222222222",
		PlanStepKey:     "fetch-news",
		StepExecutionID: "step-fetch-news",
		InputArtifacts: []ArtifactRef{{
			ArtifactType: artifacts.TypeKeyDateRange,
			LiteralJSON:  `{"startDate":"2026-01-01","endDate":"2026-01-07"}`,
		}},
		ExecutorInstallationSnapshot: ExecutorInstallationSnapshot{
			ConfigJSON: `{}`,
		},
	})
	if err != nil {
		t.Fatalf("RunIntegrationActivity() error = %v", err)
	}
	if result.Status != ExecutorResultStatusFailed {
		t.Fatalf("Status = %q, want failed", result.Status)
	}
}

func TestRunIntegrationActivityFetchNewsDeadFeedRetries(t *testing.T) {
	activities := &PlanActivities{
		Artifacts:   NewArtifactIO(&integrationArtifactRepo{}, &integrationPayloadStore{}),
		FeedFetcher: stubFeedFetcher{err: errors.New("connection refused")},
	}
	_, err := activities.RunIntegrationActivity(context.Background(), ExecutorActivityInput{
		TenantID:        "22222222-2222-2222-2222-222222222222",
		PlanStepKey:     "fetch-news",
		StepExecutionID: "step-fetch-news",
		InputArtifacts: []ArtifactRef{{
			ArtifactType: artifacts.TypeKeyDateRange,
			LiteralJSON:  `{"startDate":"2026-01-01","endDate":"2026-01-07"}`,
		}},
		ExecutorInstallationSnapshot: ExecutorInstallationSnapshot{
			ConfigJSON: `{"feeds":["https://example.com/rss"]}`,
		},
	})
	if err == nil {
		t.Fatal("expected retryable temporal application error")
	}
	var appErr *temporal.ApplicationError
	if !errors.As(err, &appErr) {
		t.Fatalf("error = %T(%v), want *temporal.ApplicationError", err, err)
	}
	if appErr.Type() != "FeedFetchError" {
		t.Fatalf("error type = %q, want FeedFetchError", appErr.Type())
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}
