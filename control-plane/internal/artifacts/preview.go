package artifacts

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const maxListSummaryTitles = 5

func BuildPreview(typeKey string, payload []byte) (*artifactsv1.PreviewArtifactResponse, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("%w: payload is required", ErrInvalidPayload)
	}
	if !json.Valid(payload) {
		return nil, fmt.Errorf("%w: payload must be valid JSON", ErrInvalidPayload)
	}

	switch strings.TrimSpace(typeKey) {
	case TypeKeyTextDraft:
		return buildTextDraftPreview(payload)
	case TypeKeyLinkedInPostDraft:
		return buildLinkedInPostDraftPreview(payload)
	case TypeKeyLinkedInPost:
		return buildLinkedInPostPreview(payload)
	case TypeKeyNewsList:
		return buildNewsListPreview(payload)
	case TypeKeyDateRange:
		return buildJSONPreview(payload, &artifactsv1.DateRange{})
	case TypeKeyPublishConfirmation:
		return buildJSONPreview(payload, &artifactsv1.PublishConfirmation{})
	case TypeKeyCarouselDraft:
		return buildCarouselDraftPreview(payload)
	case TypeKeyImageAsset:
		return buildImageAssetPreview(payload)
	default:
		// Forward-compatible fallback: artifact types not enumerated above can
		// still preview if their payload carries html or image content. This
		// lets future HtmlPage / image artifact types render without a code
		// change here.
		if resp, ok, err := buildHTMLPreview(payload); err != nil {
			return nil, err
		} else if ok {
			return resp, nil
		}
		if resp, ok, err := buildImagePreview(payload); err != nil {
			return nil, err
		} else if ok {
			return resp, nil
		}
		return nil, fmt.Errorf("%w: unsupported artifact type %q", ErrInvalidPayload, typeKey)
	}
}

func EditableTextPayload(typeKey string, currentPayload []byte, title, text string) ([]byte, error) {
	switch typeKey {
	case TypeKeyTextDraft:
		var draft artifactsv1.TextDraft
		if err := protojson.Unmarshal(currentPayload, &draft); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
		}
		if strings.TrimSpace(title) != "" {
			draft.Title = strings.TrimSpace(title)
		}
		draft.Body = text
		return protojson.Marshal(&draft)
	case TypeKeyLinkedInPostDraft:
		var draft artifactsv1.LinkedInPostDraft
		if err := protojson.Unmarshal(currentPayload, &draft); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
		}
		if strings.TrimSpace(title) != "" {
			draft.Hook = strings.TrimSpace(title)
		}
		draft.Text = text
		return protojson.Marshal(&draft)
	default:
		return nil, fmt.Errorf("%w: artifact type %q is not editable text", ErrInvalidPayload, typeKey)
	}
}

func buildTextDraftPreview(payload []byte) (*artifactsv1.PreviewArtifactResponse, error) {
	msg := &artifactsv1.TextDraft{}
	if err := protojson.Unmarshal(payload, msg); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}

	title := strings.TrimSpace(msg.Title)
	body := strings.TrimSpace(msg.Body)
	if title == "" || body == "" {
		return nil, fmt.Errorf("%w: title and body are required", ErrInvalidPayload)
	}

	return &artifactsv1.PreviewArtifactResponse{
		Preview: &artifactsv1.PreviewArtifactResponse_MarkdownPreview{
			MarkdownPreview: fmt.Sprintf("# %s\n\n%s", title, body),
		},
	}, nil
}

func buildLinkedInPostDraftPreview(payload []byte) (*artifactsv1.PreviewArtifactResponse, error) {
	msg := &artifactsv1.LinkedInPostDraft{}
	if err := protojson.Unmarshal(payload, msg); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return nil, fmt.Errorf("%w: text is required", ErrInvalidPayload)
	}

	preview := text
	if hook := strings.TrimSpace(msg.Hook); hook != "" {
		preview = hook + "\n\n" + text
	}
	if len(msg.Hashtags) > 0 {
		preview += "\n\n" + strings.Join(msg.Hashtags, " ")
	}

	return &artifactsv1.PreviewArtifactResponse{
		Preview: &artifactsv1.PreviewArtifactResponse_MarkdownPreview{
			MarkdownPreview: preview,
		},
	}, nil
}

