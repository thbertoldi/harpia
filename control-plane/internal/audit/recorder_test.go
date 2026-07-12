package audit

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

// G4: Record must not block even when the queue is saturated; a full queue
// drops the event and reports secret-free evidence.
func TestRecorderDropsWhenQueueSaturated(t *testing.T) {
	t.Parallel()

	queue := make(chan queuedEvent, 1)
	// Fill the queue so the next send has nowhere to go.
	queue <- queuedEvent{event: Event{ID: uuid.New(), TenantID: uuid.New(), EventType: eventPlanExecutionStarted}}

	rec := NewRecorder(RecorderOptions{
		Queue:  queue,
		Logger: slog.New(slog.NewTextHandler(&discardWriter{}, &slog.HandlerOptions{Level: slog.LevelError})),
	})

	// Record must return promptly (non-blocking) with a drop error.
	done := make(chan error, 1)
	go func() {
		done <- rec.Record(context.Background(), validDraft())
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected drop error on saturated queue")
		}
	case <-time.After(time.Second):
		t.Fatal("Record blocked on saturated queue")
	}
}

// Record must accept a valid event and hand it to the queue.
func TestRecorderEnqueuesValidEvent(t *testing.T) {
	t.Parallel()

	queue := make(chan queuedEvent, 4)
	rec := NewRecorder(RecorderOptions{Queue: queue})

	draft := validDraft()
	if err := rec.Record(context.Background(), draft); err != nil {
		t.Fatalf("Record: %v", err)
	}
	select {
	case ev := <-queue:
		if ev.event.EventType != eventPlanExecutionStarted {
			t.Fatalf("event type = %q", ev.event.EventType)
		}
		if ev.event.ID == uuid.Nil {
			t.Fatal("recorder must assign a stable id")
		}
	case <-time.After(time.Second):
		t.Fatal("event not enqueued")
	}
}

// G6: secrets must be redacted before the event reaches the queue.
func TestRecorderRedactsBeforeEnqueue(t *testing.T) {
	t.Parallel()

	queue := make(chan queuedEvent, 4)
	rec := NewRecorder(RecorderOptions{Queue: queue})

	draft := validDraft()
	draft.Diff = []DiffEntry{
		{Field: "status", After: "runnable", HasAfter: true},
		{Field: "api_key", After: "sk-secret", HasAfter: true},
	}
	draft.DiffAllowlist = []string{"status"}

	if err := rec.Record(context.Background(), draft); err != nil {
		t.Fatalf("Record: %v", err)
	}
	ev := <-queue

	var apiKey Entry
	for _, d := range ev.event.Diff {
		if d.Field == "api_key" {
			apiKey = d
		}
	}
	if apiKey.Field == "" {
		t.Fatal("api_key must be present (redacted) on the queue")
	}
	if apiKey.After != redactedPlaceholder {
		t.Fatalf("api_key must be redacted before enqueue, got %q", apiKey.After)
	}
}

// Record must reject an event with an unknown vocabulary without enqueueing.
func TestRecorderRejectsUnknownEventType(t *testing.T) {
	t.Parallel()

	queue := make(chan queuedEvent, 4)
	rec := NewRecorder(RecorderOptions{Queue: queue})

	draft := validDraft()
	draft.EventType = "totally.unknown.event"
	if err := rec.Record(context.Background(), draft); err == nil {
		t.Fatal("expected validation error for unknown event type")
	}
	select {
	case <-queue:
		t.Fatal("invalid event must not be enqueued")
	default:
	}
}

func TestRecorderDoesNotMutateDraft(t *testing.T) {
	t.Parallel()

	queue := make(chan queuedEvent, 4)
	rec := NewRecorder(RecorderOptions{Queue: queue})

	draft := validDraft()
	originalDiff := draft.Diff
	_ = rec.Record(context.Background(), draft)
	if len(draft.Diff) != len(originalDiff) {
		t.Fatalf("recorder mutated caller's diff slice")
	}
}

// --- writer tests ---

type stubAppender struct {
	calls atomic.Int64
	fn    func(ctx context.Context, event Event) error
}

func (s *stubAppender) Append(ctx context.Context, event Event) error {
	s.calls.Add(1)
	if s.fn != nil {
		return s.fn(ctx, event)
	}
	return nil
}

