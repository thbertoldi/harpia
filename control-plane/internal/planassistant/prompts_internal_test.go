package planassistant

import (
	"testing"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
)

// TestContentOutputFormatValue is a table test for the unexported value helper
// that maps the content_output_format enum onto the assistant option value.
func TestContentOutputFormatValue(t *testing.T) {
	cases := []struct {
		name string
		in   plansv1.ContentOutputFormat
		want string
	}{
		{"unspecified maps to empty", plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_UNSPECIFIED, ""},
		{"text post", plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_TEXT_POST, "text_post"},
		{"carousel", plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_CAROUSEL, "carousel"},
		{"image backed post", plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_IMAGE_BACKED_POST, "image_backed_post"},
		{"approval only", plansv1.ContentOutputFormat_CONTENT_OUTPUT_FORMAT_APPROVAL_ONLY, "approval_only"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := contentOutputFormatValue(tc.in); got != tc.want {
				t.Fatalf("contentOutputFormatValue(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
