package audit

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// Recorder is the narrow port domain packages use to record an audited
// operation. Domain code depends on this interface, not on the recorder
// implementation, the writer, pgx, or Connect — keeping the audit adapter
// an infrastructure concern (hexagonal boundary).
type Recorder interface {
	// Record validates and redacts a copy of the draft, then enqueues it
	// for the background writer. It does not wait for a database insert,
	// so a successful request never pays audit-write latency. A full queue
	// drops the event and emits structured evidence without blocking.
	Record(ctx context.Context, draft EventDraft) error
}

// EventDraft is the caller-supplied, pre-redaction description of an audited
// transition. The recorder normalizes it into a safe Event before enqueue.
type EventDraft struct {
	TenantID       uuid.UUID
	DedupeKey      string
	EventType      string
	BoundedContext string
	Actor          Actor
	Subject        Subject
	HasSubject     bool
	Diff           []DiffEntry
	// DiffAllowlist names the safe fields the caller expects to audit; any
	// diff field not on this list is dropped before persistence.
	DiffAllowlist []string
	Decision      string
	TraceID       string
	OccurredAt    time.Time
}

// recorder is the concrete Recorder. It hands validated events to a bounded
// queue that the background Writer drains.
type recorder struct {
	queue   chan<- queuedEvent
	logger  *slog.Logger
	now     func() time.Time
	metrics *dropMetrics
}

// queuedEvent is what travels through the bounded queue.
type queuedEvent struct {
	event Event
}

// RecorderOptions configures the concrete recorder.
type RecorderOptions struct {
	Queue   chan<- queuedEvent
	Logger  *slog.Logger
	Now     func() time.Time
	Metrics *dropMetrics
}

// NewRecorder constructs a Recorder that enqueues to the supplied queue.
func NewRecorder(opts RecorderOptions) Recorder {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now().UTC() }
	}
	if opts.Metrics == nil {
		opts.Metrics = newDropMetrics()
	}
	return &recorder{
		queue:   opts.Queue,
		logger:  opts.Logger,
		now:     opts.Now,
		metrics: opts.Metrics,
	}
}

// Record implements Recorder. It is non-blocking with respect to the
// database: the only blocking step is a bounded-channel send guarded by a
// default branch, so a saturated queue drops the event (with structured,
// secret-free evidence) instead of stalling the caller.
func (r *recorder) Record(_ context.Context, draft EventDraft) error {
	event, err := r.normalize(draft)
	if err != nil {
		// Programmer error in event shape; surface to the caller's logs but
		// never block the operation. Secret-free by construction.
		r.logger.Warn("audit event rejected before enqueue",
			"error", err.Error(),
			"event_type", draft.EventType,
			"bounded_context", draft.BoundedContext,
		)
		return err
	}

	select {
	case r.queue <- queuedEvent{event: event}:
		return nil
	default:
		// Queue saturated: drop this event and emit observable evidence
		// with no payload data, per design.md:60-64,74-79.
		r.metrics.incDropped()
		r.logger.Warn("audit queue saturated; event dropped",
			"event_type", event.EventType,
			"bounded_context", event.BoundedContext,
			"tenant_id", event.TenantID.String(),
			"dropped_total", r.metrics.dropped.Load(),
		)
		return errors.New("audit: queue saturated, event dropped")
	}
}

// normalize validates the draft, redacts its diff, and assigns stable IDs.
// It never mutates the caller's draft.
func (r *recorder) normalize(draft EventDraft) (Event, error) {
	if draft.TenantID == uuid.Nil {
		return Event{}, errors.New("audit: tenant id is required")
	}
	if draft.EventType == "" || eventTypeToProto(draft.EventType) == auditEventTypeUnspecified {
		return Event{}, errors.New("audit: a known event type is required")
	}
	if draft.BoundedContext == "" || boundedContextToProto(draft.BoundedContext) == boundedContextUnspecified {
		return Event{}, errors.New("audit: a known bounded context is required")
	}
	if draft.Actor.Kind == "" || draft.Actor.ID == "" {
		return Event{}, errors.New("audit: actor kind and id are required")
	}
	if draft.HasSubject && (draft.Subject.Type == "" || draft.Subject.ID == "") {
		return Event{}, errors.New("audit: subject requires type and id")
	}
	if draft.Decision != "" && decisionToProto(draft.Decision) == feedbackDecisionUnspecified {
		return Event{}, errors.New("audit: unknown decision value")
	}

	occurredAt := draft.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = r.now()
	}

	return Event{
		ID:             uuid.New(),
		TenantID:       draft.TenantID,
		DedupeKey:      draft.DedupeKey,
		EventType:      draft.EventType,
		BoundedContext: draft.BoundedContext,
		Actor:          draft.Actor,
		Subject:        draft.Subject,
		HasSubject:     draft.HasSubject,
		Diff:           redactDiff(cloneDiff(draft.Diff), draft.DiffAllowlist),
		Decision:       draft.Decision,
		TraceID:        draft.TraceID,
		OccurredAt:     occurredAt.UTC(),
	}, nil
}

func cloneDiff(in []DiffEntry) []DiffEntry {
	if len(in) == 0 {
		return nil
	}
	out := make([]DiffEntry, len(in))
	copy(out, in)
	return out
}
