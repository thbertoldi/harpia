package rss

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
	"github.com/harpia/control-plane/internal/artifacts"
	"github.com/harpia/control-plane/internal/executors"
)

const retryableFeedFetchCode = "FeedFetchError"

type Handler struct {
	artifacts executors.ArtifactGateway
	feeds     FeedFetcher
}

func NewHandler(artifacts executors.ArtifactGateway, feeds FeedFetcher) *Handler {
	return &Handler{artifacts: artifacts, feeds: feeds}
}

func (h *Handler) SKUKey() string {
	return executors.SKURSSNewsFeed
}

func (h *Handler) Execute(ctx context.Context, req executors.IntegrationExecutionRequest) (executors.IntegrationExecutionResult, error) {
	if h == nil || h.artifacts == nil || h.feeds == nil {
		return executors.IntegrationExecutionResult{}, fmt.Errorf("rss integration handler is not configured")
	}

	config, err := ParseInstallationConfig(req.Installation.ConfigJSON)
	if err != nil {
		return failedResult(err), nil
	}

	dateRangePayload, err := h.artifacts.LoadPayloadForType(ctx, req.TenantID, req.InputArtifacts, artifacts.TypeKeyDateRange)
	if err != nil {
		return failedResult(err), nil
	}
	dateRange := &artifactsv1.DateRange{}
	if err := protojson.Unmarshal(dateRangePayload, dateRange); err != nil {
		return failedResult(fmt.Errorf("parse date range artifact: %w", err)), nil
	}
	if err := artifacts.ValidatePayload(artifacts.TypeKeyDateRange, dateRangePayload); err != nil {
		return failedResult(err), nil
	}

	newsList, err := FetchNewsList(ctx, h.feeds, config.Feeds, dateRange)
	if err != nil {
		var feedErr *FeedFetchError
		if errors.As(err, &feedErr) {
			return executors.IntegrationExecutionResult{}, executors.NewRetryableError(
				retryableFeedFetchCode,
				feedErr.Error(),
				err,
			)
		}
		return failedResult(err), nil
	}

	payload, err := protojson.Marshal(newsList)
	if err != nil {
		return executors.IntegrationExecutionResult{}, fmt.Errorf("marshal news list: %w", err)
	}

	outputArtifactID, err := h.artifacts.CreateValidatedPayload(ctx, executors.CreateArtifactRequest{
		TenantID:        req.TenantID,
		ArtifactTypeID:  req.OutputArtifactTypeID,
		ArtifactTypeKey: artifacts.TypeKeyNewsList,
		StepExecutionID: req.StepExecutionID,
		Payload:         payload,
	})
	if err != nil {
		return executors.IntegrationExecutionResult{}, fmt.Errorf("create news list artifact: %w", err)
	}

	return executors.IntegrationExecutionResult{
		Status:           executors.IntegrationStatusCompleted,
		OutputArtifactID: outputArtifactID,
	}, nil
}

func failedResult(err error) executors.IntegrationExecutionResult {
	message := "integration failed"
	if err != nil {
		message = err.Error()
	}
	return executors.IntegrationExecutionResult{
		Status: executors.IntegrationStatusFailed,
		Error:  message,
	}
}
