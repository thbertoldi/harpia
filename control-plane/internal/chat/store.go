package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"

	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	"github.com/harpia/control-plane/internal/database"
)

// Store is the generic chat-message persistence interface. M3 wires a
// Postgres implementation; tests may stub it.
//
// Each method opens its own database.WithTenant transaction. Callers that
// need to append chat messages AND mutate other tables atomically (e.g. a
// conversational turn that binds a plan configuration) must use the tx-bound
// variants — AppendMessageTx, ListMessagesTx, LockThreadForUpdate — inside a
// single database.WithTenant callback.
type Store interface {
	// AppendMessage writes one ThreadMessage and returns the persisted row
	// (with assigned ID and sequence_number).
	AppendMessage(ctx context.Context, tenantID uuid.UUID, input AppendInput) (*chatv1.ThreadMessage, error)
	// ListMessages returns messages keyed by threads.id with sequence_number >
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

// AppendMessage opens its own transaction and delegates to AppendMessageTx.
// Existing callers (chat.Store consumers) are unaffected.
func (s *postgresStore) AppendMessage(ctx context.Context, tenantID uuid.UUID, input AppendInput) (*chatv1.ThreadMessage, error) {
	var msg *chatv1.ThreadMessage
	err := database.WithTenant(ctx, s.pool, tenantID, func(q database.Querier) error {
		var appendErr error
		msg, appendErr = AppendMessageTx(ctx, q, tenantID, input)
		return appendErr
	})
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// ListMessages opens its own transaction and delegates to ListMessagesTx.
// Existing callers (chat.Store consumers) are unaffected.
func (s *postgresStore) ListMessages(ctx context.Context, tenantID uuid.UUID, threadID string, sinceSeq int64, limit int) ([]*chatv1.ThreadMessage, error) {
	var out []*chatv1.ThreadMessage
	err := database.WithTenant(ctx, s.pool, tenantID, func(q database.Querier) error {
		var listErr error
		out, listErr = ListMessagesTx(ctx, q, tenantID, threadID, sinceSeq, limit)
		return listErr
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// AppendMessageTx appends one ThreadMessage using a caller-provided Querier
// (typically the Querier handed to a database.WithTenant callback). The caller
// owns commit/rollback and is responsible for serializing concurrent turns on
// the same thread — call LockThreadForUpdate first.
//
// It mirrors AppendMessage's MAX(sequence_number)+1 logic and single retry on
// (thread_id, sequence_number) unique violation. The retry runs inside the
// caller's transaction: each attempt is wrapped in a uniquely-named SAVEPOINT
// so a failed INSERT does not abort the surrounding transaction. (A plain
// constraint violation would otherwise poison the tx and make the retry
// impossible.) Multiple AppendMessageTx calls may share one transaction (the
// conversational-turn case), hence the per-call unique savepoint name.
//
// RLS is scoped to the tenant by database.WithTenant; the explicit tenant_id
// argument is belt-and-suspenders for the INSERT column.
func AppendMessageTx(ctx context.Context, q database.Querier, tenantID uuid.UUID, input AppendInput) (*chatv1.ThreadMessage, error) {
	args, err := prepareAppendArgs(input)
	if err != nil {
		return nil, err
	}

	for attempt := 0; attempt < 2; attempt++ {
		// Unique per call so two appends in the same tx don't collide.
		savepoint := "sp_chat_append_" + strings.ReplaceAll(uuid.NewString(), "-", "_")
		if _, err := q.Exec(ctx, "SAVEPOINT "+savepoint); err != nil {
			return nil, fmt.Errorf("chat: AppendMessage: savepoint: %w", err)
		}

		var (
			id        uuid.UUID
			seq       int64
			createdAt time.Time
		)
		scanErr := q.QueryRow(ctx, `
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
			args.threadID,
			tenantID,
			args.execArg,
			args.role,
			args.kind,
			args.text,
			args.payload,
			args.authorArg,
		).Scan(&id, &seq, &createdAt)

		if scanErr != nil {
			// Roll back to the savepoint so the caller's tx stays usable
			// whether we retry or propagate.
			if _, rbErr := q.Exec(ctx, "ROLLBACK TO SAVEPOINT "+savepoint); rbErr != nil {
				return nil, fmt.Errorf("chat: AppendMessage: rollback savepoint (%v): %w", rbErr, scanErr)
			}
			// Detect unique-violation on (thread_id, sequence_number) and retry once.
			if isUniqueViolation(scanErr) && attempt == 0 {
				continue
			}
			return nil, fmt.Errorf("chat: AppendMessage: %w", scanErr)
		}

		if _, err := q.Exec(ctx, "RELEASE SAVEPOINT "+savepoint); err != nil {
			return nil, fmt.Errorf("chat: AppendMessage: release savepoint: %w", err)
		}

		msg := &chatv1.ThreadMessage{
			Id:             id.String(),
			TenantId:       tenantID.String(),
			ThreadId:       args.threadID,
			Role:           input.Role,
			Kind:           input.Kind,
			Text:           input.Text,
			PayloadJson:    args.payload,
			SequenceNumber: seq,
			CreatedAt:      timestamppb.New(createdAt),
		}
		if input.ExecutionID != nil {
			msg.ExecutionId = input.ExecutionID.String()
		}
		if input.AuthorUserID != nil {
			msg.AuthorUserId = input.AuthorUserID.String()
		}
		return msg, nil
	}
	return nil, errors.New("chat: AppendMessage failed after retry on sequence collision")
}

// ListMessagesTx lists messages for a thread using a caller-provided Querier
// (typically a database.WithTenant transaction). Same shape as
// Store.ListMessages: messages with sequence_number > sinceSeq, ascending, up
// to `limit` rows (limit <= 0 means no limit).
func ListMessagesTx(ctx context.Context, q database.Querier, tenantID uuid.UUID, threadID string, sinceSeq int64, limit int) ([]*chatv1.ThreadMessage, error) {
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

	out := make([]*chatv1.ThreadMessage, 0)
	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("chat: ListMessages: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id, tenant uuid.UUID
			tid        string
			execID     uuid.NullUUID
			role, kind string
			text       string
			payload    string
			authorID   uuid.NullUUID
			seq        int64
			createdAt  time.Time
		)
		if err := rows.Scan(&id, &tenant, &tid, &execID, &role, &kind, &text, &payload, &authorID, &seq, &createdAt); err != nil {
			return nil, fmt.Errorf("chat: ListMessages: scan: %w", err)
		}
		msg := &chatv1.ThreadMessage{
			Id:             id.String(),
			TenantId:       tenant.String(),
			ThreadId:       tid,
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
		return nil, fmt.Errorf("chat: ListMessages: %w", err)
	}
	return out, nil
}

// LockThreadForUpdate takes a per-thread row lock on threads.id within the
// caller's transaction (the supplied Querier). Concurrent turns that lock the
// same thread serialize here, which makes the per-thread chat_messages
// sequence counter race-free: once the lock is held, MAX(sequence_number)+1
// computed later in the same transaction cannot be observed by another
// transaction.
//
// The thread is the chat aggregate (chat_messages.thread_id → threads.id), so
// this helper lives in the chat package — the plans package must not own
// thread SQL (hexagonal boundary).
//
// The caller must already be inside a database.WithTenant transaction so RLS
// scopes the SELECT to the tenant. Returns pgx.ErrNoRows when the thread does
// not exist (or belongs to a different tenant).
func LockThreadForUpdate(ctx context.Context, q database.Querier, tenantID, threadID uuid.UUID) error {
	var id uuid.UUID
	if err := q.QueryRow(ctx, `
		SELECT id FROM threads
		WHERE tenant_id = $1 AND id = $2
		FOR UPDATE
	`, tenantID, threadID).Scan(&id); err != nil {
		return fmt.Errorf("chat: lock thread %s for tenant %s: %w", threadID, tenantID, err)
	}
	return nil
}

// appendArgs is the validated, prepared argument bundle for an append.
type appendArgs struct {
	threadID  string
	role      string
	kind      string
	text      string
	payload   string
	authorArg any
	execArg   any
}

// prepareAppendArgs validates AppendInput and renders the bind arguments
// shared by every append path (the Store method and the tx-bound variant).
func prepareAppendArgs(input AppendInput) (appendArgs, error) {
	if input.ThreadID == "" {
		return appendArgs{}, errors.New("chat: AppendMessage requires ThreadID")
	}
	if input.Role == chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_UNSPECIFIED {
		return appendArgs{}, errors.New("chat: AppendMessage requires Role")
	}
	if input.Kind == chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_UNSPECIFIED {
		return appendArgs{}, errors.New("chat: AppendMessage requires Kind")
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
	return appendArgs{
		threadID:  input.ThreadID,
		role:      input.Role.String(),
		kind:      input.Kind.String(),
		text:      input.Text,
		payload:   payload,
		authorArg: authorArg,
		execArg:   execArg,
	}, nil
}

// isUniqueViolation returns true if err is a Postgres unique constraint violation.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
