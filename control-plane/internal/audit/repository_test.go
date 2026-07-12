package audit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These integration tests genuinely exercise forced RLS, the dedupe partial
// unique index, the append-only trigger, and keyset paging. They require a
// NON-SUPERUSER app-role connection so RLS is not silently bypassed.
//
// Unlike internal/database (which skips without the env), these tests FAIL
// LOUD when HARPIA_TEST_DATABASE_URL is unset, and they FAIL LOUD when the
// connection is a superuser — a superuser would bypass forced RLS and make
// the cross-tenant assertions meaningless.
//
// Set up the app role in the dev DB, e.g.:
//
//	CREATE ROLE harpia_app LOGIN PASSWORD 'harpia-app-password' NOBYPASSRLS;
//	GRANT CONNECT ON DATABASE harpia TO harpia_app;
//	GRANT USAGE ON SCHEMA public TO harpia_app;
//	GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO harpia_app;
//	GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO harpia_app;
//	ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO harpia_app;
//
//	export HARPIA_TEST_DATABASE_URL="postgres://harpia_app:harpia-app-password@localhost:15000/harpia?sslmode=disable"

func requireTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("HARPIA_TEST_DATABASE_URL")
	if url == "" {
		t.Fatalf("%s: HARPIA_TEST_DATABASE_URL must be set to a NON-SUPERUSER app-role DSN so forced RLS is genuinely exercised (see %s)",
			testOwner(), testSourcePath())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(pool.Close)

	// Fail loud if the connection is a superuser: a superuser bypasses
	// forced RLS, so the cross-tenant assertions would not prove isolation.
	var isSuperuser string
	if err := pool.QueryRow(ctx, "SHOW is_superuser").Scan(&isSuperuser); err != nil {
		t.Fatalf("check is_superuser: %v", err)
	}
	if strings.TrimSpace(strings.ToLower(isSuperuser)) != "off" {
		t.Fatalf("HARPIA_TEST_DATABASE_URL connects as is_superuser=%s; a NON-SUPERUSER app role (NOBYPASSRLS) is required so RLS is enforced", isSuperuser)
	}
	return pool
}

func testOwner() string { return "audit repository test" }
func testSourcePath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Base(file)
}

