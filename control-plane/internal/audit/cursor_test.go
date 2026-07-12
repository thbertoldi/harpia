package audit

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCursorRoundTrip(t *testing.T) {
	t.Parallel()

	scope := scopeDigest(Filters{EventTypes: []string{eventPlanExecutionStarted}})
	occurredAt := time.Date(2026, 7, 12, 10, 0, 0, 12345, time.UTC)
	eventID := uuid.New()

	token := encodeCursor(occurredAt, eventID, scope)
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	gotAt, gotID, err := decodeCursor(token, scope)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !gotAt.Equal(occurredAt) {
		t.Fatalf("occurred_at = %v, want %v", gotAt, occurredAt)
	}
	if gotID != eventID {
		t.Fatalf("event id = %v, want %v", gotID, eventID)
	}
}

func TestCursorRejectsEmptyEventID(t *testing.T) {
	t.Parallel()

	if token := encodeCursor(time.Now(), uuid.Nil, "scope"); token != "" {
		t.Fatalf("expected empty token for nil id, got %q", token)
	}
}

func TestCursorRejectsMalformed(t *testing.T) {
	t.Parallel()

	scope := scopeDigest(Filters{})
	for _, token := range []string{"!", "not-baseurl!!", "e30="} { // bad encoding, bad encoding, empty json {}
		if _, _, err := decodeCursor(token, scope); err == nil {
			t.Fatalf("expected error for token %q", token)
		}
	}
}

// G5: a token minted for filter A must not decode under filter B.
func TestCursorRejectsMismatchedScope(t *testing.T) {
	t.Parallel()

	scopeA := scopeDigest(Filters{EventTypes: []string{eventPlanExecutionStarted}})
	scopeB := scopeDigest(Filters{EventTypes: []string{eventPlanExecutionCompleted}})

	token := encodeCursor(time.Now().UTC(), uuid.New(), scopeA)
	if _, _, err := decodeCursor(token, scopeB); err == nil {
		t.Fatal("expected scope mismatch error, got nil")
	}
}

func TestScopeDigestStableForEquivalentFilters(t *testing.T) {
	t.Parallel()

	a := scopeDigest(Filters{EventTypes: []string{eventPlanExecutionStarted, eventPlanExecutionCompleted}})
	// Same set, different input order — digest must be identical.
	b := scopeDigest(Filters{EventTypes: []string{eventPlanExecutionCompleted, eventPlanExecutionStarted}})
	if a != b {
		t.Fatalf("digest must be order-independent: a=%s b=%s", a, b)
	}
}

func TestScopeDigestDistinguishesFilters(t *testing.T) {
	t.Parallel()

	base := Filters{EventTypes: []string{eventPlanExecutionStarted}}
	if scopeDigest(base) == scopeDigest(Filters{EventTypes: []string{eventPlanExecutionCompleted}}) {
		t.Fatal("digests must differ for different event types")
	}
	if scopeDigest(base) == scopeDigest(Filters{SubjectID: "subject-1"}) {
		t.Fatal("digests must differ when subject filter added")
	}
	if scopeDigest(base) == scopeDigest(Filters{Actor: &ActorFilter{Kind: actorKindHuman, ID: "u-1"}}) {
		t.Fatal("digests must differ when actor filter added")
	}
}
