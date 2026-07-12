package workflow

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
	"github.com/harpia/control-plane/internal/artifacts"
	"github.com/harpia/control-plane/internal/executors/runtime"
)

type carouselDocumentStore struct {
	post                            []byte
	documentCreates, versionCreates int
	versionRequest                  runtime.CreateArtifactVersionRequest
}

func (s *carouselDocumentStore) LoadPayloadForType(context.Context, uuid.UUID, []runtime.InputArtifactRef, string) ([]byte, error) {
	return nil, context.Canceled
}
func (s *carouselDocumentStore) LoadPinnedPayload(_ context.Context, _ uuid.UUID, ref runtime.VersionedArtifactRef) ([]byte, error) {
	if ref.ArtifactTypeKey == artifacts.TypeKeyLinkedInPost {
		return s.post, nil
	}
	return nil, context.Canceled
}
func (s *carouselDocumentStore) CreateValidatedPayload(context.Context, runtime.CreateArtifactRequest) (string, error) {
	return "", context.Canceled
}
func (s *carouselDocumentStore) CreateValidatedPayloadRef(context.Context, runtime.CreateArtifactRequest) (runtime.VersionedArtifactRef, error) {
	s.documentCreates++
	return runtime.VersionedArtifactRef{ArtifactID: "document", ArtifactVersionID: "document-v1", ArtifactTypeKey: artifacts.TypeKeyLinkedInCarouselDocument, ContentHash: "document-hash"}, nil
}
func (s *carouselDocumentStore) CreateValidatedVersionPayload(_ context.Context, request runtime.CreateArtifactVersionRequest) (runtime.VersionedArtifactRef, error) {
	s.versionCreates++
	s.versionRequest = request
	return runtime.VersionedArtifactRef{ArtifactID: "post", ArtifactVersionID: "post-v2", ArtifactTypeKey: artifacts.TypeKeyLinkedInPost, ContentHash: "post-v2-hash"}, nil
}
func carouselPostPayload(t *testing.T, document *artifactsv1.ArtifactRef) []byte {
	t.Helper()
	payload, err := protojson.Marshal(&artifactsv1.LinkedInPost{Text: &artifactsv1.LinkedInPostDraft{Text: "Pinned post"}, Carousel: &artifactsv1.CarouselDraft{Title: "Carousel", Slides: []*artifactsv1.CarouselSlide{{Heading: "One"}}, DocumentArtifact: document}})
	if err != nil {
		t.Fatal(err)
	}
	return payload
}
func candidateRef() ArtifactRef {
	return ArtifactRef{ArtifactID: "post", ArtifactVersionID: "post-v1", ArtifactTypeKey: artifacts.TypeKeyLinkedInPost, ContentHash: "post-v1-hash"}
}

func TestEnsureCarouselDocumentActivityBuildsAndAttachesReviewableDocument(t *testing.T) {
	store := &carouselDocumentStore{post: carouselPostPayload(t, nil)}
	ref, err := (&PlanActivities{ArtifactStore: store}).EnsureCarouselDocumentActivity(context.Background(), EnsureCarouselDocumentInput{TenantID: uuid.NewString(), PlanExecutionID: uuid.NewString(), StepExecutionID: uuid.NewString(), Candidate: candidateRef()})
	if err != nil {
		t.Fatal(err)
	}
	if ref.ArtifactVersionID != "post-v2" || store.documentCreates != 1 || store.versionCreates != 1 {
		t.Fatalf("ref=%#v documentCreates=%d versionCreates=%d", ref, store.documentCreates, store.versionCreates)
	}
	post := &artifactsv1.LinkedInPost{}
	if err := protojson.Unmarshal(store.versionRequest.Payload, post); err != nil {
		t.Fatal(err)
	}
	if post.Carousel.DocumentArtifact.GetContentHash() != "document-hash" {
		t.Fatalf("document ref=%#v", post.Carousel.DocumentArtifact)
	}
}
func TestEnsureCarouselDocumentActivityNoopsWhenCandidateAlreadyHasDocument(t *testing.T) {
	document := &artifactsv1.ArtifactRef{ArtifactId: "document", ArtifactVersionId: "document-v1", ArtifactTypeKey: artifacts.TypeKeyLinkedInCarouselDocument, ContentHash: "document-hash"}
	store := &carouselDocumentStore{post: carouselPostPayload(t, document)}
	ref, err := (&PlanActivities{ArtifactStore: store}).EnsureCarouselDocumentActivity(context.Background(), EnsureCarouselDocumentInput{TenantID: uuid.NewString(), Candidate: candidateRef()})
	if err != nil {
		t.Fatal(err)
	}
	if ref != candidateRef() || store.documentCreates != 0 || store.versionCreates != 0 {
		t.Fatalf("ref=%#v documentCreates=%d versionCreates=%d", ref, store.documentCreates, store.versionCreates)
	}
}
