package threads

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	chatv1 "github.com/harpia/control-plane/gen/harpia/chat/v1"

	"github.com/harpia/control-plane/internal/database"
)

func TestThreadStatusRoundTrip(t *testing.T) {
	cases := []chatv1.ThreadStatus{
		chatv1.ThreadStatus_THREAD_STATUS_OPEN,
		chatv1.ThreadStatus_THREAD_STATUS_RUNNING,
		chatv1.ThreadStatus_THREAD_STATUS_NEEDS_ATTENTION,
		chatv1.ThreadStatus_THREAD_STATUS_COMPLETED,
		chatv1.ThreadStatus_THREAD_STATUS_ARCHIVED,
	}
	for _, status := range cases {
		got, err := statusFromString(status.String())
		if err != nil {
			t.Fatalf("statusFromString(%q): %v", status.String(), err)
		}
		if got != status {
			t.Fatalf("round trip = %v, want %v", got, status)
		}
	}
}

func TestStatusFromStringRejectsUnknown(t *testing.T) {
	if _, err := statusFromString("not-a-status"); err == nil {
		t.Fatal("expected error for unknown status")
	}
}

func TestStatusToStringRejectsUnspecified(t *testing.T) {
	if _, err := statusToString(chatv1.ThreadStatus_THREAD_STATUS_UNSPECIFIED); err == nil {
		t.Fatal("expected error for unspecified status")
	}
}

func TestPageTokenRoundTrip(t *testing.T) {
	updatedAt := time.Date(2026, 6, 30, 17, 18, 19, 123456789, time.UTC)
	id := uuid.New()

	token := encodePageToken(updatedAt, id)
	gotUpdatedAt, gotID, err := decodePageToken(token)
	if err != nil {
		t.Fatalf("decodePageToken(%q): %v", token, err)
	}
	if gotUpdatedAt.Format(time.RFC3339Nano) != updatedAt.Format(time.RFC3339Nano) {
		t.Fatalf("updatedAt = %s, want %s", gotUpdatedAt.Format(time.RFC3339Nano), updatedAt.Format(time.RFC3339Nano))
	}
	if gotID != id {
		t.Fatalf("id = %s, want %s", gotID, id)
	}
}

func TestDecodePageTokenRejectsMalformedToken(t *testing.T) {
	cases := []string{
		"",
		"not-a-token",
		"2026-06-30T17:18:19Z|not-a-uuid",
		"not-a-time|" + uuid.New().String(),
	}
	for _, token := range cases {
		if _, _, err := decodePageToken(token); err == nil {
			t.Fatalf("decodePageToken(%q) expected error", token)
		}
	}
}

func TestListRejectsInvalidPageTokenBeforeDatabaseAccess(t *testing.T) {
	_, _, err := (&Repository{}).List(context.Background(), ListInput{
		TenantID:  uuid.New(),
		PageToken: "bad-token",
	})
	if err == nil {
		t.Fatal("expected error for invalid page token")
	}
}

func TestRepositoryCRUDListArchiveIntegration(t *testing.T) {
	databaseURL := os.Getenv("HARPIA_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set HARPIA_TEST_DATABASE_URL to run thread repository integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()

	slug := "thread-repo-" + strings.ReplaceAll(uuid.NewString(), "-", "")
	var tenantID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO tenants (name, slug) VALUES ($1, $2) RETURNING id`,
		"Thread repository test",
		slug,
	).Scan(&tenantID); err != nil {
		t.Fatalf("create tenant fixture: %v", err)
	}

	repo := NewRepository(pool)
	created := make([]*Thread, 0, 3)
	for _, title := range []string{"", "Investigate alerts", "Draft report"} {
		thread, err := repo.Create(ctx, CreateInput{
			TenantID: tenantID,
			Title:    title,
		})
		if err != nil {
			t.Fatalf("create thread %q: %v", title, err)
		}
		created = append(created, thread)
	}
	if created[0].Title != "Untitled chat" {
		t.Fatalf("empty title stored as %q, want Untitled chat", created[0].Title)
	}

	forcedUpdatedAt := time.Date(2026, 6, 30, 18, 19, 20, 0, time.UTC)
	err = database.WithTenant(ctx, pool, tenantID, func(q database.Querier) error {
		_, err := q.Exec(ctx, `
			UPDATE threads
			SET updated_at = $1
			WHERE tenant_id = $2 AND id IN ($3, $4, $5)
		`, forcedUpdatedAt, tenantID, created[0].ID, created[1].ID, created[2].ID)
		return err
	})
	if err != nil {
		t.Fatalf("force equal updated_at: %v", err)
	}

	firstPage, nextToken, err := repo.List(ctx, ListInput{
		TenantID: tenantID,
		PageSize: 2,
	})
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if len(firstPage) != 2 {
		t.Fatalf("first page count = %d, want 2", len(firstPage))
	}
	if nextToken == "" {
		t.Fatal("expected first page next token")
	}

	secondPage, nextToken, err := repo.List(ctx, ListInput{
		TenantID:  tenantID,
		PageSize:  2,
		PageToken: nextToken,
	})
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}
	if len(secondPage) != 1 {
		t.Fatalf("second page count = %d, want 1", len(secondPage))
	}
	if nextToken != "" {
		t.Fatalf("second page next token = %q, want empty", nextToken)
	}

	wantIDs := map[uuid.UUID]bool{
		created[0].ID: true,
		created[1].ID: true,
		created[2].ID: true,
	}
	gotIDs := make(map[uuid.UUID]bool, 3)
	for _, thread := range append(firstPage, secondPage...) {
		if !wantIDs[thread.ID] {
			t.Fatalf("listed unexpected thread id %s", thread.ID)
		}
		if gotIDs[thread.ID] {
			t.Fatalf("listed duplicate thread id %s", thread.ID)
		}
		gotIDs[thread.ID] = true
	}
	if len(gotIDs) != len(wantIDs) {
		t.Fatalf("listed ids count = %d, want %d", len(gotIDs), len(wantIDs))
	}

	archived, err := repo.Archive(ctx, tenantID, created[1].ID)
	if err != nil {
		t.Fatalf("archive thread: %v", err)
	}
	if archived.Status != chatv1.ThreadStatus_THREAD_STATUS_ARCHIVED {
		t.Fatalf("archived status = %v, want %v", archived.Status, chatv1.ThreadStatus_THREAD_STATUS_ARCHIVED)
	}
	if archived.ArchivedAt == nil {
		t.Fatal("archived_at is nil")
	}

	got, err := repo.Get(ctx, tenantID, created[1].ID)
	if err != nil {
		t.Fatalf("get archived thread: %v", err)
	}
	if got.Status != chatv1.ThreadStatus_THREAD_STATUS_ARCHIVED {
		t.Fatalf("get status = %v, want %v", got.Status, chatv1.ThreadStatus_THREAD_STATUS_ARCHIVED)
	}
	if got.ArchivedAt == nil {
		t.Fatal("get archived_at is nil")
	}
}
