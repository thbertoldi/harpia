package linkedin_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/harpia/control-plane/internal/artifacts"
	"github.com/harpia/control-plane/internal/executors"
	"github.com/harpia/control-plane/internal/executors/integrations/linkedin"
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

func newLinkedInTestArtifactStore() runtime.ExecutorArtifactStore {
	typeID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	return runtime.NewExecutorArtifactStore(&memoryArtifactRepo{
		types: map[string]*artifacts.ArtifactType{
			artifacts.TypeKeyPublishConfirmation: {ID: typeID, Key: artifacts.TypeKeyPublishConfirmation},
		},
	}, &memoryPayloadStore{})
}

func TestHandlerExecuteSuccess(t *testing.T) {
	store := newLinkedInTestArtifactStore()
	publishedAt := time.Date(2026, 2, 10, 15, 4, 5, 0, time.UTC)
	publisher := &linkedin.FakePublisher{
		Result: linkedin.PublishResult{
			PostID:      "urn:li:share:123",
			Permalink:   "https://www.linkedin.com/feed/update/urn:li:share:123",
			PublishedAt: publishedAt,
		},
	}
	handler := linkedin.NewHandler(store, publisher)

	result, err := handler.Execute(context.Background(), runtime.IntegrationExecutionRequest{
		TenantID:              uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		StepExecutionID:       "step-publish-linkedin",
		OutputArtifactTypeKey: artifacts.TypeKeyPublishConfirmation,
		InputArtifacts: []runtime.InputArtifactRef{{
			ArtifactTypeKey: artifacts.TypeKeyLinkedInPostDraft,
			LiteralJSON:     `{"text":"Launching our integration this week!","hashtags":["harpia","automation"]}`,
		}},
		Installation: runtime.InstallationSnapshot{
			ExecutorSKUKey: executors.SKULinkedInPublish,
			ConfigJSON:     json.RawMessage(`{"oauth_credential_id":"cred-123"}`),
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
	if publisher.Calls != 1 {
		t.Fatalf("publisher calls = %d, want 1", publisher.Calls)
	}
	if publisher.LastRequest.OAuthCredentialID != "cred-123" {
		t.Fatalf("oauth credential id = %q, want cred-123", publisher.LastRequest.OAuthCredentialID)
	}
}

func TestHandlerExecuteOAuthReconnectRequired(t *testing.T) {
	handler := linkedin.NewHandler(newLinkedInTestArtifactStore(), &linkedin.FakePublisher{
		Err: linkedin.NewOAuthReconnectError("token expired", nil),
	})

	result, err := handler.Execute(context.Background(), runtime.IntegrationExecutionRequest{
		TenantID:              uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		OutputArtifactTypeKey: artifacts.TypeKeyPublishConfirmation,
		InputArtifacts: []runtime.InputArtifactRef{{
			ArtifactTypeKey: artifacts.TypeKeyLinkedInPostDraft,
			LiteralJSON:     `{"text":"Test post"}`,
		}},
		Installation: runtime.InstallationSnapshot{
			ExecutorSKUKey: executors.SKULinkedInPublish,
			ConfigJSON:     json.RawMessage(`{"oauth_credential_id":"cred-123"}`),
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Status != runtime.IntegrationStatusFailed {
		t.Fatalf("Status = %q, want failed", result.Status)
	}
	if result.Error == "" {
		t.Fatal("expected reconnect failure message")
	}
	if want := runtime.ErrCodeOAuthReconnectRequired; !strings.Contains(result.Error, want) {
		t.Fatalf("Error = %q, want contains %q", result.Error, want)
	}
}

func TestHandlerExecuteTransientFailureIsRetryable(t *testing.T) {
	handler := linkedin.NewHandler(newLinkedInTestArtifactStore(), &linkedin.FakePublisher{
		Err: linkedin.NewPublishTransientError(503, errors.New("service unavailable")),
	})

	_, err := handler.Execute(context.Background(), runtime.IntegrationExecutionRequest{
		TenantID:              uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		OutputArtifactTypeKey: artifacts.TypeKeyPublishConfirmation,
		InputArtifacts: []runtime.InputArtifactRef{{
			ArtifactTypeKey: artifacts.TypeKeyLinkedInPostDraft,
			LiteralJSON:     `{"text":"Test post"}`,
		}},
		Installation: runtime.InstallationSnapshot{
			ExecutorSKUKey: executors.SKULinkedInPublish,
			ConfigJSON:     json.RawMessage(`{"oauth_credential_id":"cred-123"}`),
		},
	})
	if err == nil {
		t.Fatal("expected retryable error")
	}
	retryable, ok := runtime.IsRetryable(err)
	if !ok {
		t.Fatalf("error = %T(%v), want *runtime.RetryableError", err, err)
	}
	if retryable.Code != runtime.ErrCodeLinkedInPublish {
		t.Fatalf("retryable code = %q, want %q", retryable.Code, runtime.ErrCodeLinkedInPublish)
	}
}