func buildLinkedInPostPreview(payload []byte) (*artifactsv1.PreviewArtifactResponse, error) {
	if err := ValidatePayload(TypeKeyLinkedInPost, payload); err != nil {
		return nil, err
	}
	post := &artifactsv1.LinkedInPost{}
	if err := protojson.Unmarshal(payload, post); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}
	return &artifactsv1.PreviewArtifactResponse{
		Preview: &artifactsv1.PreviewArtifactResponse_LinkedinPostPreview{
			LinkedinPostPreview: &artifactsv1.LinkedInPostPreview{
				Text:     post.Text,
				Carousel: post.Carousel,
				Images:   post.Images,
			},
		},
	}, nil
}

func buildCarouselDraftPreview(payload []byte) (*artifactsv1.PreviewArtifactResponse, error) {
	msg := &artifactsv1.CarouselDraft{}
	if err := protojson.Unmarshal(payload, msg); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}

	title := strings.TrimSpace(msg.Title)
	if title == "" || len(msg.Slides) == 0 {
		return nil, fmt.Errorf("%w: title and at least one slide are required", ErrInvalidPayload)
	}

	var b strings.Builder
	b.WriteString("# " + title)

	if hook := strings.TrimSpace(msg.Hook); hook != "" {
		b.WriteString("\n\n> " + hook)
	}

	for i, slide := range msg.Slides {
		heading := strings.TrimSpace(slide.Heading)
		body := strings.TrimSpace(slide.Body)
		if heading == "" {
			heading = fmt.Sprintf("Slide %d", i+1)
		}
		b.WriteString("\n\n## " + heading)
		if body != "" {
			b.WriteString("\n\n" + body)
		}
		if strings.TrimSpace(slide.ImageArtifactId) != "" {
			// References an ImageAsset whose bytes live out-of-band; we
			// acknowledge the reference rather than resolving it here.
			b.WriteString("\n\n_[references image asset]_")
		}
	}

	if caption := strings.TrimSpace(msg.Caption); caption != "" {
		b.WriteString("\n\n" + caption)
	}

	if len(msg.Hashtags) > 0 {
		b.WriteString("\n\n" + strings.Join(msg.Hashtags, " "))
	}

	return &artifactsv1.PreviewArtifactResponse{
		Preview: &artifactsv1.PreviewArtifactResponse_MarkdownPreview{
			MarkdownPreview: b.String(),
		},
	}, nil
}

// buildImageAssetPreview renders an ImageAsset payload. The payload carries the
// proto metadata (prompt, mimeType, width, height, altText) plus an optional
// image_url (a presigned object-store URL, populated when the PayloadStore
// supports presigning) and/or image_base64 (inline rendering bytes for the
// non-presigning fallback). A presigned URL is preferred when present; otherwise
// the inline bytes are decoded and returned.
func buildImageAssetPreview(payload []byte) (*artifactsv1.PreviewArtifactResponse, error) {
	var shape struct {
		Prompt      string `json:"prompt"`
		MimeType    string `json:"mimeType"`
		AltText     string `json:"altText"`
		Caption     string `json:"caption"`
		ImageURL    string `json:"image_url"`
		ImageBase64 string `json:"image_base64"`
	}
	if err := json.Unmarshal(payload, &shape); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}

	altText := strings.TrimSpace(shape.AltText)
	if altText == "" {
		altText = strings.TrimSpace(shape.Caption)
	}

	url := strings.TrimSpace(shape.ImageURL)
	b64 := strings.TrimSpace(shape.ImageBase64)
	if url == "" && b64 == "" {
		// No renderable bytes: surface the metadata so the preview is still useful.
		summary := strings.TrimSpace(shape.Prompt)
		if summary == "" {
			summary = strings.TrimSpace(shape.MimeType)
		}
		return &artifactsv1.PreviewArtifactResponse{
			Preview: &artifactsv1.PreviewArtifactResponse_MarkdownPreview{
				MarkdownPreview: fmt.Sprintf("_[image asset: %s]_", fallbackLabel(summary)),
			},
		}, nil
	}

	resp := &artifactsv1.PreviewArtifactResponse{
		Preview: &artifactsv1.PreviewArtifactResponse_ImagePreview{
			ImagePreview: &artifactsv1.ImagePreview{
				Url:     url,
				AltText: altText,
			},
		},
	}
	if b64 != "" {
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid image_base64: %v", ErrInvalidPayload, err)
		}
		resp.GetImagePreview().InlineData = data
	}
	return resp, nil
}

