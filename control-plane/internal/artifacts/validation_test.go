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

func TestValidatePayloadCarouselDraft(t *testing.T) {
	valid := []byte(`{"title":"Q3 Highlights","slides":[{"heading":"Growth","body":"Up 30%"}]}`)
	if err := ValidatePayload(TypeKeyCarouselDraft, valid); err != nil {
		t.Fatalf("valid carousel draft rejected: %v", err)
	}

	cases := []struct {
		name    string
		payload []byte
	}{
		{"missing slides", []byte(`{"title":"Q3 Highlights"}`)},
		{"empty slide", []byte(`{"title":"Q3 Highlights","slides":[{"heading":"","body":""}]}`)},
		{"invalid json", []byte(`{`)},
		{"empty payload", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidatePayload(TypeKeyCarouselDraft, tc.payload); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestValidatePayloadImageAsset(t *testing.T) {
	if err := ValidatePayload(TypeKeyImageAsset, []byte(`{"mimeType":"image/png"}`)); err != nil {
		t.Fatalf("valid image asset rejected: %v", err)
	}

	if err := ValidatePayload(TypeKeyImageAsset, []byte(`{"prompt":"a sunset"}`)); err == nil {
		t.Fatal("expected error for image asset without mime_type")
	}
}

func TestValidatePayloadLinkedInCarouselDocument(t *testing.T) {
	if err := ValidatePayload(TypeKeyLinkedInCarouselDocument, []byte(`{"mimeType":"application/pdf","fileName":"carousel.pdf"}`)); err != nil {
		t.Fatalf("valid carousel document rejected: %v", err)
	}

	if err := ValidatePayload(TypeKeyLinkedInCarouselDocument, []byte(`{"mimeType":"image/png","fileName":"carousel.png"}`)); err == nil {
		t.Fatal("expected non-PDF carousel document to fail")
	}
}

func TestValidatePayloadLinkedInPost(t *testing.T) {
	valid := []byte(`{
        "text":{"text":"Launch day!"},
        "carousel":{
          "title":"Launch carousel",
          "slides":[{"heading":"One","body":"First slide"}],
          "documentArtifact":{
            "artifactId":"document-artifact",
            "artifactVersionId":"document-version",
            "artifactTypeKey":"harpia.artifacts.v1.LinkedInCarouselDocument",
            "contentHash":"sha256:document"
          }
        },
        "images":[{
          "artifactId":"image-artifact",
          "artifactVersionId":"image-version",
          "artifactTypeKey":"harpia.artifacts.v1.ImageAsset",
          "contentHash":"sha256:image"
        }]
    }`)
	if err := ValidatePayload(TypeKeyLinkedInPost, valid); err != nil {
		t.Fatalf("valid LinkedIn post rejected: %v", err)
	}

	cases := []struct {
		name    string
		payload []byte
	}{
		{"missing text", []byte(`{}`)},
		{"incomplete image ref", []byte(`{"text":{"text":"Launch day!"},"images":[{"artifactId":"image-artifact"}]}`)},
		{"incomplete document ref", []byte(`{"text":{"text":"Launch day!"},"carousel":{"title":"Carousel","slides":[{"heading":"One"}],"documentArtifact":{"artifactId":"document-artifact"}}}`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidatePayload(TypeKeyLinkedInPost, tc.payload); err == nil {
				t.Fatal("expected validation error")
			}
		})
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
