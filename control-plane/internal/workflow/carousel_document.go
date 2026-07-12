package workflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
	"github.com/harpia/control-plane/internal/artifacts"
	"github.com/harpia/control-plane/internal/executors/integrations/linkedin"
	"github.com/harpia/control-plane/internal/executors/runtime"
	"github.com/harpia/control-plane/internal/identity"
)

// EnsureCarouselDocumentInput identifies the exact agent-produced candidate to
// enrich before it is sent to review. It never addresses an Artifact's mutable
// current version.
type EnsureCarouselDocumentInput struct {
	TenantID        string      `json:"tenant_id"`
	PlanExecutionID string      `json:"plan_execution_id"`
	StepExecutionID string      `json:"step_execution_id"`
	Candidate       ArtifactRef `json:"candidate"`
}

// EnsureCarouselDocumentActivity creates the immutable PDF derivative for a
// carousel candidate, then returns the newly versioned LinkedInPost ref. It is
// intentionally a LinkedIn-only activity rather than a generic renderer hook.
func (a *PlanActivities) EnsureCarouselDocumentActivity(ctx context.Context, input EnsureCarouselDocumentInput) (ArtifactRef, error) {
	if a == nil || a.ArtifactStore == nil {
		return ArtifactRef{}, fmt.Errorf("executor artifact store is not configured")
	}
	tenantID, err := uuid.Parse(input.TenantID)
	if err != nil {
		return ArtifactRef{}, fmt.Errorf("parse tenant id: %w", err)
	}
	ctx = identity.WithRequestContext(ctx, identity.RequestContext{TenantID: tenantID})
	candidate, err := artifactRefToRuntime(input.Candidate)
	if err != nil {
		return ArtifactRef{}, err
	}
	if candidate.ArtifactTypeKey != artifacts.TypeKeyLinkedInPost {
		return ArtifactRef{}, fmt.Errorf("carousel candidate type = %q, want %q", candidate.ArtifactTypeKey, artifacts.TypeKeyLinkedInPost)
	}
	payload, err := a.ArtifactStore.LoadPinnedPayload(ctx, tenantID, candidate)
	if err != nil {
		return ArtifactRef{}, fmt.Errorf("load candidate LinkedIn post: %w", err)
	}
	if err := artifacts.ValidatePayload(artifacts.TypeKeyLinkedInPost, payload); err != nil {
		return ArtifactRef{}, err
	}
	post := &artifactsv1.LinkedInPost{}
	if err := protojson.Unmarshal(payload, post); err != nil {
		return ArtifactRef{}, fmt.Errorf("parse candidate LinkedIn post: %w", err)
	}
	if post.Carousel == nil {
		return input.Candidate, nil
	}
	if post.Carousel.DocumentArtifact != nil {
		if err := validateDocumentRef(post.Carousel.DocumentArtifact); err != nil {
			return ArtifactRef{}, err
		}
		return input.Candidate, nil
	}

	pdf, err := linkedin.BuildCarouselDocument(post.Carousel)
	if err != nil {
		return ArtifactRef{}, fmt.Errorf("build carousel PDF: %w", err)
	}
	document, err := a.ArtifactStore.CreateValidatedPayloadRef(ctx, runtime.CreateArtifactRequest{
		TenantID: tenantID, OutputArtifactTypeKey: artifacts.TypeKeyLinkedInCarouselDocument,
		PlanExecutionID: input.PlanExecutionID, StepExecutionID: input.StepExecutionID, Payload: pdf,
	})
	if err != nil {
		return ArtifactRef{}, fmt.Errorf("store carousel PDF: %w", err)
	}
	post.Carousel.DocumentArtifact = &artifactsv1.ArtifactRef{
		ArtifactId: document.ArtifactID, ArtifactVersionId: document.ArtifactVersionID,
		ArtifactTypeKey: document.ArtifactTypeKey, ContentHash: document.ContentHash,
	}
	enrichedPayload, err := protojson.Marshal(post)
	if err != nil {
		return ArtifactRef{}, fmt.Errorf("marshal carousel-enriched LinkedIn post: %w", err)
	}
	enriched, err := a.ArtifactStore.CreateValidatedVersionPayload(ctx, runtime.CreateArtifactVersionRequest{
		TenantID: tenantID, ArtifactID: candidate.ArtifactID, ExpectedContentHash: candidate.ContentHash,
		SourceArtifactVersionID: candidate.ArtifactVersionID, OutputArtifactTypeKey: artifacts.TypeKeyLinkedInPost,
		PlanExecutionID: input.PlanExecutionID, StepExecutionID: input.StepExecutionID,
		EditSummary: "Attached deterministic carousel PDF", Payload: enrichedPayload,
	})
	if err != nil {
		return ArtifactRef{}, fmt.Errorf("create carousel-enriched LinkedIn post version: %w", err)
	}
	return ArtifactRef{Source: input.Candidate.Source, StepKey: input.Candidate.StepKey, ArtifactID: enriched.ArtifactID, ArtifactVersionID: enriched.ArtifactVersionID, ArtifactTypeKey: enriched.ArtifactTypeKey, ContentHash: enriched.ContentHash}, nil
}

func artifactRefToRuntime(ref ArtifactRef) (runtime.VersionedArtifactRef, error) {
	if strings.TrimSpace(ref.ArtifactID) == "" || strings.TrimSpace(ref.ArtifactVersionID) == "" || strings.TrimSpace(ref.ArtifactTypeKey) == "" || strings.TrimSpace(ref.ContentHash) == "" {
		return runtime.VersionedArtifactRef{}, fmt.Errorf("LinkedIn post candidate must be version-pinned")
	}
	return runtime.VersionedArtifactRef{ArtifactID: ref.ArtifactID, ArtifactVersionID: ref.ArtifactVersionID, ArtifactTypeKey: ref.ArtifactTypeKey, ContentHash: ref.ContentHash}, nil
}

func validateDocumentRef(ref *artifactsv1.ArtifactRef) error {
	if ref == nil || strings.TrimSpace(ref.ArtifactId) == "" || strings.TrimSpace(ref.ArtifactVersionId) == "" || ref.ArtifactTypeKey != artifacts.TypeKeyLinkedInCarouselDocument || strings.TrimSpace(ref.ContentHash) == "" {
		return fmt.Errorf("carousel document artifact ref must be complete and typed LinkedInCarouselDocument")
	}
	return nil
}
