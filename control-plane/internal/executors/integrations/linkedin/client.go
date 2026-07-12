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

const (
	defaultBaseURL         = "https://api.linkedin.com"
	defaultLinkedInVersion = "202501"
)

type PublishRequest struct {
	OAuthCredentialID string
	AuthorURN         string
	Post              *artifactsv1.LinkedInPost
	CarouselPDF       []byte
}

type PublishResult struct {
	PostID, Permalink string
	PublishedAt       time.Time
}
type LinkedInPublisher interface {
	Publish(context.Context, PublishRequest) (PublishResult, error)
}
type OAuthTokenResolver interface {
	ResolveAccessToken(context.Context, string) (string, error)
}

type HTTPPublisher struct {
	client          *http.Client
	baseURL         string
	linkedInVersion string
	tokenResolver   OAuthTokenResolver
}

func NewHTTPPublisher(client *http.Client, tokenResolver OAuthTokenResolver) *HTTPPublisher {
	if client == nil {
		client = http.DefaultClient
	}
	if tokenResolver == nil {
		tokenResolver = noopTokenResolver{}
	}
	return &HTTPPublisher{client: client, baseURL: defaultBaseURL, linkedInVersion: defaultLinkedInVersion, tokenResolver: tokenResolver}
}

// Publish uses LinkedIn's Posts API. A carousel uses exactly initializeUpload,
// upload, then a document media post; a text-only post calls only /rest/posts.
func (p *HTTPPublisher) Publish(ctx context.Context, req PublishRequest) (PublishResult, error) {
	if err := p.validateRequest(req); err != nil {
		return PublishResult{}, err
	}
	token, err := p.resolveAccessToken(ctx, req.OAuthCredentialID)
	if err != nil {
		return PublishResult{}, err
	}
	if req.Post.GetCarousel() == nil {
		return p.createPost(ctx, token, req.AuthorURN, buildPostText(req.Post.GetText()), "")
	}
	documentURN, uploadURL, err := p.initializeDocumentUpload(ctx, token, req.AuthorURN)
	if err != nil {
		return PublishResult{}, err
	}
	if err := p.uploadDocument(ctx, token, uploadURL, req.CarouselPDF); err != nil {
		return PublishResult{}, err
	}
	return p.createPost(ctx, token, req.AuthorURN, buildPostText(req.Post.GetText()), documentURN)
}

