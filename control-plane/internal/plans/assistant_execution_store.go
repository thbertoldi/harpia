package plans

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	artifactsv1 "github.com/harpia/control-plane/gen/harpia/artifacts/v1"
	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	"github.com/harpia/control-plane/internal/planassistant"
)

// AssistantExecutionStore adapts tenant-scoped Plan Management reads to the
// dependency-free planassistant.ExecutionStore port. It verifies ownership of
// the configuration and its origin thread before exposing a projection.
type AssistantExecutionStore struct{ Repo *Repository }

func (s *AssistantExecutionStore) LoadExactExecution(ctx context.Context, tenantID, configurationID, executionID uuid.UUID) (*planassistant.ExecutionProjection, error) {
	if s == nil || s.Repo == nil {
		return nil, fmt.Errorf("assistant execution store is not configured")
	}
	execution, err := s.Repo.GetExecution(ctx, tenantID, executionID)
	if err != nil {
		return nil, err
	}
	if configurationID != uuid.Nil && execution.PlanConfigurationID != configurationID {
		return nil, fmt.Errorf("plan execution not found")
	}
	return s.projection(ctx, tenantID, execution)
}

func (s *AssistantExecutionStore) LoadLatestNonTerminalExecution(ctx context.Context, tenantID, configurationID uuid.UUID) (*planassistant.ExecutionProjection, error) {
	if s == nil || s.Repo == nil {
		return nil, fmt.Errorf("assistant execution store is not configured")
	}
	execution, err := s.Repo.GetLatestNonTerminalExecutionForConfiguration(ctx, tenantID, configurationID)
	if err != nil {
		return nil, err
	}
	if execution == nil {
		return nil, nil
	}
	return s.projection(ctx, tenantID, execution)
}

func (s *AssistantExecutionStore) projection(ctx context.Context, tenantID uuid.UUID, execution *PlanExecution) (*planassistant.ExecutionProjection, error) {
	if execution == nil || execution.TenantID != tenantID {
		return nil, fmt.Errorf("plan execution not found")
	}
	configuration, err := s.Repo.GetConfiguration(ctx, tenantID, execution.PlanConfigurationID)
	if err != nil {
		return nil, err
	}
	if configuration.OriginThreadID == uuid.Nil {
		return nil, fmt.Errorf("plan configuration has no owning origin thread")
	}
	pending, err := s.Repo.GetPendingExecutionInteractions(ctx, tenantID, execution.ID)
	if err != nil {
		return nil, err
	}
	interactions := make([]planassistant.PendingInteraction, 0, len(pending))
	for _, interaction := range pending {
		interactions = append(interactions, pendingInteractionToAssistant(interaction))
	}
	return &planassistant.ExecutionProjection{
		Execution:           executionToProto(execution),
		PendingInteractions: interactions,
	}, nil
}

func pendingInteractionToAssistant(row PendingExecutionInteraction) planassistant.PendingInteraction {
	interaction := planassistant.PendingInteraction{
		RequestID:       row.RequestID,
		PlanExecutionID: row.PlanExecutionID.String(),
		StepExecutionID: row.StepExecutionID.String(),
		PlanStepKey:     row.PlanStepKey,
	}
	switch row.Kind {
	case PendingInteractionElicitation:
		interaction.Kind = chatv1.ExecutionInteractionKind_EXECUTION_INTERACTION_KIND_ELICITATION
		interaction.Actions = []*chatv1.ExecutionPromptAction{{ActionId: "elicitation.answer", LabelKey: "execution.elicitation.answer"}}
	case PendingInteractionReview:
		interaction.Kind = chatv1.ExecutionInteractionKind_EXECUTION_INTERACTION_KIND_REVIEW
		interaction.Actions = []*chatv1.ExecutionPromptAction{{ActionId: "review.accept", LabelKey: "execution.review.accept"}, {ActionId: "review.revise", LabelKey: "execution.review.revise"}}
	case PendingInteractionApproval:
		interaction.Kind = chatv1.ExecutionInteractionKind_EXECUTION_INTERACTION_KIND_APPROVAL
		interaction.Actions = []*chatv1.ExecutionPromptAction{{ActionId: "approval.approve", LabelKey: "execution.approval.approve"}, {ActionId: "approval.reject", LabelKey: "execution.approval.reject"}}
	}
	if row.SubjectArtifactID.Valid && row.SubjectArtifactVersionID.Valid && row.SubjectArtifactTypeKey != "" && row.SubjectContentHash != "" {
		interaction.SubjectArtifactRef = &artifactsv1.ArtifactRef{
			ArtifactId:        row.SubjectArtifactID.UUID.String(),
			ArtifactVersionId: row.SubjectArtifactVersionID.UUID.String(),
			ArtifactTypeKey:   row.SubjectArtifactTypeKey,
			ContentHash:       row.SubjectContentHash,
		}
	}
	return interaction
}
