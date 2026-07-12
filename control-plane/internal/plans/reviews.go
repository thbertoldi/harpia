package plans

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/harpia/control-plane/internal/database"
)

// PlanReviewRequest is the persistence model for a candidate Review/Revision
// checkpoint. It deliberately carries a complete immutable artifact reference
// instead of reusing elicitation or approval state.
type PlanReviewRequest struct {
	ID                       uuid.UUID
	TenantID                 uuid.UUID
	PlanExecutionID          uuid.UUID
	PlanConfigurationID      uuid.UUID
	ThreadID                 uuid.UUID
	StepExecutionID          uuid.UUID
	PlanStepKey              string
	SubjectArtifactID        uuid.UUID
	SubjectArtifactVersionID uuid.UUID
	SubjectArtifactTypeKey   string
	SubjectContentHash       string
	Status                   string
	RevisionFeedback         string
	Decision                 string
	OverseerUserID           uuid.NullUUID
	DecidedByUserID          uuid.NullUUID
	RequestedAt              time.Time
	DecidedAt                *time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

type ReviewFilters struct {
	StepExecutionID *uuid.UUID
	PlanExecutionID *uuid.UUID
	Status          string
	Limit           int
	Offset          int
}

const planReviewRequestColumns = `id, tenant_id, plan_execution_id, step_execution_id, plan_step_key,
	subject_artifact_id, subject_artifact_version_id, subject_artifact_type_key, subject_content_hash,
	status, revision_feedback, COALESCE(decision, ''), overseer_user_id, decided_by_user_id,
	requested_at, decided_at, created_at, updated_at`

const planReviewRequestJoinedColumns = `pr.id, pr.tenant_id, pr.plan_execution_id, pr.step_execution_id, pr.plan_step_key,
	pr.subject_artifact_id, pr.subject_artifact_version_id, pr.subject_artifact_type_key, pr.subject_content_hash,
	pr.status, pr.revision_feedback, COALESCE(pr.decision, ''), pr.overseer_user_id, pr.decided_by_user_id,
	pr.requested_at, pr.decided_at, pr.created_at, pr.updated_at, pe.plan_configuration_id, pc.origin_thread_id`

const planReviewRequestFromJoin = ` FROM plan_review_requests pr
	JOIN plan_executions pe ON pe.id = pr.plan_execution_id AND pe.tenant_id = pr.tenant_id
	JOIN plan_configurations pc ON pc.id = pe.plan_configuration_id AND pc.tenant_id = pe.tenant_id`

// CreatePlanReviewRequest inserts a review request exactly once for a subject
// ArtifactVersion. Temporal activities are at-least-once, so replays return the
// existing request selected by the tenant/step/version natural key.
func (r *Repository) CreatePlanReviewRequest(ctx context.Context, request *PlanReviewRequest) (*PlanReviewRequest, error) {
	if request == nil {
		return nil, fmt.Errorf("plan review request is required")
	}
	if request.ID == uuid.Nil {
		request.ID = uuid.New()
	}
	if request.Status == "" {
		request.Status = ReviewRequestStatusPending
	}
	if request.TenantID == uuid.Nil || request.PlanExecutionID == uuid.Nil || request.StepExecutionID == uuid.Nil ||
		request.SubjectArtifactID == uuid.Nil || request.SubjectArtifactVersionID == uuid.Nil ||
		strings.TrimSpace(request.SubjectArtifactTypeKey) == "" || strings.TrimSpace(request.SubjectContentHash) == "" {
		return nil, fmt.Errorf("plan review request requires tenant, execution, step, and complete subject artifact reference")
	}

	var created PlanReviewRequest
	err := database.WithTenant(ctx, r.pool, request.TenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`WITH inserted AS (
				INSERT INTO plan_review_requests (
					id, tenant_id, plan_execution_id, step_execution_id, plan_step_key,
					subject_artifact_id, subject_artifact_version_id, subject_artifact_type_key,
					subject_content_hash, status, revision_feedback, overseer_user_id
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
				ON CONFLICT (tenant_id, step_execution_id, subject_artifact_version_id) DO NOTHING
				RETURNING `+planReviewRequestColumns+`
			)
			SELECT * FROM inserted
			UNION ALL
			SELECT `+planReviewRequestColumns+`
			  FROM plan_review_requests
			 WHERE tenant_id = $2
			   AND step_execution_id = $4
			   AND subject_artifact_version_id = $7
			   AND NOT EXISTS (SELECT 1 FROM inserted)
			LIMIT 1`,
			request.ID, request.TenantID, request.PlanExecutionID, request.StepExecutionID,
			request.PlanStepKey, request.SubjectArtifactID, request.SubjectArtifactVersionID,
			request.SubjectArtifactTypeKey, request.SubjectContentHash, request.Status,
			request.RevisionFeedback, nullableUUIDValue(request.OverseerUserID),
		)
		return scanPlanReviewRequest(row, &created)
	})
	if err != nil {
		return nil, fmt.Errorf("create plan review request: %w", err)
	}
	return &created, nil
}

