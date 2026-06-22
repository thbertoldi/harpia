package plans

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
)

// ApprovalRequest is the domain view of a publish approval gate.
type ApprovalRequest struct {
	ID                  string
	TenantID            string
	PlanExecutionID     string
	PlanConfigurationID string
	StepExecutionID     string
	PlanStepKey         string
	InputArtifactID     string
	Status              string
	DecisionReason      string
	RequestedAt         time.Time
	DecidedAt           *time.Time
}

// ApprovalFilters scopes list/watch queries.
type ApprovalFilters struct {
	StepExecutionID *string
	PlanExecutionID *string
	Status          string
	Limit           int
	Offset          int
}

func approvalRequestStatusToProto(status string) plansv1.ApprovalRequestStatus {
	switch status {
	case ApprovalRequestStatusPending:
		return plansv1.ApprovalRequestStatus_APPROVAL_REQUEST_STATUS_PENDING
	case ApprovalRequestStatusApproved:
		return plansv1.ApprovalRequestStatus_APPROVAL_REQUEST_STATUS_APPROVED
	case ApprovalRequestStatusRejected:
		return plansv1.ApprovalRequestStatus_APPROVAL_REQUEST_STATUS_REJECTED
	default:
		return plansv1.ApprovalRequestStatus_APPROVAL_REQUEST_STATUS_UNSPECIFIED
	}
}

func approvalRequestStatusFromProto(status plansv1.ApprovalRequestStatus) (string, error) {
	switch status {
	case plansv1.ApprovalRequestStatus_APPROVAL_REQUEST_STATUS_PENDING:
		return ApprovalRequestStatusPending, nil
	case plansv1.ApprovalRequestStatus_APPROVAL_REQUEST_STATUS_APPROVED:
		return ApprovalRequestStatusApproved, nil
	case plansv1.ApprovalRequestStatus_APPROVAL_REQUEST_STATUS_REJECTED:
		return ApprovalRequestStatusRejected, nil
	default:
		return "", fmt.Errorf("unsupported approval request status %v", status)
	}
}

func approvalRequestToProto(approval *ApprovalRequest) *plansv1.ApprovalRequest {
	if approval == nil {
		return nil
	}
	out := &plansv1.ApprovalRequest{
		Id:              approval.ID,
		TenantId:        approval.TenantID,
		StepExecutionId: approval.StepExecutionID,
		PlanExecutionId: approval.PlanExecutionID,
		PlanStepKey:     approval.PlanStepKey,
		InputArtifactId: approval.InputArtifactID,
		Status:          approvalRequestStatusToProto(approval.Status),
		DecisionReason:  approval.DecisionReason,
		RequestedAt:     approval.RequestedAt.UTC().Format(time.RFC3339),
	}
	if approval.DecidedAt != nil {
		out.DecidedAt = approval.DecidedAt.UTC().Format(time.RFC3339)
	}
	if approval.PlanConfigurationID != "" {
		out.PlanConfigurationId = approval.PlanConfigurationID
	}
	return out
}

func planApprovalRequestToDomain(row *PlanApprovalRequest) *ApprovalRequest {
	if row == nil {
		return nil
	}
	configID := ""
	if row.PlanConfigurationID != uuid.Nil {
		configID = row.PlanConfigurationID.String()
	}
	return &ApprovalRequest{
		ID:                  row.ID,
		TenantID:            row.TenantID.String(),
		PlanExecutionID:     row.PlanExecutionID.String(),
		PlanConfigurationID: configID,
		StepExecutionID:     row.StepExecutionID.String(),
		PlanStepKey:         row.PlanStepKey,
		InputArtifactID:     row.InputArtifactID,
		Status:              row.Status,
		DecisionReason:      row.DecisionReason,
		RequestedAt:         row.RequestedAt,
		DecidedAt:           row.DecidedAt,
	}
}

func validateApprovalDecision(approved bool, reason string) error {
	if approved {
		return nil
	}
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("reason is required when rejecting an approval request")
	}
	return nil
}
