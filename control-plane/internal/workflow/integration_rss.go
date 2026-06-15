package workflow

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"go.temporal.io/sdk/temporal"

	"github.com/harpia/control-plane/internal/integrations/rss"
)

func (a *PlanActivities) runRSSFetchNews(ctx context.Context, input ExecutorActivityInput) (ExecutorActivityResult, error) {
	if a == nil || a.Artifacts == nil || a.FeedFetcher == nil {
		return ExecutorActivityResult{}, fmt.Errorf("rss integration dependencies are not configured")
	}

	tenantID, err := uuid.Parse(input.TenantID)
	if err != nil {
		return integrationFailedResult(fmt.Errorf("parse tenant id: %w", err)), nil
	}

	config, err := rss.ParseInstallationConfig([]byte(input.ExecutorInstallationSnapshot.ConfigJSON))
	if err != nil {
		return integrationFailedResult(err), nil
	}

	dateRange, err := a.Artifacts.LoadDateRangeInput(ctx, tenantID, input.InputArtifacts)
	if err != nil {
		return integrationFailedResult(err), nil
	}

	newsList, err := rss.FetchNewsList(ctx, a.FeedFetcher, config.Feeds, dateRange)
	if err != nil {
		var feedErr *rss.FeedFetchError
		if errors.As(err, &feedErr) {
			return ExecutorActivityResult{}, temporal.NewApplicationError(
				feedErr.Error(),
				"FeedFetchError",
				feedErr.FeedURL,
			)
		}
		return integrationFailedResult(err), nil
	}

	outputArtifactID, err := a.Artifacts.CreateNewsListArtifact(
		ctx,
		tenantID,
		input.OutputArtifactTypeID,
		input.StepExecutionID,
		newsList,
	)
	if err != nil {
		return ExecutorActivityResult{}, fmt.Errorf("create news list artifact: %w", err)
	}

	return ExecutorActivityResult{
		Status:           ExecutorResultStatusCompleted,
		OutputArtifactID: outputArtifactID,
	}, nil
}

func integrationFailedResult(err error) ExecutorActivityResult {
	message := "integration failed"
	if err != nil {
		message = err.Error()
	}
	return ExecutorActivityResult{
		Status: ExecutorResultStatusFailed,
		Error:  message,
	}
}
