package workflow_test

import (
	"context"
	"errors"
	"testing"

	"go.temporal.io/sdk/temporal"

	"github.com/harpia/control-plane/internal/artifacts"
	"github.com/harpia/control-plane/internal/executors"
	"github.com/harpia/control-plane/internal/executors/runtime"
	"github.com/harpia/control-plane/internal/workflow"
)

type retryingRunner struct{}

func (retryingRunner) Run(_ context.Context, _ executors.IntegrationExecutionRequest) (executors.IntegrationExecutionResult, error) {
	return executors.IntegrationExecutionResult{}, runtime.NewRetryableError(runtime.ErrCodeFeedFetch, "feed fetch failed", errors.New("connection refused"))
}

func TestRunIntegrationActivityMapsRetryableError(t *testing.T) {
	activities := &workflow.PlanActivities{Integrations: retryingRunner{}}

	_, err := activities.RunIntegrationActivity(context.Background(), workflow.ExecutorActivityInput{
		TenantID:        "22222222-2222-2222-2222-222222222222",
		StepExecutionID: "step-fetch-news",
		InputArtifacts: []workflow.ArtifactRef{{
			ArtifactTypeKey: artifacts.TypeKeyDateRange,
			LiteralJSON:     `{"startDate":"2026-01-01","endDate":"2026-01-07"}`,
		}},
		ExecutorInstallationSnapshot: workflow.ExecutorInstallationSnapshot{
			ExecutorSKUKey: executors.SKURSSNewsFeed,
			ConfigJSON:     `{"feeds":["https://example.com/rss"]}`,
		},
	})
	if err == nil {
		t.Fatal("expected temporal application error")
	}
	var appErr *temporal.ApplicationError
	if !errors.As(err, &appErr) {
		t.Fatalf("error = %T(%v), want *temporal.ApplicationError", err, err)
	}
	if appErr.Type() != "FeedFetchError" {
		t.Fatalf("error type = %q, want FeedFetchError", appErr.Type())
	}
}
