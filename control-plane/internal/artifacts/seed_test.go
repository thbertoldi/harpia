package artifacts

import (
	"testing"
)

func TestDefaultArtifactTypeSeeds(t *testing.T) {
	wantKeys := []string{
		TypeKeyDateRange,
		TypeKeyNewsList,
		TypeKeyTextDraft,
		TypeKeyLinkedInPostDraft,
		TypeKeyPublishConfirmation,
		TypeKeyCarouselDraft,
		TypeKeyImageAsset,
	}

	seen := make(map[string]bool, len(wantKeys))
	for _, seed := range DefaultArtifactTypeSeeds {
		if seed.Key == "" {
			t.Fatal("artifact type seed with empty key")
		}
		if seen[seed.Key] {
			t.Fatalf("duplicate artifact type seed key %q", seed.Key)
		}
		seen[seed.Key] = true

		if seed.SchemaRef == "" {
			t.Errorf("artifact type seed %q: schema_ref is required", seed.Key)
		}
		if seed.Description == "" {
			t.Errorf("artifact type seed %q: description is required", seed.Key)
		}
		if seed.Version <= 0 {
			t.Errorf("artifact type seed %q: version must be positive, got %d", seed.Key, seed.Version)
		}
	}

	for _, key := range wantKeys {
		if !seen[key] {
			t.Errorf("expected artifact type seed for key %q", key)
		}
	}
	if len(DefaultArtifactTypeSeeds) != len(wantKeys) {
		t.Errorf("expected %d artifact type seeds, got %d", len(wantKeys), len(DefaultArtifactTypeSeeds))
	}
}
