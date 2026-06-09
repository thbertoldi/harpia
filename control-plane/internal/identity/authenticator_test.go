package identity

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
)

func TestZitadelAuthenticatorMapsServerErrorsToUnavailable(t *testing.T) {
	authenticator := NewZitadelAuthenticator(
		"http://zitadel",
		&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return textResponse(http.StatusBadGateway, "bad gateway"), nil
		})},
		time.Minute,
		1024,
	)

	_, err := authenticator.AuthenticateBearer(context.Background(), "token")
	if err == nil || connect.CodeOf(err) != connect.CodeUnavailable {
		t.Fatalf("expected unavailable, got %v", err)
	}
}

func TestZitadelAuthenticatorCachesSuccessfulUserInfo(t *testing.T) {
	var calls int32
	authenticator := NewZitadelAuthenticator(
		"http://zitadel",
		&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			atomic.AddInt32(&calls, 1)
			return jsonResponse(`{"sub":"user-1","email":"u@example.com","name":"User One"}`), nil
		})},
		time.Minute,
		1024,
	)

	for range 2 {
		user, err := authenticator.AuthenticateBearer(context.Background(), "token")
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if user.Subject != "user-1" {
			t.Fatalf("subject = %q, want user-1", user.Subject)
		}
	}
	if calls != 1 {
		t.Fatalf("userinfo calls = %d, want 1", calls)
	}
}

func TestZitadelAuthenticatorMapsClientStatuses(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   connect.Code
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, want: connect.CodeUnauthenticated},
		{name: "forbidden", status: http.StatusForbidden, want: connect.CodePermissionDenied},
		{name: "not found", status: http.StatusNotFound, want: connect.CodeUnavailable},
		{name: "rate limited", status: http.StatusTooManyRequests, want: connect.CodeResourceExhausted},
		{name: "bad request", status: http.StatusBadRequest, want: connect.CodeUnauthenticated},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authenticator := NewZitadelAuthenticator(
				"http://zitadel",
				&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return textResponse(tt.status, "rejected"), nil
				})},
				time.Minute,
				1024,
			)

			_, err := authenticator.AuthenticateBearer(context.Background(), "token")
			if err == nil || connect.CodeOf(err) != tt.want {
				t.Fatalf("expected %s, got %v", tt.want, err)
			}
		})
	}
}

func TestZitadelAuthenticatorRejectsInvalidUserInfoBodies(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty body", body: ""},
		{name: "invalid json", body: "not-json"},
		{name: "missing subject", body: `{"email":"u@example.com"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authenticator := NewZitadelAuthenticator(
				"http://zitadel",
				&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return jsonResponse(tt.body), nil
				})},
				time.Minute,
				1024,
			)

			_, err := authenticator.AuthenticateBearer(context.Background(), "token")
			if err == nil || connect.CodeOf(err) != connect.CodeUnauthenticated {
				t.Fatalf("expected unauthenticated, got %v", err)
			}
		})
	}
}

func TestZitadelAuthenticatorCacheCanBeDisabled(t *testing.T) {
	var calls int32
	authenticator := NewZitadelAuthenticator(
		"http://zitadel",
		&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			atomic.AddInt32(&calls, 1)
			return jsonResponse(`{"sub":"user-1"}`), nil
		})},
		0,
		1024,
	)

	for range 2 {
		if _, err := authenticator.AuthenticateBearer(context.Background(), "token"); err != nil {
			t.Fatalf("expected success, got %v", err)
		}
	}
	if calls != 2 {
		t.Fatalf("userinfo calls = %d, want 2", calls)
	}
}

func TestZitadelAuthenticatorEvictsExpiredCacheEntries(t *testing.T) {
	var calls int32
	authenticator := NewZitadelAuthenticator(
		"http://zitadel",
		&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			atomic.AddInt32(&calls, 1)
			return jsonResponse(`{"sub":"user-1"}`), nil
		})},
		time.Nanosecond,
		1024,
	)

	if _, err := authenticator.AuthenticateBearer(context.Background(), "token"); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	time.Sleep(time.Millisecond)
	if _, err := authenticator.AuthenticateBearer(context.Background(), "token"); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("userinfo calls = %d, want 2", calls)
	}
}

func TestZitadelAuthenticatorCapsCacheSize(t *testing.T) {
	var calls int32
	authenticator := NewZitadelAuthenticator(
		"http://zitadel",
		&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			atomic.AddInt32(&calls, 1)
			return jsonResponse(`{"sub":"user-1"}`), nil
		})},
		time.Minute,
		1,
	)

	for _, token := range []string{"token-a", "token-b"} {
		if _, err := authenticator.AuthenticateBearer(context.Background(), token); err != nil {
			t.Fatalf("expected success, got %v", err)
		}
	}
	if len(authenticator.cache) > 1 {
		t.Fatalf("cache entries = %d, want <= 1", len(authenticator.cache))
	}
	if calls != 2 {
		t.Fatalf("userinfo calls = %d, want 2", calls)
	}
}

func TestZitadelAuthenticatorMapsContextCancellation(t *testing.T) {
	authenticator := NewZitadelAuthenticator(
		"http://zitadel",
		&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, req.Context().Err()
		})},
		time.Minute,
		1024,
	)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := authenticator.AuthenticateBearer(ctx, "token")
	if err == nil || connect.CodeOf(err) != connect.CodeCanceled {
		t.Fatalf("expected canceled, got %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

func textResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
