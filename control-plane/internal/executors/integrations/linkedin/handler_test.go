package linkedin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
	"github.com/harpia/control-plane/internal/artifacts"
	"github.com/harpia/control-plane/internal/executors/runtime"
)

type handlerArtifactStore struct {
	post, pdf []byte
	loaded    []runtime.VersionedArtifactRef
}

func (s *handlerArtifactStore) LoadPayloadForType(context.Context, uuid.UUID, []runtime.InputArtifactRef, string) ([]byte, error) {
	return nil, context.Canceled
}
func (s *handlerArtifactStore) LoadPinnedPayload(_ context.Context, _ uuid.UUID, ref runtime.VersionedArtifactRef) ([]byte, error) {
	s.loaded = append(s.loaded, ref)
	if ref.ArtifactTypeKey == artifacts.TypeKeyLinkedInPost {
		return s.post, nil
	}
	if ref.ArtifactTypeKey == artifacts.TypeKeyLinkedInCarouselDocument {
		return s.pdf, nil
	}
	return nil, context.Canceled
}
func (s *handlerArtifactStore) CreateValidatedPayload(context.Context, runtime.CreateArtifactRequest) (string, error) {
	return "confirmation-id", nil
}
func (s *handlerArtifactStore) CreateValidatedPayloadRef(context.Context, runtime.CreateArtifactRequest) (runtime.VersionedArtifactRef, error) {
	return runtime.VersionedArtifactRef{}, context.Canceled
}
func (s *handlerArtifactStore) CreateValidatedVersionPayload(context.Context, runtime.CreateArtifactVersionRequest) (runtime.VersionedArtifactRef, error) {
	return runtime.VersionedArtifactRef{}, context.Canceled
}
func pinnedPostInput() runtime.InputArtifactRef {
	return runtime.InputArtifactRef{ArtifactTypeKey: artifacts.TypeKeyLinkedInPost, ArtifactID: "post-id", ArtifactVersionID: "post-v1", ContentHash: "post-hash"}
}
func handlerPost(t *testing.T, carousel bool) []byte {
	t.Helper()
	post := testPost(carousel)
	if carousel {
		post.Carousel.DocumentArtifact = &artifactsv1.ArtifactRef{ArtifactId: "doc-id", ArtifactVersionId: "doc-v1", ArtifactTypeKey: artifacts.TypeKeyLinkedInCarouselDocument, ContentHash: "doc-hash"}
	}
	payload, err := protojson.Marshal(post)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}
func oauthConfig() json.RawMessage {
	return json.RawMessage(`{"oauth_credential_id":"cred-123","author_urn":"urn:li:person:me"}`)
}

func TestHandlerPublishesOnlyVersionPinnedPostAndPinnedDocument(t *testing.T) {
	store := &handlerArtifactStore{post: handlerPost(t, true), pdf: []byte("%PDF-1.4\npinned\n%%EOF\n")}
	publisher := &FakePublisher{Result: PublishResult{PostID: "urn:li:share:123", PublishedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}}
	handler := NewHandler(store, publisher)
	result, err := handler.Execute(context.Background(), runtime.IntegrationExecutionRequest{TenantID: uuid.New(), OutputArtifactTypeKey: artifacts.TypeKeyPublishConfirmation, InputArtifacts: []runtime.InputArtifactRef{pinnedPostInput()}, Installation: runtime.InstallationSnapshot{ConfigJSON: oauthConfig()}})
	if err != nil || result.Status != runtime.IntegrationStatusCompleted {
		t.Fatalf("Execute() = %#v, %v", result, err)
	}
	if publisher.Calls != 1 || !strings.Contains(string(publisher.LastRequest.CarouselPDF), "pinned") {
		t.Fatalf("publisher = %#v", publisher)
	}
	if len(store.loaded) != 2 || store.loaded[0].ArtifactVersionID != "post-v1" || store.loaded[1].ArtifactVersionID != "doc-v1" {
		t.Fatalf("loaded refs = %#v", store.loaded)
	}
}
func TestHandlerRefusesUnpinnedPostBeforePublisher(t *testing.T) {
	handler := NewHandler(&handlerArtifactStore{}, &FakePublisher{})
	publisher := handler.publisher.(*FakePublisher)
	result, err := handler.Execute(context.Background(), runtime.IntegrationExecutionRequest{TenantID: uuid.New(), OutputArtifactTypeKey: artifacts.TypeKeyPublishConfirmation, InputArtifacts: []runtime.InputArtifactRef{{ArtifactTypeKey: artifacts.TypeKeyLinkedInPost, ArtifactID: "post-id"}}, Installation: runtime.InstallationSnapshot{ConfigJSON: oauthConfig()}})
	if err != nil || result.Status != runtime.IntegrationStatusFailed || publisher.Calls != 0 {
		t.Fatalf("result=%#v calls=%d err=%v", result, publisher.Calls, err)
	}
}
func TestHandlerRefusesCarouselWithoutPinnedDocumentBeforePublisher(t *testing.T) {
	store := &handlerArtifactStore{post: handlerPostWithoutDocument(t)}
	publisher := &FakePublisher{}
	handler := NewHandler(store, publisher)
	result, err := handler.Execute(context.Background(), runtime.IntegrationExecutionRequest{TenantID: uuid.New(), OutputArtifactTypeKey: artifacts.TypeKeyPublishConfirmation, InputArtifacts: []runtime.InputArtifactRef{pinnedPostInput()}, Installation: runtime.InstallationSnapshot{ConfigJSON: oauthConfig()}})
	if err != nil || result.Status != runtime.IntegrationStatusFailed || publisher.Calls != 0 {
		t.Fatalf("result=%#v calls=%d err=%v", result, publisher.Calls, err)
	}
}
func handlerPostWithoutDocument(t *testing.T) []byte {
	t.Helper()
	payload, err := protojson.Marshal(testPost(true))
	if err != nil {
		t.Fatal(err)
	}
	return payload
}
