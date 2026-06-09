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
	userInfoURL string
	httpClient  *http.Client
	cacheTTL    time.Duration

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

func NewZitadelAuthenticator(baseURL string, client *http.Client, cacheTTL time.Duration) *ZitadelAuthenticator {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	userInfoURL := ""
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL != "" {
		userInfoURL = baseURL + "/oidc/v1/userinfo"
	}
	return &ZitadelAuthenticator{
		userInfoURL: userInfoURL,
		httpClient:  client,
		cacheTTL:    cacheTTL,
		cache:       make(map[string]cachedUser),
	}
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

	resp, err := a.httpClient.Do(req)
	if err != nil {
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
			connect.CodeUnauthenticated,
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
	a.cache[key] = cachedUser{
		user:      user,
		expiresAt: time.Now().Add(a.cacheTTL),
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
