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
)

type handlerArtifactGateway struct {
	types     map[string]*artifacts.ArtifactType
	artifacts map[uuid.UUID]*artifacts.Artifact
	objects   map[string][]byte
}

func (m *handlerArtifactGateway) LoadPayloadForType(_ context.Context, _ uuid.UUID, refs []executors.InputArtifactRef, typeKey string) ([]byte, error) {
	for _, ref := range refs {
		if ref.ArtifactType != "" && ref.ArtifactType != typeKey {
			continue
		}
		if ref.LiteralJSON != "" {
			return []byte(ref.LiteralJSON), nil
		}
	}
	return nil, errors.New("missing input artifact")
}

func (m *handlerArtifactGateway) CreateValidatedPayload(_ context.Context, req executors.CreateArtifactRequest) (string, error) {
	if err := artifacts.ValidatePayload(req.ArtifactTypeKey, req.Payload); err != nil {
		return "", err
	}
	if m.objects == nil {
		m.objects = make(map[string][]byte)
	}
	artifactID := uuid.New()
	uri := "s3://harpia/tenant/test/" + artifactID.String()
	m.objects[uri] = append([]byte(nil), req.Payload...)
	return artifactID.String(), nil
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

func TestHandlerExecuteSuccess(t *testing.T) {
	handler := rss.NewHandler(&handlerArtifactGateway{}, stubFeedFetcher{feed: &gofeed.Feed{
		Title: "Example News",
		Items: []*gofeed.Item{{
			Title:           "Story",
			Link:            "https://example.com/story",
			Description:     "Summary",
			PublishedParsed: timePtr(time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)),
		}},
	}})

	result, err := handler.Execute(context.Background(), executors.IntegrationExecutionRequest{
		TenantID:        uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		StepExecutionID: "step-fetch-news",
		OutputArtifactTypeID: uuid.MustParse("33333333-3333-3333-3333-333333333333").String(),
		InputArtifacts: []executors.InputArtifactRef{{
			ArtifactType: artifacts.TypeKeyDateRange,
			LiteralJSON:  `{"startDate":"2026-01-01","endDate":"2026-01-07"}`,
		}},
		Installation: executors.InstallationSnapshot{
			ExecutorSKUKey: executors.SKURSSNewsFeed,
			ConfigJSON:     json.RawMessage(`{"feeds":["https://example.com/rss"]}`),
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Status != executors.IntegrationStatusCompleted {
		t.Fatalf("Status = %q, want completed (%s)", result.Status, result.Error)
	}
	if result.OutputArtifactID == "" {
		t.Fatal("expected output artifact id")
	}
}

func TestHandlerExecuteInvalidConfig(t *testing.T) {
	handler := rss.NewHandler(&handlerArtifactGateway{}, stubFeedFetcher{})
	result, err := handler.Execute(context.Background(), executors.IntegrationExecutionRequest{
		TenantID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		InputArtifacts: []executors.InputArtifactRef{{
			ArtifactType: artifacts.TypeKeyDateRange,
			LiteralJSON:  `{"startDate":"2026-01-01","endDate":"2026-01-07"}`,
		}},
		Installation: executors.InstallationSnapshot{
			ConfigJSON: json.RawMessage(`{}`),
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Status != executors.IntegrationStatusFailed {
		t.Fatalf("Status = %q, want failed", result.Status)
	}
}

func TestHandlerExecuteDeadFeedReturnsRetryableError(t *testing.T) {
	handler := rss.NewHandler(&handlerArtifactGateway{}, stubFeedFetcher{err: errors.New("connection refused")})
	_, err := handler.Execute(context.Background(), executors.IntegrationExecutionRequest{
		TenantID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		InputArtifacts: []executors.InputArtifactRef{{
			ArtifactType: artifacts.TypeKeyDateRange,
			LiteralJSON:  `{"startDate":"2026-01-01","endDate":"2026-01-07"}`,
		}},
		Installation: executors.InstallationSnapshot{
			ConfigJSON: json.RawMessage(`{"feeds":["https://example.com/rss"]}`),
		},
	})
	if err == nil {
		t.Fatal("expected retryable error")
	}
	var retryable *executors.RetryableError
	if !errors.As(err, &retryable) {
		t.Fatalf("error = %T(%v), want *executors.RetryableError", err, err)
	}
}

func TestIntegrationRegistryRoutesBySKUKey(t *testing.T) {
	registry := executors.NewIntegrationRegistry(rss.NewHandler(&handlerArtifactGateway{}, stubFeedFetcher{}))
	result, err := registry.Run(context.Background(), executors.IntegrationExecutionRequest{
		Installation: executors.InstallationSnapshot{ExecutorSKUKey: executors.SKULinkedInPublish},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != executors.IntegrationStatusFailed {
		t.Fatalf("Status = %q, want failed", result.Status)
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}
