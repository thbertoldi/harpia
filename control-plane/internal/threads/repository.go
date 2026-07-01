package threads

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"

	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	"github.com/harpia/control-plane/internal/database"
)

type Thread struct {
	ID                        uuid.UUID
	TenantID                  uuid.UUID
	Title                     string
	Status                    chatv1.ThreadStatus
	ActivePlanConfigurationID uuid.NullUUID
	CreatedByUserID           uuid.NullUUID
	ArchivedAt                *time.Time
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

type CreateInput struct {
	TenantID        uuid.UUID
	Title           string
	CreatedByUserID uuid.NullUUID
}

type ListInput struct {
	TenantID  uuid.UUID
	Status    *chatv1.ThreadStatus
	PageSize  int32
	PageToken string
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func statusFromString(s string) (chatv1.ThreadStatus, error) {
	v, ok := chatv1.ThreadStatus_value[s]
	if !ok {
		return chatv1.ThreadStatus_THREAD_STATUS_UNSPECIFIED, fmt.Errorf("threads: unknown status %q", s)
	}
	return chatv1.ThreadStatus(v), nil
}

func statusToString(status chatv1.ThreadStatus) (string, error) {
	if status == chatv1.ThreadStatus_THREAD_STATUS_UNSPECIFIED {
		return "", errors.New("threads: unspecified status")
	}
	s, ok := chatv1.ThreadStatus_name[int32(status)]
	if !ok {
		return "", fmt.Errorf("threads: unknown status %d", status)
	}
	return s, nil
}

func (t Thread) Proto() *chatv1.Thread {
	out := &chatv1.Thread{
		Id:        t.ID.String(),
		TenantId:  t.TenantID.String(),
		Title:     t.Title,
		Status:    t.Status,
		CreatedAt: timestamppb.New(t.CreatedAt),
		UpdatedAt: timestamppb.New(t.UpdatedAt),
	}
	if t.ActivePlanConfigurationID.Valid {
		out.ActivePlanConfigurationId = t.ActivePlanConfigurationID.UUID.String()
	}
	if t.CreatedByUserID.Valid {
		out.CreatedByUserId = t.CreatedByUserID.UUID.String()
	}
	if t.ArchivedAt != nil {
		out.ArchivedAt = timestamppb.New(*t.ArchivedAt)
	}
	return out
}

func (r *Repository) Create(ctx context.Context, input CreateInput) (*Thread, error) {
	title := input.Title
	if title == "" {
		title = "Untitled chat"
	}

	var thread Thread
	err := database.WithTenant(ctx, r.pool, input.TenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx, `
			INSERT INTO threads (
				tenant_id, title, status, created_by_user_id
			) VALUES ($1, $2, $3, $4)
			RETURNING id, tenant_id, title, status, active_plan_configuration_id,
			          created_by_user_id, archived_at, created_at, updated_at
		`,
			input.TenantID,
			title,
			statusToStringMust(chatv1.ThreadStatus_THREAD_STATUS_OPEN),
			nullableUUIDArg(input.CreatedByUserID),
		)
		return scanThread(row, &thread)
	})
	if err != nil {
		return nil, fmt.Errorf("threads: create: %w", err)
	}
	return &thread, nil
}

func (r *Repository) Get(ctx context.Context, tenantID, threadID uuid.UUID) (*Thread, error) {
	var thread Thread
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx, `
			SELECT id, tenant_id, title, status, active_plan_configuration_id,
			       created_by_user_id, archived_at, created_at, updated_at
			FROM threads
			WHERE tenant_id = $1 AND id = $2
		`, tenantID, threadID)
		return scanThread(row, &thread)
	})
	if err != nil {
		return nil, fmt.Errorf("threads: get: %w", err)
	}
	return &thread, nil
}

func (r *Repository) ResolveForPlanConfiguration(ctx context.Context, tenantID, planConfigurationID uuid.UUID) (uuid.UUID, error) {
	var threadID uuid.UUID
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		return q.QueryRow(ctx, `
			SELECT thread_id
			FROM plan_configurations
			WHERE tenant_id = $1 AND id = $2
		`, tenantID, planConfigurationID).Scan(&threadID)
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("threads: resolve for plan configuration: %w", err)
	}
	return threadID, nil
}

