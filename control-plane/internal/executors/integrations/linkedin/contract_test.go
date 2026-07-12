package linkedin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/harpia/control-plane/internal/artifacts"
	"github.com/harpia/control-plane/internal/executors/runtime"
)

// This contract-level adapter test deliberately goes through Handler and the
// real HTTPPublisher; it proves the pinned post is the sole source of a real
// text publish request without contacting LinkedIn.
func TestLinkedInPublishIntegrationContract(t *testing.T) {
	var posts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/posts" {
			t.Fatalf("request=%s %s", r.Method, r.URL.Path)
		}
		posts++
		w.Header().Set("X-RestLi-Id", "urn:li:share:contract")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()
	publisher := NewHTTPPublisher(server.Client(), staticTokenResolver{token: "contract-token"})
	publisher.baseURL = server.URL
	handler := NewHandler(&handlerArtifactStore{post: handlerPost(t, false)}, publisher)
	result, err := handler.Execute(context.Background(), runtime.IntegrationExecutionRequest{TenantID: uuid.New(), OutputArtifactTypeKey: artifacts.TypeKeyPublishConfirmation, InputArtifacts: []runtime.InputArtifactRef{pinnedPostInput()}, Installation: runtime.InstallationSnapshot{ConfigJSON: oauthConfig()}})
	if err != nil || result.Status != runtime.IntegrationStatusCompleted || posts != 1 {
		t.Fatalf("result=%#v posts=%d err=%v", result, posts, err)
	}
}
