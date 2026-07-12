package linkedin

import (
	"bytes"
	"testing"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
)

func TestBuildCarouselDocumentIsByteDeterministic(t *testing.T) {
	draft := &artifactsv1.CarouselDraft{Title: "Operational clarity", Hook: "A stable review artifact", Slides: []*artifactsv1.CarouselSlide{{Heading: "First", Body: "The exact same structured slide input must produce the exact same PDF bytes."}, {Heading: "Second", Body: "No timestamps, UUIDs, compression output, or unordered iteration."}}}
	first, err := BuildCarouselDocument(draft)
	if err != nil {
		t.Fatalf("first build: %v", err)
	}
	second, err := BuildCarouselDocument(draft)
	if err != nil {
		t.Fatalf("second build: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("identical CarouselDraft input produced different PDF bytes")
	}
	if !bytes.HasPrefix(first, []byte("%PDF-1.4")) || !bytes.Contains(first, []byte("startxref")) {
		t.Fatalf("output is not a complete PDF: %q", first[:min(len(first), 20)])
	}
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
