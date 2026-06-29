package linkedin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
	"github.com/harpia/control-plane/internal/artifacts"
	"github.com/harpia/control-plane/internal/executors/catalog"
	"github.com/harpia/control-plane/internal/executors/runtime"
)

type Handler struct {
	artifacts runtime.ExecutorArtifactStore
	publisher LinkedInPublisher
}

func NewHandler(artifacts runtime.ExecutorArtifactStore, publisher LinkedInPublisher) *Handler {
	return &Handler{
		artifacts: artifacts,
		publisher: publisher,
	}
}

func (h *Handler) SKUKey() string {
	return catalog.SKULinkedInPublish
}

func (h *Handler) Execute(ctx context.Context, req runtime.IntegrationExecutionRequest) (runtime.IntegrationExecutionResult, error) {
	if h == nil || h.artifacts == nil || h.publisher == nil {
		return runtime.IntegrationExecutionResult{}, fmt.Errorf("linkedin integration handler is not configured")
	}

	config, err := ParseInstallationConfig(req.Installation.ConfigJSON)
	if err != nil {
		return failedResult(err), nil
	}

	draftPayload, err := h.artifacts.LoadPayloadForType(ctx, req.TenantID, req.InputArtifacts, artifacts.TypeKeyLinkedInPostDraft)
	if err != nil {
		return failedResult(err), nil
	}
	if err := artifacts.ValidatePayload(artifacts.TypeKeyLinkedInPostDraft, draftPayload); err != nil {
		return failedResult(err), nil
	}

	draft := &artifactsv1.LinkedInPostDraft{}
	if err := protojson.Unmarshal(draftPayload, draft); err != nil {
		return failedResult(fmt.Errorf("parse linkedin post draft artifact: %w", err)), nil
	}

	if config.Mode == ModeApprovalOnly {
		return h.createConfirmation(ctx, req, &artifactsv1.PublishConfirmation{
			Platform:    "linkedin-dry-run",
			ExternalId:  "dry-run-" + strings.TrimSpace(req.StepExecutionID),
			Url:         "",
			PublishedAt: time.Now().UTC().Format(time.RFC3339),
		})
	}

	published, err := h.publisher.Publish(ctx, PublishRequest{
		OAuthCredentialID: config.OAuthCredentialID,
		Draft:             draft,
	})
	if err != nil {
		if IsOAuthReconnectRequired(err) {
			return failedResultWithCode(
				runtime.ErrCodeOAuthReconnectRequired,
				"linkedin oauth credential is expired or invalid; reconnect required",
			), nil
		}

		var transientErr *PublishTransientError
		if errors.As(err, &transientErr) {
			return runtime.IntegrationExecutionResult{}, runtime.NewRetryableError(
				runtime.ErrCodeLinkedInPublish,
				err.Error(),
				err,
			)
		}
		return failedResult(err), nil
	}

	if published.PublishedAt.IsZero() {
		published.PublishedAt = time.Now().UTC()
	}

	confirmation := &artifactsv1.PublishConfirmation{
		Platform:    "linkedin",
		ExternalId:  strings.TrimSpace(published.PostID),
		Url:         strings.TrimSpace(published.Permalink),
		PublishedAt: published.PublishedAt.UTC().Format(time.RFC3339),
	}
	return h.createConfirmation(ctx, req, confirmation)
}

func (h *Handler) createConfirmation(
	ctx context.Context,
	req runtime.IntegrationExecutionRequest,
	confirmation *artifactsv1.PublishConfirmation,
) (runtime.IntegrationExecutionResult, error) {
	payload, err := protojson.Marshal(confirmation)
	if err != nil {
		return runtime.IntegrationExecutionResult{}, fmt.Errorf("marshal publish confirmation: %w", err)
	}

	outputArtifactID, err := h.artifacts.CreateValidatedPayload(ctx, runtime.CreateArtifactRequest{
		TenantID:              req.TenantID,
		OutputArtifactTypeKey: req.OutputArtifactTypeKey,
		StepExecutionID:       req.StepExecutionID,
		Payload:               payload,
	})
	if err != nil {
		return runtime.IntegrationExecutionResult{}, fmt.Errorf("create publish confirmation artifact: %w", err)
	}

	return runtime.IntegrationExecutionResult{
		Status:           runtime.IntegrationStatusCompleted,
		OutputArtifactID: outputArtifactID,
	}, nil
}

func failedResult(err error) runtime.IntegrationExecutionResult {
	message := "integration failed"
	if err != nil {
		message = err.Error()
	}
	return runtime.IntegrationExecutionResult{
		Status: runtime.IntegrationStatusFailed,
		Error:  message,
	}
}

func failedResultWithCode(code, message string) runtime.IntegrationExecutionResult {
	trimmedCode := strings.TrimSpace(code)
	trimmedMessage := strings.TrimSpace(message)
	if trimmedCode == "" {
		return failedResult(errors.New(trimmedMessage))
	}
	if trimmedMessage == "" {
		trimmedMessage = "integration failed"
	}
	return runtime.IntegrationExecutionResult{
		Status: runtime.IntegrationStatusFailed,
		Error:  fmt.Sprintf("[%s] %s", trimmedCode, trimmedMessage),
	}
}
