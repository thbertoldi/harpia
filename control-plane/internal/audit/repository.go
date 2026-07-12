package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/harpia/control-plane/internal/database"
)

// appender is the write port used by the background writer.
type appender interface {
	Append(ctx context.Context, event Event) error
}

// lister is the read port used by the public handler.
type lister interface {
	List(ctx context.Context, tenantID uuid.UUID, filters Filters, cursorOccurredAt time.Time, cursorID uuid.UUID, pageSize int) ([]Event, error)
}

// Repository persists audit events to PostgreSQL through the tenant-safe
// database.WithTenant boundary. Every read/write sets the tenant database
// context; a caller-supplied tenant ID can never bypass forced RLS.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository constructs a Repository bound to the given pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Append inserts one event. When the event carries a DedupeKey, the insert
// uses ON CONFLICT (tenant_id, dedupe_key) DO NOTHING so at-least-once
// replays (Temporal activities, interceptor retries) never duplicate a row.
// A nil/empty dedupe key always inserts.
func (r *Repository) Append(ctx context.Context, event Event) error {
	if event.TenantID == uuid.Nil {
		return errors.New("audit: tenant id is required")
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}

	diff, err := json.Marshal(event.Diff)
	if err != nil {
		return fmt.Errorf("audit: marshal diff: %w", err)
	}

	dedupeKey := nullableString(event.DedupeKey)
	agentType := nullableString(event.Actor.AgentType)
	var subjectType, subjectID any
	if event.HasSubject {
		subjectType = event.Subject.Type
		subjectID = nullableString(event.Subject.ID)
	}
	decision := nullableString(event.Decision)
	traceID := nullableString(event.TraceID)

	// The ON CONFLICT clause targets the partial unique index
	// idx_audit_events_tenant_dedupe (WHERE dedupe_key IS NOT NULL).
	// When dedupe_key is NULL the conflict target never matches, so the
	// row always inserts — matching the index's partial predicate.
	const stmt = `
		INSERT INTO audit_events (
			id, tenant_id, dedupe_key, event_type, bounded_context,
			actor_kind, actor_id, actor_display_name, actor_agent_type,
			subject_type, subject_id, payload_diff, decision, trace_id, occurred_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		)
		ON CONFLICT (tenant_id, dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING`

	return database.WithTenant(ctx, r.pool, event.TenantID, func(q database.Querier) error {
		if _, err := q.Exec(ctx, stmt,
			event.ID, event.TenantID, dedupeKey,
			event.EventType, event.BoundedContext,
			event.Actor.Kind, event.Actor.ID, event.Actor.DisplayName, agentType,
			subjectType, subjectID, diff, decision, traceID,
			event.OccurredAt,
		); err != nil {
			return fmt.Errorf("audit: append event: %w", err)
		}
		return nil
	})
}

