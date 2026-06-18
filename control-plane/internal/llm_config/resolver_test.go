package llm_config

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestResolverNormalizesProviderForBlocklist(t *testing.T) {
	t.Parallel()

	repo := &Repository{}
	resolver := NewResolver(
		repo,
		nil,
		NewStaticPlatformKeyStore(nil),
		map[string]struct{}{"openai": {}},
	)

	_, err := resolver.Resolve(context.Background(), uuid.New(), "OpenAI", "")
	if err == nil {
		t.Fatal("expected blocked provider error")
	}
	rerr, ok := err.(*ResolveError)
	if !ok {
		t.Fatalf("error type = %T, want *ResolveError", err)
	}
	if rerr.Reason != ReasonProviderBlockedByPlatform {
		t.Fatalf("reason = %q, want %q", rerr.Reason, ReasonProviderBlockedByPlatform)
	}
}

func TestNormalizeProviderLowercasesInput(t *testing.T) {
	t.Parallel()

	got, err := normalizeProvider("  Anthropic ")
	if err != nil {
		t.Fatalf("normalizeProvider: %v", err)
	}
	if got != "anthropic" {
		t.Fatalf("provider = %q, want anthropic", got)
	}
}