// G4: the writer batches events through the repository and a successful
// flush appends each event exactly once.
func TestWriterFlushesBatch(t *testing.T) {
	t.Parallel()

	appender := &stubAppender{}
	w, queue := NewWriter(WriterOptions{
		Repo: appender,
		Config: WriterConfig{
			QueueSize: 8, BatchSize: 2, MaxRetries: 1,
			FlushInterval: 10 * time.Millisecond, RetryBaseBackoff: time.Millisecond,
		},
		Logger: slog.New(slog.NewTextHandler(&discardWriter{}, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	w.Start()

	for i := 0; i < 3; i++ {
		queue <- queuedEvent{event: Event{ID: uuid.New(), TenantID: uuid.New(), EventType: eventPlanExecutionStarted}}
	}

	if !waitFor(func() bool { return appender.calls.Load() >= 3 }, time.Second) {
		t.Fatalf("expected 3 appends, got %d", appender.calls.Load())
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := w.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

// G4: the writer drains queued events on orderly shutdown.
func TestWriterDrainsOnShutdown(t *testing.T) {
	t.Parallel()

	appender := &stubAppender{}
	w, queue := NewWriter(WriterOptions{
		Repo: appender,
		Config: WriterConfig{
			QueueSize: 16, BatchSize: 32, MaxRetries: 0,
			FlushInterval: time.Hour, RetryBaseBackoff: time.Millisecond,
		},
		Logger: slog.New(slog.NewTextHandler(&discardWriter{}, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	// A long flush interval means the only way these get written is via the
	// shutdown drain path.
	w.Start()

	for i := 0; i < 5; i++ {
		queue <- queuedEvent{event: Event{ID: uuid.New(), TenantID: uuid.New(), EventType: eventPlanExecutionStarted}}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := w.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if appender.calls.Load() != 5 {
		t.Fatalf("expected drain to flush 5 events, got %d", appender.calls.Load())
	}
	if w.Metrics().Exhausted() != 0 {
		t.Fatalf("expected 0 exhausted, got %d", w.Metrics().Exhausted())
	}
}

// A persistently failing append exhausts retries and reports loss without
// leaking payload data and without blocking forever.
func TestWriterReportsExhaustedLoss(t *testing.T) {
	t.Parallel()

	appender := &stubAppender{
		fn: func(context.Context, Event) error { return errors.New("db down") },
	}
	w, queue := NewWriter(WriterOptions{
		Repo: appender,
		Config: WriterConfig{
			QueueSize: 8, BatchSize: 1, MaxRetries: 1,
			FlushInterval: 10 * time.Millisecond, RetryBaseBackoff: time.Millisecond,
		},
		Logger: slog.New(slog.NewTextHandler(&discardWriter{}, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	w.Start()

	queue <- queuedEvent{event: Event{ID: uuid.New(), TenantID: uuid.New(), EventType: eventPlanExecutionStarted}}

	if !waitFor(func() bool { return w.Metrics().Exhausted() >= 1 }, time.Second) {
		t.Fatalf("expected exhausted metric >=1, got %d", w.Metrics().Exhausted())
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = w.Shutdown(ctx)
}

func TestWriterConfigDefaultsWhenZero(t *testing.T) {
	t.Parallel()

	w, _ := NewWriter(WriterOptions{Repo: &stubAppender{}})
	if w.cfg.QueueSize != 1024 || w.cfg.BatchSize != 32 || w.cfg.MaxRetries != 3 {
		t.Fatalf("defaults wrong: %+v", w.cfg)
	}
}

// --- helpers ---

func validDraft() EventDraft {
	return EventDraft{
		TenantID:       uuid.New(),
		EventType:      eventPlanExecutionStarted,
		BoundedContext: bcWorkflowEngine,
		Actor:          Actor{Kind: actorKindWorkflowEngine, ID: "wf-1", DisplayName: "Engine"},
		HasSubject:     true,
		Subject:        Subject{Type: subjectPlanExecution, ID: "pe-1"},
		Diff:           []DiffEntry{{Field: "status", Before: "pending", After: "running", HasBefore: true, HasAfter: true}},
		DiffAllowlist:  []string{"status"},
	}
}

func waitFor(cond func() bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return cond()
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// Entry is an alias used in recorder tests for diff lookup clarity.
type Entry = DiffEntry
