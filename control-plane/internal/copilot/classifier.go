// Package copilot routes a thread's natural-language message to a plan template
// and extracts template input values. The classifier is a port so the thread
// handler can be tested without a live model; the LLM adapter is the production
// implementation.
package copilot

import (
	"context"

	"github.com/google/uuid"
)

// InputParamSummary describes one template input parameter for the router prompt.
type InputParamSummary struct {
	Key         string
	Label       string
	Type        string // TemplateInputParameterType enum name, e.g. "TEMPLATE_INPUT_PARAMETER_TYPE_TEXT"
	Required    bool
	OptionsJSON string
}

// TemplateSummary is the catalog entry the router reasons over.
type TemplateSummary struct {
	ID          uuid.UUID
	Key         string
	Name        string
	Description string
	Inputs      []InputParamSummary
}

// Candidate is one proposed template with a confidence and extracted inputs.
type Candidate struct {
	TemplateID      uuid.UUID
	Confidence      float64
	InputValuesJSON string // JSON object keyed by input parameter key
}

// ClassifyInput is the request to the router.
type ClassifyInput struct {
	TenantID  uuid.UUID
	Text      string
	Templates []TemplateSummary
}

// PlanClassifier maps a user message to ranked template candidates.
type PlanClassifier interface {
	Classify(ctx context.Context, in ClassifyInput) ([]Candidate, error)
}
