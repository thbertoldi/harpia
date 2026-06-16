package linkedin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
)

const defaultBaseURL = "https://api.linkedin.com"

type PublishRequest struct {
	OAuthCredentialID string
	Draft             *artifactsv1.LinkedInPostDraft
}

type PublishResult struct {
	PostID      string
	Permalink   string
	PublishedAt time.Time
}

type LinkedInPublisher interface {
	Publish(ctx context.Context, req PublishRequest) (PublishResult, error)
}

type OAuthTokenResolver interface {
	ResolveAccessToken(ctx context.Context, oauthCredentialID string) (string, error)
}

type HTTPPublisher struct {
	client        *http.Client
	baseURL       string
	tokenResolver OAuthTokenResolver
}

func NewHTTPPublisher(client *http.Client, tokenResolver OAuthTokenResolver) *HTTPPublisher {
	if client == nil {
		client = http.DefaultClient
	}
	if tokenResolver == nil {
		tokenResolver = noopTokenResolver{}
	}
	return &HTTPPublisher{
		client:        client,
		baseURL:       defaultBaseURL,
		tokenResolver: tokenResolver,
	}
}

func (p *HTTPPublisher) Publish(ctx context.Context, req PublishRequest) (PublishResult, error) {
	oauthCredentialID, err := p.validateRequest(req)
	if err != nil {
		return PublishResult{}, err
	}
	token, err := p.resolveAccessToken(ctx, oauthCredentialID)
	if err != nil {
		return PublishResult{}, err
	}
	body, err := buildPublishRequestBody(req.Draft)
	if err != nil {
		return PublishResult{}, err
	}
	resp, bodyBytes, err := p.executePublishRequest(ctx, token, body)
	if err != nil {
		return PublishResult{}, err
	}
	return parsePublishResponse(resp, bodyBytes)
}

func (p *HTTPPublisher) validateRequest(req PublishRequest) (string, error) {
	if p == nil || p.client == nil {
		return "", fmt.Errorf("%w: linkedin publisher is not configured", ErrInvalidInput)
	}
	oauthCredentialID := strings.TrimSpace(req.OAuthCredentialID)
	if oauthCredentialID == "" {
		return "", fmt.Errorf("%w: oauth_credential_id is required", ErrInvalidInput)
	}
	if req.Draft == nil {
		return "", fmt.Errorf("%w: linkedin post draft is required", ErrInvalidInput)
	}
	return oauthCredentialID, nil
}

func (p *HTTPPublisher) resolveAccessToken(ctx context.Context, oauthCredentialID string) (string, error) {
	token, err := p.tokenResolver.ResolveAccessToken(ctx, oauthCredentialID)
	if err == nil {
		return token, nil
	}
	if IsOAuthReconnectRequired(err) {
		return "", err
	}
	return "", NewPublishTransientError(0, fmt.Errorf("resolve oauth token: %w", err))
}

func buildPublishRequestBody(draft *artifactsv1.LinkedInPostDraft) ([]byte, error) {
	postPayload := map[string]any{
		"author":         "urn:li:person:me",
		"lifecycleState": "PUBLISHED",
		"specificContent": map[string]any{
			"com.linkedin.ugc.ShareContent": map[string]any{
				"shareCommentary": map[string]any{
					"text": buildPostText(draft),
				},
				"shareMediaCategory": "NONE",
			},
		},
		"visibility": map[string]any{
			"com.linkedin.ugc.MemberNetworkVisibility": "PUBLIC",
		},
	}

	body, err := json.Marshal(postPayload)
	if err != nil {
		return nil, fmt.Errorf("marshal linkedin publish request: %w", err)
	}
	return body, nil
}

func (p *HTTPPublisher) executePublishRequest(ctx context.Context, token string, body []byte) (*http.Response, []byte, error) {
	endpoint := strings.TrimRight(p.baseURL, "/") + "/v2/ugcPosts"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("build linkedin publish request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Restli-Protocol-Version", "2.0.0")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, nil, NewPublishTransientError(0, fmt.Errorf("execute linkedin publish request: %w", err))
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	return resp, bodyBytes, nil
}

