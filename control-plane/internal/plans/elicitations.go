package plans

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	plansv1 "github.com/harpia/control-plane/gen/harpia/plans/v1"
	"github.com/harpia/control-plane/internal/database"
)

// Elicitation is the persistent record backing an in-app elicitation thread
// (E5.1). It mirrors the plan_elicitations table.
type Elicitation struct {
	ID                  uuid.UUID
	TenantID            uuid.UUID
	PlanExecutionID     uuid.UUID
	StepExecutionID     uuid.UUID
	PlanStepKey         string
	ElicitationThreadID string
	Status              string
	Prompt              string
	SchemaJSON          json.RawMessage
	ResponseJSON        json.RawMessage
	ResponseText        string
	TimeoutBehavior     string
	OverseerUserID      uuid.NullUUID
	RespondedBy         uuid.NullUUID
	CreatedAt           time.Time
	ExpiresAt           *time.Time
	RespondedAt         *time.Time
	UpdatedAt           time.Time
}

// ElicitationFilter scopes a List query. All filters are ANDed; nil/empty
// fields are ignored.
type ElicitationFilter struct {
	TenantID        uuid.UUID
	StepExecutionID *uuid.UUID
	PlanExecutionID *uuid.UUID
	Status          string
	OverseerUserID  *uuid.UUID
	Limit           int
	Offset          int
}

const elicitationColumns = `id, tenant_id, plan_execution_id, step_execution_id, plan_step_key,
	elicitation_thread_id, status, prompt, schema_json, response_json,
	COALESCE(response_text, ''), COALESCE(timeout_behavior, ''),
	overseer_user_id, responded_by, created_at, expires_at, responded_at, updated_at`

// UpsertElicitation inserts a pending elicitation for the step/thread, or
// returns the existing row when one already exists (idempotent for workflow
// replays).
func (r *Repository) UpsertElicitation(ctx context.Context, elicitation *Elicitation) (*Elicitation, error) {
	if elicitation == nil {
		return nil, fmt.Errorf("elicitation is required")
	}
	if elicitation.Status == "" {
		elicitation.Status = ElicitationStatusPending
	}
	if len(elicitation.SchemaJSON) == 0 {
		elicitation.SchemaJSON = json.RawMessage(`{}`)
	}
	var created Elicitation
	err := database.WithTenant(ctx, r.pool, elicitation.TenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`INSERT INTO plan_elicitations (
				tenant_id, plan_execution_id, step_execution_id, plan_step_key,
				elicitation_thread_id, status, prompt, schema_json, timeout_behavior,
				overseer_user_id, expires_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (tenant_id, step_execution_id, elicitation_thread_id) DO UPDATE
			SET updated_at = now()
			RETURNING `+elicitationColumns,
			elicitation.TenantID, elicitation.PlanExecutionID, elicitation.StepExecutionID,
			elicitation.PlanStepKey, elicitation.ElicitationThreadID, elicitation.Status,
			elicitation.Prompt, elicitation.SchemaJSON, elicitation.TimeoutBehavior,
			elicitation.OverseerUserID, elicitation.ExpiresAt,
		)
		return scanElicitation(row, &created)
	})
	if err != nil {
		return nil, fmt.Errorf("upsert elicitation: %w", err)
	}
	return &created, nil
}

func (r *Repository) GetElicitation(ctx context.Context, tenantID, elicitationID uuid.UUID) (*Elicitation, error) {
	var elicitation Elicitation
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`SELECT `+elicitationColumns+`
			 FROM plan_elicitations
			 WHERE id = $1 AND tenant_id = $2`,
			elicitationID, tenantID,
		)
		return scanElicitation(row, &elicitation)
	})
	if err != nil {
		return nil, fmt.Errorf("get elicitation: %w", err)
	}
	return &elicitation, nil
}

