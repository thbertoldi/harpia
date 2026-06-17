// Package secret holds helpers for handling raw secret material safely
// inside the control plane.
//
// The Redacted type wraps a string-typed secret (e.g., a freshly decrypted
// provider API key) and implements String, GoString, and json.Marshaler so
// that accidental formatting (fmt.Println, structured log fields, JSON
// responses) never leaks the underlying value. The plaintext value is only
// reachable through the explicit Reveal method.
package secret

import (
	"encoding/json"
)

// RedactedPlaceholder is the masked rendering used everywhere a Redacted
// secret would otherwise be serialized to text or JSON.
const RedactedPlaceholder = "***REDACTED***"

// Redacted is a string-typed secret that masks itself when formatted.
//
// Callers should always store a Redacted (not a raw string) at internal
// service boundaries that handle decrypted API keys. The value is only
// accessible via Reveal(), making accidental leaks loud and grep-able.
type Redacted struct {
	value string
}

// NewRedacted wraps a plaintext secret.
func NewRedacted(value string) Redacted {
	return Redacted{value: value}
}

// Reveal returns the underlying plaintext. Callers must immediately pass it
// to the consumer (e.g., the provider SDK) and never log or persist it.
func (r Redacted) Reveal() string {
	return r.value
}

// IsEmpty reports whether the wrapped secret is empty without revealing it.
func (r Redacted) IsEmpty() bool {
	return r.value == ""
}

// String implements fmt.Stringer. Always returns the placeholder.
func (r Redacted) String() string {
	return RedactedPlaceholder
}

// GoString implements fmt.GoStringer; used by %#v.
func (r Redacted) GoString() string {
	return RedactedPlaceholder
}

// MarshalJSON implements json.Marshaler.
func (r Redacted) MarshalJSON() ([]byte, error) {
	return json.Marshal(RedactedPlaceholder)
}

// MarshalText implements encoding.TextMarshaler.
func (r Redacted) MarshalText() ([]byte, error) {
	return []byte(RedactedPlaceholder), nil
}
