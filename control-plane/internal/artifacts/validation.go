package artifacts

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var ErrInvalidPayload = errors.New("invalid artifact payload")

const (
	TypeKeyDateRange                = "harpia.artifacts.v1.DateRange"
	TypeKeyNewsList                 = "harpia.artifacts.v1.NewsList"
	TypeKeyTextDraft                = "harpia.artifacts.v1.TextDraft"
	TypeKeyLinkedInPostDraft        = "harpia.artifacts.v1.LinkedInPostDraft"
	TypeKeyPublishConfirmation      = "harpia.artifacts.v1.PublishConfirmation"
	TypeKeyCarouselDraft            = "harpia.artifacts.v1.CarouselDraft"
	TypeKeyImageAsset               = "harpia.artifacts.v1.ImageAsset"
	TypeKeyLinkedInPost             = "harpia.artifacts.v1.LinkedInPost"
	TypeKeyLinkedInCarouselDocument = "harpia.artifacts.v1.LinkedInCarouselDocument"
)

func ValidatePayload(typeKey string, payload []byte) error {
	if len(payload) == 0 {
		return fmt.Errorf("%w: payload is required", ErrInvalidPayload)
	}
	if !json.Valid(payload) {
		return fmt.Errorf("%w: payload must be valid JSON", ErrInvalidPayload)
	}

	switch strings.TrimSpace(typeKey) {
	case TypeKeyDateRange:
		return validateProtoJSON(payload, &artifactsv1.DateRange{}, func(msg *artifactsv1.DateRange) error {
			if strings.TrimSpace(msg.StartDate) == "" || strings.TrimSpace(msg.EndDate) == "" {
				return fmt.Errorf("%w: start_date and end_date are required", ErrInvalidPayload)
			}
			return nil
		})
	case TypeKeyNewsList:
		return validateProtoJSON(payload, &artifactsv1.NewsList{}, func(msg *artifactsv1.NewsList) error {
			if len(msg.Articles) == 0 {
				return fmt.Errorf("%w: articles must not be empty", ErrInvalidPayload)
			}
			for i, article := range msg.Articles {
				if article == nil {
					return fmt.Errorf("%w: articles[%d] is required", ErrInvalidPayload, i)
				}
				if strings.TrimSpace(article.Title) == "" || strings.TrimSpace(article.Url) == "" {
					return fmt.Errorf("%w: articles[%d] requires title and url", ErrInvalidPayload, i)
				}
			}
			return nil
		})
	case TypeKeyTextDraft:
		return validateProtoJSON(payload, &artifactsv1.TextDraft{}, func(msg *artifactsv1.TextDraft) error {
			if strings.TrimSpace(msg.Title) == "" || strings.TrimSpace(msg.Body) == "" {
				return fmt.Errorf("%w: title and body are required", ErrInvalidPayload)
			}
			return nil
		})
	case TypeKeyLinkedInPostDraft:
		return validateProtoJSON(payload, &artifactsv1.LinkedInPostDraft{}, func(msg *artifactsv1.LinkedInPostDraft) error {
			if strings.TrimSpace(msg.Text) == "" {
				return fmt.Errorf("%w: text is required", ErrInvalidPayload)
			}
			return nil
		})
	case TypeKeyPublishConfirmation:
		return validateProtoJSON(payload, &artifactsv1.PublishConfirmation{}, func(msg *artifactsv1.PublishConfirmation) error {
			if strings.TrimSpace(msg.Platform) == "" {
				return fmt.Errorf("%w: platform is required", ErrInvalidPayload)
			}
			return nil
		})
	case TypeKeyCarouselDraft:
		return validateProtoJSON(payload, &artifactsv1.CarouselDraft{}, func(msg *artifactsv1.CarouselDraft) error {
			if strings.TrimSpace(msg.Title) == "" || len(msg.Slides) == 0 {
				return fmt.Errorf("%w: title and at least one slide are required", ErrInvalidPayload)
			}
			for i, slide := range msg.Slides {
				if strings.TrimSpace(slide.Heading) == "" && strings.TrimSpace(slide.Body) == "" {
					return fmt.Errorf("%w: slides[%d] requires heading or body", ErrInvalidPayload, i)
				}
			}
			if msg.DocumentArtifact != nil {
				return validateArtifactRef(msg.DocumentArtifact, "document_artifact")
			}
			return nil
		})
	case TypeKeyImageAsset:
		// ImageAsset payloads may carry preview rendering hints
		// (image_base64 / image_url) that are not modeled on the proto but
		// are consumed by the preview layer. Discard unknown fields here so
		// the hint passes validation while the proto contract (mime_type)
		// is still enforced.
		msg := &artifactsv1.ImageAsset{}
		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(payload, msg); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidPayload, err)
		}
		if strings.TrimSpace(msg.MimeType) == "" {
			return fmt.Errorf("%w: mime_type is required", ErrInvalidPayload)
		}
		return nil
	case TypeKeyLinkedInCarouselDocument:
		return validateProtoJSON(payload, &artifactsv1.LinkedInCarouselDocument{}, func(msg *artifactsv1.LinkedInCarouselDocument) error {
			if strings.TrimSpace(msg.MimeType) != "application/pdf" {
				return fmt.Errorf("%w: mime_type must be application/pdf", ErrInvalidPayload)
			}
			if strings.TrimSpace(msg.FileName) == "" {
				return fmt.Errorf("%w: file_name is required", ErrInvalidPayload)
			}
			return nil
		})
	case TypeKeyLinkedInPost:
		return validateProtoJSON(payload, &artifactsv1.LinkedInPost{}, func(msg *artifactsv1.LinkedInPost) error {
			if msg.Text == nil || strings.TrimSpace(msg.Text.Text) == "" {
				return fmt.Errorf("%w: text is required", ErrInvalidPayload)
			}
			if msg.Carousel != nil && msg.Carousel.DocumentArtifact != nil {
				if err := validateArtifactRef(msg.Carousel.DocumentArtifact, "carousel.document_artifact"); err != nil {
					return err
				}
			}
			for i, image := range msg.Images {
				if err := validateArtifactRef(image, fmt.Sprintf("images[%d]", i)); err != nil {
					return err
				}
			}
			return nil
		})
	default:
		return fmt.Errorf("%w: unsupported artifact type %q", ErrInvalidPayload, typeKey)
	}
}

func validateProtoJSON[T proto.Message](payload []byte, msg T, check func(T) error) error {
	if err := protojson.Unmarshal(payload, msg); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}
	return check(msg)
}

func validateArtifactRef(ref *artifactsv1.ArtifactRef, field string) error {
	if ref == nil {
		return fmt.Errorf("%w: %s is required", ErrInvalidPayload, field)
	}
	if strings.TrimSpace(ref.ArtifactId) == "" ||
		strings.TrimSpace(ref.ArtifactVersionId) == "" ||
		strings.TrimSpace(ref.ArtifactTypeKey) == "" ||
		strings.TrimSpace(ref.ContentHash) == "" {
		return fmt.Errorf("%w: %s requires artifact_id, artifact_version_id, artifact_type_key, and content_hash", ErrInvalidPayload, field)
	}
	return nil
}
