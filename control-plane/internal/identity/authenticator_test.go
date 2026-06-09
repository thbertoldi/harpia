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
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(strings.NewReader("bad gateway")),
				Header:     make(http.Header),
			}, nil
		})},
		time.Minute,
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
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"sub":"user-1","email":"u@example.com","name":"User One"}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		})},
		time.Minute,
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
