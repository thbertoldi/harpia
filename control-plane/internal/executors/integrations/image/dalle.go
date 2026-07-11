package image

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const openAIImagesEndpoint = "https://api.openai.com/v1/images/generations"

// openAIImagesAPI is the minimal surface of the OpenAI Images API the DALL-E
// adapter depends on, abstracted so tests can inject an httptest server.
type openAIImagesAPI interface {
	Generate(ctx context.Context, apiKey string, body imagesGenerateRequest) (imagesGenerateResponse, error)
}

// DallEProvider calls the OpenAI Images API. The API key is resolved lazily
// from the worker environment and never stored on the provider struct in
// config_json.
type DallEProvider struct {
	api    openAIImagesAPI
	apiKey string
}

// NewDallEProvider constructs a DALL-E adapter that authenticates with the
// given API key. The key is read from the worker environment by the provider
// resolver, not from installation config_json.
func NewDallEProvider(apiKey string) *DallEProvider {
	return &DallEProvider{
		api:    newHTTPOpenAIAPI(http.DefaultClient),
		apiKey: strings.TrimSpace(apiKey),
	}
}

func newHTTPOpenAIAPI(client *http.Client) openAIImagesAPI {
	if client == nil {
		client = http.DefaultClient
	}
	return &httpOpenAIAPI{client: client}
}

func (p *DallEProvider) Generate(ctx context.Context, req GenerateRequest) (GenerateResult, error) {
	if p == nil || p.api == nil {
		return GenerateResult{}, NewProviderError(ProviderOpenAI, fmt.Errorf("%w: provider is not configured", ErrInvalidInput))
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return GenerateResult{}, fmt.Errorf("%w: prompt is required", ErrInvalidInput)
	}
	if p.apiKey == "" {
		return GenerateResult{}, NewProviderError(ProviderOpenAI, fmt.Errorf("openai api key is not set"))
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = defaultModel
	}
	size := strings.TrimSpace(req.Size)
	if size == "" {
		size = defaultSize
	}
	quality := strings.TrimSpace(req.Quality)
	if quality == "" {
		quality = defaultQuality
	}

	resp, err := p.api.Generate(ctx, p.apiKey, imagesGenerateRequest{
		Model:          model,
		Prompt:         req.Prompt,
		N:              1,
		Size:           size,
		Quality:        quality,
		ResponseFormat: "b64_json",
	})
	if err != nil {
		return GenerateResult{}, err
	}
	if len(resp.Data) == 0 || strings.TrimSpace(resp.Data[0].B64JSON) == "" {
		return GenerateResult{}, NewProviderError(ProviderOpenAI, fmt.Errorf("openai response carried no image data"))
	}

	imageBytes, err := decodeBase64(resp.Data[0].B64JSON)
	if err != nil {
		return GenerateResult{}, NewProviderError(ProviderOpenAI, fmt.Errorf("decode image base64: %w", err))
	}

	width, height := parseDimensions(size, 0, 0)
	return GenerateResult{
		ImageBytes: imageBytes,
		MimeType:   "image/png",
		Width:      int32(width),
		Height:     int32(height),
	}, nil
}

type imagesGenerateRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	N              int    `json:"n"`
	Size           string `json:"size"`
	Quality        string `json:"quality,omitempty"`
	ResponseFormat string `json:"response_format"`
}

type imagesGenerateResponse struct {
	Data []struct {
		B64JSON string `json:"b64_json"`
		URL     string `json:"url"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

type httpOpenAIAPI struct {
	client *http.Client
}

func (a *httpOpenAIAPI) Generate(ctx context.Context, apiKey string, body imagesGenerateRequest) (imagesGenerateResponse, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return imagesGenerateResponse{}, NewProviderError(ProviderOpenAI, fmt.Errorf("marshal request: %w", err))
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIImagesEndpoint, bytes.NewReader(payload))
	if err != nil {
		return imagesGenerateResponse{}, NewProviderError(ProviderOpenAI, fmt.Errorf("build request: %w", err))
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return imagesGenerateResponse{}, NewProviderError(ProviderOpenAI, fmt.Errorf("execute request: %w", err))
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))

	switch {
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError:
		return imagesGenerateResponse{}, NewProviderError(ProviderOpenAI, fmt.Errorf("openai response status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw))))
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return imagesGenerateResponse{}, fmt.Errorf("%w: openai authentication failed (status %d)", ErrInvalidInput, resp.StatusCode)
	case resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices:
		return imagesGenerateResponse{}, fmt.Errorf("%w: openai request failed with status %d: %s", ErrInvalidInput, resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var parsed imagesGenerateResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return imagesGenerateResponse{}, NewProviderError(ProviderOpenAI, fmt.Errorf("decode response: %w", err))
	}
	if parsed.Error != nil {
		return imagesGenerateResponse{}, fmt.Errorf("%w: openai error: %s", ErrInvalidInput, strings.TrimSpace(parsed.Error.Message))
	}
	return parsed, nil
}