func (p *HTTPPublisher) validateRequest(req PublishRequest) error {
	if p == nil || p.client == nil {
		return fmt.Errorf("%w: linkedin publisher is not configured", ErrInvalidInput)
	}
	if strings.TrimSpace(req.OAuthCredentialID) == "" {
		return fmt.Errorf("%w: oauth_credential_id is required", ErrInvalidInput)
	}
	if strings.TrimSpace(req.AuthorURN) == "" {
		return fmt.Errorf("%w: author_urn is required", ErrInvalidInput)
	}
	if req.Post == nil || req.Post.Text == nil || strings.TrimSpace(req.Post.Text.Text) == "" {
		return fmt.Errorf("%w: version-pinned LinkedIn post text is required", ErrInvalidInput)
	}
	if req.Post.Carousel != nil && len(req.CarouselPDF) == 0 {
		return fmt.Errorf("%w: carousel PDF bytes are required", ErrInvalidInput)
	}
	return nil
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

func (p *HTTPPublisher) initializeDocumentUpload(ctx context.Context, token, authorURN string) (string, string, error) {
	body, _ := json.Marshal(map[string]any{"initializeUploadRequest": map[string]any{"owner": authorURN}})
	resp, response, err := p.request(ctx, token, http.MethodPost, strings.TrimRight(p.baseURL, "/")+"/rest/documents?action=initializeUpload", body, "application/json")
	if err != nil {
		return "", "", err
	}
	if err := classifyHTTPStatus(resp.StatusCode, response); err != nil {
		return "", "", err
	}
	var payload struct {
		Value struct {
			Document  string `json:"document"`
			UploadURL string `json:"uploadUrl"`
		} `json:"value"`
	}
	if err := json.Unmarshal(response, &payload); err != nil {
		return "", "", fmt.Errorf("parse LinkedIn document initialization: %w", err)
	}
	if strings.TrimSpace(payload.Value.Document) == "" || strings.TrimSpace(payload.Value.UploadURL) == "" {
		return "", "", fmt.Errorf("%w: LinkedIn document initialization response missing document or upload URL", ErrInvalidInput)
	}
	return payload.Value.Document, payload.Value.UploadURL, nil
}

func (p *HTTPPublisher) uploadDocument(ctx context.Context, token, uploadURL string, pdf []byte) error {
	resp, body, err := p.request(ctx, token, http.MethodPut, uploadURL, pdf, "application/pdf")
	if err != nil {
		return err
	}
	return classifyHTTPStatus(resp.StatusCode, body)
}

func (p *HTTPPublisher) createPost(ctx context.Context, token, authorURN, text, documentURN string) (PublishResult, error) {
	content := map[string]any{"shareMediaCategory": "NONE"}
	if documentURN != "" {
		content = map[string]any{"media": map[string]any{"id": documentURN}}
	}
	body, err := json.Marshal(map[string]any{
		"author": authorURN, "commentary": text, "visibility": "PUBLIC", "lifecycleState": "PUBLISHED",
		"distribution": map[string]any{"feedDistribution": "MAIN_FEED", "targetEntities": []string{}, "thirdPartyDistributionChannels": []string{}},
		"content":      content,
	})
	if err != nil {
		return PublishResult{}, fmt.Errorf("marshal LinkedIn post request: %w", err)
	}
	resp, response, err := p.request(ctx, token, http.MethodPost, strings.TrimRight(p.baseURL, "/")+"/rest/posts", body, "application/json")
	if err != nil {
		return PublishResult{}, err
	}
	if err := classifyHTTPStatus(resp.StatusCode, response); err != nil {
		return PublishResult{}, err
	}
	postID, permalink := parsePublishIdentifiers(resp, response)
	return PublishResult{PostID: postID, Permalink: permalink, PublishedAt: time.Now().UTC()}, nil
}

func (p *HTTPPublisher) request(ctx context.Context, token, method, endpoint string, body []byte, contentType string) (*http.Response, []byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("build LinkedIn request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", contentType)
	httpReq.Header.Set("X-Restli-Protocol-Version", "2.0.0")
	httpReq.Header.Set("Linkedin-Version", p.linkedInVersion)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, nil, NewPublishTransientError(0, fmt.Errorf("execute LinkedIn request: %w", err))
	}
	defer resp.Body.Close()
	response, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	return resp, response, nil
}

func classifyHTTPStatus(statusCode int, body []byte) error {
	switch {
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return NewOAuthReconnectError("linkedin oauth credential is expired or invalid; reconnect required", fmt.Errorf("linkedin response status %d", statusCode))
	case statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError:
		return NewPublishTransientError(statusCode, fmt.Errorf("linkedin response status %d: %s", statusCode, strings.TrimSpace(string(body))))
	case statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices:
		return fmt.Errorf("%w: LinkedIn publish failed with status %d: %s", ErrInvalidInput, statusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func parsePublishIdentifiers(resp *http.Response, body []byte) (string, string) {
	postID, permalink := strings.TrimSpace(resp.Header.Get("X-RestLi-Id")), ""
	var payload struct {
		ID        string `json:"id"`
		URN       string `json:"urn"`
		Permalink string `json:"permalink"`
	}
	if json.Unmarshal(body, &payload) == nil {
		if postID == "" {
			postID = firstNonEmpty(payload.ID, payload.URN)
		}
		permalink = strings.TrimSpace(payload.Permalink)
	}
	return postID, permalink
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

type noopTokenResolver struct{}

func (noopTokenResolver) ResolveAccessToken(context.Context, string) (string, error) {
	return "", NewOAuthReconnectError("linkedin oauth credential is missing or disconnected; reconnect required", nil)
}

type noopPublisher struct{}

func NewNoopPublisher() LinkedInPublisher { return noopPublisher{} }
func (noopPublisher) Publish(context.Context, PublishRequest) (PublishResult, error) {
	return PublishResult{}, NewOAuthReconnectError("linkedin publishing is not configured; reconnect required", nil)
}

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
	if hook := strings.TrimSpace(draft.Hook); hook != "" {
		parts = append(parts, hook)
	}
	if text := strings.TrimSpace(draft.Text); text != "" {
		parts = append(parts, text)
	}
	tags := make([]string, 0, len(draft.Hashtags))
	for _, tag := range draft.Hashtags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			if !strings.HasPrefix(tag, "#") {
				tag = "#" + tag
			}
			tags = append(tags, tag)
		}
	}
	if len(tags) > 0 {
		parts = append(parts, strings.Join(tags, " "))
	}
	return strings.Join(parts, "\n\n")
}