func (r *Repository) ListElicitations(ctx context.Context, filter ElicitationFilter) ([]Elicitation, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	elicitations := make([]Elicitation, 0)
	err := database.WithTenant(ctx, r.pool, filter.TenantID, func(q database.Querier) error {
		conditions := []string{"tenant_id = $1"}
		args := []any{filter.TenantID}
		if filter.StepExecutionID != nil {
			args = append(args, *filter.StepExecutionID)
			conditions = append(conditions, fmt.Sprintf("step_execution_id = $%d", len(args)))
		}
		if filter.PlanExecutionID != nil {
			args = append(args, *filter.PlanExecutionID)
			conditions = append(conditions, fmt.Sprintf("plan_execution_id = $%d", len(args)))
		}
		if strings.TrimSpace(filter.Status) != "" {
			args = append(args, filter.Status)
			conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
		}
		if filter.OverseerUserID != nil {
			args = append(args, *filter.OverseerUserID)
			conditions = append(conditions, fmt.Sprintf("overseer_user_id = $%d", len(args)))
		}
		args = append(args, limit)
		limitPlaceholder := len(args)
		args = append(args, filter.Offset)
		offsetPlaceholder := len(args)

		query := `SELECT ` + elicitationColumns + `
			 FROM plan_elicitations
			 WHERE ` + strings.Join(conditions, " AND ") + `
			 ORDER BY created_at DESC
			 LIMIT $` + fmt.Sprint(limitPlaceholder) + ` OFFSET $` + fmt.Sprint(offsetPlaceholder)

		rows, err := q.Query(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("list elicitations: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var elicitation Elicitation
			if err := scanElicitation(rows, &elicitation); err != nil {
				return err
			}
			elicitations = append(elicitations, elicitation)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return elicitations, nil
}

// MarkElicitationAnswered records the overseer response and flips the
// elicitation to ANSWERED. It only transitions rows that are still pending so
// concurrent or post-timeout responses are rejected.
func (r *Repository) MarkElicitationAnswered(
	ctx context.Context,
	tenantID, elicitationID uuid.UUID,
	payloadJSON json.RawMessage,
	responseText string,
	respondedBy uuid.NullUUID,
) (*Elicitation, error) {
	var responsePayload any
	if len(payloadJSON) > 0 {
		responsePayload = payloadJSON
	}
	var updated Elicitation
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx,
			`UPDATE plan_elicitations
			 SET status = $1,
			     response_json = $2,
			     response_text = $3,
			     responded_by = $4,
			     responded_at = now(),
			     updated_at = now()
			 WHERE id = $5 AND tenant_id = $6 AND status = $7
			 RETURNING `+elicitationColumns,
			ElicitationStatusAnswered, responsePayload, responseText, respondedBy,
			elicitationID, tenantID, ElicitationStatusPending,
		)
		return scanElicitation(row, &updated)
	})
	if err != nil {
		return nil, fmt.Errorf("mark elicitation answered: %w", err)
	}
	return &updated, nil
}

// MarkElicitationTimedOutByStep flips any pending elicitation for the step/thread
// to TIMED_OUT, honouring FR-12 timeout enforcement.
func (r *Repository) MarkElicitationTimedOutByStep(ctx context.Context, tenantID, stepExecutionID uuid.UUID, threadID string) error {
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		_, err := q.Exec(ctx,
			`UPDATE plan_elicitations
			 SET status = $1, updated_at = now()
			 WHERE tenant_id = $2
			   AND step_execution_id = $3
			   AND elicitation_thread_id = $4
			   AND status = $5`,
			ElicitationStatusTimedOut, tenantID, stepExecutionID, threadID, ElicitationStatusPending,
		)
		return err
	})
	if err != nil {
		return fmt.Errorf("mark elicitation timed out: %w", err)
	}
	return nil
}

func scanElicitation(row pgx.Row, elicitation *Elicitation) error {
	return row.Scan(
		&elicitation.ID, &elicitation.TenantID, &elicitation.PlanExecutionID,
		&elicitation.StepExecutionID, &elicitation.PlanStepKey, &elicitation.ElicitationThreadID,
		&elicitation.Status, &elicitation.Prompt, &elicitation.SchemaJSON, &elicitation.ResponseJSON,
		&elicitation.ResponseText, &elicitation.TimeoutBehavior, &elicitation.OverseerUserID,
		&elicitation.RespondedBy, &elicitation.CreatedAt, &elicitation.ExpiresAt,
		&elicitation.RespondedAt, &elicitation.UpdatedAt,
	)
}

