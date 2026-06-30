package artifacts

import (
	"bytes"
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
	case TypeKeyNewsList:
		return buildNewsListPreview(payload)
	case TypeKeyDateRange:
		return buildJSONPreview(payload, &artifactsv1.DateRange{})
	case TypeKeyPublishConfirmation:
		return buildJSONPreview(payload, &artifactsv1.PublishConfirmation{})
	default:
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
		Preview: &artifactsv1.PreviewArtifactResponse_TextPreview{
			TextPreview: fmt.Sprintf("# %s\n\n%s", title, body),
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
		Preview: &artifactsv1.PreviewArtifactResponse_TextPreview{
			TextPreview: preview,
		},
	}, nil
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
