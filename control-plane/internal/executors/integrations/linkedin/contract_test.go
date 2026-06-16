package linkedin_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/harpia/control-plane/internal/artifacts"
	"github.com/harpia/control-plane/internal/executors"
	"github.com/harpia/control-plane/internal/executors/contracttest"
	"github.com/harpia/control-plane/internal/executors/integrations/linkedin"
	"github.com/harpia/control-plane/internal/executors/runtime"
)

func TestLinkedInPublishIntegrationContract(t *testing.T) {
	store := contracttest.NewLinkedInArtifactStore()
	handler := linkedin.NewHandler(store, &linkedin.FakePublisher{
		Result: linkedin.PublishResult{
			PostID:      "urn:li:share:123",
			Permalink:   "https://www.linkedin.com/feed/update/urn:li:share:123",
			PublishedAt: time.Date(2026, 2, 10, 15, 4, 5, 0, time.UTC),
		},
	})

	contracttest.Run(t, contracttest.Suite{
		Name:                    executors.SKULinkedInPublish,
		Handler:                 handler,
		Store:                   store,
		SKUKey:                  executors.SKULinkedInPublish,
		InputArtifactTypeKey:    artifacts.TypeKeyLinkedInPostDraft,
		OutputArtifactTypeKey:   artifacts.TypeKeyPublishConfirmation,
		ValidInstallationConfig: json.RawMessage(`{"oauth_credential_id":"cred-123"}`),
		ValidInputLiteralJSON:   contracttest.ValidLinkedInPostDraftLiteral,
		RetryableTrigger: func(ctx context.Context, req runtime.IntegrationExecutionRequest) error {
			retryingHandler := linkedin.NewHandler(store, &linkedin.FakePublisher{
				Err: linkedin.NewPublishTransientError(503, errors.New("service unavailable")),
			})
			_, err := retryingHandler.Execute(ctx, req)
			return err
		},
	})
}