func parsePublishResponse(resp *http.Response, bodyBytes []byte) (PublishResult, error) {
	if err := classifyHTTPStatus(resp.StatusCode, bodyBytes); err != nil {
		return PublishResult{}, err
	}
	postID, permalink := parsePublishIdentifiers(resp, bodyBytes)
	return PublishResult{
		PostID:      postID,
		Permalink:   permalink,
		PublishedAt: time.Now().UTC(),
	}, nil
}

func classifyHTTPStatus(statusCode int, bodyBytes []byte) error {
	switch {
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return NewOAuthReconnectError(
			"linkedin oauth credential is expired or invalid; reconnect required",
			fmt.Errorf("linkedin response status %d", statusCode),
		)
	case statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError:
		return NewPublishTransientError(
			statusCode,
			fmt.Errorf("linkedin response status %d: %s", statusCode, strings.TrimSpace(string(bodyBytes))),
		)
	case statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices:
		return fmt.Errorf(
			"%w: linkedin publish failed with status %d: %s",
			ErrInvalidInput,
			statusCode,
			strings.TrimSpace(string(bodyBytes)),
		)
	}
	return nil
}

func parsePublishIdentifiers(resp *http.Response, bodyBytes []byte) (string, string) {
	postID := strings.TrimSpace(resp.Header.Get("X-RestLi-Id"))
	permalink := ""
	if len(bodyBytes) > 0 {
		var payload struct {
			ID        string `json:"id"`
			URN       string `json:"urn"`
			Permalink string `json:"permalink"`
		}
		if err := json.Unmarshal(bodyBytes, &payload); err == nil {
			if postID == "" {
				postID = strings.TrimSpace(payload.ID)
			}
			if postID == "" {
				postID = strings.TrimSpace(payload.URN)
			}
			permalink = strings.TrimSpace(payload.Permalink)
		}
	}
	return postID, permalink
}

type noopTokenResolver struct{}

func (noopTokenResolver) ResolveAccessToken(_ context.Context, _ string) (string, error) {
	return "", NewOAuthReconnectError("linkedin oauth credential is missing or disconnected; reconnect required", nil)
}

type noopPublisher struct{}

func NewNoopPublisher() LinkedInPublisher {
	return noopPublisher{}
}

func (noopPublisher) Publish(_ context.Context, _ PublishRequest) (PublishResult, error) {
	return PublishResult{}, NewOAuthReconnectError(
		"linkedin publishing is not configured; reconnect required",
		nil,
	)
}

// FakePublisher is a deterministic test double for handler tests.
type FakePublisher struct {
	Result      PublishResult
	Err         error
	LastRequest PublishRequest
	Calls       int
}

func (p *FakePublisher) Publish(_ context.Context, req PublishRequest) (PublishResult, error) {
	p.LastRequest = req
	p.Calls++
	if p.Err != nil {
		return PublishResult{}, p.Err
	}
	return p.Result, nil
}

func buildPostText(draft *artifactsv1.LinkedInPostDraft) string {
	if draft == nil {
		return ""
	}

	parts := make([]string, 0, 3)
	if hook := strings.TrimSpace(draft.GetHook()); hook != "" {
		parts = append(parts, hook)
	}
	if text := strings.TrimSpace(draft.GetText()); text != "" {
		parts = append(parts, text)
	}
	if len(draft.GetHashtags()) > 0 {
		tags := make([]string, 0, len(draft.GetHashtags()))
		for _, hashtag := range draft.GetHashtags() {
			tag := strings.TrimSpace(hashtag)
			if tag == "" {
				continue
			}
			if !strings.HasPrefix(tag, "#") {
				tag = "#" + tag
			}
			tags = append(tags, tag)
		}
		if len(tags) > 0 {
			parts = append(parts, strings.Join(tags, " "))
		}
	}

	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}