func (r *Repository) List(ctx context.Context, input ListInput) ([]*Thread, string, error) {
	limit := int(input.PageSize)
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	query := `
		SELECT id, tenant_id, title, status, active_plan_configuration_id,
		       created_by_user_id, archived_at, created_at, updated_at
		FROM threads
		WHERE tenant_id = $1
	`
	args := []any{input.TenantID}

	if input.Status != nil {
		status, err := statusToString(*input.Status)
		if err != nil {
			return nil, "", err
		}
		args = append(args, status)
		query += fmt.Sprintf(" AND status = $%d", len(args))
	}

	if input.PageToken != "" {
		updatedAt, id, err := decodePageToken(input.PageToken)
		if err != nil {
			return nil, "", fmt.Errorf("threads: invalid page token: %w", err)
		}
		args = append(args, updatedAt, id)
		query += fmt.Sprintf(" AND (updated_at, id) < ($%d, $%d)", len(args)-1, len(args))
	}

	args = append(args, limit+1)
	query += fmt.Sprintf(" ORDER BY updated_at DESC, id DESC LIMIT $%d", len(args))

	out := make([]*Thread, 0, limit)
	err := database.WithTenant(ctx, r.pool, input.TenantID, func(q database.Querier) error {
		rows, err := q.Query(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var thread Thread
			if err := scanThread(rows, &thread); err != nil {
				return err
			}
			out = append(out, &thread)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, "", fmt.Errorf("threads: list: %w", err)
	}

	nextPageToken := ""
	if len(out) > limit {
		out = out[:limit]
		last := out[len(out)-1]
		nextPageToken = encodePageToken(last.UpdatedAt, last.ID)
	}
	return out, nextPageToken, nil
}

func (r *Repository) Archive(ctx context.Context, tenantID, threadID uuid.UUID) (*Thread, error) {
	var thread Thread
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		row := q.QueryRow(ctx, `
			UPDATE threads
			SET status = $1,
			    archived_at = now(),
			    updated_at = now()
			WHERE tenant_id = $2 AND id = $3
			RETURNING id, tenant_id, title, status, active_plan_configuration_id,
			          created_by_user_id, archived_at, created_at, updated_at
		`,
			statusToStringMust(chatv1.ThreadStatus_THREAD_STATUS_ARCHIVED),
			tenantID,
			threadID,
		)
		return scanThread(row, &thread)
	})
	if err != nil {
		return nil, fmt.Errorf("threads: archive: %w", err)
	}
	return &thread, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanThread(row scanner, thread *Thread) error {
	var status string
	var archivedAt *time.Time
	if err := row.Scan(
		&thread.ID,
		&thread.TenantID,
		&thread.Title,
		&status,
		&thread.ActivePlanConfigurationID,
		&thread.CreatedByUserID,
		&archivedAt,
		&thread.CreatedAt,
		&thread.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return fmt.Errorf("scan thread: %w", err)
	}

	parsedStatus, err := statusFromString(status)
	if err != nil {
		return err
	}
	thread.Status = parsedStatus
	thread.ArchivedAt = archivedAt
	return nil
}

func nullableUUIDArg(id uuid.NullUUID) any {
	if id.Valid {
		return id.UUID
	}
	return nil
}

func encodePageToken(updatedAt time.Time, id uuid.UUID) string {
	return updatedAt.Format(time.RFC3339Nano) + "|" + id.String()
}

func decodePageToken(token string) (time.Time, uuid.UUID, error) {
	updatedAtToken, idToken, ok := strings.Cut(token, "|")
	if !ok || updatedAtToken == "" || idToken == "" {
		return time.Time{}, uuid.Nil, errors.New("malformed token")
	}

	updatedAt, err := time.Parse(time.RFC3339Nano, updatedAtToken)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("parse updated_at: %w", err)
	}
	id, err := uuid.Parse(idToken)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("parse id: %w", err)
	}
	return updatedAt, id, nil
}

func statusToStringMust(status chatv1.ThreadStatus) string {
	s, err := statusToString(status)
	if err != nil {
		panic(err)
	}
	return s
}