func (r *Repository) GetPlanReviewRequest(ctx context.Context, tenantID, reviewID uuid.UUID) (*PlanReviewRequest, error) {
	var request PlanReviewRequest
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`SELECT `+planReviewRequestJoinedColumns+planReviewRequestFromJoin+`
			 WHERE pr.id = $1 AND pr.tenant_id = $2`,
			reviewID, tenantID,
		)
		return scanPlanReviewRequestJoined(row, &request)
	})
	if err != nil {
		return nil, fmt.Errorf("get plan review request: %w", err)
	}
	return &request, nil
}

func (r *Repository) ListPlanReviewRequests(ctx context.Context, tenantID uuid.UUID, filters ReviewFilters) ([]*PlanReviewRequest, error) {
	limit := filters.Limit
	if limit <= 0 {
		limit = 50
	}
	requests := make([]*PlanReviewRequest, 0)
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		conditions := []string{"pr.tenant_id = $1"}
		args := []any{tenantID}
		if filters.StepExecutionID != nil {
			args = append(args, *filters.StepExecutionID)
			conditions = append(conditions, fmt.Sprintf("pr.step_execution_id = $%d", len(args)))
		}
		if filters.PlanExecutionID != nil {
			args = append(args, *filters.PlanExecutionID)
			conditions = append(conditions, fmt.Sprintf("pr.plan_execution_id = $%d", len(args)))
		}
		if strings.TrimSpace(filters.Status) != "" {
			args = append(args, filters.Status)
			conditions = append(conditions, fmt.Sprintf("pr.status = $%d", len(args)))
		}
		args = append(args, limit, filters.Offset)
		query := `SELECT ` + planReviewRequestJoinedColumns + planReviewRequestFromJoin + `
			 WHERE ` + strings.Join(conditions, " AND ") + `
			 ORDER BY pr.requested_at DESC
			 LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))
		rows, err := q.Query(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("list plan review requests: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var request PlanReviewRequest
			if err := scanPlanReviewRequestJoined(rows, &request); err != nil {
				return err
			}
			row := request
			requests = append(requests, &row)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return requests, nil
}

// MarkReviewDecided atomically resolves a pending review. The pending-status
// predicate makes a Temporal/API retry safe: only the first authorized
// decision changes the request.
func (r *Repository) MarkReviewDecided(ctx context.Context, tenantID, reviewID uuid.UUID, decision, feedback string, decidedBy uuid.NullUUID) (*PlanReviewRequest, error) {
	status := ReviewRequestStatusAccepted
	if decision == "revise" {
		status = ReviewRequestStatusRevisionRequested
	}
	var updated PlanReviewRequest
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx, `UPDATE plan_review_requests
			SET status = $1, decision = $2, revision_feedback = $3,
			    decided_by_user_id = $4, decided_at = now(), updated_at = now()
			WHERE id = $5 AND tenant_id = $6 AND status = $7
			RETURNING `+planReviewRequestColumns,
			status, decision, feedback, nullableUUIDValue(decidedBy), reviewID, tenantID, ReviewRequestStatusPending)
		return scanPlanReviewRequest(row, &updated)
	})
	if err != nil {
		return nil, fmt.Errorf("mark review decided: %w", err)
	}
	return &updated, nil
}

func scanPlanReviewRequest(row pgx.Row, dest *PlanReviewRequest) error {
	return row.Scan(
		&dest.ID, &dest.TenantID, &dest.PlanExecutionID, &dest.StepExecutionID, &dest.PlanStepKey,
		&dest.SubjectArtifactID, &dest.SubjectArtifactVersionID, &dest.SubjectArtifactTypeKey, &dest.SubjectContentHash,
		&dest.Status, &dest.RevisionFeedback, &dest.Decision, &dest.OverseerUserID, &dest.DecidedByUserID,
		&dest.RequestedAt, &dest.DecidedAt, &dest.CreatedAt, &dest.UpdatedAt,
	)
}

func scanPlanReviewRequestJoined(row pgx.Row, dest *PlanReviewRequest) error {
	return row.Scan(
		&dest.ID, &dest.TenantID, &dest.PlanExecutionID, &dest.StepExecutionID, &dest.PlanStepKey,
		&dest.SubjectArtifactID, &dest.SubjectArtifactVersionID, &dest.SubjectArtifactTypeKey, &dest.SubjectContentHash,
		&dest.Status, &dest.RevisionFeedback, &dest.Decision, &dest.OverseerUserID, &dest.DecidedByUserID,
		&dest.RequestedAt, &dest.DecidedAt, &dest.CreatedAt, &dest.UpdatedAt,
		&dest.PlanConfigurationID, &dest.ThreadID,
	)
}
