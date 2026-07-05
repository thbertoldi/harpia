package chat

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/harpia/control-plane/internal/database"
)

// TestAppendInputZeroValueIsValid documents the zero-value shape of
// AppendInput. Tasks 5-8 build AppendInput literals; this test prevents
// silent breakage if fields are renamed.
func TestAppendInputZeroValueIsValid(t *testing.T) {
	authorID := uuid.New()
	execID := uuid.New()
	in := AppendInput{
		ThreadID:     uuid.New().String(),
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

// setupChatTxFixtures creates a fresh tenant + thread for one chat-tx
// integration test. Each test gets its own tenant so cases are isolated.
func setupChatTxFixtures(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (tenantID, threadID uuid.UUID) {
	t.Helper()
	slug := "chat-tx-" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := pool.QueryRow(ctx,
		`INSERT INTO tenants (name, slug) VALUES ($1, $2) RETURNING id`,
		"chat tx test", slug,
	).Scan(&tenantID); err != nil {
		t.Fatalf("create tenant fixture: %v", err)
	}
	if err := database.WithTenant(ctx, pool, tenantID, func(q database.Querier) error {
		return q.QueryRow(ctx,
			`INSERT INTO threads (tenant_id, title, status) VALUES ($1, $2, $3) RETURNING id`,
			tenantID, "chat tx test", "THREAD_STATUS_OPEN",
		).Scan(&threadID)
	}); err != nil {
		t.Fatalf("create thread fixture: %v", err)
	}
	return tenantID, threadID
}

// TestAppendMessageTxAssignsNextSequence proves a tx-bound append inside a
// database.WithTenant callback writes the message with the correct next
// sequence number on a fresh thread.
func TestAppendMessageTxAssignsNextSequence(t *testing.T) {
	databaseURL := os.Getenv("HARPIA_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set HARPIA_TEST_DATABASE_URL to run chat AppendMessageTx integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()

	tenantID, threadID := setupChatTxFixtures(t, ctx, pool)

	var msg *chatv1.ThreadMessage
	err = database.WithTenant(ctx, pool, tenantID, func(q database.Querier) error {
		var appendErr error
		msg, appendErr = AppendMessageTx(ctx, q, tenantID, AppendInput{
			ThreadID:    threadID.String(),
			Role:        chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_OVERSEER,
			Kind:        chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_TEXT,
			Text:        "hello tx",
			PayloadJSON: "{}",
		})
		return appendErr
	})
	if err != nil {
		t.Fatalf("AppendMessageTx: %v", err)
	}
	if msg.SequenceNumber != 1 {
		t.Fatalf("SequenceNumber = %d, want 1 (fresh thread)", msg.SequenceNumber)
	}
	if msg.ThreadId != threadID.String() {
		t.Fatalf("ThreadId = %q, want %q", msg.ThreadId, threadID)
	}

	// Verify the write is durable once the WithTenant commits.
	store := NewPostgresStore(pool)
	msgs, err := store.ListMessages(ctx, tenantID, threadID.String(), 0, 0)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(msgs) != 1 || msgs[0].SequenceNumber != 1 {
		t.Fatalf("persisted = %+v", msgs)
	}
}

// TestAppendMessageTxConsecutiveInSameTransaction proves two appends issued
// in the SAME transaction receive consecutive sequence numbers — the
// conversational-turn case where multiple chat writes + plan mutations must
// be atomic.
func TestAppendMessageTxConsecutiveInSameTransaction(t *testing.T) {
	databaseURL := os.Getenv("HARPIA_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set HARPIA_TEST_DATABASE_URL to run chat AppendMessageTx integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()

	tenantID, threadID := setupChatTxFixtures(t, ctx, pool)
	threadIDStr := threadID.String()

	var first, second *chatv1.ThreadMessage
	err = database.WithTenant(ctx, pool, tenantID, func(q database.Querier) error {
		// Lock once up front; the two appends then share the tx's view of
		// MAX(sequence_number), so they pick seq=1 then seq=2.
		if err := LockThreadForUpdate(ctx, q, tenantID, threadID); err != nil {
			return err
		}
		var appendErr error
		first, appendErr = AppendMessageTx(ctx, q, tenantID, AppendInput{
			ThreadID: threadIDStr,
			Role:     chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_OVERSEER,
			Kind:     chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_USER_TEXT,
			Text:     "first",
		})
		if appendErr != nil {
			return appendErr
		}
		second, appendErr = AppendMessageTx(ctx, q, tenantID, AppendInput{
			ThreadID: threadIDStr,
			Role:     chatv1.ThreadMessageRole_THREAD_MESSAGE_ROLE_SYSTEM,
			Kind:     chatv1.ThreadMessageKind_THREAD_MESSAGE_KIND_ASSISTANT_PROMPT,
			Text:     "second",
		})
		return appendErr
	})
	if err != nil {
		t.Fatalf("tx-bound double append: %v", err)
	}
	if first.SequenceNumber != 1 {
		t.Fatalf("first SequenceNumber = %d, want 1", first.SequenceNumber)
	}
	if second.SequenceNumber != 2 {
		t.Fatalf("second SequenceNumber = %d, want 2", second.SequenceNumber)
	}

	store := NewPostgresStore(pool)
	msgs, err := store.ListMessages(ctx, tenantID, threadIDStr, 0, 0)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("persisted count = %d, want 2", len(msgs))
	}
	if msgs[0].SequenceNumber != 1 || msgs[1].SequenceNumber != 2 {
		t.Fatalf("persisted sequence order = %d, %d", msgs[0].SequenceNumber, msgs[1].SequenceNumber)
	}
}

// TestLockThreadForUpdateBlocksConcurrentTransactions proves SELECT ... FOR
// UPDATE actually serializes: a second transaction trying to lock the same
// thread blocks while the first holds the lock, then proceeds once the first
// commits.
func TestLockThreadForUpdateBlocksConcurrentTransactions(t *testing.T) {
	databaseURL := os.Getenv("HARPIA_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set HARPIA_TEST_DATABASE_URL to run chat LockThreadForUpdate integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()

	tenantID, threadID := setupChatTxFixtures(t, ctx, pool)

	aLocked := make(chan struct{})
	releaseA := make(chan struct{})
	aDone := make(chan error, 1)

	// Goroutine A acquires the lock and holds it until releaseA fires.
	go func() {
		aDone <- database.WithTenant(ctx, pool, tenantID, func(q database.Querier) error {
			if err := LockThreadForUpdate(ctx, q, tenantID, threadID); err != nil {
				return err
			}
			close(aLocked)
			select {
			case <-releaseA:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()

	// Wait for A to actually grab the lock.
	select {
	case <-aLocked:
	case <-time.After(5 * time.Second):
		t.Fatal("A did not acquire lock in time")
	}

	bLocked := make(chan struct{})
	bDone := make(chan error, 1)

	// Goroutine B tries to lock the same thread — must block while A holds it.
	go func() {
		bDone <- database.WithTenant(ctx, pool, tenantID, func(q database.Querier) error {
			if err := LockThreadForUpdate(ctx, q, tenantID, threadID); err != nil {
				return err
			}
			close(bLocked)
			return nil
		})
	}()

	// B should NOT acquire the lock while A holds it.
	select {
	case <-bLocked:
		t.Fatal("B acquired lock while A held it — FOR UPDATE is not serializing")
	case <-time.After(300 * time.Millisecond):
		// expected: B is blocked on the row lock
	}

	// Release A; B should now proceed.
	close(releaseA)

	select {
	case <-bLocked:
		// expected
	case <-time.After(10 * time.Second):
		t.Fatal("B did not acquire the lock after A released")
	}

	if err := <-aDone; err != nil {
		t.Fatalf("A: %v", err)
	}
	if err := <-bDone; err != nil {
		t.Fatalf("B: %v", err)
	}
}
