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
	TypeKeyDateRange            = "harpia.artifacts.v1.DateRange"
	TypeKeyNewsList             = "harpia.artifacts.v1.NewsList"
	TypeKeyTextDraft            = "harpia.artifacts.v1.TextDraft"
	TypeKeyLinkedInPostDraft    = "harpia.artifacts.v1.LinkedInPostDraft"
	TypeKeyPublishConfirmation  = "harpia.artifacts.v1.PublishConfirmation"
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
