package copilot

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/harpia/control-plane/internal/llm_config"
	"github.com/harpia/control-plane/internal/llm_config/secret"
)

type fakeResolver struct {
	cred *llm_config.ResolvedCredential
	err  error
}

func (f fakeResolver) Resolve(ctx context.Context, tenantID uuid.UUID, provider, model string) (*llm_config.ResolvedCredential, error) {
	return f.cred, f.err
}

type fakeDoer struct {
	resp *http.Response
	err  error
}

func (f fakeDoer) Do(*http.Request) (*http.Response, error) { return f.resp, f.err }

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{},
	}
}

func okResolver() fakeResolver {
	return fakeResolver{cred: &llm_config.ResolvedCredential{
		Provider:     "deepseek",
		DefaultModel: "deepseek-chat",
		APIKey:       secret.NewRedacted("sk-test"),
	}}
}

func templates() []TemplateSummary {
	return []TemplateSummary{{
		ID:   uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		Key:  "linkedin",
		Name: "LinkedIn Post",
		Inputs: []InputParamSummary{
			{Key: "theme", Type: "TEMPLATE_INPUT_PARAMETER_TYPE_TEXT"},
			{Key: "language", Type: "TEMPLATE_INPUT_PARAMETER_TYPE_LANGUAGE"},
		},
	}}
}

func TestClassifyParsesCandidatesAndFiltersInputs(t *testing.T) {
	// The model returns a known template key, plus an unknown input key that
	// must be dropped.
	content := `{"candidates":[{"template_key":"linkedin","confidence":0.92,"inputs":{"theme":"retail","language":"pt-BR","bogus":"x"}}]}`
	body := `{"choices":[{"message":{"content":` + quote(content) + `}}]}`
	c := NewLLMClassifier(okResolver(), "deepseek", fakeDoer{resp: jsonResponse(body)})

	got, err := c.Classify(context.Background(), ClassifyInput{Text: "post about retail", Templates: templates()})
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("candidates = %d, want 1", len(got))
	}
	if got[0].TemplateID != templates()[0].ID {
		t.Fatalf("template id = %s", got[0].TemplateID)
	}
	if got[0].Confidence != 0.92 {
		t.Fatalf("confidence = %v", got[0].Confidence)
	}
	if strings.Contains(got[0].InputValuesJSON, "bogus") {
		t.Fatalf("unknown input key not dropped: %s", got[0].InputValuesJSON)
	}
	if !strings.Contains(got[0].InputValuesJSON, "retail") {
		t.Fatalf("expected theme in inputs: %s", got[0].InputValuesJSON)
	}
}

func TestClassifyUsesProviderDefaultModelWhenCredentialModelEmpty(t *testing.T) {
	// Platform env-key credentials supply a key but no DefaultModel; the
	// classifier must fall back to the per-provider default and still work.
	resolver := fakeResolver{cred: &llm_config.ResolvedCredential{
		Provider:     "deepseek",
		DefaultModel: "",
		APIKey:       secret.NewRedacted("sk-test"),
	}}
	content := `{"candidates":[{"template_key":"linkedin","confidence":0.9,"inputs":{"theme":"retail"}}]}`
	body := `{"choices":[{"message":{"content":` + quote(content) + `}}]}`
	c := NewLLMClassifier(resolver, "deepseek", fakeDoer{resp: jsonResponse(body)})

	got, err := c.Classify(context.Background(), ClassifyInput{Text: "x", Templates: templates()})
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 candidate with default model fallback, got %d", len(got))
	}
}

func TestClassifyDropsUnknownTemplateKeys(t *testing.T) {
	content := `{"candidates":[{"template_key":"nope","confidence":0.9,"inputs":{}}]}`
	body := `{"choices":[{"message":{"content":` + quote(content) + `}}]}`
	c := NewLLMClassifier(okResolver(), "deepseek", fakeDoer{resp: jsonResponse(body)})
	got, err := c.Classify(context.Background(), ClassifyInput{Text: "x", Templates: templates()})
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected unknown template dropped, got %+v", got)
	}
}

func TestClassifyDegradesToEmptyOnResolverError(t *testing.T) {
	c := NewLLMClassifier(fakeResolver{err: errors.New("no llm configured")}, "deepseek", fakeDoer{})
	got, err := c.Classify(context.Background(), ClassifyInput{Text: "x", Templates: templates()})
	if err != nil {
		t.Fatalf("expected graceful degradation, got err: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty candidates, got %+v", got)
	}
}

func TestClassifyDegradesToEmptyOnHTTPError(t *testing.T) {
	c := NewLLMClassifier(okResolver(), "deepseek", fakeDoer{err: errors.New("boom")})
	got, err := c.Classify(context.Background(), ClassifyInput{Text: "x", Templates: templates()})
	if err != nil {
		t.Fatalf("expected graceful degradation, got err: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty candidates, got %+v", got)
	}
}

// quote JSON-encodes a string (adds surrounding quotes and escapes) for
// embedding model content inside a response body literal.
func quote(s string) string {
	b, _ := jsonMarshal(s)
	return string(b)
}
