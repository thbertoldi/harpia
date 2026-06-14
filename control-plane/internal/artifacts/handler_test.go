package artifacts

import (
	"context"
	"testing"

	"github.com/google/uuid"
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
