package artifacts

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
)

type mockPayloadStore struct {
	putPath string
	putBody []byte
	objects map[string][]byte
}

func (m *mockPayloadStore) Put(_ context.Context, objectPath string, payload []byte) (string, error) {
	m.putPath = objectPath
	m.putBody = append([]byte(nil), payload...)
	if m.objects == nil {
		m.objects = make(map[string][]byte)
	}
	uri := formatStorageURI("harpia", "tenant/test/"+objectPath)
	m.objects[uri] = append([]byte(nil), payload...)
	return uri, nil
}

func (m *mockPayloadStore) Get(_ context.Context, storageURI string) ([]byte, error) {
	payload, ok := m.objects[storageURI]
	if !ok {
		return nil, context.Canceled
	}
	return append([]byte(nil), payload...), nil
}

func TestArtifactObjectPath(t *testing.T) {
	artifactID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	got := ArtifactObjectPath(artifactID, "step-1")
	want := "artifacts/steps/step-1/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee.json"
	if got != want {
		t.Fatalf("ArtifactObjectPath = %q, want %q", got, want)
	}
}

func TestNewHandlerRequiresDependencies(t *testing.T) {
	if _, err := NewHandler(nil, &mockPayloadStore{}); err == nil {
		t.Fatal("expected repository required error")
	}
	if _, err := NewHandler(&Repository{}, nil); err == nil {
		t.Fatal("expected payload store required error")
	}
}

func TestCreateArtifactWithPayloadReturnsInitialVersion(t *testing.T) {
	tenantID := uuid.New()
	typeID := uuid.New()
	repo := &mockRepository{types: map[uuid.UUID]*ArtifactType{
		typeID: {ID: typeID, Key: TypeKeyTextDraft},
	}}
	handler, err := NewHandler(repo, &mockPayloadStore{})
	if err != nil {
		t.Fatalf("NewHandler() error = %v", err)
	}

	response, err := handler.CreateArtifactWithPayload(tenantContext(tenantID), connect.NewRequest(&artifactsv1.CreateArtifactWithPayloadRequest{
		TenantId: tenantID.String(), ArtifactTypeId: typeID.String(), PayloadJson: []byte(`{"title":"Title","body":"Body"}`),
	}))
	if err != nil {
		t.Fatalf("CreateArtifactWithPayload() error = %v", err)
	}
	if response.Msg.Artifact == nil || response.Msg.ArtifactVersion == nil {
		t.Fatalf("response = %#v, want artifact and artifact version", response.Msg)
	}
	if got, want := response.Msg.ArtifactVersion.VersionNumber, int32(1); got != want {
		t.Fatalf("version number = %d, want %d", got, want)
	}
	if got, want := response.Msg.ArtifactVersion.ContentHash, response.Msg.Artifact.ContentHash; got != want {
		t.Fatalf("version hash = %q, artifact hash = %q", got, want)
	}
}
