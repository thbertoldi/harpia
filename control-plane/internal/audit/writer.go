package audit

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// WriterConfig tunes the bounded background writer. Defaults are conservative
// and exposed through config.go so operators can size them per deployment.
type WriterConfig struct {
	// QueueSize is the bounded in-process channel depth. A full queue drops
	// events (best-effort delivery, not an exactly-once ledger).
	QueueSize int
	// BatchSize is the maximum number of events appended per flush.
	BatchSize int
	// MaxRetries is the number of bounded-backoff retry attempts before a
	// failed batch is reported as lost.
	MaxRetries int
	// FlushInterval bounds how long the writer waits to fill a batch before
	// flushing a partial one.
	FlushInterval time.Duration
	// RetryBaseBackoff is the initial retry delay; it doubles per attempt.
	RetryBaseBackoff time.Duration
}

// DefaultWriterConfig returns conservative defaults used when configuration
// is absent.
func DefaultWriterConfig() WriterConfig {
	return WriterConfig{
		QueueSize:        1024,
		BatchSize:        32,
		MaxRetries:       3,
		FlushInterval:    2 * time.Second,
		RetryBaseBackoff: 250 * time.Millisecond,
	}
}

// Writer drains the recorder queue and appends batches through the
// tenant-safe Repository. It runs for the lifetime of the API process and
// performs an orderly drain on shutdown.
type Writer struct {
	repo   appender
	cfg    WriterConfig
	queue  chan queuedEvent
	logger *slog.Logger
	now    func() time.Time

	stop    chan struct{}
	done    chan struct{}
	metrics *dropMetrics
}

// WriterOptions configures a Writer.
type WriterOptions struct {
	Repo   appender
	Config WriterConfig
	Logger *slog.Logger
	Now    func() time.Time
}

// NewWriter constructs (but does not start) the background writer and its
// bounded queue. Callers should keep the returned queue to construct a
// Recorder, then call Start.
func NewWriter(opts WriterOptions) (*Writer, chan queuedEvent) {
	cfg := opts.Config
	if cfg.QueueSize <= 0 {
		cfg = DefaultWriterConfig()
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 32
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 3
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 2 * time.Second
	}
	if cfg.RetryBaseBackoff <= 0 {
		cfg.RetryBaseBackoff = 250 * time.Millisecond
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now().UTC() }
	}
	queue := make(chan queuedEvent, cfg.QueueSize)
	return &Writer{
		repo:    opts.Repo,
		cfg:     cfg,
		queue:   queue,
		logger:  opts.Logger,
		now:     opts.Now,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
		metrics: newDropMetrics(),
	}, queue
}

// Metrics returns the writer's operational counters (dropped/exhausted
// events) for health-check exposure.
func (w *Writer) Metrics() *dropMetrics { return w.metrics }

// Start launches the background flush loop. It returns immediately.
func (w *Writer) Start() { go w.run() }

// Shutdown signals the writer to stop accepting new events and drain its
// queue. After Shutdown returns, queued events have been flushed (or
// reported lost with secret-free evidence). It blocks until the loop exits.
func (w *Writer) Shutdown(ctx context.Context) error {
	close(w.stop)
	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *Writer) run() {
	defer close(w.done)
	batch := make([]Event, 0, w.cfg.BatchSize)
	flushTimer := time.NewTimer(w.cfg.FlushInterval)
	defer flushTimer.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		w.flushBatch(batch)
		batch = batch[:0]
		if !flushTimer.Stop() {
			select {
			case <-flushTimer.C:
			default:
			}
		}
		flushTimer.Reset(w.cfg.FlushInterval)
	}

	for {
		select {
		case <-w.stop:
			// Drain: consume everything still queued without blocking, then
			// flush the remainder before exiting.
			drained := true
			for drained {
				select {
				case ev := <-w.queue:
					batch = append(batch, ev.event)
					if len(batch) >= w.cfg.BatchSize {
						flush()
					}
				default:
					drained = false
				}
			}
			flush()
			return
		case ev := <-w.queue:
			batch = append(batch, ev.event)
			if len(batch) >= w.cfg.BatchSize {
				flush()
			}
		case <-flushTimer.C:
			flush()
			flushTimer.Reset(w.cfg.FlushInterval)
		}
	}
}

// flushBatch appends each event through the tenant-safe Repository, retrying
// a failed event with bounded backoff. Events that exhaust their retries are
// reported lost via structured logs and the drop counter — never with
// payload data and never blocking the writer loop.
func (w *Writer) flushBatch(batch []Event) {
	ctx := context.Background()
	for i := range batch {
		event := batch[i]
		var lastErr error
		for attempt := 0; attempt <= w.cfg.MaxRetries; attempt++ {
			if err := w.repo.Append(ctx, event); err != nil {
				lastErr = err
				if attempt < w.cfg.MaxRetries {
					backoff := w.cfg.RetryBaseBackoff << attempt
					select {
					case <-time.After(backoff):
					case <-w.stop:
						// On shutdown, stop retrying and report.
						w.reportLost(event, lastErr)
						lastErr = nil
						goto next
					}
				}
				continue
			}
			lastErr = nil
			break
		}
		if lastErr != nil {
			w.reportLost(event, lastErr)
		}
	next:
	}
}

func (w *Writer) reportLost(event Event, err error) {
	w.metrics.incExhausted()
	w.logger.Error("audit event lost after retries",
		"error", err.Error(),
		"event_type", event.EventType,
		"bounded_context", event.BoundedContext,
		"tenant_id", event.TenantID.String(),
		"exhausted_total", w.metrics.exhausted.Load(),
	)
}

// dropMetrics holds atomic counters for events the writer could not deliver.
// It is secret-free: only counts and stable vocab strings are ever logged.
type dropMetrics struct {
	dropped   atomic.Int64
	exhausted atomic.Int64
}

func newDropMetrics() *dropMetrics { return &dropMetrics{} }

// Dropped reports events dropped because the queue was saturated.
func (m *dropMetrics) Dropped() int64 { return m.dropped.Load() }

// Exhausted reports events lost after exhausting write retries.
func (m *dropMetrics) Exhausted() int64 { return m.exhausted.Load() }

func (m *dropMetrics) incDropped()   { m.dropped.Add(1) }
func (m *dropMetrics) incExhausted() { m.exhausted.Add(1) }
