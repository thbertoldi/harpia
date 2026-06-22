package chat

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"

	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
)

// Store is the generic chat-message persistence interface. M3 wires a
// Postgres implementation; tests may stub it.
type Store interface {
	// AppendMessage writes one ThreadMessage and returns the persisted row
	// (with assigned ID and sequence_number).
	AppendMessage(ctx context.Context, tenantID uuid.UUID, input AppendInput) (*chatv1.ThreadMessage, error)
	// ListMessages returns messages for thread_id with sequence_number >
	// sinceSeq, ordered ascending, limited to `limit` rows. limit <= 0 means
	// no limit.
	ListMessages(ctx context.Context, tenantID uuid.UUID, threadID string, sinceSeq int64, limit int) ([]*chatv1.ThreadMessage, error)
}

// AppendInput is the payload for Store.AppendMessage. ThreadID and Role and
// Kind are required; other fields are optional.
type AppendInput struct {
	ThreadID     string
	Role         chatv1.ThreadMessageRole
	Kind         chatv1.ThreadMessageKind
	Text         string
	PayloadJSON  string
	AuthorUserID *uuid.UUID
	ExecutionID  *uuid.UUID
}

// NewPostgresStore returns a Store backed by the given pgxpool.Pool.
func NewPostgresStore(pool *pgxpool.Pool) Store {
	return &postgresStore{pool: pool}
}

type postgresStore struct {
	pool *pgxpool.Pool
}

func (s *postgresStore) AppendMessage(ctx context.Context, tenantID uuid.UUID, input AppendInput) (*chatv1.ThreadMessage, error) {
	if input.ThreadID == "" {
		return nil, errors.New("chat: AppendMessage requires ThreadID")
	}
	if input.Role == chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_UNSPECIFIED {
		return nil, errors.New("chat: AppendMessage requires Role")
	}
	if input.Kind == chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_UNSPECIFIED {
		return nil, errors.New("chat: AppendMessage requires Kind")
	}
	payload := input.PayloadJSON
	if payload == "" {
		payload = "{}"
	}
	var authorArg any
	if input.AuthorUserID != nil {
		authorArg = *input.AuthorUserID
	}
	var execArg any
	if input.ExecutionID != nil {
		execArg = *input.ExecutionID
	}

	// sequence_number is computed per-thread as MAX(existing)+1. The unique
	// constraint on (thread_id, sequence_number) catches races; on conflict
	// we retry once before giving up.
	for attempt := 0; attempt < 2; attempt++ {
		var msg chatv1.ThreadMessage
		var id uuid.UUID
		var createdAt time.Time
		var seq int64
		err := s.pool.QueryRow(ctx, `
			WITH next AS (
				SELECT COALESCE(MAX(sequence_number), 0) + 1 AS seq
				FROM chat_messages
				WHERE thread_id = $1
			)
			INSERT INTO chat_messages (
				tenant_id, thread_id, execution_id, role, kind, text, payload_json,
				author_user_id, sequence_number
			)
			SELECT $2, $1, $3, $4, $5, $6, $7::jsonb, $8, next.seq FROM next
			RETURNING id, sequence_number, created_at
		`,
			input.ThreadID,
			tenantID,
			execArg,
			input.Role.String(),
			input.Kind.String(),
			input.Text,
			payload,
			authorArg,
		).Scan(&id, &seq, &createdAt)
		if err != nil {
			// Detect unique-violation on (thread_id, sequence_number) and retry.
			if isUniqueViolation(err) && attempt == 0 {
				continue
			}
			return nil, fmt.Errorf("chat: AppendMessage: %w", err)
		}
		msg.Id = id.String()
		msg.TenantId = tenantID.String()
		msg.ThreadId = input.ThreadID
		if input.ExecutionID != nil {
			msg.ExecutionId = input.ExecutionID.String()
		}
		msg.Role = input.Role
		msg.Kind = input.Kind
		msg.Text = input.Text
		msg.PayloadJson = payload
		if input.AuthorUserID != nil {
			msg.AuthorUserId = input.AuthorUserID.String()
		}
		msg.SequenceNumber = seq
		msg.CreatedAt = timestamppb.New(createdAt)
		return &msg, nil
	}
	return nil, errors.New("chat: AppendMessage failed after retry on sequence collision")
}

func (s *postgresStore) ListMessages(ctx context.Context, tenantID uuid.UUID, threadID string, sinceSeq int64, limit int) ([]*chatv1.ThreadMessage, error) {
	query := `
		SELECT id, tenant_id, thread_id, execution_id, role, kind, text,
		       payload_json::text, author_user_id, sequence_number, created_at
		FROM chat_messages
		WHERE tenant_id = $1 AND thread_id = $2 AND sequence_number > $3
		ORDER BY sequence_number ASC
	`
	args := []any{tenantID, threadID, sinceSeq}
	if limit > 0 {
		query += " LIMIT $4"
		args = append(args, limit)
	}
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("chat: ListMessages: %w", err)
	}
	defer rows.Close()

	out := make([]*chatv1.ThreadMessage, 0)
	for rows.Next() {
		var (
			id, tenant uuid.UUID
			threadID   string
			execID     uuid.NullUUID
			role, kind string
			text       string
			payload    string
			authorID   uuid.NullUUID
			seq        int64
			createdAt  time.Time
		)
		if err := rows.Scan(&id, &tenant, &threadID, &execID, &role, &kind, &text, &payload, &authorID, &seq, &createdAt); err != nil {
			return nil, fmt.Errorf("chat: ListMessages: scan: %w", err)
		}
		msg := &chatv1.ThreadMessage{
			Id:             id.String(),
			TenantId:       tenant.String(),
			ThreadId:       threadID,
			Role:           chatv1.ThreadMessageRole(chatv1.ThreadMessageRole_value[role]),
			Kind:           chatv1.ThreadMessageKind(chatv1.ThreadMessageKind_value[kind]),
			Text:           text,
			PayloadJson:    payload,
			SequenceNumber: seq,
			CreatedAt:      timestamppb.New(createdAt),
		}
		if execID.Valid {
			msg.ExecutionId = execID.UUID.String()
		}
		if authorID.Valid {
			msg.AuthorUserId = authorID.UUID.String()
		}
		out = append(out, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chat: ListMessages: rows: %w", err)
	}
	return out, nil
}

// isUniqueViolation returns true if err is a Postgres unique constraint violation.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