// createTenant inserts a fresh tenant row the app role can reference. The
// tenants table has no RLS, so the app role can insert directly.
func createTenant(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3)`,
		id, "Audit-"+id.String()[:8], "audit-"+id.String()[:8],
	)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	return id
}

// G1: forced RLS — an event written under tenant A's context is invisible
// to a tenant B list, and a cross-tenant write is rejected.
func TestRepositoryRLSIolatesTenants(t *testing.T) {
	pool := requireTestDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	tenantA := createTenant(t, pool)
	tenantB := createTenant(t, pool)

	event := Event{
		TenantID:  tenantA,
		EventType: eventPlanConfigurationCreated, BoundedContext: bcPlanManagement,
		Actor:      Actor{Kind: actorKindHuman, ID: "u-1", DisplayName: "User One"},
		HasSubject: true,
		Subject:    Subject{Type: subjectPlanConfiguration, ID: "pc-1"},
		OccurredAt: time.Now().UTC(),
	}
	if err := repo.Append(ctx, event); err != nil {
		t.Fatalf("append: %v", err)
	}

	// Tenant A sees its event.
	aEvents, err := repo.List(ctx, tenantA, Filters{}, time.Time{}, uuid.Nil, 10)
	if err != nil {
		t.Fatalf("list tenant A: %v", err)
	}
	if len(aEvents) != 1 {
		t.Fatalf("tenant A expected 1 event, got %d", len(aEvents))
	}

	// Tenant B sees nothing — forced RLS hides tenant A's row even though
	// the query targets the same table.
	bEvents, err := repo.List(ctx, tenantB, Filters{}, time.Time{}, uuid.Nil, 10)
	if err != nil {
		t.Fatalf("list tenant B: %v", err)
	}
	if len(bEvents) != 0 {
		t.Fatalf("tenant B expected 0 events (RLS), got %d — CROSS-TENANT LEAK", len(bEvents))
	}

	// Cross-tenant WRITE protection: a row that claims tenant A while the
	// database session context is tenant B is rejected by the RLS WITH
	// CHECK policy. (The repository always derives the DB context from the
	// event's own TenantID, so this exercises the DB-level boundary
	// directly rather than going through Append.)
	_, err = pool.Exec(ctx,
		`SELECT set_config('harpia.tenant_id', $1, true);
		 INSERT INTO audit_events (tenant_id, event_type, bounded_context, actor_kind, actor_id)
		 VALUES ($2, 'plan_configuration.created', 'plan_management', 'human', 'attacker')`,
		tenantB.String(), tenantA.String(),
	)
	if err == nil {
		t.Fatal("RLS WITH CHECK must reject a cross-tenant insert")
	}
}

// G3: a duplicate (tenant_id, dedupe_key) insert is a no-op (exactly one row).
func TestRepositoryDedupeInsertIsNoOp(t *testing.T) {
	pool := requireTestDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	tenant := createTenant(t, pool)

	mk := func(dedupe string) Event {
		return Event{
			TenantID: tenant, DedupeKey: dedupe,
			EventType: eventPlanExecutionStarted, BoundedContext: bcWorkflowEngine,
			Actor:      Actor{Kind: actorKindWorkflowEngine, ID: "wf-1"},
			HasSubject: true, Subject: Subject{Type: subjectPlanExecution, ID: "pe-1"},
			OccurredAt: time.Now().UTC(),
		}
	}

	// Same dedupe key, two inserts → exactly one row.
	if err := repo.Append(ctx, mk("dedupe-1")); err != nil {
		t.Fatalf("first append: %v", err)
	}
	if err := repo.Append(ctx, mk("dedupe-1")); err != nil {
		t.Fatalf("second append: %v", err)
	}
	events, err := repo.List(ctx, tenant, Filters{}, time.Time{}, uuid.Nil, 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("dedupe expected exactly 1 row, got %d", len(events))
	}

	// A different dedupe key inserts a second distinct row.
	if err := repo.Append(ctx, mk("dedupe-2")); err != nil {
		t.Fatalf("third append: %v", err)
	}
	events, _ = repo.List(ctx, tenant, Filters{}, time.Time{}, uuid.Nil, 50)
	if len(events) != 2 {
		t.Fatalf("expected 2 rows after distinct dedupe key, got %d", len(events))
	}

	// An empty dedupe key always inserts (no dedupe).
	if err := repo.Append(ctx, mk("")); err != nil {
		t.Fatalf("empty-dedupe append: %v", err)
	}
	events, _ = repo.List(ctx, tenant, Filters{}, time.Time{}, uuid.Nil, 50)
	if len(events) != 3 {
		t.Fatalf("expected 3 rows after empty-dedupe insert, got %d", len(events))
	}
}

// G2: with equal timestamps, keyset paging over (occurred_at, id) stays
// strictly monotonic with no overlap across pages.
func TestRepositoryKeysetStrictOrderingEqualTimestamps(t *testing.T) {
	pool := requireTestDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	tenant := createTenant(t, pool)
	occurredAt := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)

	// 6 events all sharing the exact same occurred_at — the id DESC
	// tiebreak is the only thing keeping pages monotonic.
	for i := 0; i < 6; i++ {
		if err := repo.Append(ctx, Event{
			TenantID:  tenant,
			EventType: eventPlanExecutionStarted, BoundedContext: bcWorkflowEngine,
			Actor:      Actor{Kind: actorKindWorkflowEngine, ID: "wf-1"},
			OccurredAt: occurredAt,
		}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	pageSize := 2
	seen := make(map[uuid.UUID]struct{})
	var prevID uuid.UUID
	prevOccurred := occurredAt.Add(time.Hour) // sentinel: newer than any row
	page := 0
	cursorAt, cursorID := time.Time{}, uuid.Nil

	for {
		page++
		if page > 10 {
			t.Fatal("paged too far — possible infinite loop")
		}
		events, err := repo.List(ctx, tenant, Filters{}, cursorAt, cursorID, pageSize)
		if err != nil {
			t.Fatalf("page %d: %v", page, err)
		}
		if len(events) == 0 {
			break
		}
		for _, e := range events {
			// No overlap: each event appears on exactly one page.
			if _, dup := seen[e.ID]; dup {
				t.Fatalf("event %s appeared on two pages — OVERLAP", e.ID)
			}
			seen[e.ID] = struct{}{}
			// Strictly monotonic under (occurred_at DESC, id DESC): each
			// subsequent row is older-or-equal-timestamp with a lower id.
			if e.OccurredAt.After(prevOccurred) {
				t.Fatalf("page ordering broke: %v after %v", e.OccurredAt, prevOccurred)
			}
			if e.OccurredAt.Equal(prevOccurred) && e.ID.String() >= prevID.String() {
				t.Fatalf("id tiebreak broke at equal timestamps: %s >= %s", e.ID, prevID)
			}
			prevOccurred = e.OccurredAt
			prevID = e.ID
		}
		last := events[len(events)-1]
		cursorAt, cursorID = last.OccurredAt, last.ID
		if len(events) < pageSize {
			break // last page
		}
	}

	if len(seen) != 6 {
		t.Fatalf("expected to page through all 6 events, saw %d", len(seen))
	}
}

// Append-only: UPDATE and DELETE must be rejected at the DB level (trigger),
// independent of the connecting role.
func TestRepositoryAppendOnlyRejectsMutation(t *testing.T) {
	pool := requireTestDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	tenant := createTenant(t, pool)
	if err := repo.Append(ctx, Event{
		TenantID:  tenant,
		EventType: eventPlanConfigurationCreated, BoundedContext: bcPlanManagement,
		Actor: Actor{Kind: actorKindHuman, ID: "u-1"}, OccurredAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("append: %v", err)
	}

	events, _ := repo.List(ctx, tenant, Filters{}, time.Time{}, uuid.Nil, 1)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	id := events[0].ID

	// A direct UPDATE through the pool must be rejected by the trigger.
	_, err := pool.Exec(ctx, fmt.Sprintf(
		"SELECT set_config('harpia.tenant_id','%s',true); UPDATE audit_events SET event_type='tampered' WHERE id='%s'",
		tenant.String(), id.String(),
	))
	if err == nil {
		t.Fatal("UPDATE on append-only table must be rejected by the trigger")
	}
	if !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("expected append-only trigger error, got: %v", err)
	}
}

// Repository filters compose correctly across the supported predicates.
func TestRepositoryAppliesFilters(t *testing.T) {
	pool := requireTestDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	tenant := createTenant(t, pool)
	base := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)

	events := []Event{
		{TenantID: tenant, EventType: eventPlanExecutionStarted, BoundedContext: bcWorkflowEngine, Actor: Actor{Kind: actorKindWorkflowEngine, ID: "wf-1"}, OccurredAt: base},
		{TenantID: tenant, EventType: eventPlanExecutionCompleted, BoundedContext: bcWorkflowEngine, Actor: Actor{Kind: actorKindWorkflowEngine, ID: "wf-1"}, OccurredAt: base.Add(time.Second)},
		{TenantID: tenant, EventType: eventApprovalCreated, BoundedContext: bcHumanInteraction, Actor: Actor{Kind: actorKindHuman, ID: "u-1"}, OccurredAt: base.Add(2 * time.Second), Decision: decisionApprove},
	}
	for _, e := range events {
		if err := repo.Append(ctx, e); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	// Filter by event type.
	etFilter := Filters{EventTypes: []string{eventPlanExecutionStarted}}
	got, err := repo.List(ctx, tenant, etFilter, time.Time{}, uuid.Nil, 10)
	if err != nil {
		t.Fatalf("list by event type: %v", err)
	}
	if len(got) != 1 || got[0].EventType != eventPlanExecutionStarted {
		t.Fatalf("event-type filter wrong: %+v", got)
	}

	// Filter by actor.
	actorFilter := Filters{Actor: &ActorFilter{Kind: actorKindHuman, ID: "u-1"}}
	got, err = repo.List(ctx, tenant, actorFilter, time.Time{}, uuid.Nil, 10)
	if err != nil {
		t.Fatalf("list by actor: %v", err)
	}
	if len(got) != 1 || got[0].EventType != eventApprovalCreated {
		t.Fatalf("actor filter wrong: %+v", got)
	}

	// Filter by decision.
	decFilter := Filters{Decision: decisionApprove}
	got, err = repo.List(ctx, tenant, decFilter, time.Time{}, uuid.Nil, 10)
	if err != nil {
		t.Fatalf("list by decision: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("decision filter wrong: %+v", got)
	}

	// Filter by date range (inclusive bounds).
	dateFilter := Filters{DateFrom: base.Add(time.Second), DateTo: base.Add(2 * time.Second)}
	got, err = repo.List(ctx, tenant, dateFilter, time.Time{}, uuid.Nil, 10)
	if err != nil {
		t.Fatalf("list by date: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("date-range filter expected 2, got %d", len(got))
	}
}
