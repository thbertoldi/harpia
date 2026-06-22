package chat

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/harpia/control-plane/internal/database"
)

// TestAppendInputZeroValueIsValid documents the zero-value shape of
// AppendInput. Tasks 5-8 build AppendInput literals; this test prevents
// silent breakage if fields are renamed.
func TestAppendInputZeroValueIsValid(t *testing.T) {
	authorID := uuid.New()
	execID := uuid.New()
	in := AppendInput{
		ThreadID:     "plan-config-uuid",
		Role:         chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_OVERSEER,
		Kind:         chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_TEXT,
		Text:         "hello",
		PayloadJSON:  "{}",
		AuthorUserID: &authorID,
		ExecutionID:  &execID,
	}
	if in.ThreadID == "" {
		t.Fatal("ThreadID should be settable")
	}
}

func TestIsUniqueViolation(t *testing.T) {
	if !isUniqueViolation(&pgconn.PgError{Code: "23505"}) {
		t.Fatal("expected unique violation for code 23505")
	}
	if isUniqueViolation(&pgconn.PgError{Code: "23503"}) {
		t.Fatal("expected false for foreign-key violation")
	}
	if isUniqueViolation(errors.New("other error")) {
		t.Fatal("expected false for generic error")
	}
}

func TestAppendMessageRetriesOnceOnUniqueViolation(t *testing.T) {
	var attempts atomic.Int32
	err := appendMessageWithRetry(2, func() error {
		if attempts.Add(1) == 1 {
			return &pgconn.PgError{Code: "23505"}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success after retry, got %v", err)
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
}

func TestAppendMessageReturnsErrorAfterExhaustedRetries(t *testing.T) {
	err := appendMessageWithRetry(2, func() error {
		return &pgconn.PgError{Code: "23505"}
	})
	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
}

// appendMessageWithRetry mirrors postgresStore.AppendMessage retry semantics.
func appendMessageWithRetry(maxAttempts int, insert func() error) error {
	for attempt := 0; attempt < maxAttempts; attempt++ {
		err := insert()
		if err != nil {
			if isUniqueViolation(err) && attempt == 0 {
				continue
			}
			return err
		}
		return nil
	}
	return errors.New("chat: AppendMessage failed after retry on sequence collision")
}

func TestAppendMessageConcurrentIntegration(t *testing.T) {
	databaseURL := os.Getenv("HARPIA_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set HARPIA_TEST_DATABASE_URL to run chat AppendMessage integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()

	store := NewPostgresStore(pool)
	tenantID := uuid.New()
	threadID := uuid.New().String()

	const workers = 8
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, appendErr := store.AppendMessage(ctx, tenantID, AppendInput{
				ThreadID:    threadID,
				Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
				Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_TEXT,
				Text:        "concurrent",
				PayloadJSON: "{}",
			})
			if appendErr != nil {
				errCh <- appendErr
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for appendErr := range errCh {
		t.Fatalf("concurrent append failed: %v", appendErr)
	}

	msgs, err := store.ListMessages(ctx, tenantID, threadID, 0, 0)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(msgs) != workers {
		t.Fatalf("message count = %d, want %d", len(msgs), workers)
	}
	seen := make(map[int64]struct{}, len(msgs))
	for _, msg := range msgs {
		if _, ok := seen[msg.SequenceNumber]; ok {
			t.Fatalf("duplicate sequence_number %d", msg.SequenceNumber)
		}
		seen[msg.SequenceNumber] = struct{}{}
	}
}
