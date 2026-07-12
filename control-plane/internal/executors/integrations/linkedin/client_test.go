package linkedin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
)

type staticTokenResolver struct {
	token string
	err   error
}

func (r staticTokenResolver) ResolveAccessToken(context.Context, string) (string, error) {
	return r.token, r.err
}
func testPost(carousel bool) *artifactsv1.LinkedInPost {
	post := &artifactsv1.LinkedInPost{Text: &artifactsv1.LinkedInPostDraft{Text: "Pinned text", Hashtags: []string{"harpia"}}}
	if carousel {
		post.Carousel = &artifactsv1.CarouselDraft{Title: "Pinned carousel", Slides: []*artifactsv1.CarouselSlide{{Heading: "One"}}}
	}
	return post
}

func TestHTTPPublisherPublishTextOnlyPostsPinnedTextOnce(t *testing.T) {
	var calls []string
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		if r.URL.Path != "/rest/posts" || r.Method != http.MethodPost {
			t.Fatalf("request = %s %s, want POST /rest/posts", r.Method, r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["commentary"] != "Pinned text\n\n#harpia" {
			t.Fatalf("commentary = %#v", body["commentary"])
		}
		content := body["content"].(map[string]any)
		if content["shareMediaCategory"] != "NONE" {
			t.Fatalf("content = %#v, want text-only media category", content)
		}
		w.Header().Set("X-RestLi-Id", "urn:li:share:123")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()
	publisher := NewHTTPPublisher(server.Client(), staticTokenResolver{token: "token-123"})
	publisher.baseURL = server.URL
	if _, err := publisher.Publish(context.Background(), PublishRequest{OAuthCredentialID: "cred", AuthorURN: "urn:li:person:me", Post: testPost(false)}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if len(calls) != 1 || calls[0] != "/rest/posts" {
		t.Fatalf("calls = %#v, want one text post", calls)
	}
}

func TestHTTPPublisherPublishCarouselUsesDocumentSequenceAndExactBytes(t *testing.T) {
	pdf := []byte("%PDF-1.4\npinned-carousel-bytes\n%%EOF\n")
	var calls []string
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/rest/documents":
			if r.URL.Query().Get("action") != "initializeUpload" {
				t.Fatalf("query = %s", r.URL.RawQuery)
			}
			var body map[string]map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["initializeUploadRequest"]["owner"] != "urn:li:person:me" {
				t.Fatalf("initialize body = %#v", body)
			}
			_, _ = w.Write([]byte(`{"value":{"document":"urn:li:document:99","uploadUrl":"` + server.URL + `/upload/99"}}`))
		case "/upload/99":
			uploaded, _ := io.ReadAll(r.Body)
			if !bytes.Equal(uploaded, pdf) {
				t.Fatalf("uploaded bytes = %q, want %q", uploaded, pdf)
			}
			w.WriteHeader(http.StatusCreated)
		case "/rest/posts":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			media := body["content"].(map[string]any)["media"].(map[string]any)
			if media["id"] != "urn:li:document:99" {
				t.Fatalf("post body = %#v", body)
			}
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request %s", r.URL.Path)
		}
	}))
	defer server.Close()
	publisher := NewHTTPPublisher(server.Client(), staticTokenResolver{token: "token-123"})
	publisher.baseURL = server.URL
	if _, err := publisher.Publish(context.Background(), PublishRequest{OAuthCredentialID: "cred", AuthorURN: "urn:li:person:me", Post: testPost(true), CarouselPDF: pdf}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	want := []string{"POST /rest/documents", "PUT /upload/99", "POST /rest/posts"}
	if !bytes.Equal([]byte(join(calls)), []byte(join(want))) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}
func join(values []string) string {
	var out string
	for _, value := range values {
		out += "|" + value
	}
	return out
}
func TestHTTPPublisherPublishOAuthReconnect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	defer server.Close()
	publisher := NewHTTPPublisher(server.Client(), staticTokenResolver{token: "token"})
	publisher.baseURL = server.URL
	_, err := publisher.Publish(context.Background(), PublishRequest{OAuthCredentialID: "cred", AuthorURN: "urn:li:person:me", Post: testPost(false)})
	if err == nil || !IsOAuthReconnectRequired(err) {
		t.Fatalf("error = %v, want reconnect", err)
	}
}
func TestHTTPPublisherPublishTransient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer server.Close()
	publisher := NewHTTPPublisher(server.Client(), staticTokenResolver{token: "token"})
	publisher.baseURL = server.URL
	_, err := publisher.Publish(context.Background(), PublishRequest{OAuthCredentialID: "cred", AuthorURN: "urn:li:person:me", Post: testPost(false)})
	var transient *PublishTransientError
	if !errors.As(err, &transient) {
		t.Fatalf("error = %v, want transient", err)
	}
}