func fallbackLabel(s string) string {
	if s == "" {
		return "no metadata"
	}
	return s
}

func buildNewsListPreview(payload []byte) (*artifactsv1.PreviewArtifactResponse, error) {
	msg := &artifactsv1.NewsList{}
	if err := protojson.Unmarshal(payload, msg); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}
	if len(msg.Articles) == 0 {
		return nil, fmt.Errorf("%w: articles must not be empty", ErrInvalidPayload)
	}

	titles := make([]string, 0, min(len(msg.Articles), maxListSummaryTitles))
	for i, article := range msg.Articles {
		if i >= maxListSummaryTitles {
			break
		}
		if article == nil {
			continue
		}
		title := strings.TrimSpace(article.Title)
		if title != "" {
			titles = append(titles, title)
		}
	}

	return &artifactsv1.PreviewArtifactResponse{
		Preview: &artifactsv1.PreviewArtifactResponse_ListSummary{
			ListSummary: &artifactsv1.ArtifactListSummary{
				ArticleCount: int32(len(msg.Articles)),
				Titles:       titles,
			},
		},
	}, nil
}

func buildJSONPreview(payload []byte, msg proto.Message) (*artifactsv1.PreviewArtifactResponse, error) {
	if err := protojson.Unmarshal(payload, msg); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}

	encoded, err := protojson.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal preview json: %w", err)
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, encoded); err != nil {
		return nil, fmt.Errorf("compact preview json: %w", err)
	}

	return &artifactsv1.PreviewArtifactResponse{
		Preview: &artifactsv1.PreviewArtifactResponse_JsonPreview{
			JsonPreview: compact.String(),
		},
	}, nil
}

// buildHTMLPreview returns an html_preview when the payload is a JSON object
// carrying a non-empty "html" string. ok is false (no error) when the shape
// does not match, so the caller can fall through to other heuristics.
func buildHTMLPreview(payload []byte) (*artifactsv1.PreviewArtifactResponse, bool, error) {
	var shape struct {
		HTML string `json:"html"`
	}
	if err := json.Unmarshal(payload, &shape); err != nil {
		return nil, false, nil // not a JSON object — let caller decide
	}
	if strings.TrimSpace(shape.HTML) == "" {
		return nil, false, nil
	}
	return &artifactsv1.PreviewArtifactResponse{
		Preview: &artifactsv1.PreviewArtifactResponse_HtmlPreview{
			HtmlPreview: shape.HTML,
		},
	}, true, nil
}

// buildImagePreview returns an image_preview when the payload carries an
// image_url and/or image_base64 field. inline bytes are decoded from base64.
func buildImagePreview(payload []byte) (*artifactsv1.PreviewArtifactResponse, bool, error) {
	var shape struct {
		ImageURL    string `json:"image_url"`
		ImageBase64 string `json:"image_base64"`
		AltText     string `json:"alt_text"`
	}
	if err := json.Unmarshal(payload, &shape); err != nil {
		return nil, false, nil
	}
	url := strings.TrimSpace(shape.ImageURL)
	b64 := strings.TrimSpace(shape.ImageBase64)
	if url == "" && b64 == "" {
		return nil, false, nil
	}
	resp := &artifactsv1.PreviewArtifactResponse{
		Preview: &artifactsv1.PreviewArtifactResponse_ImagePreview{
			ImagePreview: &artifactsv1.ImagePreview{
				Url:     url,
				AltText: strings.TrimSpace(shape.AltText),
			},
		},
	}
	if b64 != "" {
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, false, fmt.Errorf("%w: invalid image_base64: %v", ErrInvalidPayload, err)
		}
		resp.GetImagePreview().InlineData = data
	}
	return resp, true, nil
}
