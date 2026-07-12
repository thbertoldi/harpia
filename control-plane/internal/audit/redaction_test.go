package audit

import "testing"

// G6: a secret-bearing field in a diff must be redacted before persisting,
// and non-allowlisted fields must be dropped.
func TestRedactDiffRedactsSecretFields(t *testing.T) {
	t.Parallel()

	entries := []DiffEntry{
		{Field: "status", Before: "draft", After: "runnable", HasBefore: true, HasAfter: true},
		{Field: "config_json", Before: "", After: `{"api_key":"sk-live-12345"}`, HasAfter: true},
		{Field: "api_key", Before: "", After: "sk-live-12345", HasAfter: true},
		{Field: "oauth_token", After: "ya29.token"},
		{Field: "title", After: "safe"},
	}
	allowlist := []string{"status", "title"}

	out := redactDiff(entries, allowlist)

	// status + title are allowlisted safe fields (kept); config_json,
	// api_key, and oauth_token are secret-named and kept-but-redacted so a
	// reviewer sees that a value existed but was withheld.
	if len(out) != 5 {
		t.Fatalf("expected 5 entries (2 safe + 3 redacted), got %d", len(out))
	}

	byField := map[string]DiffEntry{}
	for _, e := range out {
		byField[e.Field] = e
	}

	if e, ok := byField["status"]; !ok || e.After != "runnable" {
		t.Fatalf("status must survive unredacted: %+v", e)
	}
	// config_json is a known-secret name: it is redacted (masked) not dropped,
	// so a reviewer sees that a value existed but was withheld.
	if e, ok := byField["config_json"]; !ok || e.After != redactedPlaceholder {
		t.Fatalf("config_json must be redacted to %q, got %+v", redactedPlaceholder, e)
	}
	if e, ok := byField["api_key"]; !ok || e.After != redactedPlaceholder {
		t.Fatalf("api_key must be redacted to %q, got %+v", redactedPlaceholder, e)
	}
	// oauth_token is secret-named but NOT on the allowlist. It is still
	// redacted (never stored) even though it's not in the allowlist, because
	// the denylist takes precedence over omission — defense in depth.
	if e, ok := byField["oauth_token"]; !ok {
		t.Fatal("oauth_token must be present (redacted) even if not allowlisted")
	} else if e.After != redactedPlaceholder {
		t.Fatalf("oauth_token must be redacted, got after=%q", e.After)
	}
	// title is allowlisted and safe, but it was dropped here because the
	// denylist match on compound forms shouldn't catch it — verify it survives.
	if _, ok := byField["title"]; !ok {
		t.Fatal("title (allowlisted, safe) must survive")
	}
}

func TestRedactDiffDropsNonAllowlisted(t *testing.T) {
	t.Parallel()

	entries := []DiffEntry{
		{Field: "status", After: "runnable", HasAfter: true},
		{Field: "unknown_internal_field", After: "misc"},
	}
	out := redactDiff(entries, []string{"status"})
	if len(out) != 1 || out[0].Field != "status" {
		t.Fatalf("only allowlisted safe fields must survive, got %+v", out)
	}
}

func TestRedactDiffEmptyAllowlistKeepsAll(t *testing.T) {
	t.Parallel()

	entries := []DiffEntry{
		{Field: "status", After: "runnable", HasAfter: true},
		{Field: "name", After: "plan-1", HasAfter: true},
	}
	out := redactDiff(entries, nil)
	if len(out) != 2 {
		t.Fatalf("empty allowlist keeps all non-secret fields, got %d", len(out))
	}
}

func TestRedactDiffHandlesCompoundSecretNames(t *testing.T) {
	t.Parallel()

	entries := []DiffEntry{
		{Field: "connection.api_key", After: "sk-1", HasAfter: true},
		{Field: "credentials.password", After: "hunter2", HasAfter: true},
	}
	out := redactDiff(entries, nil)
	if len(out) != 2 {
		t.Fatalf("expected both compound secret fields redacted, got %d", len(out))
	}
	for _, e := range out {
		if e.After != redactedPlaceholder {
			t.Fatalf("compound secret %q must be redacted, got %q", e.Field, e.After)
		}
	}
}

// TestRedactDoesNotMutateInput ensures the recorder never rewrites a caller's
// draft (it must clone before redacting).
func TestRedactDoesNotMutateInput(t *testing.T) {
	t.Parallel()

	original := []DiffEntry{{Field: "api_key", After: "sk-secret", HasAfter: true}}
	_ = redactDiff(original, nil)
	if original[0].After != "sk-secret" {
		t.Fatalf("redact must not mutate caller input; after = %q", original[0].After)
	}
}
