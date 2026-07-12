package artifacts

import (
	"strings"
	"testing"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
)

func TestBuildPreviewTextDraft(t *testing.T) {
	payload := []byte(`{"title":"Weekly Update","body":"Hello team"}`)
	resp, err := BuildPreview(TypeKeyTextDraft, payload)
	if err != nil {
		t.Fatalf("BuildPreview: %v", err)
	}

	md := resp.GetMarkdownPreview()
	if md != "# Weekly Update\n\nHello team" {
		t.Fatalf("markdown preview = %q", md)
	}
	if resp.GetTextPreview() != "" {
		t.Fatal("expected markdown variant, not text")
	}
}

func TestBuildPreviewLinkedInPostDraft(t *testing.T) {
	payload := []byte(`{"hook":"Big news","text":"We launched","hashtags":["#launch","#ai"]}`)
	resp, err := BuildPreview(TypeKeyLinkedInPostDraft, payload)
	if err != nil {
		t.Fatalf("BuildPreview: %v", err)
	}

	md := resp.GetMarkdownPreview()
	want := "Big news\n\nWe launched\n\n#launch #ai"
	if md != want {
		t.Fatalf("markdown preview = %q, want %q", md, want)
	}
}

func TestBuildPreviewLinkedInPost(t *testing.T) {
	payload := []byte(`{
        "text":{"hook":"Big news","text":"We launched","hashtags":["#launch"]},
        "carousel":{"title":"Launch deck","slides":[{"heading":"One","body":"First slide"}]},
        "images":[{
          "artifactId":"image-artifact",
          "artifactVersionId":"image-version",
          "artifactTypeKey":"harpia.artifacts.v1.ImageAsset",
          "contentHash":"image-hash"
        }]
    }`)
	resp, err := BuildPreview(TypeKeyLinkedInPost, payload)
	if err != nil {
		t.Fatalf("BuildPreview: %v", err)
	}
	preview := resp.GetLinkedinPostPreview()
	if preview == nil || preview.GetText().GetText() != "We launched" {
		t.Fatalf("LinkedIn post preview = %#v", preview)
	}
	if preview.GetCarousel().GetTitle() != "Launch deck" || len(preview.GetImages()) != 1 {
		t.Fatalf("composite preview lost carousel/images: %#v", preview)
	}
}

func TestBuildPreviewCarouselDraft(t *testing.T) {
	payload := []byte(`{
		"title":"Q3 Highlights",
		"hook":"A quarter to remember",
		"slides":[
			{"heading":"Growth","body":"Up 30% YoY","image_artifact_id":"img-1"},
			{"heading":"Launches","body":"Shipped 4 products"}
		],
		"caption":"Thanks to the team",
		"hashtags":["#growth","#product"]
	}`)
	resp, err := BuildPreview(TypeKeyCarouselDraft, payload)
	if err != nil {
		t.Fatalf("BuildPreview: %v", err)
	}

	md := resp.GetMarkdownPreview()
	if md == "" {
		t.Fatal("expected markdown preview")
	}
	for _, want := range []string{"# Q3 Highlights", "## Growth", "#growth #product", "_[references image asset]_"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q; got:\n%s", want, md)
		}
	}
	if resp.GetTextPreview() != "" {
		t.Error("expected markdown variant, not text")
	}
}

func TestBuildPreviewCarouselDraftRequiresTitle(t *testing.T) {
	payload := []byte(`{"slides":[{"heading":"Growth","body":"Up 30%"}]}`)
	if _, err := BuildPreview(TypeKeyCarouselDraft, payload); err == nil {
		t.Fatal("expected error for carousel draft without title")
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

func TestBuildPreviewHTMLFallback(t *testing.T) {
	payload := []byte(`{"html":"<h1>Hi</h1><p>rendered</p>"}`)
	resp, err := BuildPreview("harpia.artifacts.v1.FutureHtmlPage", payload)
	if err != nil {
		t.Fatalf("BuildPreview: %v", err)
	}
	if resp.GetHtmlPreview() != "<h1>Hi</h1><p>rendered</p>" {
		t.Fatalf("html preview = %q", resp.GetHtmlPreview())
	}
}

func TestBuildPreviewImageURLFallback(t *testing.T) {
	payload := []byte(`{"image_url":"https://example.com/a.png","alt_text":"diagram"}`)
	resp, err := BuildPreview("harpia.artifacts.v1.FutureImage", payload)
	if err != nil {
		t.Fatalf("BuildPreview: %v", err)
	}
	img := resp.GetImagePreview()
	if img == nil || img.Url != "https://example.com/a.png" || img.AltText != "diagram" {
		t.Fatalf("image preview = %+v", img)
	}
	if len(img.InlineData) != 0 {
		t.Fatalf("expected no inline data, got %d bytes", len(img.InlineData))
	}
}

func TestBuildPreviewImageBase64Fallback(t *testing.T) {
	payload := []byte(`{"image_base64":"aGVsbG8="}`) // "hello"
	resp, err := BuildPreview("harpia.artifacts.v1.FutureImage", payload)
	if err != nil {
		t.Fatalf("BuildPreview: %v", err)
	}
	img := resp.GetImagePreview()
	if img == nil || string(img.InlineData) != "hello" {
		t.Fatalf("inline image data = %q", string(img.GetInlineData()))
	}
}
