package artifacts

import (
	"testing"
)

func TestValidatePayloadDateRange(t *testing.T) {
	valid := []byte(`{"startDate":"2026-01-01","endDate":"2026-01-07"}`)
	if err := ValidatePayload(TypeKeyDateRange, valid); err != nil {
		t.Fatalf("valid date range rejected: %v", err)
	}

	cases := []struct {
		name    string
		payload []byte
	}{
		{"missing end date", []byte(`{"startDate":"2026-01-01"}`)},
		{"invalid json", []byte(`{`)},
		{"empty payload", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidatePayload(TypeKeyDateRange, tc.payload); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestValidatePayloadNewsList(t *testing.T) {
	valid := []byte(`{"articles":[{"title":"Story","url":"https://example.com"}]}`)
	if err := ValidatePayload(TypeKeyNewsList, valid); err != nil {
		t.Fatalf("valid news list rejected: %v", err)
	}

	if err := ValidatePayload(TypeKeyNewsList, []byte(`{"articles":[]}`)); err == nil {
		t.Fatal("expected empty articles to fail")
	}
}

func TestValidatePayloadTextDraft(t *testing.T) {
	valid := []byte(`{"title":"Weekly","body":"Hello world"}`)
	if err := ValidatePayload(TypeKeyTextDraft, valid); err != nil {
		t.Fatalf("valid text draft rejected: %v", err)
	}
}

func TestValidatePayloadLinkedInPostDraft(t *testing.T) {
	valid := []byte(`{"text":"Launch day!"}`)
	if err := ValidatePayload(TypeKeyLinkedInPostDraft, valid); err != nil {
		t.Fatalf("valid linkedin draft rejected: %v", err)
	}
}

func TestValidatePayloadPublishConfirmation(t *testing.T) {
	valid := []byte(`{"platform":"linkedin","externalId":"123"}`)
	if err := ValidatePayload(TypeKeyPublishConfirmation, valid); err != nil {
		t.Fatalf("valid publish confirmation rejected: %v", err)
	}
}

func TestValidatePayloadUnsupportedType(t *testing.T) {
	if err := ValidatePayload("unknown.type", []byte(`{}`)); err == nil {
		t.Fatal("expected unsupported type error")
	}
}

func TestContentHashDeterministic(t *testing.T) {
	payload := []byte(`{"title":"Weekly","body":"Hello"}`)
	first := ContentHash(payload)
	second := ContentHash(payload)
	if first != second || first == "" {
		t.Fatalf("content hash mismatch: %q vs %q", first, second)
	}
}

func TestParseStorageURI(t *testing.T) {
	bucket, key, err := parseStorageURI("s3://harpia/tenant/abc/artifacts/id.json")
	if err != nil {
		t.Fatalf("parseStorageURI: %v", err)
	}
	if bucket != "harpia" || key != "tenant/abc/artifacts/id.json" {
		t.Fatalf("got bucket=%q key=%q", bucket, key)
	}
}