func elicitationToProto(e *Elicitation) *plansv1.ElicitationRequest {
	if e == nil {
		return nil
	}
	out := &plansv1.ElicitationRequest{
		Id:                  e.ID.String(),
		TenantId:            e.TenantID.String(),
		StepExecutionId:     e.StepExecutionID.String(),
		PlanExecutionId:     e.PlanExecutionID.String(),
		PlanStepKey:         e.PlanStepKey,
		ElicitationThreadId: e.ElicitationThreadID,
		Prompt:              e.Prompt,
		SchemaJson:          string(e.SchemaJSON),
		Status:              stringToElicitationStatus(e.Status),
		TimeoutBehavior:     stringToTimeoutBehavior(e.TimeoutBehavior),
		CreatedAt:           e.CreatedAt.Format(time.RFC3339),
	}
	if e.OverseerUserID.Valid {
		out.OverseerUserId = e.OverseerUserID.UUID.String()
	}
	if e.RespondedBy.Valid {
		out.RespondedByUserId = e.RespondedBy.UUID.String()
	}
	if e.ExpiresAt != nil {
		out.ExpiresAt = e.ExpiresAt.Format(time.RFC3339)
	}
	if e.RespondedAt != nil {
		out.RespondedAt = e.RespondedAt.Format(time.RFC3339)
	}
	out.Thread = elicitationThread(e)
	return out
}

func elicitationThread(e *Elicitation) []*plansv1.ThreadMessage {
	thread := make([]*plansv1.ThreadMessage, 0, 2)
	if strings.TrimSpace(e.Prompt) != "" {
		thread = append(thread, &plansv1.ThreadMessage{
			Role:      plansv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_AGENT,
			Text:      e.Prompt,
			CreatedAt: e.CreatedAt.Format(time.RFC3339),
		})
	}
	if e.Status == ElicitationStatusAnswered && (strings.TrimSpace(e.ResponseText) != "" || len(e.ResponseJSON) > 0) {
		msg := &plansv1.ThreadMessage{
			Role:        plansv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_OVERSEER,
			Text:        e.ResponseText,
			PayloadJson: string(e.ResponseJSON),
		}
		if e.RespondedBy.Valid {
			msg.AuthorUserId = e.RespondedBy.UUID.String()
		}
		if e.RespondedAt != nil {
			msg.CreatedAt = e.RespondedAt.Format(time.RFC3339)
		}
		thread = append(thread, msg)
	}
	return thread
}

func stringToElicitationStatus(s string) plansv1.ElicitationStatus {
	switch s {
	case ElicitationStatusPending:
		return plansv1.ElicitationStatus_ELICITATION_STATUS_PENDING
	case ElicitationStatusAnswered:
		return plansv1.ElicitationStatus_ELICITATION_STATUS_ANSWERED
	case ElicitationStatusTimedOut:
		return plansv1.ElicitationStatus_ELICITATION_STATUS_TIMED_OUT
	case ElicitationStatusCancelled:
		return plansv1.ElicitationStatus_ELICITATION_STATUS_CANCELLED
	default:
		return plansv1.ElicitationStatus_ELICITATION_STATUS_UNSPECIFIED
	}
}

func elicitationStatusToString(status plansv1.ElicitationStatus) string {
	switch status {
	case plansv1.ElicitationStatus_ELICITATION_STATUS_PENDING:
		return ElicitationStatusPending
	case plansv1.ElicitationStatus_ELICITATION_STATUS_ANSWERED:
		return ElicitationStatusAnswered
	case plansv1.ElicitationStatus_ELICITATION_STATUS_TIMED_OUT:
		return ElicitationStatusTimedOut
	case plansv1.ElicitationStatus_ELICITATION_STATUS_CANCELLED:
		return ElicitationStatusCancelled
	default:
		return ""
	}
}

func stringToTimeoutBehavior(s string) plansv1.ElicitationTimeoutBehavior {
	if value, ok := plansv1.ElicitationTimeoutBehavior_value[s]; ok {
		return plansv1.ElicitationTimeoutBehavior(value)
	}
	return plansv1.ElicitationTimeoutBehavior_ELICITATION_TIMEOUT_BEHAVIOR_UNSPECIFIED
}
