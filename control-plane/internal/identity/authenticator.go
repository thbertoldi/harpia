package identity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
)

type AuthenticatedUser struct {
	Subject string
	Email   string
	Name    string
}

type BearerAuthenticator interface {
	AuthenticateBearer(ctx context.Context, token string) (AuthenticatedUser, error)
}

type ZitadelAuthenticator struct {
	userInfoURL     string
	hostHeader      string
	httpClient      *http.Client
	cacheTTL        time.Duration
	cacheMaxEntries int

	mu    sync.Mutex
	cache map[string]cachedUser
}

type cachedUser struct {
	user      AuthenticatedUser
	expiresAt time.Time
}

type zitadelUserInfo struct {
	Subject           string `json:"sub"`
	Email             string `json:"email"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
}

func NewZitadelAuthenticator(baseURL string, client *http.Client, cacheTTL time.Duration, cacheMaxEntries int) *ZitadelAuthenticator {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	userInfoURL := ""
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL != "" {
		userInfoURL = baseURL + "/oidc/v1/userinfo"
	}
	if cacheMaxEntries <= 0 {
		cacheTTL = 0
	}
	return &ZitadelAuthenticator{
		userInfoURL:     userInfoURL,
		httpClient:      client,
		cacheTTL:        cacheTTL,
		cacheMaxEntries: cacheMaxEntries,
		cache:           make(map[string]cachedUser),
	}
}

func (a *ZitadelAuthenticator) WithHostHeader(host string) *ZitadelAuthenticator {
	a.hostHeader = strings.TrimSpace(host)
	return a
}

func (a *ZitadelAuthenticator) AuthenticateBearer(ctx context.Context, token string) (AuthenticatedUser, error) {
	if a.userInfoURL == "" {
		return AuthenticatedUser{}, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("userinfo endpoint is not configured"),
		)
	}

	cacheKey := tokenCacheKey(token)
	if user, ok := a.getCached(cacheKey); ok {
		return user, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.userInfoURL, nil)
	if err != nil {
		return AuthenticatedUser{}, connect.NewError(connect.CodeInternal, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if a.hostHeader != "" {
		req.Host = a.hostHeader
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return AuthenticatedUser{}, connect.NewError(connect.CodeCanceled, err)
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return AuthenticatedUser{}, connect.NewError(connect.CodeDeadlineExceeded, err)
		}
		return AuthenticatedUser{}, connect.NewError(connect.CodeUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		_, _ = io.Copy(io.Discard, resp.Body)
		return AuthenticatedUser{}, connect.NewError(
			connect.CodeUnavailable,
			fmt.Errorf("userinfo unavailable with status %d", resp.StatusCode),
		)
	}
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return AuthenticatedUser{}, connect.NewError(
			userInfoStatusCode(resp.StatusCode),
			fmt.Errorf("userinfo rejected token with status %d", resp.StatusCode),
		)
	}

	var info zitadelUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return AuthenticatedUser{}, connect.NewError(connect.CodeUnauthenticated, err)
	}
	if info.Subject == "" {
		return AuthenticatedUser{}, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("missing user identity"),
		)
	}

	user := AuthenticatedUser{
		Subject: info.Subject,
		Email:   info.Email,
		Name:    firstNonEmpty(info.Name, info.PreferredUsername, info.Email, info.Subject),
	}
	a.setCached(cacheKey, user)
	return user, nil
}

func (a *ZitadelAuthenticator) getCached(key string) (AuthenticatedUser, bool) {
	if a.cacheTTL <= 0 {
		return AuthenticatedUser{}, false
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	cached, ok := a.cache[key]
	if !ok {
		return AuthenticatedUser{}, false
	}
	if time.Now().After(cached.expiresAt) {
		delete(a.cache, key)
		return AuthenticatedUser{}, false
	}
	return cached.user, true
}

func (a *ZitadelAuthenticator) setCached(key string, user AuthenticatedUser) {
	if a.cacheTTL <= 0 {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	for key, cached := range a.cache {
		if now.After(cached.expiresAt) {
			delete(a.cache, key)
		}
	}
	if a.cacheMaxEntries > 0 && len(a.cache) >= a.cacheMaxEntries {
		for key := range a.cache {
			delete(a.cache, key)
			break
		}
	}
	a.cache[key] = cachedUser{
		user:      user,
		expiresAt: now.Add(a.cacheTTL),
	}
}

func userInfoStatusCode(status int) connect.Code {
	switch status {
	case http.StatusUnauthorized:
		return connect.CodeUnauthenticated
	case http.StatusForbidden:
		return connect.CodePermissionDenied
	case http.StatusTooManyRequests:
		return connect.CodeResourceExhausted
	case http.StatusNotFound:
		return connect.CodeUnavailable
	default:
		return connect.CodeUnauthenticated
	}
}

func tokenCacheKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
