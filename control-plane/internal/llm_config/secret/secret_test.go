package secret

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestRedactedHidesValueAcrossFormatters(t *testing.T) {
	t.Parallel()

	r := NewRedacted("sk-ant-VERY-SECRET")

	cases := map[string]string{
		"String":       r.String(),
		"GoString":     r.GoString(),
		"Sprintf-v":    fmt.Sprintf("%v", r),
		"Sprintf-s":    fmt.Sprintf("%s", r),
		"Sprintf-q":    fmt.Sprintf("%q", r),
		"Sprintf-go":   fmt.Sprintf("%#v", r),
		"Sprintf-plus": fmt.Sprintf("%+v", r),
	}
	for name, got := range cases {
		if strings.Contains(got, "VERY-SECRET") {
			t.Errorf("%s leaked secret: %q", name, got)
		}
		if got == "" {
			t.Errorf("%s produced empty output", name)
		}
	}
}

func TestRedactedJSONMarshalNeverLeaks(t *testing.T) {
	t.Parallel()
	r := NewRedacted("sk-ant-DO-NOT-LEAK")
	payload := struct {
		APIKey Redacted `json:"api_key"`
	}{APIKey: r}

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), "DO-NOT-LEAK") {
		t.Fatalf("json leaked secret: %s", encoded)
	}
	if !strings.Contains(string(encoded), RedactedPlaceholder) {
		t.Fatalf("expected placeholder in JSON, got %s", encoded)
	}
}

func TestRedactedRevealReturnsValue(t *testing.T) {
	t.Parallel()
	r := NewRedacted("sk-ant-ABCDEF")
	if got := r.Reveal(); got != "sk-ant-ABCDEF" {
		t.Fatalf("Reveal = %q", got)
	}
	if NewRedacted("").IsEmpty() != true {
		t.Fatal("empty Redacted should be IsEmpty")
	}
}
