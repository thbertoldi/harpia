package plans

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/copilot"
)

// CopilotCatalog adapts the plan template repository to the copilot router's
// TemplateSummary view. Templates are global (not tenant-scoped) in the current
// schema, matching Repository.ListTemplates.
type CopilotCatalog struct {
	repo *Repository
}

func NewCopilotCatalog(repo *Repository) *CopilotCatalog {
	return &CopilotCatalog{repo: repo}
}

// ListTemplateSummaries returns all templates as router catalog entries.
func (c *CopilotCatalog) ListTemplateSummaries(ctx context.Context) ([]copilot.TemplateSummary, error) {
	tpls, err := c.repo.ListTemplates(ctx, "", 100, 0)
	if err != nil {
		return nil, fmt.Errorf("list template summaries: %w", err)
	}
	out := make([]copilot.TemplateSummary, 0, len(tpls))
	for i := range tpls {
		proto := templateToProto(&tpls[i])
		out = append(out, templateSummaryFromProto(proto, tpls[i].ID))
	}
	return out, nil
}

// templateSummaryFromProto maps a proto template to a router summary. It reuses
// templateToProto's parsing of input_parameters (see plans/handler.go).
func templateSummaryFromProto(p *plansv1.PlanTemplate, id uuid.UUID) copilot.TemplateSummary {
	summary := copilot.TemplateSummary{
		ID:          id,
		Key:         p.GetKey(),
		Name:        p.GetName(),
		Description: p.GetDescription(),
	}
	for _, ip := range p.GetInputParameters() {
		summary.Inputs = append(summary.Inputs, copilot.InputParamSummary{
			Key:         ip.GetKey(),
			Label:       ip.GetLabel(),
			Type:        ip.GetType().String(),
			Required:    ip.GetRequired(),
			OptionsJSON: ip.GetOptionsJson(),
		})
	}
	return summary
}
