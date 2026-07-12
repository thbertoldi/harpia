package linkedin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
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
	return &Handler{artifacts: artifacts, publisher: publisher}
}
func (h *Handler) SKUKey() string { return catalog.SKULinkedInPublish }

func (h *Handler) Execute(ctx context.Context, req runtime.IntegrationExecutionRequest) (runtime.IntegrationExecutionResult, error) {
	if h == nil || h.artifacts == nil || h.publisher == nil {
		return runtime.IntegrationExecutionResult{}, fmt.Errorf("linkedin integration handler is not configured")
	}
	config, err := ParseInstallationConfig(req.Installation.ConfigJSON)
	if err != nil {
		return failedResult(err), nil
	}
	postRef, err := linkedInPostInputRef(req.InputArtifacts)
	if err != nil {
		return failedResult(err), nil
	}
	payload, err := h.artifacts.LoadPinnedPayload(ctx, req.TenantID, postRef)
	if err != nil {
		return failedResult(err), nil
	}
	if err := artifacts.ValidatePayload(artifacts.TypeKeyLinkedInPost, payload); err != nil {
		return failedResult(err), nil
	}
	post := &artifactsv1.LinkedInPost{}
	if err := protojson.Unmarshal(payload, post); err != nil {
		return failedResult(fmt.Errorf("parse version-pinned LinkedIn post artifact: %w", err)), nil
	}
	if config.Mode == ModeApprovalOnly {
		return h.createConfirmation(ctx, req, &artifactsv1.PublishConfirmation{Platform: "linkedin-dry-run", ExternalId: "dry-run-" + strings.TrimSpace(req.StepExecutionID), PublishedAt: time.Now().UTC().Format(time.RFC3339)})
	}
	pdf, err := h.loadCarouselPDF(ctx, req.TenantID, post)
	if err != nil {
		return failedResult(err), nil
	}
	published, err := h.publisher.Publish(ctx, PublishRequest{OAuthCredentialID: config.OAuthCredentialID, AuthorURN: config.AuthorURN, Post: post, CarouselPDF: pdf})
	if err != nil {
		if IsOAuthReconnectRequired(err) {
			return failedResultWithCode(runtime.ErrCodeOAuthReconnectRequired, "linkedin oauth credential is expired or invalid; reconnect required"), nil
		}
		var transientErr *PublishTransientError
		if errors.As(err, &transientErr) {
			return runtime.IntegrationExecutionResult{}, runtime.NewRetryableError(runtime.ErrCodeLinkedInPublish, err.Error(), err)
		}
		return failedResult(err), nil
	}
	if published.PublishedAt.IsZero() {
		published.PublishedAt = time.Now().UTC()
	}
	return h.createConfirmation(ctx, req, &artifactsv1.PublishConfirmation{Platform: "linkedin", ExternalId: strings.TrimSpace(published.PostID), Url: strings.TrimSpace(published.Permalink), PublishedAt: published.PublishedAt.UTC().Format(time.RFC3339)})
}

func (h *Handler) loadCarouselPDF(ctx context.Context, tenantID uuid.UUID, post *artifactsv1.LinkedInPost) ([]byte, error) {
	if post == nil || post.Carousel == nil {
		return nil, nil
	}
	ref := post.Carousel.DocumentArtifact
	if ref == nil || strings.TrimSpace(ref.ArtifactId) == "" || strings.TrimSpace(ref.ArtifactVersionId) == "" || ref.ArtifactTypeKey != artifacts.TypeKeyLinkedInCarouselDocument || strings.TrimSpace(ref.ContentHash) == "" {
		return nil, fmt.Errorf("carousel document artifact ref must be complete and version-pinned")
	}
	pdf, err := h.artifacts.LoadPinnedPayload(ctx, tenantID, runtime.VersionedArtifactRef{ArtifactID: ref.ArtifactId, ArtifactVersionID: ref.ArtifactVersionId, ArtifactTypeKey: ref.ArtifactTypeKey, ContentHash: ref.ContentHash})
	if err != nil {
		return nil, fmt.Errorf("load pinned carousel document: %w", err)
	}
	if err := artifacts.ValidatePayload(artifacts.TypeKeyLinkedInCarouselDocument, pdf); err != nil {
		return nil, err
	}
	return pdf, nil
}

func linkedInPostInputRef(refs []runtime.InputArtifactRef) (runtime.VersionedArtifactRef, error) {
	for _, ref := range refs {
		if ref.ArtifactTypeKey != artifacts.TypeKeyLinkedInPost {
			continue
		}
		if strings.TrimSpace(ref.ArtifactID) == "" || strings.TrimSpace(ref.ArtifactVersionID) == "" || strings.TrimSpace(ref.ContentHash) == "" {
			return runtime.VersionedArtifactRef{}, fmt.Errorf("LinkedIn post input must be version-pinned")
		}
		return runtime.VersionedArtifactRef{ArtifactID: ref.ArtifactID, ArtifactVersionID: ref.ArtifactVersionID, ArtifactTypeKey: ref.ArtifactTypeKey, ContentHash: ref.ContentHash}, nil
	}
	return runtime.VersionedArtifactRef{}, fmt.Errorf("missing version-pinned LinkedIn post input")
}

func (h *Handler) createConfirmation(ctx context.Context, req runtime.IntegrationExecutionRequest, confirmation *artifactsv1.PublishConfirmation) (runtime.IntegrationExecutionResult, error) {
	payload, err := protojson.Marshal(confirmation)
	if err != nil {
		return runtime.IntegrationExecutionResult{}, fmt.Errorf("marshal publish confirmation: %w", err)
	}
	outputArtifactID, err := h.artifacts.CreateValidatedPayload(ctx, runtime.CreateArtifactRequest{TenantID: req.TenantID, OutputArtifactTypeKey: req.OutputArtifactTypeKey, PlanExecutionID: req.PlanExecutionID, StepExecutionID: req.StepExecutionID, Payload: payload})
	if err != nil {
		return runtime.IntegrationExecutionResult{}, fmt.Errorf("create publish confirmation artifact: %w", err)
	}
	return runtime.IntegrationExecutionResult{Status: runtime.IntegrationStatusCompleted, OutputArtifactID: outputArtifactID}, nil
}
func failedResult(err error) runtime.IntegrationExecutionResult {
	message := "integration failed"
	if err != nil {
		message = err.Error()
	}
	return runtime.IntegrationExecutionResult{Status: runtime.IntegrationStatusFailed, Error: message}
}
func failedResultWithCode(code, message string) runtime.IntegrationExecutionResult {
	if strings.TrimSpace(code) == "" {
		return failedResult(errors.New(message))
	}
	if strings.TrimSpace(message) == "" {
		message = "integration failed"
	}
	return runtime.IntegrationExecutionResult{Status: runtime.IntegrationStatusFailed, Error: fmt.Sprintf("[%s] %s", code, message)}
}
