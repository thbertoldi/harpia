package linkedin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
)

type staticTokenResolver struct {
	token string
	err   error
}

func (r staticTokenResolver) ResolveAccessToken(_ context.Context, _ string) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	return r.token, nil
}

func TestHTTPPublisherPublishSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		w.Header().Set("X-RestLi-Id", "urn:li:share:123")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"permalink":"https://www.linkedin.com/feed/update/urn:li:share:123"}`))
	}))
	defer server.Close()

	publisher := NewHTTPPublisher(server.Client(), staticTokenResolver{token: "token-123"})
	publisher.baseURL = server.URL

	result, err := publisher.Publish(context.Background(), PublishRequest{
		OAuthCredentialID: "cred-123",
		Draft:             &artifactsv1.LinkedInPostDraft{Text: "hello"},
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if result.PostID != "urn:li:share:123" {
		t.Fatalf("PostID = %q, want urn:li:share:123", result.PostID)
	}
	if result.Permalink == "" {
		t.Fatal("expected permalink")
	}
}

func TestHTTPPublisherPublishOAuthReconnect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	publisher := NewHTTPPublisher(server.Client(), staticTokenResolver{token: "token-123"})
	publisher.baseURL = server.URL

	_, err := publisher.Publish(context.Background(), PublishRequest{
		OAuthCredentialID: "cred-123",
		Draft:             &artifactsv1.LinkedInPostDraft{Text: "hello"},
	})
	if err == nil {
		t.Fatal("expected oauth reconnect error")
	}
	if !IsOAuthReconnectRequired(err) {
		t.Fatalf("error = %T(%v), want oauth reconnect", err, err)
	}
}

func TestHTTPPublisherPublishTransient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("downstream unavailable"))
	}))
	defer server.Close()

	publisher := NewHTTPPublisher(server.Client(), staticTokenResolver{token: "token-123"})
	publisher.baseURL = server.URL

	_, err := publisher.Publish(context.Background(), PublishRequest{
		OAuthCredentialID: "cred-123",
		Draft:             &artifactsv1.LinkedInPostDraft{Text: "hello"},
	})
	if err == nil {
		t.Fatal("expected transient error")
	}
	var transientErr *PublishTransientError
	if !errors.As(err, &transientErr) {
		t.Fatalf("error = %T(%v), want *PublishTransientError", err, err)
	}
}
