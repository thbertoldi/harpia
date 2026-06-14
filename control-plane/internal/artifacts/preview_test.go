package artifacts

import (
	"testing"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
)

func TestBuildPreviewTextDraft(t *testing.T) {
	payload := []byte(`{"title":"Weekly Update","body":"Hello team"}`)
	resp, err := BuildPreview(TypeKeyTextDraft, payload)
	if err != nil {
		t.Fatalf("BuildPreview: %v", err)
	}

	text := resp.GetTextPreview()
	if text != "# Weekly Update\n\nHello team" {
		t.Fatalf("text preview = %q", text)
	}
}

func TestBuildPreviewLinkedInPostDraft(t *testing.T) {
	payload := []byte(`{"hook":"Big news","text":"We launched","hashtags":["#launch","#ai"]}`)
	resp, err := BuildPreview(TypeKeyLinkedInPostDraft, payload)
	if err != nil {
		t.Fatalf("BuildPreview: %v", err)
	}

	text := resp.GetTextPreview()
	want := "Big news\n\nWe launched\n\n#launch #ai"
	if text != want {
		t.Fatalf("text preview = %q, want %q", text, want)
	}
}

func TestBuildPreviewNewsList(t *testing.T) {
	payload := []byte(`{"articles":[
		{"title":"First story","url":"https://example.com/1"},
		{"title":"Second story","url":"https://example.com/2"}
	]}`)
	resp, err := BuildPreview(TypeKeyNewsList, payload)
	if err != nil {
		t.Fatalf("BuildPreview: %v", err)
	}

	summary := resp.GetListSummary()
	if summary == nil {
		t.Fatal("expected list summary preview")
	}
	if summary.ArticleCount != 2 {
		t.Fatalf("article count = %d, want 2", summary.ArticleCount)
	}
	if len(summary.Titles) != 2 || summary.Titles[0] != "First story" {
		t.Fatalf("titles = %#v", summary.Titles)
	}
}

func TestBuildPreviewDateRangeJSON(t *testing.T) {
	payload := []byte(`{"startDate":"2026-01-01","endDate":"2026-01-07"}`)
	resp, err := BuildPreview(TypeKeyDateRange, payload)
	if err != nil {
		t.Fatalf("BuildPreview: %v", err)
	}

	jsonPreview := resp.GetJsonPreview()
	if jsonPreview == "" {
		t.Fatal("expected json preview")
	}
	if resp.GetTextPreview() != "" || resp.GetListSummary() != nil {
		t.Fatal("expected only json preview field to be set")
	}
}

func TestBuildPreviewPublishConfirmationJSON(t *testing.T) {
	payload := []byte(`{"platform":"linkedin","externalId":"post-123","url":"https://linkedin.com/in/post"}`)
	resp, err := BuildPreview(TypeKeyPublishConfirmation, payload)
	if err != nil {
		t.Fatalf("BuildPreview: %v", err)
	}
	if resp.GetJsonPreview() == "" {
		t.Fatal("expected json preview")
	}
}

func TestBuildPreviewUnsupportedType(t *testing.T) {
	_, err := BuildPreview("unknown.type", []byte(`{}`))
	if err == nil {
		t.Fatal("expected unsupported type error")
	}
}

func TestBuildPreviewNewsListLimitsTitles(t *testing.T) {
	payload := []byte(`{"articles":[
		{"title":"One","url":"https://example.com/1"},
		{"title":"Two","url":"https://example.com/2"},
		{"title":"Three","url":"https://example.com/3"},
		{"title":"Four","url":"https://example.com/4"},
		{"title":"Five","url":"https://example.com/5"},
		{"title":"Six","url":"https://example.com/6"}
	]}`)
	resp, err := BuildPreview(TypeKeyNewsList, payload)
	if err != nil {
		t.Fatalf("BuildPreview: %v", err)
	}

	summary := resp.GetListSummary()
	if summary.ArticleCount != 6 {
		t.Fatalf("article count = %d, want 6", summary.ArticleCount)
	}
	if len(summary.Titles) != maxListSummaryTitles {
		t.Fatalf("titles len = %d, want %d", len(summary.Titles), maxListSummaryTitles)
	}
}

func TestPreviewResponseOneofFields(t *testing.T) {
	resp := &artifactsv1.PreviewArtifactResponse{
		Preview: &artifactsv1.PreviewArtifactResponse_TextPreview{TextPreview: "hello"},
	}
	if resp.GetTextPreview() != "hello" {
		t.Fatalf("GetTextPreview = %q", resp.GetTextPreview())
	}
}