// List returns up to pageSize events for the tenant, newest-first by
// (occurred_at DESC, id DESC), after applying the filters and the keyset
// cursor predicate. The cursor tuple (when non-zero) excludes already-paged
// rows via the strict tuple predicate (occurred_at, id) < ($c, $c).
func (r *Repository) List(ctx context.Context, tenantID uuid.UUID, filters Filters, cursorOccurredAt time.Time, cursorID uuid.UUID, pageSize int) ([]Event, error) {
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}

	query, args := buildListQuery(filters, cursorOccurredAt, cursorID, pageSize)
	var out []Event
	err := database.WithTenant(ctx, r.pool, tenantID, func(q database.Querier) error {
		rows, err := q.Query(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("audit: query events: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			event, err := scanEvent(rows)
			if err != nil {
				return err
			}
			out = append(out, event)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// buildListQuery composes the parameterized SELECT with the filters and the
// keyset tuple predicate. Filters are AND-ed; the cursor predicate is only
// added when a cursor tuple was decoded.
func buildListQuery(filters Filters, cursorOccurredAt time.Time, cursorID uuid.UUID, pageSize int) (string, []any) {
	var (
		sb       stringBuilder
		args     []any
		argIndex = 1
	)

	sb.WriteString(`
		SELECT id, tenant_id, event_type, bounded_context,
			actor_kind, actor_id, actor_display_name, actor_agent_type,
			subject_type, subject_id, payload_diff, decision, trace_id, occurred_at
		FROM audit_events
		WHERE TRUE`)

	if len(filters.EventTypes) > 0 {
		sb.WriteString(" AND event_type IN (")
		for i, et := range filters.EventTypes {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteArg(argIndex)
			args = append(args, et)
			argIndex++
		}
		sb.WriteString(")")
	}
	if filters.Actor != nil {
		sb.WriteString(" AND actor_kind = ")
		sb.WriteArg(argIndex)
		args = append(args, filters.Actor.Kind)
		argIndex++
		sb.WriteString(" AND actor_id = ")
		sb.WriteArg(argIndex)
		args = append(args, filters.Actor.ID)
		argIndex++
		if filters.Actor.AgentType != "" {
			sb.WriteString(" AND actor_agent_type = ")
			sb.WriteArg(argIndex)
			args = append(args, filters.Actor.AgentType)
			argIndex++
		}
	}
	if filters.SubjectID != "" {
		sb.WriteString(" AND subject_id = ")
		sb.WriteArg(argIndex)
		args = append(args, filters.SubjectID)
		argIndex++
	}
	if filters.Decision != "" {
		sb.WriteString(" AND decision = ")
		sb.WriteArg(argIndex)
		args = append(args, filters.Decision)
		argIndex++
	}
	if !filters.DateFrom.IsZero() {
		sb.WriteString(" AND occurred_at >= ")
		sb.WriteArg(argIndex)
		args = append(args, filters.DateFrom)
		argIndex++
	}
	if !filters.DateTo.IsZero() {
		sb.WriteString(" AND occurred_at <= ")
		sb.WriteArg(argIndex)
		args = append(args, filters.DateTo)
		argIndex++
	}

	// Strict keyset tuple predicate: strictly older than the cursor.
	// (occurred_at, id) < (c_occ, c_id) under DESC ordering means the next
	// page contains only rows older than (or equal-timestamp-but-lower-id
	// than) the last row of the previous page, with no overlap even when
	// newer events arrive between pages.
	if cursorID != uuid.Nil {
		sb.WriteString(" AND (occurred_at, id) < (")
		sb.WriteArg(argIndex)
		args = append(args, cursorOccurredAt)
		argIndex++
		sb.WriteString(", ")
		sb.WriteArg(argIndex)
		args = append(args, cursorID)
		argIndex++
		sb.WriteString(")")
	}

	// Limit to pageSize+1 so the handler can tell whether another page
	// exists without a second count query.
	sb.WriteString(" ORDER BY occurred_at DESC, id DESC LIMIT ")
	sb.WriteArg(argIndex)
	args = append(args, pageSize+1)

	return sb.String(), args
}

// scanner covers both pgx.Row and pgx.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanEvent(s scanner) (Event, error) {
	var (
		event       Event
		agentType   *string
		subjectType *string
		subjectID   *string
		decision    *string
		traceID     *string
		diffRaw     []byte
	)
	if err := s.Scan(
		&event.ID, &event.TenantID, &event.EventType, &event.BoundedContext,
		&event.Actor.Kind, &event.Actor.ID, &event.Actor.DisplayName, &agentType,
		&subjectType, &subjectID, &diffRaw, &decision, &traceID, &event.OccurredAt,
	); err != nil {
		return Event{}, fmt.Errorf("audit: scan event: %w", err)
	}
	if agentType != nil {
		event.Actor.AgentType = *agentType
	}
	if subjectType != nil && *subjectType != "" {
		event.HasSubject = true
		event.Subject.Type = *subjectType
		if subjectID != nil {
			event.Subject.ID = *subjectID
		}
	}
	if decision != nil {
		event.Decision = *decision
	}
	if traceID != nil {
		event.TraceID = *traceID
	}
	if len(diffRaw) > 0 && string(diffRaw) != "[]" {
		if err := json.Unmarshal(diffRaw, &event.Diff); err != nil {
			return Event{}, fmt.Errorf("audit: unmarshal diff: %w", err)
		}
	}
	return event, nil
}

func nullableString(v string) any {
	if v == "" {
		return nil
	}
	return v
}

// stringBuilder is a tiny helper that renders positional $n placeholders.
type stringBuilder struct {
	buf []byte
}

func (b *stringBuilder) WriteString(s string) { b.buf = append(b.buf, s...) }
func (b *stringBuilder) WriteArg(n int) {
	b.buf = append(b.buf, '$')
	b.buf = appendInt(b.buf, n)
}
func (b *stringBuilder) String() string { return string(b.buf) }

func appendInt(buf []byte, n int) []byte {
	if n == 0 {
		return append(buf, '0')
	}
	var digits [20]byte
	pos := len(digits)
	for n > 0 {
		pos--
		digits[pos] = byte('0' + n%10)
		n /= 10
	}
	return append(buf, digits[pos:]...)
}
