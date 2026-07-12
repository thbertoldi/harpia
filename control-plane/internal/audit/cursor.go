package audit

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// cursorVersion lets future token formats reject cleanly instead of
// mis-decoding. v1 is the only recognized version today.
const cursorVersion = 1

// cursorPayload is the internal shape of the opaque page token. It carries
// the last (occurred_at, id) tuple returned and a short digest of the
// normalized filter scope, so a token minted under one filter set cannot be
// replayed against a different one.
type cursorPayload struct {
	Version    int    `json:"v"`
	OccurredAt string `json:"o"`
	EventID    string `json:"i"`
	Scope      string `json:"s"`
}

// encodeCursor mints an opaque base64url token for the given tuple and
// filter scope. It returns "" when there is no next page.
func encodeCursor(occurredAt time.Time, eventID uuid.UUID, scope string) string {
	if eventID == uuid.Nil {
		return ""
	}
	payload := cursorPayload{
		Version:    cursorVersion,
		OccurredAt: occurredAt.UTC().Format(time.RFC3339Nano),
		EventID:    eventID.String(),
		Scope:      scope,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

// decodeCursor reverses encodeCursor. It returns ErrInvalidCursor when the
// token is malformed, the wrong version, or its scope digest does not match
// the supplied filter scope (a token minted for filter A may not page under
// filter B).
func decodeCursor(token, scope string) (time.Time, uuid.UUID, error) {
	if token == "" {
		return time.Time{}, uuid.Nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("%w: bad encoding", ErrInvalidCursor)
	}
	var payload cursorPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("%w: bad payload", ErrInvalidCursor)
	}
	if payload.Version != cursorVersion {
		return time.Time{}, uuid.Nil, fmt.Errorf("%w: unsupported version", ErrInvalidCursor)
	}
	if payload.Scope != scope {
		return time.Time{}, uuid.Nil, fmt.Errorf("%w: filter scope mismatch", ErrInvalidCursor)
	}
	occurredAt, err := time.Parse(time.RFC3339Nano, payload.OccurredAt)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("%w: bad occurred_at", ErrInvalidCursor)
	}
	eventID, err := uuid.Parse(payload.EventID)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("%w: bad event id", ErrInvalidCursor)
	}
	if eventID == uuid.Nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("%w: nil event id", ErrInvalidCursor)
	}
	return occurredAt.UTC(), eventID, nil
}

// scopeDigest returns a short, stable hex digest over the normalized filter
// set. Two requests with the same effective filters produce the same digest,
// so a page token is only valid for the filter scope that minted it.
func scopeDigest(filters Filters) string {
	h := sha256.New()
	enc := json.NewEncoder(h)
	// Deterministic representation of the filter scope: the struct field
	// order is fixed, and event types are pre-sorted, so two equivalent
	// filter sets always hash identically.
	scope := struct {
		EventTypes []string `json:"e,omitempty"`
		ActorKind  string   `json:"ak,omitempty"`
		ActorID    string   `json:"ai,omitempty"`
		AgentType  string   `json:"at,omitempty"`
		DateFrom   string   `json:"df,omitempty"`
		DateTo     string   `json:"dt,omitempty"`
		SubjectID  string   `json:"s,omitempty"`
		Decision   string   `json:"d,omitempty"`
	}{
		EventTypes: sortedCopy(filters.EventTypes),
		SubjectID:  filters.SubjectID,
		Decision:   filters.Decision,
	}
	if filters.Actor != nil {
		scope.ActorKind = filters.Actor.Kind
		scope.ActorID = filters.Actor.ID
		scope.AgentType = filters.Actor.AgentType
	}
	if !filters.DateFrom.IsZero() {
		scope.DateFrom = filters.DateFrom.UTC().Format(time.RFC3339Nano)
	}
	if !filters.DateTo.IsZero() {
		scope.DateTo = filters.DateTo.UTC().Format(time.RFC3339Nano)
	}
	_ = enc.Encode(scope)
	return hex.EncodeToString(h.Sum(nil))
}

func sortedCopy(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
