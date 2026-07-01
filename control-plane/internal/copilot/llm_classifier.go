package copilot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/harpia/control-plane/internal/llm_config"
)

// HTTPDoer is the subset of *http.Client the adapter needs (for testability).
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// CredentialResolver resolves a tenant's LLM credential. *llm_config.Resolver
// satisfies this.
type CredentialResolver interface {
	Resolve(ctx context.Context, tenantID uuid.UUID, provider, model string) (*llm_config.ResolvedCredential, error)
}

// providerBaseURLs maps a provider to its OpenAI-compatible chat completions
// base URL. DeepSeek is OpenAI-compatible.
var providerBaseURLs = map[string]string{
	"deepseek": "https://api.deepseek.com",
	"openai":   "https://api.openai.com",
}

// providerDefaultModels is used when the resolved credential has no DefaultModel
// (e.g. the platform env-key fallback, which supplies a key but no model). This
// lets the classifier work with just HARPIA_PLATFORM_<PROVIDER>_API_KEY set.
var providerDefaultModels = map[string]string{
	"deepseek": "deepseek-chat",
	"openai":   "gpt-4o-mini",
}

const classifyTimeout = 20 * time.Second

// LLMClassifier is the production PlanClassifier. It calls the tenant's
// configured OpenAI-compatible provider once and degrades to an empty result on
// any failure so the chat flow never hard-fails on the model.
type LLMClassifier struct {
	resolver CredentialResolver
	provider string
	http     HTTPDoer
}

func NewLLMClassifier(resolver CredentialResolver, provider string, doer HTTPDoer) *LLMClassifier {
	if provider == "" {
		provider = "deepseek"
	}
	if doer == nil {
		doer = &http.Client{Timeout: classifyTimeout}
	}
	return &LLMClassifier{resolver: resolver, provider: provider, http: doer}
}

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string        `json:"model"`
	Messages       []chatMessage `json:"messages"`
	Temperature    float64       `json:"temperature"`
	ResponseFormat struct {
		Type string `json:"type"`
	} `json:"response_format"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type modelCandidates struct {
	Summary    string `json:"summary"`
	Candidates []struct {
		TemplateKey string         `json:"template_key"`
		Confidence  float64        `json:"confidence"`
		Inputs      map[string]any `json:"inputs"`
	} `json:"candidates"`
}

// Classify returns ranked candidates and an optional summary, or an empty
// result (nil error) on any soft failure (no credential, HTTP error,
// unparseable response).
func (c *LLMClassifier) Classify(ctx context.Context, in ClassifyInput) (ClassifyResult, error) {
	empty := ClassifyResult{Candidates: []Candidate{}}
	cred, err := c.resolver.Resolve(ctx, in.TenantID, c.provider, "")
	if err != nil || cred == nil || cred.APIKey.IsEmpty() {
		return empty, nil
	}
	baseURL, ok := providerBaseURLs[c.provider]
	if !ok {
		return empty, nil
	}
	model := cred.DefaultModel
	if model == "" {
		// Platform env-key credentials carry no model; fall back to a sane
		// per-provider default so a bare HARPIA_PLATFORM_<PROVIDER>_API_KEY works.
		model = providerDefaultModels[c.provider]
	}
	if model == "" {
		return empty, nil
	}

	reqBody := chatRequest{
		Model:       model,
		Temperature: 0,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt(in.Templates)},
			{Role: "user", Content: in.Text},
		},
	}
	reqBody.ResponseFormat.Type = "json_object"
	raw, err := jsonMarshal(reqBody)
	if err != nil {
		return empty, nil
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return empty, nil
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cred.APIKey.Reveal())

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return empty, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return empty, nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return empty, nil
	}

	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil || len(parsed.Choices) == 0 {
		return empty, nil
	}
	var mc modelCandidates
	if err := json.Unmarshal([]byte(parsed.Choices[0].Message.Content), &mc); err != nil {
		return empty, nil
	}

	byKey := map[string]TemplateSummary{}
	for _, t := range in.Templates {
		byKey[t.Key] = t
	}

	out := make([]Candidate, 0, len(mc.Candidates))
	for _, cand := range mc.Candidates {
		tpl, ok := byKey[cand.TemplateKey]
		if !ok {
			continue // drop unknown template keys
		}
		allowed := map[string]struct{}{}
		for _, p := range tpl.Inputs {
			allowed[p.Key] = struct{}{}
		}
		filtered := map[string]any{}
		for k, v := range cand.Inputs {
			if _, ok := allowed[k]; ok {
				filtered[k] = v
			}
		}
		inputsJSON, err := jsonMarshal(filtered)
		if err != nil {
			inputsJSON = []byte("{}")
		}
		out = append(out, Candidate{
			TemplateID:      tpl.ID,
			Confidence:      cand.Confidence,
			InputValuesJSON: string(inputsJSON),
		})
	}
	return ClassifyResult{Candidates: out, Summary: strings.TrimSpace(mc.Summary)}, nil
}

func systemPrompt(templates []TemplateSummary) string {
	type promptParam struct {
		Key      string `json:"key"`
		Label    string `json:"label"`
		Type     string `json:"type"`
		Required bool   `json:"required"`
	}
	type promptTemplate struct {
		Key         string        `json:"key"`
		Name        string        `json:"name"`
		Description string        `json:"description"`
		Inputs      []promptParam `json:"inputs"`
	}
	catalog := make([]promptTemplate, 0, len(templates))
	for _, t := range templates {
		pt := promptTemplate{Key: t.Key, Name: t.Name, Description: t.Description}
		for _, p := range t.Inputs {
			pt.Inputs = append(pt.Inputs, promptParam{Key: p.Key, Label: p.Label, Type: p.Type, Required: p.Required})
		}
		catalog = append(catalog, pt)
	}
	catalogJSON, _ := jsonMarshal(catalog)
	return fmt.Sprintf(`You route a user's request to plan templates and extract input values.
Template catalog (JSON): %s

Respond ONLY with a JSON object of this exact shape:
{"summary":"<one-line restatement of the user's request in the same language as their message>","candidates":[{"template_key":"<key from catalog>","confidence":<0.0-1.0>,"inputs":{"<input key>":"<value>"}}]}

Rules:
- summary: one short sentence restating what the user wants, in the same language as their message. Omit or use "" if unclear.
- Include at most 3 candidates, ranked by confidence (highest first).
- Only use template_key values that appear in the catalog.
- Only use input keys declared for that template. Omit inputs you cannot infer.
- If nothing in the catalog fits, return {"summary":"...","candidates":[]}.`, string(catalogJSON))
}
